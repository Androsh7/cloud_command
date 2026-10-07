package sshx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	ShellCommand = "env PS1= PS2= PROMPT_COMMAND= TERM=dumb /bin/bash --noediting --norc --noprofile"

	doneMarker        = "__CLOUD_COMMAND_DONE__"
	stderrTag         = "\x02"
	interruptSequence = "\x03"
	terminalName      = "dumb"
	DefaultRows       = 50
	DefaultColumns    = 200
	eventBufferSize   = 256
	readChunkSize     = 32 * 1024
	maxPartialLine    = 256 * 1024
	idleFlushInterval = 200 * time.Millisecond
	finishGracePeriod = 400 * time.Millisecond
	setupSequence     = 1
)

// shellSetup routes stderr through a tagging relay and installs a prompt hook that runs after every command line.
// The relay ignores interrupts so a Ctrl-C while idle cannot kill it, the hook reopens it if it has died anyway,
// re-asserts the terminal modes, and prints a numbered exit marker on stdout and, from a subshell so a broken
// pipe can never take bash down, on stderr.
const shellSetup = `__cc_relay() { trap '' INT QUIT; while IFS= read -r line; do printf '\002%s\n' "$line"; done; }; ` +
	`__cc_open_relay() { exec 2> >(__cc_relay); __cc_relay_pid=$!; }; __cc_open_relay; ` +
	`PROMPT_COMMAND='__cc_rc=$?; __cc_seq=$((__cc_seq+1)); kill -0 "$__cc_relay_pid" 2>/dev/null || __cc_open_relay; ` +
	`stty -echo -opost noflsh 2>/dev/null; ` +
	`printf "` + doneMarker + ` %d %d\n" "$__cc_seq" "$__cc_rc"; ` +
	`( printf "` + doneMarker + ` %d %d\n" "$__cc_seq" "$__cc_rc" >&2 )'` + "\n"

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

// pendingFinish is a completion seen on stdout that waits briefly for its stderr twin so output order is kept.
type pendingFinish struct {
	sequence uint64
	exitCode int
	expires  time.Time
}

// Shell is a persistent remote bash session whose output is streamed as events.
type Shell struct {
	session      *ssh.Session
	stdin        io.WriteCloser
	events       chan ShellEvent
	writeLock    sync.Mutex
	closeOnce    sync.Once
	closed       atomic.Bool
	lastSequence uint64
	pending      *pendingFinish
}

