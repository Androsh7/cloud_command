package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	outputFileExtension     = ".txt"
	outputTimestampLayout   = "2006-01-02_15-04-05"
	outputFilePermissions   = 0o600
	outputFolderPermissions = 0o700
)

// Transcript is the text captured from one agent's shell.
type Transcript struct {
	AgentName string
	Lines     []string
}

// WriteTranscripts saves transcripts under a directory and returns the path written.
// One transcript becomes a single file named <timestamp>-<agent>.txt; several become a <timestamp> directory with one file per agent.
func WriteTranscripts(outputDirectory string, transcripts []Transcript, finished time.Time) (string, error) {
	if len(transcripts) == 0 {
		return "", fmt.Errorf("no transcripts to save")
	}
	if err := os.MkdirAll(outputDirectory, outputFolderPermissions); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", outputDirectory, err)
	}
	timestamp := finished.Local().Format(outputTimestampLayout)

	if len(transcripts) == 1 {
		path := filepath.Join(outputDirectory, timestamp+"-"+transcripts[0].AgentName+outputFileExtension)
		if err := writeTranscript(path, transcripts[0]); err != nil {
			return "", err
		}
		return path, nil
	}

	directory := filepath.Join(outputDirectory, timestamp)
	if err := os.MkdirAll(directory, outputFolderPermissions); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", directory, err)
	}
	for _, transcript := range transcripts {
		if err := writeTranscript(filepath.Join(directory, transcript.AgentName+outputFileExtension), transcript); err != nil {
			return "", err
		}
	}
	return directory, nil
}

// writeTranscript writes one transcript's lines to a file.
func writeTranscript(path string, transcript Transcript) error {
	content := strings.Join(transcript.Lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), outputFilePermissions); err != nil {
		return fmt.Errorf("writing transcript %q: %w", path, err)
	}
	return nil
}
