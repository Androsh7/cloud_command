package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/ec2"
)

// agentsState holds the agent list, its cursor and the multi-selection.
type agentsState struct {
	agents     []*agent.Agent
	infos      map[string]ec2.InstanceInfo
	detached   []ec2.DetachedAgent
	cursor     int
	selected   map[string]bool
	refreshing bool
	scanning   bool
	busy       string
}

// newAgentsState returns an empty list that is about to refresh.
func newAgentsState() agentsState {
	return agentsState{
		infos:      map[string]ec2.InstanceInfo{},
		selected:   map[string]bool{},
		refreshing: true,
		scanning:   true,
	}
}

// visibleDetached returns the detached instances that have no local record.
func (state agentsState) visibleDetached() []ec2.DetachedAgent {
	managed := map[string]bool{}
	for _, storedAgent := range state.agents {
		managed[storedAgent.Record.InstanceId] = true
	}
	visible := []ec2.DetachedAgent{}
	for _, detached := range state.detached {
		if !managed[detached.Id] {
			visible = append(visible, detached)
		}
	}
	return visible
}

// rowCount returns the number of selectable rows: managed agents followed by detached instances.
func (state agentsState) rowCount() int {
	return len(state.agents) + len(state.visibleDetached())
}

// cursorAgent returns the managed agent under the cursor, or nil when a detached row is selected.
func (state agentsState) cursorAgent() *agent.Agent {
	if state.cursor < len(state.agents) {
		return state.agents[state.cursor]
	}
	return nil
}

// cursorDetached returns the detached instance under the cursor, or false when a managed row is selected.
func (state agentsState) cursorDetached() (ec2.DetachedAgent, bool) {
	index := state.cursor - len(state.agents)
	visible := state.visibleDetached()
	if index >= 0 && index < len(visible) {
		return visible[index], true
	}
	return ec2.DetachedAgent{}, false
}

// targets returns the selected agents, or the agent under the cursor when nothing is selected.
func (state agentsState) targets() []*agent.Agent {
	targets := []*agent.Agent{}
	for _, storedAgent := range state.agents {
		if state.selected[storedAgent.Name()] {
			targets = append(targets, storedAgent)
		}
	}
	if len(targets) == 0 {
		if cursorAgent := state.cursorAgent(); cursorAgent != nil {
			targets = append(targets, cursorAgent)
		}
	}
	return targets
}

// clampCursor keeps the cursor inside the list.
func (state *agentsState) clampCursor() {
	if state.cursor >= state.rowCount() {
		state.cursor = max(state.rowCount()-1, 0)
	}
}

// updateAgentsKey handles a key press on the agents screen.
func (model Model) updateAgentsKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	state := &model.agents
	switch message.String() {
	case "q":
		return model, tea.Quit
	case "up", "k":
		if state.cursor > 0 {
			state.cursor--
		}
	case "down", "j":
		if state.cursor < state.rowCount()-1 {
			state.cursor++
		}
	case "r":
		if state.refreshing || state.busy != "" {
			return model, nil
		}
		state.refreshing = true
		state.scanning = true
		model.setStatus(nil)
		return model, tea.Batch(model.spinner.Tick, model.refreshAgents(), model.scanDetached())
	case " ":
		if cursorAgent := state.cursorAgent(); cursorAgent != nil {
			state.selected[cursorAgent.Name()] = !state.selected[cursorAgent.Name()]
		}
	case "a":
		allSelected := len(state.agents) > 0
		for _, storedAgent := range state.agents {
			if !state.selected[storedAgent.Name()] {
				allSelected = false
			}
		}
		for _, storedAgent := range state.agents {
			state.selected[storedAgent.Name()] = !allSelected
		}
	case "n":
		return model.openCreate(false), nil
	case "b":
		return model.openCreate(true), nil
	case "enter":
		if cursorAgent := state.cursorAgent(); cursorAgent != nil {
			return model.openDetail(cursorAgent), nil
		}
	case "c":
		targets := state.targets()
		if len(targets) == 0 {
			return model, nil
		}
		if err := model.checkReady(targets); err != nil {
			model.setStatus(err)
			return model, nil
		}
		return model.openShell(targets, screenAgents)
	case "t":
		if state.busy != "" {
			return model, nil
		}
		if targets := state.targets(); len(targets) > 0 {
			prompt := fmt.Sprintf("Terminate agent %q (instance %s)?", targets[0].Name(), targets[0].Record.InstanceId)
			if len(targets) > 1 {
				prompt = fmt.Sprintf("Terminate %d agents (%s)?", len(targets), targetNames(targets))
			}
			return model.openConfirm(prompt, screenAgents, func(model Model) (Model, tea.Cmd) {
				for _, target := range targets {
					model.closeShell(target.Name())
					delete(model.agents.selected, target.Name())
				}
				model.agents.busy = fmt.Sprintf("terminating %d agent(s)", len(targets))
				return model, tea.Batch(model.spinner.Tick, destroyAgents(model.ctx, targets))
			}), nil
		}
		if detached, found := state.cursorDetached(); found {
			return model.openConfirm(
				fmt.Sprintf("Terminate detached instance %s in %s?", detached.Id, detached.Region),
				screenAgents,
				func(model Model) (Model, tea.Cmd) {
					model.agents.busy = "terminating " + detached.Id
					return model, tea.Batch(model.spinner.Tick, terminateDetached(model.ctx, detached))
				},
			), nil
		}
	}
	return model, nil
}

