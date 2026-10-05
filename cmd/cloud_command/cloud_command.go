package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/pkg/ec2"
	"github.com/androsh7/cloud_command/pkg/sshx"
)

func main() {
	// Load config
	appConfig := config.New()
	logFile, err := os.OpenFile(appConfig.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Configure logger
	logger := slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo, 
		AddSource: true,
	}))
	slog.SetDefault(logger)

	// Connect to ssh host
	sshHost := sshx.NewSshHost("127.0.0.1", "deploy", 2222, 10 * time.Second, filepath.Join(appConfig.HomeDirectory, ".ssh\\sshd_container"))
	sshHost.Connect()
	response, err := sshHost.RunCommand("cd ~ && cat /etc/passwd && pwd")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response.Stdout)
	fmt.Println(sshHost.IsConnectionAlive())
	fmt.Println("Hello, World")

	// Get AWS regions
	ctx := context.Background()
	out, err := ec2.GetEc2Regions(ctx)
	println(strings.Join(out, ", "))
}