package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/androsh7/cloud_command/pkg/sshx"
)

var outputTestTime = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func TestWriteCommandResultsSavesSingleResultAsFile(t *testing.T) {
	directory := t.TempDir()
	results := []CommandResult{{
		AgentName: "alpha",
		Response:  sshx.CommandResponse{Stdout: "hello\n", Stderr: "careful\n", ReturnCode: 3},
	}}

	path, err := WriteCommandResults(directory, results, outputTestTime)
	if err != nil {
		t.Fatalf("WriteCommandResults returned error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if filepath.Dir(path) != directory || !strings.HasSuffix(path, "-alpha.txt") {
		t.Fatalf("unexpected path %q", path)
	}
	for _, expected := range []string{"exit code: 3", "hello", "careful"} {
		if !strings.Contains(string(content), expected) {
			t.Fatalf("saved file lacks %q:\n%s", expected, content)
		}
	}
}

func TestWriteCommandResultsSavesSeveralResultsAsDirectory(t *testing.T) {
	directory := t.TempDir()
	results := []CommandResult{
		{AgentName: "alpha", Response: sshx.CommandResponse{Stdout: "one\n"}},
		{AgentName: "bravo", Err: errors.New("ssh failed")},
	}

	path, err := WriteCommandResults(directory, results, outputTestTime)
	if err != nil {
		t.Fatalf("WriteCommandResults returned error: %v", err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("reading saved directory: %v", err)
	}
	names := map[string]bool{}
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	if !names["alpha.txt"] || !names["bravo.txt"] || len(names) != 2 {
		t.Fatalf("got files %v", names)
	}
	bravo, _ := os.ReadFile(filepath.Join(path, "bravo.txt"))
	if !strings.Contains(string(bravo), "ssh failed") {
		t.Fatalf("error result not saved:\n%s", bravo)
	}
}

func TestWriteCommandResultsRejectsEmptyResults(t *testing.T) {
	_, err := WriteCommandResults(t.TempDir(), nil, outputTestTime)

	if err == nil {
		t.Fatal("expected an error")
	}
}
