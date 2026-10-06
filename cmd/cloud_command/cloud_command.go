// Package main starts the cloud command terminal user interface.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/internal/tui"
	"github.com/androsh7/cloud_command/pkg/agent"
)

const logFilePermissions = 0o600

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run loads the settings, configures logging and starts the interface.
func run() error {
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	if err := config.EnsureDirectories(paths); err != nil {
		return err
	}
	settings, err := config.LoadSettings(paths.SettingsPath)
	if err != nil {
		return err
	}

	logFile, err := os.OpenFile(paths.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFilePermissions)
	if err != nil {
		return fmt.Errorf("opening log file %q: %w", paths.LogPath, err)
	}
	defer logFile.Close()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level:     parseLogLevel(settings.LogLevel),
		AddSource: true,
	})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	deps := tui.Dependencies{
		Settings: settings,
		Store:    agent.NewStore(paths.AgentsDirectory),
		Paths:    paths,
	}
	if err := tui.Run(ctx, deps); err != nil {
		slog.Error("tui exited with error", "error", err)
		return err
	}
	return nil
}

// parseLogLevel converts a settings log level name to a slog level, defaulting to info.
func parseLogLevel(name string) slog.Level {
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
