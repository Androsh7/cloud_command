package sshx

import "testing"

func TestParseDoneMarkerReadsNonceAndExitCode(t *testing.T) {
	nonce, exitCode, isMarker := ParseDoneMarker("__CLOUD_COMMAND_DONE__ 42 3")

	if !isMarker || nonce != "42" || exitCode != 3 {
		t.Fatalf("got %q %d %v", nonce, exitCode, isMarker)
	}
}

func TestParseDoneMarkerIgnoresOrdinaryLines(t *testing.T) {
	_, _, isMarker := ParseDoneMarker("hello 42 3")

	if isMarker {
		t.Fatal("ordinary output must not parse as a marker")
	}
}
