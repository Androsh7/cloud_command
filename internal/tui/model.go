// Package tui defines the terminal user interface for browsing cloud command agents.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/androsh7/cloud_command/pkg/ec2"
)

const unlimitedSearchThreads = 0

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
)

// agentsLoadedMsg carries the agents found by a search.
type agentsLoadedMsg struct {
	agents []ec2.DetachedAgent
}

// searchFailedMsg carries the error from a failed search.
type searchFailedMsg struct {
	err error
}

// Model holds the state of the agent browser.
type Model struct {
	ctx       context.Context
	nameTag   string
	spinner   spinner.Model
	agents    []ec2.DetachedAgent
	cursor    int
	searching bool
	err       error
}

// New returns a model that searches for agents carrying the given Type tag.
func New(ctx context.Context, nameTag string) Model {
	loadingSpinner := spinner.New()
	loadingSpinner.Spinner = spinner.Dot
	loadingSpinner.Style = titleStyle
	return Model{
		ctx:       ctx,
		nameTag:   nameTag,
		spinner:   loadingSpinner,
		searching: true,
	}
}

// Init starts the spinner and the first search.
func (model Model) Init() tea.Cmd {
	return tea.Batch(model.spinner.Tick, model.searchAgents())
}

// Update applies one message to the model.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyMsg:
		return model.handleKey(message)
	case agentsLoadedMsg:
		model.agents = message.agents
		model.err = nil
		model.searching = false
		if model.cursor >= len(model.agents) {
			model.cursor = max(len(model.agents)-1, 0)
		}
		return model, nil
	case searchFailedMsg:
		model.err = message.err
		model.searching = false
		return model, nil
	case spinner.TickMsg:
		var command tea.Cmd
		model.spinner, command = model.spinner.Update(message)
		return model, command
	}
	return model, nil
}

// View renders the agent list.
func (model Model) View() string {
	var builder strings.Builder
	builder.WriteString(titleStyle.Render("Cloud Command Agents"))
	builder.WriteString("\n\n")

	switch {
	case model.searching:
		builder.WriteString(model.spinner.View())
		builder.WriteString(" Searching every region for agents tagged ")
		builder.WriteString(model.nameTag)
		builder.WriteString("...\n")
	case model.err != nil:
		builder.WriteString(errorStyle.Render("Search failed: " + model.err.Error()))
		builder.WriteString("\n")
	case len(model.agents) == 0:
		builder.WriteString(dimStyle.Render("No agents found."))
		builder.WriteString("\n")
	default:
		for index, agent := range model.agents {
			line := fmt.Sprintf("%-22s %-16s %s", agent.Id, agent.Region, agent.KeyPair)
			if index == model.cursor {
				builder.WriteString(selectedStyle.Render("> " + line))
			} else {
				builder.WriteString("  ")
				builder.WriteString(line)
			}
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n")
	builder.WriteString(dimStyle.Render("up/down: move  r: refresh  q: quit"))
	builder.WriteString("\n")
	return builder.String()
}

// handleKey applies one key press to the model.
func (model Model) handleKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "q", "ctrl+c", "esc":
		return model, tea.Quit
	case "r":
		if model.searching {
			return model, nil
		}
		model.searching = true
		model.err = nil
		return model, tea.Batch(model.spinner.Tick, model.searchAgents())
	case "up", "k":
		if model.cursor > 0 {
			model.cursor--
		}
	case "down", "j":
		if model.cursor < len(model.agents)-1 {
			model.cursor++
		}
	}
	return model, nil
}

// searchAgents returns a command that searches every region for agents.
func (model Model) searchAgents() tea.Cmd {
	ctx := model.ctx
	nameTag := model.nameTag
	return func() tea.Msg {
		agents, err := ec2.ListAllEc2s(ctx, nameTag, unlimitedSearchThreads)
		if err != nil {
			return searchFailedMsg{err: err}
		}
		return agentsLoadedMsg{agents: agents}
	}
}

// Run starts the TUI and blocks until the user quits.
func Run(ctx context.Context, nameTag string) error {
	program := tea.NewProgram(New(ctx, nameTag), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("running tui: %w", err)
	}
	return nil
}
