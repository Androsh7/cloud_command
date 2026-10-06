// Package tui defines the terminal user interface for managing cloud command agents.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/pkg/agent"
)

// Dependencies holds everything the interface needs from the rest of the application.
type Dependencies struct {
	Settings config.Settings
	Store    *agent.Store
	Paths    config.Paths
}

// screen identifies which view is active.
type screen int

const (
	screenAgents screen = iota
	screenCreate
	screenCreating
	screenDetail
	screenCommand
	screenCommandRunning
	screenShell
	screenJobs
	screenJobDetail
	screenConfirm
)

const (
	defaultWidth    = 100
	defaultHeight   = 30
	chromeHeight    = 6
	messageLifetime = 8 * time.Second
)

// expireMessageAfter clears the footer messages once the lifetime passes, unless a newer message replaced them.
func expireMessageAfter(lifetime time.Duration, generation int) tea.Cmd {
	return tea.Tick(lifetime, func(time.Time) tea.Msg { return clearMessageMsg{generation: generation} })
}

// Model holds the state of every screen.
type Model struct {
	ctx               context.Context
	deps              Dependencies
	screen            screen
	width             int
	height            int
	spinner           spinner.Model
	status            string
	notice            string
	messageGeneration int
	pollGeneration    int
	agents            agentsState
	create            createState
	creating          creatingState
	detail            detailState
	command           commandState
	shell             shellState
	shells            map[string]*hostShell
	shellEvents       chan tea.Msg
	jobs              jobsState
	jobDetail         jobDetailState
	confirm           confirmState
}

// New returns a model that starts on the agents screen.
func New(ctx context.Context, deps Dependencies) Model {
	loadingSpinner := spinner.New()
	loadingSpinner.Spinner = spinner.Dot
	loadingSpinner.Style = titleStyle
	return Model{
		ctx:     ctx,
		deps:    deps,
		screen:  screenAgents,
		width:   defaultWidth,
		height:  defaultHeight,
		spinner: loadingSpinner,
		agents:  newAgentsState(),
		command: commandState{textarea: textarea.New()},
		shell: shellState{
			input:    newShellInput(),
			viewport: viewport.New(defaultWidth, defaultHeight-chromeHeight),
		},
		shells:      map[string]*hostShell{},
		shellEvents: make(chan tea.Msg, shellEventBuffer),
		jobDetail: jobDetailState{
			viewport: viewport.New(defaultWidth, defaultHeight-chromeHeight),
		},
	}
}

// Init starts the spinner and the first agent refresh.
func (model Model) Init() tea.Cmd {
	model.agents.refreshing = true
	return tea.Batch(model.spinner.Tick, model.refreshAgents(), model.scanDetached(), waitFor(model.shellEvents))
}

// Update applies one message to the model.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	previousStatus, previousNotice := model.status, model.notice
	updated, command := model.update(message)
	next := updated.(Model)
	statusChanged := next.status != "" && next.status != previousStatus
	noticeChanged := next.notice != "" && next.notice != previousNotice
	if statusChanged || noticeChanged {
		next.messageGeneration++
		command = tea.Batch(command, expireMessageAfter(messageLifetime, next.messageGeneration))
	}
	return next, command
}

// update applies one message to the model without the footer message bookkeeping.
func (model Model) update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case clearMessageMsg:
		if message.generation == model.messageGeneration {
			model.status = ""
			model.notice = ""
		}
		return model, nil
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		model.resizeComponents()
		return model, nil
	case spinner.TickMsg:
		var command tea.Cmd
		model.spinner, command = model.spinner.Update(message)
		return model, command
	case tea.KeyMsg:
		if message.String() == "ctrl+c" {
			model.closeAllShells()
			return model, tea.Quit
		}
		return model.handleKey(message)
	}
	return model.handleData(message)
}

// handleKey routes a key press to the active screen.
func (model Model) handleKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch model.screen {
	case screenAgents:
		return model.updateAgentsKey(message)
	case screenCreate:
		return model.updateCreateKey(message)
	case screenCreating:
		return model.updateCreatingKey(message)
	case screenDetail:
		return model.updateDetailKey(message)
	case screenCommand:
		return model.updateCommandKey(message)
	case screenCommandRunning:
		return model.updateCommandRunningKey(message)
	case screenShell:
		return model.updateShellKey(message)
	case screenJobs:
		return model.updateJobsKey(message)
	case screenJobDetail:
		return model.updateJobDetailKey(message)
	case screenConfirm:
		return model.updateConfirmKey(message)
	}
	return model, nil
}

