// Package history records every command and job ever sent to an agent, surviving the agent itself.
package history

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gofrs/flock"
	"github.com/google/uuid"
)

const (
	FileName        = "history.toml"
	MaxEntries      = 2000
	filePermissions = 0o600
	temporarySuffix = ".tmp"
	lockSuffix      = ".lock"
)

// Kind says whether an entry was a one-shot command or a persistent job.
type Kind string

const (
	KindCommand Kind = "command"
	KindJob     Kind = "job"
)

// Source says which client sent the entry.
type Source string

const (
	SourceTui Source = "tui"
	SourceCli Source = "cli"
	SourceMcp Source = "mcp"
)

var ErrEntryNotFound = errors.New("history entry not found")

// Entry is one command or job that was sent to one agent.
type Entry struct {
	Id       string    `toml:"id"`
	Time     time.Time `toml:"time"`
	Agent    string    `toml:"agent"`
	Kind     Kind      `toml:"kind"`
	Source   Source    `toml:"source"`
	Script   string    `toml:"script"`
	Sudo     bool      `toml:"sudo"`
	JobId    string    `toml:"job_id,omitempty"`
	ExitCode *int      `toml:"exit_code,omitempty"`
	Error    string    `toml:"error,omitempty"`
}

// file is the on-disk layout of history.toml.
type file struct {
	Entries []Entry `toml:"entries"`
}

// Store reads and appends the history file.
type Store struct {
	Path string
	lock sync.Mutex
}

// NewStore returns a store for one history file.
func NewStore(path string) *Store {
	return &Store{Path: path}
}

// lockFile takes the lock that serialises access to the history file, both inside this process and across processes.
// The caller runs the returned function to release it.
func (store *Store) lockFile() (func(), error) {
	store.lock.Lock()
	fileLock := flock.New(store.Path + lockSuffix)
	if err := fileLock.Lock(); err != nil {
		store.lock.Unlock()
		return nil, fmt.Errorf("locking history %q: %w", store.Path, err)
	}
	return func() {
		_ = fileLock.Unlock()
		store.lock.Unlock()
	}, nil
}

// Record appends an entry, filling its id and time when they are empty, and returns it.
func (store *Store) Record(entry Entry) (Entry, error) {
	unlock, err := store.lockFile()
	if err != nil {
		return Entry{}, err
	}
	defer unlock()
	if entry.Id == "" {
		entry.Id = uuid.NewString()
	}
	if entry.Time.IsZero() {
		entry.Time = time.Now().UTC()
	}
	entries, err := store.read()
	if err != nil {
		return Entry{}, err
	}
	entries = append(entries, entry)
	if len(entries) > MaxEntries {
		entries = entries[len(entries)-MaxEntries:]
	}
	return entry, store.write(entries)
}

// SetResult stores the exit code, or the error, of an earlier entry.
func (store *Store) SetResult(id string, exitCode *int, failure string) error {
	unlock, err := store.lockFile()
	if err != nil {
		return err
	}
	defer unlock()
	entries, err := store.read()
	if err != nil {
		return err
	}
	for index := range entries {
		if entries[index].Id == id {
			entries[index].ExitCode = exitCode
			entries[index].Error = failure
			return store.write(entries)
		}
	}
	return fmt.Errorf("%w: %q", ErrEntryNotFound, id)
}

// List returns every entry, newest first.
func (store *Store) List() ([]Entry, error) {
	unlock, err := store.lockFile()
	if err != nil {
		return nil, err
	}
	defer unlock()
	entries, err := store.read()
	if err != nil {
		return nil, err
	}
	reversed := make([]Entry, 0, len(entries))
	for index := len(entries) - 1; index >= 0; index-- {
		reversed = append(reversed, entries[index])
	}
	return reversed, nil
}

// read loads the file, treating a missing file as empty.
func (store *Store) read() ([]Entry, error) {
	var content file
	_, err := toml.DecodeFile(store.Path, &content)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading history %q: %w", store.Path, err)
	}
	return content.Entries, nil
}

// write replaces the file atomically.
func (store *Store) write(entries []Entry) error {
	temporaryPath := store.Path + temporarySuffix
	handle, err := os.OpenFile(temporaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePermissions)
	if err != nil {
		return fmt.Errorf("opening history %q: %w", temporaryPath, err)
	}
	encodeErr := toml.NewEncoder(handle).Encode(file{Entries: entries})
	closeErr := handle.Close()
	if encodeErr != nil {
		return fmt.Errorf("writing history %q: %w", temporaryPath, encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing history %q: %w", temporaryPath, closeErr)
	}
	if err := os.Rename(temporaryPath, store.Path); err != nil {
		return fmt.Errorf("replacing history %q: %w", filepath.Base(store.Path), err)
	}
	return nil
}
