// Package agent manages EC2 instances as agents: creation, command execution, persistent jobs and teardown.
package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/androsh7/cloud_command/pkg/ec2"
)

const (
	RecordFileName = "agent.toml"

	directoryPermissions = 0o700
	recordPermissions    = 0o600
	temporaryRecordName  = RecordFileName + ".tmp"
)

var (
	ErrAgentExists   = errors.New("agent already exists")
	ErrAgentNotFound = errors.New("agent not found")
)

// Job records a persistent systemd job started on an agent.
type Job struct {
	Id         string    `toml:"id"`
	Script     string    `toml:"script"`
	Sudo       bool      `toml:"sudo"`
	StdoutPath string    `toml:"stdout_path"`
	StderrPath string    `toml:"stderr_path"`
	StartedAt  time.Time `toml:"started_at"`
}

// Record is the on-disk content of agent.toml.
type Record struct {
	Name            string           `toml:"name"`
	Region          string           `toml:"region"`
	InstanceType    string           `toml:"instance_type"`
	Architecture    ec2.Architecture `toml:"architecture"`
	SshUser         string           `toml:"ssh_user"`
	SshPort         int              `toml:"ssh_port"`
	InstanceId      string           `toml:"instance_id"`
	KeyPairName     string           `toml:"key_pair_name"`
	AmiId           string           `toml:"ami_id"`
	VpcId           string           `toml:"vpc_id"`
	SecurityGroupId string           `toml:"security_group_id"`
	SubnetId        string           `toml:"subnet_id"`
	PublicIp        string           `toml:"public_ip"`
	PrivateKeyFile  string           `toml:"private_key_file"`
	PublicKeyFile   string           `toml:"public_key_file"`
	CreatedAt       time.Time        `toml:"created_at"`
	Jobs            []Job            `toml:"jobs"`
}

// Store reads and writes agent directories under one root.
type Store struct {
	AgentsDirectory string
}

// NewStore returns a store rooted at one agents directory.
func NewStore(agentsDirectory string) *Store {
	return &Store{AgentsDirectory: agentsDirectory}
}

// AgentDirectory returns the directory that holds one agent's files.
func (store *Store) AgentDirectory(name string) string {
	return filepath.Join(store.AgentsDirectory, name)
}

// List returns every agent with a readable record, sorted by name.
func (store *Store) List() ([]*Agent, error) {
	entries, err := os.ReadDir(store.AgentsDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return []*Agent{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading agents directory %q: %w", store.AgentsDirectory, err)
	}
	agents := []*Agent{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		loadedAgent, err := store.Load(entry.Name())
		if errors.Is(err, ErrAgentNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		agents = append(agents, loadedAgent)
	}
	sort.Slice(agents, func(left int, right int) bool { return agents[left].Record.Name < agents[right].Record.Name })
	return agents, nil
}

// Load reads one agent's record.
func (store *Store) Load(name string) (*Agent, error) {
	directory := store.AgentDirectory(name)
	var record Record
	_, err := toml.DecodeFile(filepath.Join(directory, RecordFileName), &record)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", ErrAgentNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("reading agent record for %q: %w", name, err)
	}
	return &Agent{Record: record, Directory: directory, store: store}, nil
}

// Save writes one agent's record atomically.
func (store *Store) Save(agentToSave *Agent) error {
	temporaryPath := filepath.Join(agentToSave.Directory, temporaryRecordName)
	file, err := os.OpenFile(temporaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, recordPermissions)
	if err != nil {
		return fmt.Errorf("opening record %q: %w", temporaryPath, err)
	}
	encodeErr := toml.NewEncoder(file).Encode(agentToSave.Record)
	closeErr := file.Close()
	if encodeErr != nil {
		return fmt.Errorf("writing record %q: %w", temporaryPath, encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing record %q: %w", temporaryPath, closeErr)
	}
	finalPath := filepath.Join(agentToSave.Directory, RecordFileName)
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return fmt.Errorf("replacing record %q: %w", finalPath, err)
	}
	return nil
}

// NewAgentDirectory creates the directory for a new agent and returns its path.
func (store *Store) NewAgentDirectory(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	directory := store.AgentDirectory(name)
	if err := os.Mkdir(directory, directoryPermissions); errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("%w: %q", ErrAgentExists, name)
	} else if err != nil {
		return "", fmt.Errorf("creating agent directory %q: %w", directory, err)
	}
	return directory, nil
}

// Remove deletes one agent's directory and everything in it.
func (store *Store) Remove(name string) error {
	if err := os.RemoveAll(store.AgentDirectory(name)); err != nil {
		return fmt.Errorf("removing agent directory for %q: %w", name, err)
	}
	return nil
}
