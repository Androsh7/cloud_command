// Package sshx runs commands on remote hosts over SSH with public key authentication.
package sshx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	keepAliveRequestName    = "keepalive@openssh.com"
	defaultKeepAliveTimeout = 10 * time.Second
)

// CommandResponse is the captured result of one remote command.
type CommandResponse struct {
	Stdout     string
	Stderr     string
	ReturnCode int
}

// SshHost is a lazily connected, reusable SSH target.
type SshHost struct {
	Host           string
	User           string
	Port           int
	Timeout        time.Duration
	KeyPath        string
	connection     *ssh.Client
	connectionLock sync.Mutex
}

// NewSshHost returns a host that connects on first use.
func NewSshHost(host string, user string, port int, timeout time.Duration, keyPath string) *SshHost {
	return &SshHost{
		Host:    host,
		User:    user,
		Port:    port,
		Timeout: timeout,
		KeyPath: keyPath,
	}
}

// Address returns the host:port pair the client dials.
func (sshHost *SshHost) Address() string {
	return net.JoinHostPort(sshHost.Host, strconv.Itoa(sshHost.Port))
}

// createSshClientConfig builds the client config from the private key on disk.
func (sshHost *SshHost) createSshClientConfig() (*ssh.ClientConfig, error) {
	keyBytes, err := os.ReadFile(sshHost.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading private key %q: %w", sshHost.KeyPath, err)
	}
	key, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key %q: %w", sshHost.KeyPath, err)
	}
	return &ssh.ClientConfig{
		User:            sshHost.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(key)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         sshHost.Timeout,
	}, nil
}

// IsConnectionAlive reports whether the current connection answers a keepalive request.
func (sshHost *SshHost) IsConnectionAlive() bool {
	sshHost.connectionLock.Lock()
	defer sshHost.connectionLock.Unlock()
	return sshHost.answersKeepAlive(context.Background())
}

// keepAliveTimeout returns how long a keepalive may take before the connection counts as dead.
func (sshHost *SshHost) keepAliveTimeout() time.Duration {
	if sshHost.Timeout > 0 {
		return sshHost.Timeout
	}
	return defaultKeepAliveTimeout
}

// answersKeepAlive reports whether the current connection answers a keepalive request before the timeout or the
// context ends. The caller holds the connection lock.
func (sshHost *SshHost) answersKeepAlive(ctx context.Context) bool {
	connection := sshHost.connection
	if connection == nil {
		return false
	}
	answered := make(chan error, 1)
	go func() {
		_, _, err := connection.SendRequest(keepAliveRequestName, true, nil)
		answered <- err
	}()
	timer := time.NewTimer(sshHost.keepAliveTimeout())
	defer timer.Stop()
	select {
	case err := <-answered:
		return err == nil
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

// Connect opens the connection when it is not already alive.
func (sshHost *SshHost) Connect() error {
	return sshHost.ConnectContext(context.Background())
}

// ConnectContext opens the connection when it is not already alive, honouring cancellation while dialing.
func (sshHost *SshHost) ConnectContext(ctx context.Context) error {
	_, err := sshHost.client(ctx)
	return err
}

// client returns a live connection, replacing one that no longer answers.
func (sshHost *SshHost) client(ctx context.Context) (*ssh.Client, error) {
	sshHost.connectionLock.Lock()
	defer sshHost.connectionLock.Unlock()
	if sshHost.answersKeepAlive(ctx) {
		return sshHost.connection, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", sshHost.Address(), err)
	}
	if sshHost.connection != nil {
		_ = sshHost.connection.Close()
		sshHost.connection = nil
	}
	sshClientConfig, err := sshHost.createSshClientConfig()
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: sshHost.Timeout}
	tcpConnection, err := dialer.DialContext(ctx, "tcp", sshHost.Address())
	if err != nil {
		return nil, fmt.Errorf("dialing %s: %w", sshHost.Address(), err)
	}
	clientConnection, channels, requests, err := ssh.NewClientConn(tcpConnection, sshHost.Address(), sshClientConfig)
	if err != nil {
		tcpConnection.Close()
		return nil, fmt.Errorf("ssh handshake with %s@%s: %w", sshHost.User, sshHost.Address(), err)
	}
	sshHost.connection = ssh.NewClient(clientConnection, channels, requests)
	slog.Info("connected to host", "user", sshHost.User, "address", sshHost.Address())
	return sshHost.connection, nil
}

// Disconnect closes the connection when one is open.
func (sshHost *SshHost) Disconnect() error {
	sshHost.connectionLock.Lock()
	defer sshHost.connectionLock.Unlock()
	if sshHost.connection == nil {
		return nil
	}
	err := sshHost.connection.Close()
	sshHost.connection = nil
	if err != nil {
		return fmt.Errorf("closing connection to %s: %w", sshHost.Address(), err)
	}
	return nil
}

// RunCommand runs one command and captures its output. A non-zero exit status is reported in the response, not as an error.
func (sshHost *SshHost) RunCommand(command string) (CommandResponse, error) {
	return sshHost.RunCommandContext(context.Background(), command)
}

// RunCommandContext runs one command, killing it when the context is cancelled.
func (sshHost *SshHost) RunCommandContext(ctx context.Context, command string) (CommandResponse, error) {
	connection, err := sshHost.client(ctx)
	if err != nil {
		return CommandResponse{}, err
	}
	session, err := connection.NewSession()
	if err != nil {
		return CommandResponse{}, fmt.Errorf("opening session on %s: %w", sshHost.Address(), err)
	}
	defer session.Close()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Start(command); err != nil {
		return CommandResponse{}, fmt.Errorf("starting command on %s: %w", sshHost.Address(), err)
	}
	finished := make(chan error, 1)
	go func() { finished <- session.Wait() }()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return CommandResponse{}, fmt.Errorf("running command on %s: %w", sshHost.Address(), ctx.Err())
	case err = <-finished:
	}

	returnCode := 0
	var exitError *ssh.ExitError
	switch {
	case errors.As(err, &exitError):
		returnCode = exitError.ExitStatus()
	case err != nil:
		return CommandResponse{}, fmt.Errorf("running command on %s: %w", sshHost.Address(), err)
	}
	slog.Debug("executed command", "address", sshHost.Address(), "return_code", returnCode)
	return CommandResponse{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ReturnCode: returnCode,
	}, nil
}

// WaitForSsh retries connecting until it succeeds or the context ends.
func (sshHost *SshHost) WaitForSsh(ctx context.Context, retryInterval time.Duration) error {
	var lastErr error
	for {
		lastErr = sshHost.ConnectContext(ctx)
		if lastErr == nil {
			return nil
		}
		slog.Debug("ssh not ready", "address", sshHost.Address(), "error", lastErr)
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for ssh on %s: %w (last error: %v)", sshHost.Address(), ctx.Err(), lastErr)
		case <-time.After(retryInterval):
		}
	}
}
