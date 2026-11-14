package agent

import (
	"testing"
)

func TestForgetJobKeepsJobsSavedByAnotherLoad(t *testing.T) {
	store := NewStore(t.TempDir())
	directory, err := store.NewAgentDirectory("alpha")
	if err != nil {
		t.Fatalf("NewAgentDirectory returned error: %v", err)
	}
	record := sampleRecord("alpha")
	record.Jobs = []Job{{Id: "job1"}, {Id: "job2"}, {Id: "job3"}}
	if err := store.Save(&Agent{Record: record, Directory: directory, store: store}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	first, err := store.Load("alpha")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	stale, err := store.Load("alpha")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if err := first.ForgetJob(Job{Id: "job1"}); err != nil {
		t.Fatalf("first ForgetJob returned error: %v", err)
	}
	if err := stale.ForgetJob(Job{Id: "job2"}); err != nil {
		t.Fatalf("stale ForgetJob returned error: %v", err)
	}

	reloaded, err := store.Load("alpha")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(reloaded.Record.Jobs) != 1 || reloaded.Record.Jobs[0].Id != "job3" {
		t.Fatalf("got jobs %+v, want only job3", reloaded.Record.Jobs)
	}
}

func TestReloadPicksUpJobsSavedByAnotherLoad(t *testing.T) {
	store := NewStore(t.TempDir())
	directory, err := store.NewAgentDirectory("alpha")
	if err != nil {
		t.Fatalf("NewAgentDirectory returned error: %v", err)
	}
	if err := store.Save(&Agent{Record: sampleRecord("alpha"), Directory: directory, store: store}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	stale, err := store.Load("alpha")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	other, err := store.Load("alpha")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if err := other.ForgetJob(Job{Id: "job1"}); err != nil {
		t.Fatalf("ForgetJob returned error: %v", err)
	}

	if err := stale.Reload(); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}

	if len(stale.Record.Jobs) != 0 {
		t.Fatalf("got jobs %+v, want none", stale.Record.Jobs)
	}
}
