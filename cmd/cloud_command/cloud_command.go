// Package main starts the cloud command terminal user interface, its command line and its MCP server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/internal/control"
	"github.com/androsh7/cloud_command/internal/tui"
	"github.com/androsh7/cloud_command/internal/version"
	"github.com/androsh7/cloud_command/pkg/agent"
)

const logFilePermissions = 0o600

// application holds everything a command needs after bootstrap.
type application struct {
	paths      config.Paths
	settings   config.Settings
	store      *agent.Store
	controller *control.Controller
	closeLog   func()
}

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCommand builds the command tree. Running with no subcommand opens the interface.
func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "cloud_command",
		Short:         "Manage disposable EC2 agents",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				deps := tui.Dependencies{Settings: app.settings, Store: app.store, Paths: app.paths}
				if err := tui.Run(ctx, deps); err != nil {
					slog.Error("tui exited with error", "error", err)
					return err
				}
				return nil
			})
		},
	}
	root.AddCommand(newMcpCommand(), newAgentsCommand(), newJobsCommand())
	return root
}

// withApplication bootstraps the application, runs one command and releases the log file.
func withApplication(parent context.Context, run func(ctx context.Context, app *application) error) error {
	app, err := bootstrap()
	if err != nil {
		return err
	}
	defer app.closeLog()
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt)
	defer stop()
	return run(ctx, app)
}

// bootstrap loads the settings and configures logging.
func bootstrap() (*application, error) {
	paths, err := config.ResolvePaths()
	if err != nil {
		return nil, err
	}
	if err := config.EnsureDirectories(paths); err != nil {
		return nil, err
	}
	settings, err := config.LoadSettings(paths.SettingsPath)
	if err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(paths.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFilePermissions)
	if err != nil {
		return nil, fmt.Errorf("opening log file %q: %w", paths.LogPath, err)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level:     parseLogLevel(settings.LogLevel),
		AddSource: true,
	})))
	store := agent.NewStore(paths.AgentsDirectory)
	return &application{
		paths:      paths,
		settings:   settings,
		store:      store,
		controller: control.New(store, settings),
		closeLog:   func() { _ = logFile.Close() },
	}, nil
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