// applyAgentsLoaded stores a finished refresh.
func (model Model) applyAgentsLoaded(message agentsLoadedMsg) (tea.Model, tea.Cmd) {
	model.agents.refreshing = false
	if message.err != nil {
		model.setStatus(message.err)
		return model, nil
	}
	model.agents.agents = message.agents
	model.agents.infos = message.infos
	known := map[string]bool{}
	for _, storedAgent := range message.agents {
		known[storedAgent.Name()] = true
	}
	for name := range model.agents.selected {
		if !known[name] {
			delete(model.agents.selected, name)
		}
	}
	model.agents.clampCursor()
	if model.detail.agent != nil {
		for _, storedAgent := range message.agents {
			if storedAgent.Name() == model.detail.agent.Name() {
				model.detail.agent = storedAgent
			}
		}
	}
	return model, nil
}

// applyDetachedLoaded stores a finished region scan, hiding instances that have a local record.
func (model Model) applyDetachedLoaded(message detachedLoadedMsg) (tea.Model, tea.Cmd) {
	model.agents.scanning = false
	if message.err != nil {
		model.setStatus(fmt.Errorf("scanning regions: %w", message.err))
		return model, nil
	}
	model.agents.detached = message.detached
	model.agents.clampCursor()
	return model, nil
}

// applyTerminateFinished refreshes the list after a termination.
func (model Model) applyTerminateFinished(message terminateFinishedMsg) (tea.Model, tea.Cmd) {
	model.agents.busy = ""
	model.setStatus(message.err)
	if model.screen == screenDetail || model.screen == screenJobs || model.screen == screenJobDetail {
		model.screen = screenAgents
		model.detail.agent = nil
	}
	model.agents.refreshing = true
	model.agents.scanning = true
	return model, tea.Batch(model.spinner.Tick, model.refreshAgents(), model.scanDetached())
}

// viewAgents renders the agent list.
func (model Model) viewAgents() string {
	state := model.agents
	var builder strings.Builder
	builder.WriteString(headerStyle.Render("Agents"))
	builder.WriteString("\n")
	if state.busy != "" {
		builder.WriteString(model.spinner.View() + " " + state.busy + "...\n")
	}

	if len(state.agents) == 0 && !state.refreshing {
		builder.WriteString(dimStyle.Render("  No agents yet. Press n to create one or b to create a batch."))
		builder.WriteString("\n")
	}
	if len(state.agents) > 0 {
		builder.WriteString(dimStyle.Render(fmt.Sprintf("      %-20s %-12s %-14s %-12s %-16s %-20s %-5s %s", "NAME", "STATE", "REGION", "TYPE", "PUBLIC IP", "INSTANCE", "JOBS", "SHELL")))
		builder.WriteString("\n")
	}
	for index, storedAgent := range state.agents {
		info := state.infos[storedAgent.Name()]
		if buildingState, building := model.creating.buildingState(storedAgent.Name()); building {
			info.State = buildingState
		} else if storedAgent.Record.InstanceId == "" {
			info.State = "incomplete"
		}
		mark := "[ ]"
		if state.selected[storedAgent.Name()] {
			mark = "[x]"
		}
		line := fmt.Sprintf("%s %-20s %-12s %-14s %-12s %-16s %-20s %-5d %s",
			mark, storedAgent.Name(), info.State, storedAgent.Record.Region, storedAgent.Record.InstanceType,
			storedAgent.Record.PublicIp, storedAgent.Record.InstanceId, len(storedAgent.Record.Jobs),
			model.shellIndicator(storedAgent.Name()))
		builder.WriteString(model.renderRow(line, index == state.cursor))
	}

	unmanaged := state.visibleDetached()
	if len(unmanaged) > 0 {
		builder.WriteString("\n")
		builder.WriteString(headerStyle.Render("Detached instances (tagged, no local record)"))
		builder.WriteString("\n")
	}
	if len(unmanaged) > 0 {
		builder.WriteString(dimStyle.Render(fmt.Sprintf("      %-22s %-14s %-16s %s", "INSTANCE", "STATE", "REGION", "KEY PAIR")))
		builder.WriteString("\n")
	}
	for index, detached := range unmanaged {
		line := fmt.Sprintf("    %-22s %-14s %-16s %s", detached.Id, detached.State, detached.Region, detached.KeyPair)
		builder.WriteString(model.renderRow(line, len(state.agents)+index == state.cursor))
	}
	return builder.String()
}

// renderRow renders one list row with a cursor marker.
func (model Model) renderRow(line string, selected bool) string {
	if selected {
		return selectedStyle.Render("> "+line) + "\n"
	}
	return "  " + line + "\n"
}
