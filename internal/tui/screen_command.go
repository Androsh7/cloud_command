package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/sshx"
)

// commandMode says whether the script runs once or as a persistent job.
type commandMode int

const (
	commandModeRun commandMode = iota
	commandModeJob
)

const scriptCharacterLimit = 0

// commandState holds the script editor.
type commandState struct {
	textarea textarea.Model
	sudo     bool
	mode     commandMode
	targets  []*agent.Agent
	returnTo screen
}

// outputState holds the results of the last command run.
type outputState struct {
	viewport viewport.Model
	results  []agent.CommandResult
	returnTo screen
}

// openCommand switches to the script editor for the given targets.
func (model Model) openCommand(targets []*agent.Agent, mode commandMode, returnTo screen) Model {
	editor := model.command.textarea
	editor.Reset()
	editor.Placeholder = "echo hello from $(hostname)"
	editor.CharLimit = scriptCharacterLimit
	editor.ShowLineNumbers = true
	editor.Focus()
	model.command = commandState{textarea: editor, mode: mode, targets: targets, returnTo: returnTo}
	model.screen = screenCommand
	model.resizeComponents()
	model.setStatus(nil)
	return model
}

// updateCommandKey handles a key press in the script editor.
func (model Model) updateCommandKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	state := &model.command
	switch message.String() {
	case "esc":
		model.screen = state.returnTo
		return model, nil
	case "ctrl+s":
		state.sudo = !state.sudo
		return model, nil
	case "ctrl+r":
		return model.submitCommand()
	}
	var command tea.Cmd
	state.textarea, command = state.textarea.Update(message)
	return model, command
}

// submitCommand runs the script once or starts it as a job.
func (model Model) submitCommand() (tea.Model, tea.Cmd) {
	state := model.command
	script := strings.TrimSpace(state.textarea.Value())
	if script == "" {
		model.setStatus(fmt.Errorf("the script is empty"))
		return model, nil
	}
	command := sshx.Command{Script: script, Sudo: state.sudo}
	model.screen = screenCommandRunning
	model.setStatus(nil)
	if state.mode == commandModeJob {
		return model, tea.Batch(model.spinner.Tick, startJobOn(model.ctx, state.targets[0], command))
	}
	return model, tea.Batch(model.spinner.Tick, runCommandOn(model.ctx, state.targets, command))
}

// applyCommandFinished shows the output screen.
func (model Model) applyCommandFinished(message commandFinishedMsg) (tea.Model, tea.Cmd) {
	model.output = outputState{
		viewport: viewport.New(model.width, model.contentHeight()-2),
		results:  message.results,
		returnTo: model.command.returnTo,
	}
	model.output.viewport.SetContent(renderResults(message.results))
	model.screen = screenOutput
	return model, nil
}

// applyJobStarted returns to the jobs screen after a job starts.
func (model Model) applyJobStarted(message jobStartedMsg) (tea.Model, tea.Cmd) {
	if message.err != nil {
		model.setStatus(message.err)
		model.screen = screenCommand
		return model, nil
	}
	if model.detail.agent == nil || model.detail.agent.Name() != message.agentName {
		model.screen = screenAgents
		return model, nil
	}
	return model.openJobs(model.detail.agent)
}

// renderResults formats one section per agent.
func renderResults(results []agent.CommandResult) string {
	var builder strings.Builder
	for _, result := range results {
		switch {
		case result.Err != nil:
			builder.WriteString(errorStyle.Render(fmt.Sprintf("== %s: error ==", result.AgentName)))
			builder.WriteString("\n" + result.Err.Error() + "\n\n")
		default:
			style := successStyle
			if result.Response.ReturnCode != 0 {
				style = errorStyle
			}
			builder.WriteString(style.Render(fmt.Sprintf("== %s (exit %d) ==", result.AgentName, result.Response.ReturnCode)))
			builder.WriteString("\n")
			builder.WriteString(result.Response.Stdout)
			if !strings.HasSuffix(result.Response.Stdout, "\n") && result.Response.Stdout != "" {
				builder.WriteString("\n")
			}
			if result.Response.Stderr != "" {
				builder.WriteString(dimStyle.Render("-- stderr --"))
				builder.WriteString("\n" + result.Response.Stderr)
				if !strings.HasSuffix(result.Response.Stderr, "\n") {
					builder.WriteString("\n")
				}
			}
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

// updateOutputKey handles a key press on the output screen.
func (model Model) updateOutputKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc", "q", "enter":
		model.screen = model.output.returnTo
		return model, nil
	}
	var command tea.Cmd
	model.output.viewport, command = model.output.viewport.Update(message)
	return model, command
}

// targetNames joins the names of the command targets.
func targetNames(targets []*agent.Agent) string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, target.Name())
	}
	return strings.Join(names, ", ")
}

// viewCommand renders the script editor.
func (model Model) viewCommand() string {
	state := model.command
	var builder strings.Builder
	if state.mode == commandModeJob {
		builder.WriteString(headerStyle.Render("Start persistent job on " + targetNames(state.targets)))
	} else {
		builder.WriteString(headerStyle.Render(fmt.Sprintf("Run command on %d agent(s): %s", len(state.targets), targetNames(state.targets))))
	}
	builder.WriteString("\n")
	sudoLabel := "sudo: off"
	if state.sudo {
		sudoLabel = "sudo: ON"
	}
	builder.WriteString(labelStyle.Render(sudoLabel))
	builder.WriteString("\n\n")
	builder.WriteString(state.textarea.View())
	builder.WriteString("\n")
	return builder.String()
}

// viewCommandRunning renders the spinner while a command runs.
func (model Model) viewCommandRunning() string {
	state := model.command
	if state.mode == commandModeJob {
		return model.spinner.View() + " Starting job on " + targetNames(state.targets) + "...\n"
	}
	return model.spinner.View() + fmt.Sprintf(" Running on %d agent(s)...\n", len(state.targets))
}

// viewOutput renders the scrolling command output.
func (model Model) viewOutput() string {
	var builder strings.Builder
	builder.WriteString(headerStyle.Render(fmt.Sprintf("Output (%d agent(s))", len(model.output.results))))
	builder.WriteString("\n")
	builder.WriteString(model.output.viewport.View())
	builder.WriteString("\n")
	return builder.String()
}
