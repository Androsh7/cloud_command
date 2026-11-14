// Package systemd builds the shell commands that start, inspect and stop persistent jobs as transient systemd units.
package systemd

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	UnitPrefix        = "job-"
	OutputDirectory   = "/tmp"
	DefaultExecutable = "/bin/bash"
	RootUser          = "root"

	DefaultOutputLines = 200
	MaxOutputBytes     = 4 * 1024 * 1024

	LoadStateNotFound = "not-found"
	ActiveStateActive = "active"
	ActiveStateFailed = "failed"
	SubStateRunning   = "running"
	SubStateExited    = "exited"

	showTimestampLayout = "Mon 2006-01-02 15:04:05 MST"
)

// JobSpec describes a persistent job to start with systemd-run.
type JobSpec struct {
	Id         string
	Script     string
	Executable string
	User       string
}

// UnitName returns the transient unit name for the job.
func (spec JobSpec) UnitName() string {
	return UnitPrefix + spec.Id
}

// StdoutPath returns the file the unit writes standard output to.
func (spec JobSpec) StdoutPath() string {
	return OutputDirectory + "/" + spec.UnitName() + ".stdout"
}

// StderrPath returns the file the unit writes standard error to.
func (spec JobSpec) StderrPath() string {
	return OutputDirectory + "/" + spec.UnitName() + ".stderr"
}

// RunCommand returns the systemd-run command line that starts the job.
// The script reaches the executable as a file, so commands in it that read standard input cannot consume the rest of it.
func (spec JobSpec) RunCommand() string {
	executable := spec.Executable
	if executable == "" {
		executable = DefaultExecutable
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(spec.Script))
	return fmt.Sprintf(
		"sudo systemd-run --unit=%s -p RemainAfterExit=yes -p StandardOutput=file:%s -p StandardError=file:%s -p User=%s -- %s -c 'exec %s <(echo %s | base64 -d)'",
		spec.UnitName(), spec.StdoutPath(), spec.StderrPath(), spec.User, DefaultExecutable, executable, encoded,
	)
}

// ShowCommand returns the systemctl command that prints the unit properties ParseShow understands.
func ShowCommand(unitName string) string {
	return "systemctl show --property=LoadState,ActiveState,SubState,ExecMainStatus,ExecMainStartTimestamp " + unitName
}

// ReadOutputCommand returns the command that prints the last lines of one output file, or nothing when it is missing.
// The output is also capped in bytes so a file with very long lines cannot be downloaded whole.
func ReadOutputCommand(path string, lines int) string {
	if lines <= 0 {
		lines = DefaultOutputLines
	}
	return fmt.Sprintf("tail -c %d %s 2>/dev/null | tail -n %d", MaxOutputBytes, path, lines)
}

// StopCommand returns the command that stops the unit.
func StopCommand(unitName string) string {
	return "sudo systemctl stop " + unitName
}

// CleanupCommand returns the command that forgets a finished unit and removes its output files.
func CleanupCommand(spec JobSpec) string {
	return fmt.Sprintf("sudo systemctl reset-failed %s 2>/dev/null; sudo rm -f %s %s", spec.UnitName(), spec.StdoutPath(), spec.StderrPath())
}

// UnitStatus is the parsed output of ShowCommand.
type UnitStatus struct {
	LoadState   string
	ActiveState string
	SubState    string
	ExitCode    int
	StartedAt   time.Time
}

// ParseShow parses the key=value lines printed by ShowCommand.
func ParseShow(output string) (UnitStatus, error) {
	status := UnitStatus{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return UnitStatus{}, fmt.Errorf("parsing systemctl show line %q: missing '='", line)
		}
		switch key {
		case "LoadState":
			status.LoadState = value
		case "ActiveState":
			status.ActiveState = value
		case "SubState":
			status.SubState = value
		case "ExecMainStatus":
			exitCode, err := strconv.Atoi(value)
			if err != nil {
				return UnitStatus{}, fmt.Errorf("parsing ExecMainStatus %q: %w", value, err)
			}
			status.ExitCode = exitCode
		case "ExecMainStartTimestamp":
			startedAt, err := time.Parse(showTimestampLayout, value)
			if err == nil {
				status.StartedAt = startedAt
			}
		}
	}
	return status, nil
}

// IsRunning reports whether the unit's process is still running.
func (status UnitStatus) IsRunning() bool {
	return status.ActiveState == ActiveStateActive && status.SubState == SubStateRunning
}

// Summary returns a short human readable state such as "running" or "exited (0)".
func (status UnitStatus) Summary() string {
	switch {
	case status.LoadState == LoadStateNotFound:
		return "not found"
	case status.IsRunning():
		return "running"
	case status.ActiveState == ActiveStateFailed:
		return fmt.Sprintf("failed (%d)", status.ExitCode)
	case status.SubState == SubStateExited:
		return fmt.Sprintf("exited (%d)", status.ExitCode)
	default:
		return status.ActiveState + "/" + status.SubState
	}
}
