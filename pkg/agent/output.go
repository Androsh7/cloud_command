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

// FormatCommandResult returns the plain text saved for one agent's result.
func FormatCommandResult(result CommandResult) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "agent: %s\n", result.AgentName)
	if result.Err != nil {
		fmt.Fprintf(&builder, "error: %s\n", result.Err.Error())
		return builder.String()
	}
	fmt.Fprintf(&builder, "exit code: %d\n\n--- stdout ---\n", result.Response.ReturnCode)
	builder.WriteString(result.Response.Stdout)
	if result.Response.Stdout != "" && !strings.HasSuffix(result.Response.Stdout, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString("\n--- stderr ---\n")
	builder.WriteString(result.Response.Stderr)
	if result.Response.Stderr != "" && !strings.HasSuffix(result.Response.Stderr, "\n") {
		builder.WriteString("\n")
	}
	return builder.String()
}

// WriteCommandResults saves results under a directory and returns the path written.
// One result becomes a single file named <timestamp>-<agent>.txt; several become a <timestamp> directory with one file per agent.
func WriteCommandResults(outputDirectory string, results []CommandResult, finished time.Time) (string, error) {
	if len(results) == 0 {
		return "", fmt.Errorf("no results to save")
	}
	if err := os.MkdirAll(outputDirectory, outputFolderPermissions); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", outputDirectory, err)
	}
	timestamp := finished.Local().Format(outputTimestampLayout)

	if len(results) == 1 {
		path := filepath.Join(outputDirectory, timestamp+"-"+results[0].AgentName+outputFileExtension)
		if err := os.WriteFile(path, []byte(FormatCommandResult(results[0])), outputFilePermissions); err != nil {
			return "", fmt.Errorf("writing output %q: %w", path, err)
		}
		return path, nil
	}

	directory := filepath.Join(outputDirectory, timestamp)
	if err := os.MkdirAll(directory, outputFolderPermissions); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", directory, err)
	}
	for _, result := range results {
		path := filepath.Join(directory, result.AgentName+outputFileExtension)
		if err := os.WriteFile(path, []byte(FormatCommandResult(result)), outputFilePermissions); err != nil {
			return "", fmt.Errorf("writing output %q: %w", path, err)
		}
	}
	return directory, nil
}
