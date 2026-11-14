package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var outputTestTime = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func TestWriteTranscriptsSavesSingleTranscriptAsFile(t *testing.T) {
	directory := t.TempDir()
	transcripts := []Transcript{{AgentName: "alpha", Lines: []string{"$ echo hi", "hi"}}}

	path, err := WriteTranscripts(directory, transcripts, outputTestTime)
	if err != nil {
		t.Fatalf("WriteTranscripts returned error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if filepath.Dir(path) != directory || !strings.HasSuffix(path, "-alpha.txt") {
		t.Fatalf("unexpected path %q", path)
	}
	if string(content) != "$ echo hi\nhi\n" {
		t.Fatalf("unexpected content %q", content)
	}
}

func TestWriteTranscriptsSavesSeveralTranscriptsAsDirectory(t *testing.T) {
	directory := t.TempDir()
	transcripts := []Transcript{
		{AgentName: "alpha", Lines: []string{"one"}},
		{AgentName: "bravo", Lines: []string{"two"}},
	}

	path, err := WriteTranscripts(directory, transcripts, outputTestTime)
	if err != nil {
		t.Fatalf("WriteTranscripts returned error: %v", err)
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
}

func TestWriteTranscriptsRejectsEmptyInput(t *testing.T) {
	_, err := WriteTranscripts(t.TempDir(), nil, outputTestTime)

	if err == nil {
		t.Fatal("expected an error")
	}
}
