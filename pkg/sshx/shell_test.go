package sshx

import "testing"

func TestParseDoneMarkerReadsExitCode(t *testing.T) {
	exitCode, isMarker := ParseDoneMarker("__CLOUD_COMMAND_DONE__ 3")

	if !isMarker || exitCode != 3 {
		t.Fatalf("got %d %v", exitCode, isMarker)
	}
}

func TestParseDoneMarkerIgnoresOrdinaryLines(t *testing.T) {
	_, isMarker := ParseDoneMarker("hello 3")

	if isMarker {
		t.Fatal("ordinary output must not parse as a marker")
	}
}

func TestHandleLineSwallowsSetupMarkerThenReportsFinished(t *testing.T) {
	shell := &Shell{events: make(chan ShellEvent, 8)}

	shell.handleLine("\x02__CLOUD_COMMAND_DONE__ 0")
	shell.handleLine("\x02partial__CLOUD_COMMAND_DONE__ 7")

	first := <-shell.events
	second := <-shell.events
	if first.Text != "partial" || first.Stream != StreamStderr {
		t.Fatalf("partial stderr text not emitted: %+v", first)
	}
	if !second.Finished || second.ExitCode != 7 {
		t.Fatalf("finished event wrong: %+v", second)
	}
	if len(shell.events) != 0 {
		t.Fatal("setup marker should not produce an event")
	}
}
