package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/internal/tui"
	"github.com/androsh7/cloud_command/pkg/ec2"
)

func main() {
	// Load config
	appConfig := config.New()
	logFile, err := os.OpenFile(appConfig.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Configure logger
	logger := slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := tui.Run(ctx, ec2.AgentTypeTagValue); err != nil {
		slog.Error("tui exited with error", "error", err)
		fmt.Println(err)
		os.Exit(1)
	}
}