// handleData applies a background result regardless of the active screen.
func (model Model) handleData(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case agentsLoadedMsg:
		return model.applyAgentsLoaded(message)
	case agentsPollMsg:
		return model.applyAgentsPoll(message)
	case detachedLoadedMsg:
		return model.applyDetachedLoaded(message)
	case createProgressMsg:
		return model.applyCreateProgress(message)
	case createFinishedMsg:
		return model.applyCreateFinished(message)
	case shellOpenedMsg:
		return model.applyShellOpened(message)
	case shellEventMsg:
		return model.applyShellEvent(message)
	case jobStartedMsg:
		return model.applyJobStarted(message)
	case jobStatusesMsg:
		return model.applyJobStatuses(message)
	case jobDetailMsg:
		return model.applyJobDetail(message)
	case jobPollMsg:
		return model.applyJobPoll(message)
	case jobActionMsg:
		return model.applyJobAction(message)
	case terminateFinishedMsg:
		return model.applyTerminateFinished(message)
	case errorMsg:
		model.status = message.err.Error()
		return model, nil
	}
	return model, nil
}

// View renders the active screen with a shared title and footer.
func (model Model) View() string {
	var builder strings.Builder
	builder.WriteString(titleStyle.Render("Cloud Command"))
	if model.isRefreshing() {
		builder.WriteString(" " + model.spinner.View())
	}
	builder.WriteString("\n\n")
	switch model.screen {
	case screenAgents:
		builder.WriteString(model.viewAgents())
	case screenCreate:
		builder.WriteString(model.viewCreate())
	case screenCreating:
		builder.WriteString(model.viewCreating())
	case screenDetail:
		builder.WriteString(model.viewDetail())
	case screenCommand:
		builder.WriteString(model.viewCommand())
	case screenCommandRunning:
		builder.WriteString(model.viewCommandRunning())
	case screenShell:
		builder.WriteString(model.viewShell())
	case screenJobs:
		builder.WriteString(model.viewJobs())
	case screenJobDetail:
		builder.WriteString(model.viewJobDetail())
	case screenConfirm:
		builder.WriteString(model.viewConfirm())
	}
	builder.WriteString("\n")
	if model.status != "" {
		builder.WriteString(errorStyle.Render(model.status))
		builder.WriteString("\n")
	}
	if model.notice != "" {
		builder.WriteString(successStyle.Render(model.notice))
		builder.WriteString("\n")
	}
	builder.WriteString(dimStyle.Render(model.helpLine()))
	builder.WriteString("\n")
	return builder.String()
}

// helpLine returns the key hints for the active screen.
func (model Model) helpLine() string {
	switch model.screen {
	case screenAgents:
		return "enter: detail  n: new  b: batch  space: select  a: all  c: shell on selection  t: terminate selection  r: refresh  q: quit"
	case screenCreate:
		return "tab/shift+tab: field  enter: create  esc: back"
	case screenCreating:
		if model.creating.finished {
			return "enter/esc: back to agents"
		}
		return "esc: cancel  enter: back to agents (creation continues)"
	case screenDetail:
		return "c: shell  J: start job  j: jobs  t: terminate  r: refresh  esc: back"
	case screenCommand:
		return "ctrl+r: start job  ctrl+s: toggle sudo  esc: back"
	case screenCommandRunning:
		return "ctrl+x: cancel"
	case screenShell:
		return "enter: send  up/down: history  pgup/pgdn: scroll  ctrl+x: interrupt  tab: all/one host  ctrl+o: streams  ctrl+l: clear  ctrl+s: save  esc: back  ctrl+q: close shells and back"
	case screenJobs:
		return "enter: detail  s: stop  d: remove  r: refresh  esc: back"
	case screenJobDetail:
		return "o: toggle stdout/stderr  s: stop  r: refresh  esc: back"
	case screenConfirm:
		return "y: confirm  n/esc: cancel"
	}
	return ""
}

// isRefreshing reports whether any background refresh is in flight, which the title line shows as a spinner.
func (model Model) isRefreshing() bool {
	return model.agents.refreshing || model.agents.scanning || model.jobs.loading || model.jobDetail.loading
}

// contentHeight returns the rows available to a scrolling component.
func (model Model) contentHeight() int {
	return max(model.height-chromeHeight, 5)
}

// resizeComponents adjusts every sized component to the terminal.
func (model *Model) resizeComponents() {
	model.command.textarea.SetWidth(max(model.width-4, 20))
	model.command.textarea.SetHeight(max(model.contentHeight()-6, 3))
	model.layoutShell()
	model.jobDetail.viewport.Width = model.width
	model.jobDetail.viewport.Height = max(model.contentHeight()-6, 3)
}

// setStatus records an error message for the footer, or clears it when err is nil.
func (model *Model) setStatus(err error) {
	if err == nil {
		model.status = ""
		return
	}
	model.status = err.Error()
}

// Run starts the interface and blocks until the user quits.
func Run(ctx context.Context, deps Dependencies) error {
	program := tea.NewProgram(New(ctx, deps), tea.WithAltScreen(), tea.WithContext(ctx))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("running tui: %w", err)
	}
	return nil
}
