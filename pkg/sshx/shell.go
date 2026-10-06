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
	stderrTag         = "\x02"
	interruptSequence = "\x03"
	terminalName      = "dumb"
	terminalRows      = 50
	terminalColumns   = 200
	eventBufferSize   = 256
	initialLineBuffer = 64 * 1024
	maximumLineBuffer = 4 * 1024 * 1024
)

// shellSetup routes stderr through a tagging loop and makes bash print an exit marker after every command line.
const shellSetup = `exec 2> >(while IFS= read -r line; do printf '\002%s\n' "$line"; done); PROMPT_COMMAND='printf "` + doneMarker + ` %d\n" "$?" >&2'` + "\n"

var ErrShellClosed = errors.New("shell is closed")

// Stream says which output stream a line came from.
type Stream int

const (
	StreamStdout Stream = iota
	StreamStderr
)

// ShellEvent is one thing that happened on a remote shell: a line of output, a finished command, or the shell closing.
// Finished events arrive in the order commands were sent.
type ShellEvent struct {
	Text     string
	Stream   Stream
	Finished bool
	ExitCode int
	Closed   bool
	Err      error
}

// Shell is a persistent remote bash session whose output is streamed as events.
type Shell struct {
	session    *ssh.Session
	stdin      io.WriteCloser
	events     chan ShellEvent
	writeLock  sync.Mutex
	closeOnce  sync.Once
	closed     atomic.Bool
	setupSeen  atomic.Bool
	setupError error
}

// OpenShell starts a persistent bash session on the host under a pseudo-terminal.
// Standard error is routed through a tagging loop so its lines can be told apart from standard output,
// and bash itself reports each command's exit code the moment the command finishes.
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
	if err := shell.write(shellSetup); err != nil {
		_ = shell.Close()
		return nil, err
	}
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

// Run sends one command line. Bash reports its completion as a Finished event once it has run.
func (shell *Shell) Run(command string) error {
	return shell.write(command + "\n")
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
		for _, segment := range strings.Split(scanner.Text(), "\r") {
			if segment != "" {
				shell.handleLine(segment)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		shell.setupError = err
	}
	waitErr := shell.session.Wait()
	var exitError *ssh.ExitError
	if errors.As(waitErr, &exitError) || errors.Is(waitErr, io.EOF) {
		waitErr = nil
	}
	if waitErr == nil {
		waitErr = shell.setupError
	}
	shell.closed.Store(true)
	shell.events <- ShellEvent{Closed: true, Err: waitErr}
	close(shell.events)
}

// handleLine classifies one line of terminal output and emits the matching events.
// The first marker belongs to the setup line and is swallowed.
func (shell *Shell) handleLine(line string) {
	stream := StreamStdout
	if strings.HasPrefix(line, stderrTag) {
		stream = StreamStderr
		line = strings.TrimPrefix(line, stderrTag)
	}
	if index := strings.Index(line, doneMarker); index >= 0 {
		if exitCode, isMarker := ParseDoneMarker(line[index:]); isMarker {
			if partial := line[:index]; partial != "" {
				shell.events <- ShellEvent{Text: partial, Stream: stream}
			}
			if shell.setupSeen.CompareAndSwap(false, true) {
				return
			}
			shell.events <- ShellEvent{Finished: true, ExitCode: exitCode}
			return
		}
	}
	shell.events <- ShellEvent{Text: line, Stream: stream}
}

// ParseDoneMarker extracts the exit code from a completion marker line.
func ParseDoneMarker(line string) (int, bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 || fields[0] != doneMarker {
		return 0, false
	}
	exitCode, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, false
	}
	return exitCode, true
}
