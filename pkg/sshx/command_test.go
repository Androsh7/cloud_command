package sshx

import "testing"

func TestEncodeScriptWrapsBase64Payload(t *testing.T) {
	line := EncodeScript("echo hi", "/bin/bash")

	expected := `/bin/bash -c "$(echo ZWNobyBoaQ== | base64 -d)"`
	if line != expected {
		t.Fatalf("got %q, want %q", line, expected)
	}
}

func TestEncodeScriptDefaultsExecutable(t *testing.T) {
	line := EncodeScript("true", "")

	expected := EncodeScript("true", DefaultExecutable)
	if line != expected {
		t.Fatalf("got %q, want %q", line, expected)
	}
}

func TestCommandLineAddsSudoPrefix(t *testing.T) {
	command := Command{Script: "id -u", Sudo: true}

	line := command.Line()

	expected := "sudo " + EncodeScript("id -u", DefaultExecutable)
	if line != expected {
		t.Fatalf("got %q, want %q", line, expected)
	}
}
