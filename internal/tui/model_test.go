package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/ec2"
	"github.com/androsh7/cloud_command/pkg/history"
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

	opened.(Model).shells["alpha"].pending = 1
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

func TestShellTabFocusesOneHost(t *testing.T) {
	model := testModel(t)
	targets := []*agent.Agent{
		{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}},
		{Record: agent.Record{Name: "bravo", PublicIp: "203.0.113.6"}},
	}
	opened, _ := model.openShell(targets, screenAgents)
	updated, _ := opened.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Text: "from-alpha"}})
	updated, _ = updated.Update(shellEventMsg{agentName: "bravo", event: sshx.ShellEvent{Text: "from-bravo"}})

	focused, _ := updated.Update(tea.KeyMsg{Type: tea.KeyTab})

	view := focused.View()
	if !strings.Contains(view, "from-alpha") || strings.Contains(view, "from-bravo") || !strings.Contains(view, "alpha only") {
		t.Fatalf("focused view wrong:\n%s", view)
	}
	if len(focused.(Model).focusedTargets()) != 1 {
		t.Fatal("input should target the focused host only")
	}
}

func TestShellStreamFilterHidesStdout(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)
	updated, _ := opened.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Text: "out-line", Stream: sshx.StreamStdout}})
	updated, _ = updated.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Text: "err-line", Stream: sshx.StreamStderr}})

	stdoutOnly, _ := updated.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	stderrOnly, _ := stdoutOnly.Update(tea.KeyMsg{Type: tea.KeyCtrlO})

	if strings.Contains(stdoutOnly.View(), "err-line") || !strings.Contains(stdoutOnly.View(), "out-line") {
		t.Fatalf("stdout filter wrong:\n%s", stdoutOnly.View())
	}
	if strings.Contains(stderrOnly.View(), "out-line") || !strings.Contains(stderrOnly.View(), "err-line") {
		t.Fatalf("stderr filter wrong:\n%s", stderrOnly.View())
	}
}

func TestShellFinishedEventAddsExitLine(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)
	opened.(Model).shells["alpha"].pending = 1

	finished, _ := opened.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Finished: true, ExitCode: 2}})

	host := finished.(Model).shells["alpha"]
	if host.pending != 0 || !strings.Contains(finished.View(), "[exit 2]") {
		t.Fatalf("exit line missing or pending wrong (%d):\n%s", host.pending, finished.View())
	}
}

func TestShellIgnoresFinishedWithoutPendingCommand(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)

	finished, _ := opened.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Finished: true, ExitCode: 0}})

	if strings.Contains(finished.View(), "[exit") {
		t.Fatalf("stray exit line shown:\n%s", finished.View())
	}
}

func TestFooterMessagesExpire(t *testing.T) {
	model := testModel(t)
	model.screen = screenCommandRunning
	model.command.cancel = func() {}
	withNotice, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if withNotice.(Model).notice == "" || command == nil {
		t.Fatal("expected a notice and an expiry command")
	}

	cleared, _ := withNotice.Update(clearMessageMsg{generation: withNotice.(Model).messageGeneration})

	if cleared.(Model).notice != "" {
		t.Fatalf("notice still shown: %q", cleared.(Model).notice)
	}
}

func TestStaleClearDoesNotRemoveNewerMessage(t *testing.T) {
	model := testModel(t)
	model.screen = screenCommandRunning
	model.command.cancel = func() {}
	first, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	staleGeneration := first.(Model).messageGeneration
	second, _ := first.Update(errorMsg{err: errors.New("newer problem")})

	cleared, _ := second.Update(clearMessageMsg{generation: staleGeneration})

	if cleared.(Model).status != "newer problem" {
		t.Fatalf("newer message was cleared: %q", cleared.(Model).status)
	}
}

func TestSendingShellLineClearsNotice(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)
	opened.(Model).shells["alpha"].pending = 1
	noticed, _ := opened.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	typed := pressKey(noticed, "echo hi")

	sent := pressKey(typed, "enter")

	if noticed.(Model).notice == "" || sent.(Model).notice != "" {
		t.Fatalf("notice before %q after %q", noticed.(Model).notice, sent.(Model).notice)
	}
}

func TestAgentsLoadedSchedulesNextStateCheck(t *testing.T) {
	model := testModel(t)

	updated, command := model.Update(agentsLoadedMsg{agents: []*agent.Agent{}, infos: map[string]ec2.InstanceInfo{}})

	if command == nil || updated.(Model).pollGeneration == 0 {
		t.Fatal("expected a scheduled state check")
	}
}

func TestAgentsPollStartsRefreshOnAgentsScreen(t *testing.T) {
	model := testModel(t)
	loaded, _ := model.Update(agentsLoadedMsg{agents: []*agent.Agent{}, infos: map[string]ec2.InstanceInfo{}})

	polled, _ := loaded.Update(agentsPollMsg{generation: loaded.(Model).pollGeneration})

	if !polled.(Model).agents.refreshing || !strings.Contains(polled.View(), "Cloud Command DEVELOPMENT ") {
		t.Fatalf("expected a refresh with the title spinner, refreshing=%v", polled.(Model).agents.refreshing)
	}
}

