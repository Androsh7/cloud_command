// Package tui defines the terminal user interface for managing cloud command agents.
package tui

import (
	"context"
	"fmt"
	"strings"

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
	screenOutput
	screenJobs
	screenJobDetail
	screenConfirm
)

const (
	defaultWidth  = 100
	defaultHeight = 30
	chromeHeight  = 6
)

// Model holds the state of every screen.
type Model struct {
	ctx       context.Context
	deps      Dependencies
	screen    screen
	width     int
	height    int
	spinner   spinner.Model
	status    string
	agents    agentsState
	create    createState
	creating  creatingState
	detail    detailState
	command   commandState
	output    outputState
	jobs      jobsState
	jobDetail jobDetailState
	confirm   confirmState
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
		output:  outputState{viewport: viewport.New(defaultWidth, defaultHeight-chromeHeight)},
		jobDetail: jobDetailState{
			viewport: viewport.New(defaultWidth, defaultHeight-chromeHeight),
		},
	}
}

// Init starts the spinner and the first agent refresh.
func (model Model) Init() tea.Cmd {
	model.agents.refreshing = true
	return tea.Batch(model.spinner.Tick, model.refreshAgents(), model.scanDetached())
}

// Update applies one message to the model.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
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
		return model, nil
	case screenOutput:
		return model.updateOutputKey(message)
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
	case detachedLoadedMsg:
		return model.applyDetachedLoaded(message)
	case createProgressMsg:
		return model.applyCreateProgress(message)
	case createFinishedMsg:
		return model.applyCreateFinished(message)
	case commandFinishedMsg:
		return model.applyCommandFinished(message)
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
	case screenOutput:
		builder.WriteString(model.viewOutput())
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
	builder.WriteString(dimStyle.Render(model.helpLine()))
	builder.WriteString("\n")
	return builder.String()
}

// helpLine returns the key hints for the active screen.
func (model Model) helpLine() string {
	switch model.screen {
	case screenAgents:
		return "enter: detail  n: new  b: batch  space: select  a: all  c: command  t: terminate  r: refresh  q: quit"
	case screenCreate:
		return "tab/shift+tab: field  enter: create  esc: back"
	case screenCreating:
		if model.creating.finished {
			return "enter/esc: back to agents"
		}
		return "esc: cancel  enter: back to agents (creation continues)"
	case screenDetail:
		return "c: command  J: start job  j: jobs  t: terminate  r: refresh  esc: back"
	case screenCommand:
		return "ctrl+r: run  ctrl+s: toggle sudo  esc: back"
	case screenCommandRunning:
		return "running..."
	case screenOutput:
		return "up/down/pgup/pgdn: scroll  esc: back"
	case screenJobs:
		return "enter: detail  s: stop  d: remove  r: refresh  esc: back"
	case screenJobDetail:
		return "o: toggle stdout/stderr  s: stop  r: refresh  esc: back"
	case screenConfirm:
		return "y: confirm  n/esc: cancel"
	}
	return ""
}

// contentHeight returns the rows available to a scrolling component.
func (model Model) contentHeight() int {
	return max(model.height-chromeHeight, 5)
}

// resizeComponents adjusts every sized component to the terminal.
func (model *Model) resizeComponents() {
	model.command.textarea.SetWidth(max(model.width-4, 20))
	model.command.textarea.SetHeight(max(model.contentHeight()-6, 3))
	model.output.viewport.Width = model.width
	model.output.viewport.Height = model.contentHeight() - 2
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
