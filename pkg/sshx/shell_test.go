package sshx

import (
	"strings"
	"testing"
	"time"
)

func TestParseDoneMarkerReadsSequenceAndExitCode(t *testing.T) {
	sequence, exitCode, isMarker := ParseDoneMarker("__CLOUD_COMMAND_DONE__ 4 3")

	if !isMarker || sequence != 4 || exitCode != 3 {
		t.Fatalf("got %d %d %v", sequence, exitCode, isMarker)
	}
}

func TestParseDoneMarkerIgnoresOrdinaryLines(t *testing.T) {
	_, _, isMarker := ParseDoneMarker("hello 4 3")

	if isMarker {
		t.Fatal("ordinary output must not parse as a marker")
	}
}

func TestHandleLineSwallowsSetupMarker(t *testing.T) {
	shell := &Shell{events: make(chan ShellEvent, 8)}

	shell.handleLine("__CLOUD_COMMAND_DONE__ 1 0")
	shell.handleLine("\x02__CLOUD_COMMAND_DONE__ 1 0")

	if len(shell.events) != 0 {
		t.Fatalf("setup marker produced %d events", len(shell.events))
	}
}

func TestStderrMarkerFinishesImmediatelyAndStdoutTwinIsDropped(t *testing.T) {
	shell := &Shell{events: make(chan ShellEvent, 8), lastSequence: setupSequence}

	shell.handleLine("\x02partial__CLOUD_COMMAND_DONE__ 2 7")
	shell.handleLine("__CLOUD_COMMAND_DONE__ 2 7")

	first := <-shell.events
	second := <-shell.events
	if first.Text != "partial" || first.Stream != StreamStderr {
		t.Fatalf("partial stderr text not emitted: %+v", first)
	}
	if !second.Finished || second.ExitCode != 7 || len(shell.events) != 0 {
		t.Fatalf("expected exactly one finished event, got %+v and %d more", second, len(shell.events))
	}
}

func TestStdoutMarkerWaitsForStderrTwinThenFinishes(t *testing.T) {
	shell := &Shell{events: make(chan ShellEvent, 8), lastSequence: setupSequence}

	shell.handleLine("__CLOUD_COMMAND_DONE__ 2 0")
	if len(shell.events) != 0 {
		t.Fatal("stdout marker must not finish before its grace period")
	}
	shell.handleLine("\x02late stderr line")
	shell.handleLine("\x02__CLOUD_COMMAND_DONE__ 2 0")

	first := <-shell.events
	second := <-shell.events
	if first.Text != "late stderr line" || !second.Finished || len(shell.events) != 0 {
		t.Fatalf("expected stderr line then one finish, got %+v %+v", first, second)
	}
}

func TestStdoutMarkerFinishesAloneAfterGracePeriod(t *testing.T) {
	shell := &Shell{events: make(chan ShellEvent, 8), lastSequence: setupSequence}

	shell.handleLine("__CLOUD_COMMAND_DONE__ 2 5")
	shell.expirePending(time.Now().Add(2 * finishGracePeriod))

	event := <-shell.events
	if !event.Finished || event.ExitCode != 5 {
		t.Fatalf("expected finish from stdout copy, got %+v", event)
	}
}

func TestMayHoldMarkerPrefixDetectsPartialMarker(t *testing.T) {
	if !mayHoldMarkerPrefix("some output __CLOUD_COMM") || !mayHoldMarkerPrefix("\x02__") {
		t.Fatal("partial marker not detected")
	}
	if mayHoldMarkerPrefix("plain progress text") {
		t.Fatal("plain text must not look like a marker")
	}
}

func TestConsumeKeepsCharacterSplitAcrossChunks(t *testing.T) {
	shell := &Shell{events: make(chan ShellEvent, 8), lastSequence: setupSequence}
	var partial strings.Builder
	encoded := []byte("café\n")

	incompleteRune := shell.consume(encoded[:4], &partial)
	shell.consume(append(incompleteRune, encoded[4:]...), &partial)

	line := <-shell.events
	if line.Text != "café" || len(incompleteRune) != 1 {
		t.Fatalf("got %q with carry %v", line.Text, incompleteRune)
	}
}

func TestConsumeSplitsOnCarriageReturnAndStripsTags(t *testing.T) {
	shell := &Shell{events: make(chan ShellEvent, 8), lastSequence: setupSequence}
	var partial strings.Builder

	shell.consume([]byte("one\rtwo\nno newline"), &partial)

	first := <-shell.events
	second := <-shell.events
	if first.Text != "one" || second.Text != "two" || len(shell.events) != 0 || partial.String() != "no newline" {
		t.Fatalf("got %+v %+v partial %q", first, second, partial.String())
	}
}
