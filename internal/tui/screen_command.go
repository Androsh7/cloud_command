package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/sshx"
)

const scriptCharacterLimit = 0

// commandState holds the script editor used to start persistent jobs.
type commandState struct {
	textarea textarea.Model
	sudo     bool
	target   *agent.Agent
	returnTo screen
	cancel   context.CancelFunc
}

// openJobEditor switches to the script editor for one agent.
func (model Model) openJobEditor(target *agent.Agent, returnTo screen) Model {
	editor := model.command.textarea
	editor.Reset()
	editor.Placeholder = "sleep 60; echo done"
	editor.CharLimit = scriptCharacterLimit
	editor.ShowLineNumbers = true
	editor.Focus()
	model.command = commandState{textarea: editor, target: target, returnTo: returnTo}
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
		return model.submitJob()
	}
	var command tea.Cmd
	state.textarea, command = state.textarea.Update(message)
	return model, command
}

// submitJob starts the script as a persistent job.
func (model Model) submitJob() (tea.Model, tea.Cmd) {
	state := model.command
	script := strings.TrimSpace(state.textarea.Value())
	if script == "" {
		model.setStatus(fmt.Errorf("the script is empty"))
		return model, nil
	}
	command := sshx.Command{Script: script, Sudo: state.sudo}
	runCtx, cancel := context.WithCancel(model.ctx)
	model.command.cancel = cancel
	model.screen = screenCommandRunning
	model.setStatus(nil)
	return model, tea.Batch(model.spinner.Tick, startJobOn(runCtx, state.target, command, model.deps.History))
}

// updateCommandRunningKey handles a key press while a job is being started.
func (model Model) updateCommandRunningKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.String() == "ctrl+x" && model.command.cancel != nil {
		model.command.cancel()
		model.notice = "cancelling..."
	}
	return model, nil
}

// finishRun releases the run context once the result has arrived.
func (model *Model) finishRun() {
	if model.command.cancel != nil {
		model.command.cancel()
		model.command.cancel = nil
	}
	model.notice = ""
}

// applyJobStarted returns to the jobs screen after a job starts.
func (model Model) applyJobStarted(message jobStartedMsg) (tea.Model, tea.Cmd) {
	model.finishRun()
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

// viewCommand renders the script editor.
func (model Model) viewCommand() string {
	state := model.command
	var builder strings.Builder
	builder.WriteString(headerStyle.Render("Start persistent job on " + state.target.Name()))
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

// viewCommandRunning renders the spinner while a job starts.
func (model Model) viewCommandRunning() string {
	return model.spinner.View() + " Starting job on " + model.command.target.Name() + "... (ctrl+x to cancel)\n"
}
