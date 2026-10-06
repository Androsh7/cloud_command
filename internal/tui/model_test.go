package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/ec2"
	"github.com/androsh7/cloud_command/pkg/sshx"
)

func testModel(t *testing.T) Model {
	t.Helper()
	paths := config.PathsUnder(t.TempDir())
	if err := config.EnsureDirectories(paths); err != nil {
		t.Fatalf("EnsureDirectories returned error: %v", err)
	}
	return New(context.Background(), Dependencies{
		Settings: config.DefaultSettings(),
		Store:    agent.NewStore(paths.AgentsDirectory),
		Paths:    paths,
	})
}

func pressKey(model tea.Model, key string) tea.Model {
	var message tea.KeyMsg
	switch key {
	case "enter":
		message = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		message = tea.KeyMsg{Type: tea.KeyEsc}
	default:
		message = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := model.Update(message)
	return updated
}

func TestWindowResizeBeforeAnyScreenDoesNotPanic(t *testing.T) {
	model := testModel(t)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 205, Height: 50})
	updated = pressKey(updated, "n")

	if updated.(Model).screen != screenCreate {
		t.Fatalf("got screen %d, want create", updated.(Model).screen)
	}
}

func TestResizeWhileEditingScriptKeepsEditorUsable(t *testing.T) {
	model := testModel(t)
	updated, _ := model.Update(agentsLoadedMsg{agents: []*agent.Agent{}, infos: map[string]ec2.InstanceInfo{}})
	editing := updated.(Model).openCommand([]*agent.Agent{}, commandModeRun, screenAgents)

	resized, _ := editing.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	typed := pressKey(resized, "uname")

	if !strings.Contains(typed.View(), "uname") {
		t.Fatalf("editor lost its content: %s", typed.View())
	}
}

func TestAgentsScreenShowsEmptyHint(t *testing.T) {
	model := testModel(t)

	updated, _ := model.Update(agentsLoadedMsg{agents: []*agent.Agent{}, infos: map[string]ec2.InstanceInfo{}})
	view := updated.View()

	if !strings.Contains(view, "No agents yet") {
		t.Fatalf("view does not show the empty hint:\n%s", view)
	}
}

func TestPressingNOpensCreateForm(t *testing.T) {
	model := testModel(t)

	updated := pressKey(model, "n")

	if updated.(Model).screen != screenCreate {
		t.Fatalf("got screen %d, want create", updated.(Model).screen)
	}
	if !strings.Contains(updated.View(), "Create an agent") {
		t.Fatal("create form is not rendered")
	}
}

func TestPressingBOpensBatchForm(t *testing.T) {
	model := testModel(t)

	updated := pressKey(model, "b")

	if !strings.Contains(updated.View(), "Count") {
		t.Fatal("batch form does not show the count field")
	}
}

func TestCreateFormRejectsInvalidName(t *testing.T) {
	model := testModel(t)
	updated := pressKey(model, "n")
	updated = pressKey(updated, "bad name")

	updated = pressKey(updated, "enter")

	if updated.(Model).screen != screenCreate || updated.(Model).status == "" {
		t.Fatalf("expected to stay on the form with an error, got screen %d status %q", updated.(Model).screen, updated.(Model).status)
	}
}

func TestEscapeReturnsFromCreateForm(t *testing.T) {
	model := testModel(t)
	updated := pressKey(model, "n")

	updated = pressKey(updated, "esc")

	if updated.(Model).screen != screenAgents {
		t.Fatalf("got screen %d, want agents", updated.(Model).screen)
	}
}

func TestRenderResultsShowsExitCodeAndStderr(t *testing.T) {
	results := []agent.CommandResult{{
		AgentName: "alpha",
		Response:  sshx.CommandResponse{Stdout: "hello\n", Stderr: "warning\n", ReturnCode: 2},
	}}

	rendered := renderResults(results)

	for _, expected := range []string{"alpha", "exit 2", "hello", "warning"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered output lacks %q:\n%s", expected, rendered)
		}
	}
}

func TestConfirmNoReturnsToPreviousScreen(t *testing.T) {
	model := testModel(t)
	called := false
	confirming := model.openConfirm("Sure?", screenAgents, func(model Model) (Model, tea.Cmd) {
		called = true
		return model, nil
	})

	updated := pressKey(confirming, "n")

	if updated.(Model).screen != screenAgents || called {
		t.Fatal("expected to return without running the action")
	}
}

func TestConfirmYesRunsAction(t *testing.T) {
	model := testModel(t)
	called := false
	confirming := model.openConfirm("Sure?", screenAgents, func(model Model) (Model, tea.Cmd) {
		called = true
		return model, nil
	})

	pressKey(confirming, "y")

	if !called {
		t.Fatal("expected the action to run")
	}
}

func TestCreatingReturnsToAgentsOnceEveryInstanceLaunched(t *testing.T) {
	model := testModel(t)
	model.screen = screenCreating
	model.creating = creatingState{
		order:  []string{"web-1", "web-2"},
		steps:  map[string]agent.Step{},
		errors: map[string]error{},
		active: true,
		cancel: func() {},
	}

	updated, _ := model.Update(createProgressMsg{name: "web-1", step: agent.StepWaitRunning})
	stillCreating := updated.(Model).screen
	updated, _ = updated.Update(createProgressMsg{name: "web-2", step: agent.StepWaitSsh})

	if stillCreating != screenCreating || updated.(Model).screen != screenAgents {
		t.Fatalf("got screens %d then %d, want creating then agents", stillCreating, updated.(Model).screen)
	}
	if label, building := updated.(Model).creating.buildingState("web-1"); !building || label != "starting" {
		t.Fatalf("got %q %v, want starting", label, building)
	}
}

func TestBuildingStateBeforeLaunchIsBuilding(t *testing.T) {
	state := creatingState{active: true, steps: map[string]agent.Step{"web-1": agent.StepImportKey}, errors: map[string]error{}}

	label, building := state.buildingState("web-1")

	if !building || label != "building" {
		t.Fatalf("got %q %v", label, building)
	}
}
