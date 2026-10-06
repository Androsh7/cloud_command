//go:build live

package sshx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	liveHostVariable = "CLOUD_COMMAND_SSH_HOST"
	livePortVariable = "CLOUD_COMMAND_SSH_PORT"
	liveUserVariable = "CLOUD_COMMAND_SSH_USER"
	liveKeyVariable  = "CLOUD_COMMAND_SSH_KEY"

	defaultLiveHost = "127.0.0.1"
	defaultLivePort = 2222
	defaultLiveUser = "deploy"
	defaultLiveKey  = "sshd_container"

	liveTimeout       = 5 * time.Second
	liveRetryInterval = time.Second
)

// liveHost builds the host described by the environment, skipping the test when the key is missing.
func liveHost(t *testing.T) *SshHost {
	t.Helper()
	keyPath := os.Getenv(liveKeyVariable)
	if keyPath == "" {
		homeDirectory, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home directory: %v", err)
		}
		keyPath = filepath.Join(homeDirectory, ".ssh", defaultLiveKey)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Skipf("private key %q not found", keyPath)
	}
	host := os.Getenv(liveHostVariable)
	if host == "" {
		host = defaultLiveHost
	}
	user := os.Getenv(liveUserVariable)
	if user == "" {
		user = defaultLiveUser
	}
	port := defaultLivePort
	if portText := os.Getenv(livePortVariable); portText != "" {
		parsedPort, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parsing %s: %v", livePortVariable, err)
		}
		port = parsedPort
	}
	sshHost := NewSshHost(host, user, port, liveTimeout, keyPath)
	t.Cleanup(func() { _ = sshHost.Disconnect() })
	return sshHost
}

func TestLiveRunCommandReturnsStdout(t *testing.T) {
	sshHost := liveHost(t)

	response, err := sshHost.RunCommand(Command{Script: "echo hello"}.Line())
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}

	if strings.TrimSpace(response.Stdout) != "hello" || response.ReturnCode != 0 {
		t.Fatalf("got %+v, want stdout hello and return code 0", response)
	}
}

func TestLiveRunCommandReportsExitCodeWithoutError(t *testing.T) {
	sshHost := liveHost(t)

	response, err := sshHost.RunCommand(Command{Script: "exit 3"}.Line())
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}

	if response.ReturnCode != 3 {
		t.Fatalf("got return code %d, want 3", response.ReturnCode)
	}
}

func TestLiveSudoRunsAsRoot(t *testing.T) {
	sshHost := liveHost(t)

	response, err := sshHost.RunCommand(Command{Script: "id -u", Sudo: true}.Line())
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}

	if strings.TrimSpace(response.Stdout) != "0" {
		t.Fatalf("got uid %q, want 0", strings.TrimSpace(response.Stdout))
	}
}

func TestLiveWaitForSshConnects(t *testing.T) {
	sshHost := liveHost(t)
	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()

	if err := sshHost.WaitForSsh(ctx, liveRetryInterval); err != nil {
		t.Fatalf("WaitForSsh returned error: %v", err)
	}
}

func TestLiveRunCommandContextStopsOnCancel(t *testing.T) {
	sshHost := liveHost(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	started := time.Now()
	_, err := sshHost.RunCommandContext(ctx, Command{Script: "sleep 30"}.Line())

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got error %v, want deadline exceeded", err)
	}
	if time.Since(started) > liveTimeout {
		t.Fatal("cancelled command did not return promptly")
	}
}