func TestStaleAgentsPollIsIgnored(t *testing.T) {
	model := testModel(t)
	loaded, _ := model.Update(agentsLoadedMsg{agents: []*agent.Agent{}, infos: map[string]ec2.InstanceInfo{}})

	polled, _ := loaded.Update(agentsPollMsg{generation: loaded.(Model).pollGeneration - 1})

	if polled.(Model).agents.refreshing {
		t.Fatal("stale poll must not refresh")
	}
}

func TestTitleShowsVersion(t *testing.T) {
	model := testModel(t)

	view := model.View()

	if !strings.Contains(view, "Cloud Command DEVELOPMENT") {
		t.Fatalf("title lacks version:\n%s", view)
	}
}

func TestHistoryScreenListsRecordedEntries(t *testing.T) {
	model := testModel(t)
	model.deps.History = history.NewStore(filepath.Join(t.TempDir(), history.FileName))
	_, _ = model.deps.History.Record(history.Entry{Agent: "alpha", Kind: history.KindCommand, Source: history.SourceCli, Script: "uname -a"})
	opened, _ := model.openHistory()
	loaded, _ := opened.Update(opened.(Model).loadHistory()())

	view := loaded.View()

	if loaded.(Model).screen != screenHistory || !strings.Contains(view, "uname -a") || !strings.Contains(view, "alpha") {
		t.Fatalf("history not shown:\n%s", view)
	}
}

func TestHistoryDetailShowsFullScript(t *testing.T) {
	model := testModel(t)
	model.deps.History = history.NewStore(filepath.Join(t.TempDir(), history.FileName))
	_, _ = model.deps.History.Record(history.Entry{Agent: "alpha", Kind: history.KindJob, Source: history.SourceMcp, Script: "line one\nline two is long", JobId: "job-1"})
	opened, _ := model.openHistory()
	loaded, _ := opened.Update(opened.(Model).loadHistory()())

	detail := pressKey(loaded, "enter")

	view := detail.View()
	if detail.(Model).screen != screenHistoryDetail || !strings.Contains(view, "line two is long") || !strings.Contains(view, "job-1") {
		t.Fatalf("detail missing full script:\n%s", view)
	}
}

func TestShellLineIsRecordedWithExitCode(t *testing.T) {
	model := testModel(t)
	model.deps.History = history.NewStore(filepath.Join(t.TempDir(), history.FileName))
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)
	host := opened.(Model).shells["alpha"]
	host.pending = 1
	host.pendingIds = []string{model.recordHistory(history.Entry{Agent: "alpha", Kind: history.KindCommand, Source: history.SourceTui, Script: "false"})}

	finished, _ := opened.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Finished: true, ExitCode: 1}})

	entries, _ := finished.(Model).deps.History.List()
	if len(entries) != 1 || entries[0].ExitCode == nil || *entries[0].ExitCode != 1 {
		t.Fatalf("exit code not recorded: %+v", entries)
	}
}

func TestShellClosingDropsUnfinishedHistoryIds(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)
	host := opened.(Model).shells["alpha"]
	host.pending = 1
	host.pendingIds = []string{"entry-for-exit"}

	closed, _ := opened.Update(shellEventMsg{agentName: "alpha", event: sshx.ShellEvent{Closed: true}})

	remaining := closed.(Model).shells["alpha"]
	if remaining.pending != 0 || len(remaining.pendingIds) != 0 {
		t.Fatalf("pending=%d pendingIds=%v, want none", remaining.pending, remaining.pendingIds)
	}
}

func TestShellEventBatchShowsEveryLine(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	opened, _ := model.openShell([]*agent.Agent{target}, screenAgents)

	updated, _ := opened.Update(shellEventsMsg{events: []shellEventMsg{
		{agentName: "alpha", event: sshx.ShellEvent{Text: "first-line"}},
		{agentName: "alpha", event: sshx.ShellEvent{Text: "second-line"}},
	}})

	view := updated.View()
	if !strings.Contains(view, "first-line") || !strings.Contains(view, "second-line") {
		t.Fatalf("batched lines missing:\n%s", view)
	}
}

func TestJobDetailShowsFullScript(t *testing.T) {
	model := testModel(t)
	target := &agent.Agent{Record: agent.Record{Name: "alpha", PublicIp: "203.0.113.5"}}
	model.detail.agent = target
	model.jobDetail.job = agent.Job{Id: "job-1", Script: "first line\nsecond line of the script"}
	model.screen = screenJobDetail

	view := model.View()

	if !strings.Contains(view, "second line of the script") {
		t.Fatalf("job detail truncates the script:\n%s", view)
	}
}
