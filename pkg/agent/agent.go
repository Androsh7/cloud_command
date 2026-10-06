package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/androsh7/cloud_command/pkg/ec2"
	"github.com/androsh7/cloud_command/pkg/sshx"
)

const (
	KeyPairNamePrefix = "ssh-key-"
	MaxNameLength     = 63
)

var (
	ErrInvalidName = errors.New("invalid agent name")
	ErrNotReady    = errors.New("agent has no public address yet")

	namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
)

// Agent is a managed EC2 instance plus its local directory.
type Agent struct {
	Record    Record
	Directory string
	store     *Store
	host      *sshx.SshHost
	hostLock  sync.Mutex
	saveLock  sync.Mutex
	sshConfig SshOptions
}

// SshOptions controls how the agent connects over SSH.
type SshOptions struct {
	Timeout time.Duration
}

// ValidateName checks that a name is safe to use as a directory and tag value.
func ValidateName(name string) error {
	if name == "" || len(name) > MaxNameLength || !namePattern.MatchString(name) {
		return fmt.Errorf("%w: %q must match %s and be at most %d characters", ErrInvalidName, name, namePattern, MaxNameLength)
	}
	return nil
}

// Name returns the agent's name.
func (managedAgent *Agent) Name() string {
	return managedAgent.Record.Name
}

// PrivateKeyPath returns the absolute path of the private key.
func (managedAgent *Agent) PrivateKeyPath() string {
	return filepath.Join(managedAgent.Directory, managedAgent.Record.PrivateKeyFile)
}

// PublicKeyPath returns the absolute path of the public key.
func (managedAgent *Agent) PublicKeyPath() string {
	return filepath.Join(managedAgent.Directory, managedAgent.Record.PublicKeyFile)
}

// SetSshOptions changes the options used for future connections.
func (managedAgent *Agent) SetSshOptions(options SshOptions) {
	managedAgent.hostLock.Lock()
	defer managedAgent.hostLock.Unlock()
	managedAgent.sshConfig = options
	managedAgent.host = nil
}

// Save persists the record.
func (managedAgent *Agent) Save() error {
	managedAgent.saveLock.Lock()
	defer managedAgent.saveLock.Unlock()
	return managedAgent.store.Save(managedAgent)
}

// SshHost returns the reusable SSH connection for the agent's current public address.
func (managedAgent *Agent) SshHost() (*sshx.SshHost, error) {
	managedAgent.hostLock.Lock()
	defer managedAgent.hostLock.Unlock()
	if managedAgent.Record.PublicIp == "" {
		return nil, ErrNotReady
	}
	if managedAgent.host != nil && managedAgent.host.Host == managedAgent.Record.PublicIp {
		return managedAgent.host, nil
	}
	if managedAgent.host != nil {
		_ = managedAgent.host.Disconnect()
	}
	managedAgent.host = sshx.NewSshHost(
		managedAgent.Record.PublicIp,
		managedAgent.Record.SshUser,
		managedAgent.Record.SshPort,
		managedAgent.sshConfig.Timeout,
		managedAgent.PrivateKeyPath(),
	)
	return managedAgent.host, nil
}

// Refresh reads the instance state from AWS and stores the public address.
func (managedAgent *Agent) Refresh(ctx context.Context) (ec2.InstanceInfo, error) {
	if managedAgent.Record.InstanceId == "" {
		return ec2.InstanceInfo{}, fmt.Errorf("agent %q has no instance", managedAgent.Name())
	}
	info, err := ec2.DescribeInstance(ctx, managedAgent.Record.Region, managedAgent.Record.InstanceId)
	if err != nil {
		return ec2.InstanceInfo{}, err
	}
	if info.PublicIp != managedAgent.Record.PublicIp {
		managedAgent.Record.PublicIp = info.PublicIp
		if err := managedAgent.Save(); err != nil {
			return info, err
		}
	}
	return info, nil
}

// RunCommand runs one command over SSH and returns its captured output.
func (managedAgent *Agent) RunCommand(ctx context.Context, command sshx.Command) (sshx.CommandResponse, error) {
	host, err := managedAgent.SshHost()
	if err != nil {
		return sshx.CommandResponse{}, err
	}
	slog.Debug("running command", "agent", managedAgent.Name(), "sudo", command.Sudo, "script", command.Script)
	response, err := host.RunCommandContext(ctx, command.Line())
	if err != nil {
		return sshx.CommandResponse{}, fmt.Errorf("running command on agent %q: %w", managedAgent.Name(), err)
	}
	slog.Info("ran command", "agent", managedAgent.Name(), "exit_code", response.ReturnCode)
	return response, nil
}

// Disconnect closes the SSH connection when one is open.
func (managedAgent *Agent) Disconnect() error {
	managedAgent.hostLock.Lock()
	defer managedAgent.hostLock.Unlock()
	if managedAgent.host == nil {
		return nil
	}
	return managedAgent.host.Disconnect()
}

// Destroy terminates the instance, deletes the key pair and removes the local directory.
func (managedAgent *Agent) Destroy(ctx context.Context) error {
	_ = managedAgent.Disconnect()
	var errs []error
	if managedAgent.Record.InstanceId != "" {
		if err := ec2.TerminateInstance(ctx, managedAgent.Record.Region, managedAgent.Record.InstanceId); err != nil {
			errs = append(errs, err)
		}
	}
	if managedAgent.Record.KeyPairName != "" {
		if err := ec2.DeleteKeyPair(ctx, managedAgent.Record.Region, managedAgent.Record.KeyPairName); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("destroying agent %q: %w", managedAgent.Name(), errors.Join(errs...))
	}
	if err := managedAgent.store.Remove(managedAgent.Name()); err != nil {
		return err
	}
	slog.Info("destroyed agent", "agent", managedAgent.Name(), "instance_id", managedAgent.Record.InstanceId)
	return nil
}
