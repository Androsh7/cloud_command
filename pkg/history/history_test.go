package history

import (
	"errors"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), FileName))
}

func TestRecordAssignsIdAndTimeAndRoundTrips(t *testing.T) {
	store := testStore(t)

	recorded, err := store.Record(Entry{Agent: "alpha", Kind: KindCommand, Source: SourceTui, Script: "echo hi\necho two", Sudo: true})
	if err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	entries, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if recorded.Id == "" || recorded.Time.IsZero() {
		t.Fatalf("id or time not assigned: %+v", recorded)
	}
	if len(entries) != 1 || entries[0].Script != "echo hi\necho two" || !entries[0].Sudo || entries[0].Agent != "alpha" {
		t.Fatalf("got %+v", entries)
	}
}

func TestListReturnsNewestFirst(t *testing.T) {
	store := testStore(t)
	_, _ = store.Record(Entry{Agent: "alpha", Kind: KindCommand, Source: SourceCli, Script: "first"})
	_, _ = store.Record(Entry{Agent: "alpha", Kind: KindJob, Source: SourceMcp, Script: "second", JobId: "j1"})

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(entries) != 2 || entries[0].Script != "second" || entries[1].Script != "first" {
		t.Fatalf("wrong order: %+v", entries)
	}
}

func TestSetResultStoresExitCode(t *testing.T) {
	store := testStore(t)
	recorded, _ := store.Record(Entry{Agent: "alpha", Kind: KindCommand, Source: SourceTui, Script: "false"})

	exitCode := 1
	if err := store.SetResult(recorded.Id, &exitCode, ""); err != nil {
		t.Fatalf("SetResult returned error: %v", err)
	}

	entries, _ := store.List()
	if entries[0].ExitCode == nil || *entries[0].ExitCode != 1 {
		t.Fatalf("exit code not stored: %+v", entries[0])
	}
}

func TestSetResultRejectsUnknownEntry(t *testing.T) {
	store := testStore(t)

	err := store.SetResult("missing", nil, "boom")

	if !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("got %v, want ErrEntryNotFound", err)
	}
}

func TestListOnMissingFileIsEmpty(t *testing.T) {
	store := testStore(t)

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("got %d entries", len(entries))
	}
}
