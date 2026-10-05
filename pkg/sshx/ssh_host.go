package sshx

import (
	"bytes"
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

type CommandResponse struct {
	Stdout string
	Stderr string
	ReturnCode int
}

type SshHost struct {
	Host string
	User string
	Port int
	Timeout time.Duration
	KeyPath string
	connection *ssh.Client
	connectionLock sync.Mutex
}

func NewSshHost(host string, user string, port int, timeout time.Duration, keyPath string) *SshHost {
	return &SshHost{
		Host: host,
		User: user,
		Port: port,
		Timeout: timeout,
		KeyPath: keyPath,
	}
}

func (sshHost *SshHost) createSshClientConfig() (*ssh.ClientConfig, error) {
	keyBytes, err := os.ReadFile(sshHost.KeyPath)
	if err != nil { return &ssh.ClientConfig{}, err}
	key, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil { return &ssh.ClientConfig{}, err}
	return &ssh.ClientConfig{
		User: sshHost.User,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(key)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: sshHost.Timeout,
	}, nil
}

func (sshHost *SshHost) IsConnectionAlive() (bool) {
	if sshHost.connection == nil {
		return false
	}
	_, _, err := sshHost.connection.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil { return false }
	return true
}

func (sshHost *SshHost) Connect() error {
	sshHost.connectionLock.Lock()
	defer sshHost.connectionLock.Unlock()
	if sshHost.IsConnectionAlive() {
		return nil
	}
	sshClientConfig, err := sshHost.createSshClientConfig()
	if err != nil { return err }
	slog.Info(fmt.Sprintf("Connected to host %s@%s:%d", sshHost.User, sshHost.Host, sshHost.Port))
	client, err := ssh.Dial("tcp", net.JoinHostPort(sshHost.Host, strconv.Itoa(sshHost.Port)), sshClientConfig)
	if err != nil { return err }
	sshHost.connection = client
	return nil 
}

func (sshHost *SshHost) Disconnect() error {
	sshHost.connectionLock.Lock()
	defer sshHost.connectionLock.Unlock()
	if sshHost.connection == nil {
		return nil
	}
	err := sshHost.connection.Close()
	if err != nil { return err }
	sshHost.connection = nil
	return nil
}

func (sshHost *SshHost) RunCommand(command string) (CommandResponse, error) {
	if !sshHost.IsConnectionAlive() {
		err := sshHost.Connect()
		if err != nil { return CommandResponse{}, err }
	}
	session, err := sshHost.connection.NewSession()
	if err != nil { return CommandResponse{}, err }
	defer session.Close()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	var returnCode int = 0
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		returnCode = exitErr.ExitStatus()
		slog.Info(fmt.Sprintf("Failed (%d) to execute command: %s", returnCode, command))
	} else if err != nil {
		return CommandResponse{}, err
	} else {
		slog.Info(fmt.Sprintf("Successfully executed command: %s", command))
	}
	return CommandResponse{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		ReturnCode: returnCode,
	}, err
}
