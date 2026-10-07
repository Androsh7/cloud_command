package systemd

import (
	"testing"
	"time"
)

func TestRunCommandBuildsSystemdRunLine(t *testing.T) {
	spec := JobSpec{Id: "abc", Script: "echo hi", User: "ec2-user"}

	line := spec.RunCommand()

	expected := "sudo systemd-run --unit=job-abc -p RemainAfterExit=yes" +
		" -p StandardOutput=file:/tmp/job-abc.stdout -p StandardError=file:/tmp/job-abc.stderr" +
		" -p User=ec2-user -- /bin/bash -c 'exec /bin/bash <(echo ZWNobyBoaQ== | base64 -d)'"
	if line != expected {
		t.Fatalf("got\n%s\nwant\n%s", line, expected)
	}
}

func TestReadOutputCommandTailsTheFile(t *testing.T) {
	line := ReadOutputCommand("/tmp/job-abc.stdout", 50)

	if line != "tail -c 4194304 /tmp/job-abc.stdout 2>/dev/null | tail -n 50" {
		t.Fatalf("got %q", line)
	}
}

func TestShowCommandNamesUnit(t *testing.T) {
	line := ShowCommand("job-abc")

	expected := "systemctl show --property=LoadState,ActiveState,SubState,ExecMainStatus,ExecMainStartTimestamp job-abc"
	if line != expected {
		t.Fatalf("got %q, want %q", line, expected)
	}
}

func TestStopCommandNamesUnit(t *testing.T) {
	line := StopCommand("job-abc")

	if line != "sudo systemctl stop job-abc" {
		t.Fatalf("got %q", line)
	}
}

func TestParseShowReadsExitedUnit(t *testing.T) {
	output := "LoadState=loaded\nActiveState=active\nSubState=exited\nExecMainStatus=0\nExecMainStartTimestamp=Sun 2026-10-05 12:00:00 UTC\n"

	status, err := ParseShow(output)
	if err != nil {
		t.Fatalf("ParseShow returned error: %v", err)
	}

	expectedStart := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	if status.Summary() != "exited (0)" || !status.StartedAt.Equal(expectedStart) {
		t.Fatalf("got %+v with summary %q", status, status.Summary())
	}
}

func TestParseShowReadsRunningUnit(t *testing.T) {
	status, err := ParseShow("LoadState=loaded\nActiveState=active\nSubState=running\nExecMainStatus=0\nExecMainStartTimestamp=\n")
	if err != nil {
		t.Fatalf("ParseShow returned error: %v", err)
	}

	if !status.IsRunning() || status.Summary() != "running" {
		t.Fatalf("got %+v", status)
	}
}

func TestParseShowReadsFailedUnit(t *testing.T) {
	status, err := ParseShow("LoadState=loaded\nActiveState=failed\nSubState=failed\nExecMainStatus=1\nExecMainStartTimestamp=\n")
	if err != nil {
		t.Fatalf("ParseShow returned error: %v", err)
	}

	if status.Summary() != "failed (1)" {
		t.Fatalf("got summary %q", status.Summary())
	}
}

func TestParseShowReadsMissingUnit(t *testing.T) {
	status, err := ParseShow("LoadState=not-found\nActiveState=inactive\nSubState=dead\nExecMainStatus=0\nExecMainStartTimestamp=\n")
	if err != nil {
		t.Fatalf("ParseShow returned error: %v", err)
	}

	if status.Summary() != "not found" {
		t.Fatalf("got summary %q", status.Summary())
	}
}

func TestParseShowRejectsMalformedLine(t *testing.T) {
	_, err := ParseShow("garbage without equals")

	if err == nil {
		t.Fatal("expected an error")
	}
}
