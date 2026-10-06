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

// collectUntilFinished drains events until the command with the nonce finishes, returning the output lines.
func collectUntilFinished(t *testing.T, shell *Shell, nonce string) ([]string, int) {
	t.Helper()
	lines := []string{}
	timeout := time.After(liveTimeout * 3)
	for {
		select {
		case event, open := <-shell.Events():
			if !open {
				t.Fatal("shell closed before the command finished")
			}
			if event.Finished && event.Nonce == nonce {
				return lines, event.ExitCode
			}
			if event.Text != "" {
				lines = append(lines, event.Text)
			}
		case <-timeout:
			t.Fatalf("timed out waiting for command %s; got %v", nonce, lines)
		}
	}
}

func TestLiveShellKeepsStateBetweenCommands(t *testing.T) {
	sshHost := liveHost(t)
	shell, err := sshHost.OpenShell(context.Background())
	if err != nil {
		t.Fatalf("OpenShell returned error: %v", err)
	}
	defer shell.Close()

	nonce, err := shell.Run("cd /tmp && export LIVE_SHELL_VALUE=persisted")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	_, exitCode := collectUntilFinished(t, shell, nonce)
	nonce, _ = shell.Run("pwd; echo $LIVE_SHELL_VALUE; echo to-stderr >&2")
	lines, secondExitCode := collectUntilFinished(t, shell, nonce)

	if exitCode != 0 || secondExitCode != 0 {
		t.Fatalf("got exit codes %d and %d", exitCode, secondExitCode)
	}
	joined := strings.Join(lines, "\n")
	for _, expected := range []string{"/tmp", "persisted", "to-stderr"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("output lacks %q:\n%s", expected, joined)
		}
	}
}

func TestLiveShellInterruptStopsForegroundCommand(t *testing.T) {
	sshHost := liveHost(t)
	shell, err := sshHost.OpenShell(context.Background())
	if err != nil {
		t.Fatalf("OpenShell returned error: %v", err)
	}
	defer shell.Close()

	nonce, err := shell.Run("sleep 30")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	time.Sleep(time.Second)
	if err := shell.Interrupt(); err != nil {
		t.Fatalf("Interrupt returned error: %v", err)
	}
	_, exitCode := collectUntilFinished(t, shell, nonce)

	if exitCode == 0 {
		t.Fatal("interrupted sleep should not exit with 0")
	}
	nonce, _ = shell.Run("echo still-alive")
	lines, _ := collectUntilFinished(t, shell, nonce)
	if !strings.Contains(strings.Join(lines, "\n"), "still-alive") {
		t.Fatalf("shell did not survive the interrupt: %v", lines)
	}
}
