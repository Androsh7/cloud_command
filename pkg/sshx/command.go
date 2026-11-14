package sshx

import (
	"encoding/base64"
	"fmt"
)

const DefaultExecutable = "/bin/bash"

// Command describes one script to run on a remote host.
type Command struct {
	Script     string
	Executable string
	Sudo       bool
}

// EncodeScript returns a one-line command that decodes and runs a script with the given executable.
func EncodeScript(script string, executable string) string {
	if executable == "" {
		executable = DefaultExecutable
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	return fmt.Sprintf(`%s -c "$(echo %s | base64 -d)"`, executable, encoded)
}

// Line returns the full remote command line, prefixed with sudo when requested.
func (command Command) Line() string {
	line := EncodeScript(command.Script, command.Executable)
	if command.Sudo {
		return "sudo " + line
	}
	return line
}
