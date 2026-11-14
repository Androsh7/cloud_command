// Package config resolves the ~/.cloud-command directory and loads the user settings stored in it.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/androsh7/cloud_command/pkg/ec2"
)

const (
	RootDirectoryName   = ".cloud-command"
	SettingsFileName    = "settings.toml"
	AgentsDirectoryName = "agents"
	OutputDirectoryName = "output"
	HistoryFileName     = "history.toml"
	LogFileName         = "cloud-command.log"

	DefaultRegion               = "us-east-1"
	DefaultInstanceType         = "t4g.nano"
	DefaultArchitecture         = ec2.ArchitectureArm64
	DefaultSshUser              = "ec2-user"
	DefaultSshPort              = 22
	DefaultSshTimeout           = 10 * time.Second
	DefaultReadyTimeout         = 5 * time.Minute
	DefaultPollInterval         = 5 * time.Second
	DefaultStateRefreshInterval = 15 * time.Second
	DefaultRegionSearchThreads  = 0
	DefaultLogLevel             = "info"

	directoryPermissions = 0o700
	filePermissions      = 0o600
)

// Paths holds every filesystem location the application uses.
type Paths struct {
	RootDirectory   string
	AgentsDirectory string
	OutputDirectory string
	SettingsPath    string
	HistoryPath     string
	LogPath         string
}

// Settings holds the user-editable defaults read from settings.toml.
type Settings struct {
	Region               string           `toml:"region"`
	InstanceType         string           `toml:"instance_type"`
	Architecture         ec2.Architecture `toml:"architecture"`
	SshUser              string           `toml:"ssh_user"`
	SshPort              int              `toml:"ssh_port"`
	SshTimeout           time.Duration    `toml:"ssh_timeout"`
	ReadyTimeout         time.Duration    `toml:"ready_timeout"`
	PollInterval         time.Duration    `toml:"poll_interval"`
	StateRefreshInterval time.Duration    `toml:"state_refresh_interval"`
	RegionSearchThreads  int              `toml:"region_search_threads"`
	LogLevel             string           `toml:"log_level"`
}

// DefaultSettings returns the settings written on first run.
func DefaultSettings() Settings {
	return Settings{
		Region:               DefaultRegion,
		InstanceType:         DefaultInstanceType,
		Architecture:         DefaultArchitecture,
		SshUser:              DefaultSshUser,
		SshPort:              DefaultSshPort,
		SshTimeout:           DefaultSshTimeout,
		ReadyTimeout:         DefaultReadyTimeout,
		PollInterval:         DefaultPollInterval,
		StateRefreshInterval: DefaultStateRefreshInterval,
		RegionSearchThreads:  DefaultRegionSearchThreads,
		LogLevel:             DefaultLogLevel,
	}
}

// Validate returns an error describing the first invalid setting.
func (settings Settings) Validate() error {
	switch settings.Architecture {
	case ec2.ArchitectureX86_64, ec2.ArchitectureArm64:
	default:
		return fmt.Errorf("architecture %q must be %q or %q", settings.Architecture, ec2.ArchitectureX86_64, ec2.ArchitectureArm64)
	}
	if settings.Region == "" {
		return errors.New("region must not be empty")
	}
	if settings.InstanceType == "" {
		return errors.New("instance_type must not be empty")
	}
	if settings.SshUser == "" {
		return errors.New("ssh_user must not be empty")
	}
	if settings.SshPort <= 0 {
		return fmt.Errorf("ssh_port %d must be positive", settings.SshPort)
	}
	if settings.SshTimeout <= 0 || settings.ReadyTimeout <= 0 || settings.PollInterval <= 0 || settings.StateRefreshInterval <= 0 {
		return errors.New("ssh_timeout, ready_timeout, poll_interval and state_refresh_interval must be positive")
	}
	if settings.RegionSearchThreads < 0 {
		return fmt.Errorf("region_search_threads %d must not be negative", settings.RegionSearchThreads)
	}
	return nil
}

// ResolvePaths returns the paths under the user's home directory.
func ResolvePaths() (Paths, error) {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolving home directory: %w", err)
	}
	return PathsUnder(filepath.Join(homeDirectory, RootDirectoryName)), nil
}

// PathsUnder returns the paths rooted at one directory.
func PathsUnder(rootDirectory string) Paths {
	return Paths{
		RootDirectory:   rootDirectory,
		AgentsDirectory: filepath.Join(rootDirectory, AgentsDirectoryName),
		OutputDirectory: filepath.Join(rootDirectory, OutputDirectoryName),
		SettingsPath:    filepath.Join(rootDirectory, SettingsFileName),
		HistoryPath:     filepath.Join(rootDirectory, HistoryFileName),
		LogPath:         filepath.Join(rootDirectory, LogFileName),
	}
}

// EnsureDirectories creates the root and agents directories when they are missing.
func EnsureDirectories(paths Paths) error {
	for _, directory := range []string{paths.RootDirectory, paths.AgentsDirectory, paths.OutputDirectory} {
		if err := os.MkdirAll(directory, directoryPermissions); err != nil {
			return fmt.Errorf("creating directory %q: %w", directory, err)
		}
	}
	return nil
}

// LoadSettings reads the settings file, writing and returning the defaults when it does not exist.
func LoadSettings(path string) (Settings, error) {
	settings := DefaultSettings()
	_, err := toml.DecodeFile(path, &settings)
	if errors.Is(err, os.ErrNotExist) {
		if saveErr := SaveSettings(path, settings); saveErr != nil {
			return Settings{}, saveErr
		}
		return settings, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("reading settings %q: %w", path, err)
	}
	if err := settings.Validate(); err != nil {
		return Settings{}, fmt.Errorf("validating settings %q: %w", path, err)
	}
	return settings, nil
}

// SaveSettings writes the settings file.
func SaveSettings(path string, settings Settings) error {
	if err := settings.Validate(); err != nil {
		return fmt.Errorf("validating settings: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePermissions)
	if err != nil {
		return fmt.Errorf("opening settings %q: %w", path, err)
	}
	defer file.Close()
	if err := toml.NewEncoder(file).Encode(settings); err != nil {
		return fmt.Errorf("writing settings %q: %w", path, err)
	}
	return nil
}
