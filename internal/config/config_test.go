package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/androsh7/cloud_command/pkg/ec2"
)

func TestLoadSettingsWritesDefaultsWhenMissing(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), SettingsFileName)

	settings, err := LoadSettings(settingsPath)
	if err != nil {
		t.Fatalf("LoadSettings returned error: %v", err)
	}

	if settings != DefaultSettings() {
		t.Fatalf("got %+v, want defaults %+v", settings, DefaultSettings())
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("settings file was not written: %v", err)
	}
}

func TestSaveSettingsRoundTrips(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), SettingsFileName)
	expected := Settings{
		Region:              "eu-west-1",
		InstanceType:        "t3.micro",
		Architecture:        ec2.ArchitectureX86_64,
		SshUser:             "admin",
		SshPort:             2222,
		SshTimeout:          3 * time.Second,
		ReadyTimeout:        90 * time.Second,
		PollInterval:        time.Second,
		RegionSearchThreads: 4,
		LogLevel:            "debug",
	}

	if err := SaveSettings(settingsPath, expected); err != nil {
		t.Fatalf("SaveSettings returned error: %v", err)
	}
	loaded, err := LoadSettings(settingsPath)
	if err != nil {
		t.Fatalf("LoadSettings returned error: %v", err)
	}

	if loaded != expected {
		t.Fatalf("got %+v, want %+v", loaded, expected)
	}
}

func TestValidateRejectsUnknownArchitecture(t *testing.T) {
	settings := DefaultSettings()
	settings.Architecture = "riscv"

	err := settings.Validate()

	if err == nil {
		t.Fatal("expected an error for an unknown architecture")
	}
}

func TestEnsureDirectoriesCreatesAgentsDirectory(t *testing.T) {
	paths := PathsUnder(filepath.Join(t.TempDir(), RootDirectoryName))

	if err := EnsureDirectories(paths); err != nil {
		t.Fatalf("EnsureDirectories returned error: %v", err)
	}

	info, err := os.Stat(paths.AgentsDirectory)
	if err != nil || !info.IsDir() {
		t.Fatalf("agents directory missing: %v", err)
	}
}
