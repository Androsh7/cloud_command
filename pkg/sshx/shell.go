package sshx

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/ssh"
)

const (
	ShellCommand = "env PS1= PS2= PROMPT_COMMAND= TERM=dumb /bin/bash --noediting --norc --noprofile"

	doneMarker        = "__CLOUD_COMMAND_DONE__"
	interruptSequence = "\x03"
	terminalName      = "dumb"
	terminalRows      = 50
	terminalColumns   = 200
	eventBufferSize   = 256
	initialLineBuffer = 64 * 1024
	maximumLineBuffer = 4 * 1024 * 1024
)

var (
	ErrShellClosed = errors.New("shell is closed")

	shellRunCounter atomic.Uint64
)

// ShellEvent is one thing that happened on a remote shell: a line of output, a finished command, or the shell closing.
type ShellEvent struct {
	Text     string
	Finished bool
	Nonce    string
	ExitCode int
	Closed   bool
	Err      error
}

// Shell is a persistent remote bash session whose output is streamed as events.
type Shell struct {
	session   *ssh.Session
	stdin     io.WriteCloser
	events    chan ShellEvent
	writeLock sync.Mutex
	closeOnce sync.Once
	closed    atomic.Bool
}

// OpenShell starts a persistent bash session on the host under a pseudo-terminal.
func (sshHost *SshHost) OpenShell(ctx context.Context) (*Shell, error) {
	if err := sshHost.ConnectContext(ctx); err != nil {
		return nil, err
	}
	session, err := sshHost.connection.NewSession()
	if err != nil {
		return nil, fmt.Errorf("opening shell session on %s: %w", sshHost.Address(), err)
	}
	modes := ssh.TerminalModes{ssh.ECHO: 0, ssh.OPOST: 0, ssh.NOFLSH: 1}
	if err := session.RequestPty(terminalName, terminalRows, terminalColumns, modes); err != nil {
		session.Close()
		return nil, fmt.Errorf("requesting pty on %s: %w", sshHost.Address(), err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("opening shell stdin on %s: %w", sshHost.Address(), err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("opening shell stdout on %s: %w", sshHost.Address(), err)
	}
	if err := session.Start(ShellCommand); err != nil {
		session.Close()
		return nil, fmt.Errorf("starting shell on %s: %w", sshHost.Address(), err)
	}
	shell := &Shell{session: session, stdin: stdin, events: make(chan ShellEvent, eventBufferSize)}
	go shell.readLoop(stdout)
	return shell, nil
}

// Events returns the stream of output lines and completion events. It is closed when the shell ends.
func (shell *Shell) Events() <-chan ShellEvent {
	return shell.events
}

// IsClosed reports whether the remote shell has ended.
func (shell *Shell) IsClosed() bool {
	return shell.closed.Load()
}

// Run sends a command followed by a completion marker and returns the nonce that identifies its Finished event.
func (shell *Shell) Run(command string) (string, error) {
	nonce := strconv.FormatUint(shellRunCounter.Add(1), 10)
	text := command + "\n" + fmt.Sprintf("printf '%s %%s %%d\\n' %s \"$?\"\n", doneMarker, nonce)
	if err := shell.write(text); err != nil {
		return "", err
	}
	return nonce, nil
}

// Interrupt sends Ctrl-C to the terminal, interrupting the foreground command.
func (shell *Shell) Interrupt() error {
	return shell.write(interruptSequence)
}

// Close ends the shell session.
func (shell *Shell) Close() error {
	var err error
	shell.closeOnce.Do(func() {
		shell.closed.Store(true)
		_ = shell.stdin.Close()
		err = shell.session.Close()
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("closing shell: %w", err)
	}
	return nil
}

// write sends raw text to the shell's terminal.
func (shell *Shell) write(text string) error {
	if shell.closed.Load() {
		return ErrShellClosed
	}
	shell.writeLock.Lock()
	defer shell.writeLock.Unlock()
	if _, err := io.WriteString(shell.stdin, text); err != nil {
		return fmt.Errorf("writing to shell: %w", err)
	}
	return nil
}

// readLoop turns terminal output into events until the session ends.
func (shell *Shell) readLoop(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, initialLineBuffer), maximumLineBuffer)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if nonce, exitCode, isMarker := ParseDoneMarker(line); isMarker {
			shell.events <- ShellEvent{Finished: true, Nonce: nonce, ExitCode: exitCode}
			continue
		}
		shell.events <- ShellEvent{Text: line}
	}
	waitErr := shell.session.Wait()
	var exitError *ssh.ExitError
	if errors.As(waitErr, &exitError) || errors.Is(waitErr, io.EOF) {
		waitErr = nil
	}
	shell.closed.Store(true)
	shell.events <- ShellEvent{Closed: true, Err: waitErr}
	close(shell.events)
}

// ParseDoneMarker extracts the nonce and exit code from a completion marker line.
func ParseDoneMarker(line string) (string, int, bool) {
	fields := strings.Fields(line)
	if len(fields) != 3 || fields[0] != doneMarker {
		return "", 0, false
	}
	exitCode, err := strconv.Atoi(fields[2])
	if err != nil {
		return "", 0, false
	}
	return fields[1], exitCode, true
}
