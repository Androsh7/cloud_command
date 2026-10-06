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
	editing := updated.(Model).openJobEditor(&agent.Agent{Record: agent.Record{Name: "alpha"}}, screenAgents)

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

func TestControlXCancelsRunningCommand(t *testing.T) {
	model := testModel(t)
	cancelled := false
	model.screen = screenCommandRunning
	model.command.cancel = func() { cancelled = true }

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})

	if !cancelled || updated.(Model).notice == "" {
		t.Fatal("expected the run context to be cancelled and a notice shown")
	}
}

func TestEscapeOnAgentsScreenDoesNotQuit(t *testing.T) {
	model := testModel(t)

	_, command := model.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if command != nil {
		t.Fatal("escape on the agents screen should do nothing")
	}
}

func TestCommandOnBuildingAgentIsRefused(t *testing.T) {
	model := testModel(t)
	building := &agent.Agent{Record: agent.Record{Name: "web-1", InstanceId: "i-1"}}
	model.creating = creatingState{active: true, steps: map[string]agent.Step{"web-1": agent.StepWaitSsh}, errors: map[string]error{}}
	updated, _ := model.Update(agentsLoadedMsg{agents: []*agent.Agent{building}, infos: map[string]ec2.InstanceInfo{}})

	updated = pressKey(updated, "c")

	if updated.(Model).screen != screenAgents || !strings.Contains(updated.(Model).status, "web-1") {
		t.Fatalf("expected to stay on agents with a warning, got screen %d status %q", updated.(Model).screen, updated.(Model).status)
	}
}

func TestShellScreenQueuesLineWhileConnecting(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5", SshUser: "ec2-user", SshPort: 22}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)
	typed := pressKey(opened, "echo hi")

	submitted := pressKey(typed, "enter")

	host := submitted.(Model).shells["alpha"]
	if len(host.queue) != 1 || host.queue[0] != "echo hi" {
		t.Fatalf("line was not queued: %+v", host.queue)
	}
	if !strings.Contains(submitted.View(), "$ echo hi") {
		t.Fatalf("typed line not echoed: %s", submitted.View())
	}
}

func TestShellEventAppendsOutputToView(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)

	updated, _ := opened.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Text: "streamed-line"}})
	finished, _ := updated.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Finished: true, ExitCode: 7}})

	if !strings.Contains(finished.View(), "streamed-line") {
		t.Fatalf("output not shown: %s", finished.View())
	}
	if finished.(Model).shells["alpha"].lastExit != 7 {
		t.Fatalf("exit code not recorded: %+v", finished.(Model).shells["alpha"])
	}
}

func TestShellViewPrefixesHostsWhenSeveral(t *testing.T) {
	model := testModel(t)
	targets := []*agent.Agent{
		{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}},
		{Record: agent.Record{Name: "bravo", PublicIp: "203.0.113.6"}},
	}
	opened, _ := model.openShell(targets, screenAgents)

	updated, _ := opened.Update(shellEventMsg{agentName: "bravo", event: sshx.ShellEvent{Text: "from-bravo"}})

	if !strings.Contains(updated.View(), "[bravo]") {
		t.Fatalf("host prefix missing: %s", updated.View())
	}
}

func TestShellTranscriptSurvivesLeavingScreen(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)
	updated, _ := opened.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Text: "remembered"}})
	left := pressKey(updated, "esc")

	reopened, _ := left.(Model).openShell([]*agent.Agent{target}, screenAgents)

	if left.(Model).screen != screenAgents || !strings.Contains(reopened.View(), "remembered") {
		t.Fatalf("transcript lost: %s", reopened.View())
	}
}

func TestTerminateWithSelectionPromptsForAllSelected(t *testing.T) {
	model := testModel(t)
	agents := []*agent.Agent{
		{Record: agent.Record{Name: "alpha", InstanceId: "i-1"}},
		{Record: agent.Record{Name: "bravo", InstanceId: "i-2"}},
		{Record: agent.Record{Name: "charlie", InstanceId: "i-3"}},
	}
	updated, _ := model.Update(agentsLoadedMsg{agents: agents, infos: map[string]ec2.InstanceInfo{}})
	updated = pressKey(updated, " ")
	updated = pressKey(updated, "j")
	updated = pressKey(updated, " ")

	confirming := pressKey(updated, "t")

	prompt := confirming.(Model).confirm.prompt
	if confirming.(Model).screen != screenConfirm || !strings.Contains(prompt, "2 agents") || strings.Contains(prompt, "charlie") {
		t.Fatalf("unexpected confirm prompt %q on screen %d", prompt, confirming.(Model).screen)
	}
}

func TestControlQClosesShellsAndLeaves(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)

	left, _ := opened.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})

	if left.(Model).screen != screenAgents || left.(Model).shells["alpha"] != nil {
		t.Fatalf("expected shells closed and agents screen, got screen %d shells %v", left.(Model).screen, left.(Model).shells)
	}
}

func TestAgentListShowsShellIndicatorAfterLeaving(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", InstanceId: "i-1", PublicIp: "203.0.113.5"}}
	loaded, _ := model.Update(agentsLoadedMsg{agents: []*agent.Agent{target}, infos: map[string]ec2.InstanceInfo{}})
	opened, _ := loaded.(Model).openShell([]*agent.Agent{target}, screenAgents)

	left := pressKey(opened, "esc")

	if !strings.Contains(left.View(), "connecting") {
		t.Fatalf("agent list lacks shell indicator:\n%s", left.View())
	}
}

func TestRefreshShowsSpinnerInTitleOnly(t *testing.T) {
	model := testModel(t)

	view := model.View()

	if strings.Contains(view, "Refreshing") || strings.Contains(view, "Scanning") {
		t.Fatalf("refresh text should be gone:\n%s", view)
	}
}
