package control

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/ec2"
)

func testController(t *testing.T) *Controller {
	t.Helper()
	return New(agent.NewStore(t.TempDir()), config.DefaultSettings(), nil, "test")
}

func TestResolveSpecAppliesSettingsDefaults(t *testing.T) {
	controller := testController(t)

	spec, count, err := controller.resolveSpec(CreateRequest{Name: "alpha"})
	if err != nil {
		t.Fatalf("resolveSpec returned error: %v", err)
	}

	settings := config.DefaultSettings()
	if count != 1 || spec.Region != settings.Region || spec.InstanceType != settings.InstanceType ||
		spec.Architecture != settings.Architecture || spec.SshUser != settings.SshUser {
		t.Fatalf("defaults not applied: %+v count=%d", spec, count)
	}
}

func TestResolveSpecRejectsBadName(t *testing.T) {
	controller := testController(t)

	_, _, err := controller.resolveSpec(CreateRequest{Name: "../bad"})

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("got %v, want ErrInvalidRequest", err)
	}
}

func TestResolveSpecRejectsUnknownArchitecture(t *testing.T) {
	controller := testController(t)

	_, _, err := controller.resolveSpec(CreateRequest{Name: "alpha", Architecture: "riscv"})

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("got %v, want ErrInvalidRequest", err)
	}
}

func TestResolveSpecRejectsOversizedBatch(t *testing.T) {
	controller := testController(t)

	_, _, err := controller.resolveSpec(CreateRequest{Name: "alpha", Count: MaxBatchCount + 1})

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("got %v, want ErrInvalidRequest", err)
	}
}

func TestListAgentsOnEmptyStoreReturnsNoAgents(t *testing.T) {
	controller := testController(t)

	agents, detached, err := controller.ListAgents(context.Background(), false)
	if err != nil {
		t.Fatalf("ListAgents returned error: %v", err)
	}

	if len(agents) != 0 || len(detached) != 0 {
		t.Fatalf("got %d agents and %d detached", len(agents), len(detached))
	}
}

func TestGetAgentReportsMissingAgent(t *testing.T) {
	controller := testController(t)

	_, err := controller.GetAgent(context.Background(), "ghost")

	if !errors.Is(err, agent.ErrAgentNotFound) {
		t.Fatalf("got %v, want ErrAgentNotFound", err)
	}
}

func TestRunCommandRejectsEmptyScript(t *testing.T) {
	controller := testController(t)

	_, err := controller.RunCommand(context.Background(), []string{"alpha"}, "  ", false, 0)

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("got %v, want ErrInvalidRequest", err)
	}
}

func TestFindJobAcceptsUniquePrefix(t *testing.T) {
	stored := &agent.Agent{Record: agent.Record{Name: "alpha", Jobs: []agent.Job{{Id: "abc-1"}, {Id: "xyz-2"}}}}

	job, err := findJob(stored, "abc")
	if err != nil {
		t.Fatalf("findJob returned error: %v", err)
	}

	if job.Id != "abc-1" {
		t.Fatalf("got %q", job.Id)
	}
}

func TestFindJobRejectsAmbiguousPrefix(t *testing.T) {
	stored := &agent.Agent{Record: agent.Record{Name: "alpha", Jobs: []agent.Job{{Id: "abc-1"}, {Id: "abc-2"}}}}

	_, err := findJob(stored, "abc")

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("got %v, want ErrInvalidRequest", err)
	}
}

func TestTailLinesKeepsLastLines(t *testing.T) {
	tail := TailLines("one\ntwo\nthree\n", 2)

	if tail != "two\nthree" {
		t.Fatalf("got %q", tail)
	}
}

func TestSummaryMarshalsSnakeCase(t *testing.T) {
	summary := AgentSummary{Name: "alpha", Architecture: string(ec2.ArchitectureArm64), CreatedAt: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)}

	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	for _, key := range []string{"name", "instance_type", "instance_id", "public_ip", "job_count", "created_at"} {
		if _, found := decoded[key]; !found {
			t.Fatalf("key %q missing in %s", key, encoded)
		}
	}
}