// OpenShell starts a persistent bash session on the host under a pseudo-terminal of the given size.
// Standard error is routed through a tagging loop so its lines can be told apart from standard output,
// and bash itself reports each command's exit code the moment the command finishes.
func (sshHost *SshHost) OpenShell(ctx context.Context, rows int, columns int) (*Shell, error) {
	connection, err := sshHost.client(ctx)
	if err != nil {
		return nil, err
	}
	session, err := connection.NewSession()
	if err != nil {
		return nil, fmt.Errorf("opening shell session on %s: %w", sshHost.Address(), err)
	}
	modes := ssh.TerminalModes{ssh.ECHO: 0, ssh.OPOST: 0, ssh.NOFLSH: 1}
	if err := session.RequestPty(terminalName, max(rows, 1), max(columns, 1), modes); err != nil {
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

// Resize tells the remote terminal its new size.
func (shell *Shell) Resize(rows int, columns int) error {
	if shell.closed.Load() {
		return ErrShellClosed
	}
	if err := shell.session.WindowChange(max(rows, 1), max(columns, 1)); err != nil {
		return fmt.Errorf("resizing shell: %w", err)
	}
	return nil
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
// Lines end at a newline or carriage return; a partial line is flushed once output pauses.
func (shell *Shell) readLoop(reader io.Reader) {
	chunks := make(chan []byte, 16)
	go func() {
		defer close(chunks)
		buffer := make([]byte, readChunkSize)
		for {
			count, err := reader.Read(buffer)
			if count > 0 {
				chunk := make([]byte, count)
				copy(chunk, buffer[:count])
				chunks <- chunk
			}
			if err != nil {
				return
			}
		}
	}()

	var partial strings.Builder
	partialIdle := false
	ticker := time.NewTicker(idleFlushInterval)
	defer ticker.Stop()
	for running := true; running; {
		select {
		case chunk, open := <-chunks:
			if !open {
				running = false
				break
			}
			partialIdle = false
			shell.consume(chunk, &partial)
		case now := <-ticker.C:
			if partial.Len() > 0 && partialIdle && !mayHoldMarkerPrefix(partial.String()) {
				shell.handleLine(partial.String())
				partial.Reset()
			}
			partialIdle = partial.Len() > 0
			shell.expirePending(now)
		}
	}
	if partial.Len() > 0 {
		shell.handleLine(partial.String())
	}
	shell.expirePending(time.Now().Add(finishGracePeriod))

	waitErr := shell.session.Wait()
	var exitError *ssh.ExitError
	if errors.As(waitErr, &exitError) || errors.Is(waitErr, io.EOF) {
		waitErr = nil
	}
	shell.closed.Store(true)
	shell.events <- ShellEvent{Closed: true, Err: waitErr}
	close(shell.events)
}

// consume splits a chunk into lines, handling every complete one and keeping the remainder.
func (shell *Shell) consume(chunk []byte, partial *strings.Builder) {
	for _, char := range string(chunk) {
		if char == '\n' || char == '\r' {
			if partial.Len() > 0 {
				shell.handleLine(partial.String())
				partial.Reset()
			}
			continue
		}
		partial.WriteRune(char)
		if partial.Len() >= maxPartialLine && !mayHoldMarkerPrefix(partial.String()) {
			shell.handleLine(partial.String())
			partial.Reset()
		}
	}
}

// mayHoldMarkerPrefix reports whether the end of a partial line could be the start of a marker still arriving.
func mayHoldMarkerPrefix(partial string) bool {
	for _, candidate := range []string{stderrTag + doneMarker, doneMarker} {
		limit := min(len(partial), len(candidate))
		for length := 1; length <= limit; length++ {
			if strings.HasSuffix(partial, candidate[:length]) {
				return true
			}
		}
	}
	return false
}

// handleLine classifies one line of terminal output and emits the matching events.
func (shell *Shell) handleLine(line string) {
	stream := StreamStdout
	if strings.HasPrefix(line, stderrTag) {
		stream = StreamStderr
	}
	line = strings.ReplaceAll(line, stderrTag, "")
	if index := strings.Index(line, doneMarker); index >= 0 {
		sequence, exitCode, isMarker := ParseDoneMarker(line[index:])
		if !isMarker {
			slog.Debug("unparsed shell marker", "line", line)
		} else {
			if partial := line[:index]; partial != "" {
				shell.events <- ShellEvent{Text: partial, Stream: stream}
			}
			shell.handleMarker(sequence, exitCode, stream)
			return
		}
	}
	if line != "" {
		shell.events <- ShellEvent{Text: line, Stream: stream}
	}
}

// handleMarker turns a numbered marker into at most one Finished event per command.
// The stderr copy is preferred because it queues behind the command's stderr; the stdout copy is a fallback.
func (shell *Shell) handleMarker(sequence uint64, exitCode int, stream Stream) {
	if sequence <= shell.lastSequence || sequence == setupSequence {
		if sequence == setupSequence {
			shell.lastSequence = max(shell.lastSequence, setupSequence)
		}
		return
	}
	if shell.pending != nil && shell.pending.sequence < sequence {
		shell.emitFinished(shell.pending.sequence, shell.pending.exitCode)
		shell.pending = nil
	}
	if stream == StreamStderr {
		shell.pending = nil
		shell.emitFinished(sequence, exitCode)
		return
	}
	if shell.pending == nil {
		shell.pending = &pendingFinish{sequence: sequence, exitCode: exitCode, expires: time.Now().Add(finishGracePeriod)}
	}
}

// expirePending emits a stdout-only completion whose stderr twin never arrived in time.
func (shell *Shell) expirePending(now time.Time) {
	if shell.pending != nil && !now.Before(shell.pending.expires) {
		slog.Debug("stderr marker missing, finishing from stdout copy", "sequence", shell.pending.sequence)
		shell.emitFinished(shell.pending.sequence, shell.pending.exitCode)
		shell.pending = nil
	}
}

// emitFinished records a completed command and reports it.
func (shell *Shell) emitFinished(sequence uint64, exitCode int) {
	shell.lastSequence = sequence
	shell.events <- ShellEvent{Finished: true, ExitCode: exitCode}
}

// ParseDoneMarker extracts the sequence number and exit code from a completion marker line.
func ParseDoneMarker(line string) (uint64, int, bool) {
	fields := strings.Fields(line)
	if len(fields) != 3 || fields[0] != doneMarker {
		return 0, 0, false
	}
	sequence, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	exitCode, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, 0, false
	}
	return sequence, exitCode, true
}
