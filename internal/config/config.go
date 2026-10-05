package config

import (
	"log"
	"os"
	"path/filepath"
)

const logName string = "cloud_command.log"

type Config struct {
	ParentDirectory string
	HomeDirectory string
	LogPath string
}

func New() *Config {
	ParentDirectory, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	HomeDirectory, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	LogPath := filepath.Join(ParentDirectory, logName)

	return &Config{
		ParentDirectory: ParentDirectory,
		HomeDirectory: HomeDirectory,
		LogPath: LogPath,
	}
}