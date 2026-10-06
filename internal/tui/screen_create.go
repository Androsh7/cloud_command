package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/ec2"
)

const (
	fieldName = iota
	fieldRegion
	fieldInstanceType
	fieldArchitecture
	fieldCount
	fieldTotal

	maxBatchCount  = 50
	fieldWidth     = 40
	fieldLabelSize = 14
)

var fieldLabels = [fieldTotal]string{"Name", "Region", "Instance type", "Architecture", "Count"}

// createState holds the create agent form.
type createState struct {
	inputs [fieldTotal]textinput.Model
	focus  int
	batch  bool
}

// creatingState tracks a creation run in progress.
type creatingState struct {
	order    []string
	steps    map[string]agent.Step
	errors   map[string]error
	active   bool
	finished bool
	cancel   context.CancelFunc
	channel  <-chan tea.Msg
}

// launched reports whether an agent has passed the launch step, failed, or finished.
func (state creatingState) launched(name string) bool {
	if state.errors[name] != nil {
		return true
	}
	switch state.steps[name] {
	case agent.StepWaitRunning, agent.StepWaitSsh, agent.StepDone:
		return true
	}
	return false
}

// allLaunched reports whether every agent in the run has launched, failed or finished.
func (state creatingState) allLaunched() bool {
	for _, name := range state.order {
		if !state.launched(name) {
			return false
		}
	}
	return true
}

// buildingState returns the state label for an agent that is still being created, or false when it is not.
func (state creatingState) buildingState(name string) (string, bool) {
	if !state.active || state.errors[name] != nil {
		return "", false
	}
	step, found := state.steps[name]
	if !found || step == agent.StepDone {
		return "", false
	}
	if state.launched(name) {
		return "starting", true
	}
	return "building", true
}

// openCreate switches to the create form, prefilled from the settings.
func (model Model) openCreate(batch bool) Model {
	if model.creating.active {
		model.setStatus(fmt.Errorf("a creation is already in progress"))
		return model
	}
	settings := model.deps.Settings
	defaults := [fieldTotal]string{"", settings.Region, settings.InstanceType, string(settings.Architecture), "2"}
	for index := range model.create.inputs {
		input := textinput.New()
		input.Prompt = ""
		input.Width = fieldWidth
		input.SetValue(defaults[index])
		input.Blur()
		model.create.inputs[index] = input
	}
	model.create.inputs[fieldName].Placeholder = "agent name"
	model.create.focus = fieldName
	model.create.inputs[fieldName].Focus()
	model.create.batch = batch
	model.screen = screenCreate
	model.setStatus(nil)
	return model
}

// fieldCount returns how many fields the form shows.
func (state createState) visibleFields() int {
	if state.batch {
		return fieldTotal
	}
	return fieldTotal - 1
}

// updateCreateKey handles a key press on the create form.
func (model Model) updateCreateKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	state := &model.create
	switch message.String() {
	case "esc":
		model.screen = screenAgents
		return model, nil
	case "tab", "down":
		state.inputs[state.focus].Blur()
		state.focus = (state.focus + 1) % state.visibleFields()
		return model, state.inputs[state.focus].Focus()
	case "shift+tab", "up":
		state.inputs[state.focus].Blur()
		state.focus = (state.focus + state.visibleFields() - 1) % state.visibleFields()
		return model, state.inputs[state.focus].Focus()
	case "enter":
		return model.submitCreate()
	}
	var command tea.Cmd
	state.inputs[state.focus], command = state.inputs[state.focus].Update(message)
	return model, command
}

// submitCreate validates the form and starts creation.
func (model Model) submitCreate() (tea.Model, tea.Cmd) {
	state := model.create
	spec := agent.Spec{
		Name:         strings.TrimSpace(state.inputs[fieldName].Value()),
		Region:       strings.TrimSpace(state.inputs[fieldRegion].Value()),
		InstanceType: strings.TrimSpace(state.inputs[fieldInstanceType].Value()),
		Architecture: ec2.Architecture(strings.TrimSpace(state.inputs[fieldArchitecture].Value())),
		SshUser:      model.deps.Settings.SshUser,
		SshPort:      model.deps.Settings.SshPort,
	}
	if err := agent.ValidateName(spec.Name); err != nil {
		model.setStatus(err)
		return model, nil
	}
	if spec.Region == "" || spec.InstanceType == "" {
		model.setStatus(fmt.Errorf("region and instance type are required"))
		return model, nil
	}
	if spec.Architecture != ec2.ArchitectureX86_64 && spec.Architecture != ec2.ArchitectureArm64 {
		model.setStatus(fmt.Errorf("architecture must be %s or %s", ec2.ArchitectureX86_64, ec2.ArchitectureArm64))
		return model, nil
	}
	count := 1
	if state.batch {
		parsedCount, err := strconv.Atoi(strings.TrimSpace(state.inputs[fieldCount].Value()))
		if err != nil || parsedCount < 1 || parsedCount > maxBatchCount {
			model.setStatus(fmt.Errorf("count must be a number between 1 and %d", maxBatchCount))
			return model, nil
		}
		count = parsedCount
	}

	names := []string{spec.Name}
	if count > 1 {
		names = agent.BatchNames(spec.Name, count)
	}
	ctx, cancel := context.WithCancel(model.ctx)
	channel, waitCommand := model.startCreation(ctx, spec, count)
	model.creating = creatingState{
		order:   names,
		steps:   map[string]agent.Step{},
		errors:  map[string]error{},
		active:  true,
		cancel:  cancel,
		channel: channel,
	}
	model.screen = screenCreating
	model.setStatus(nil)
	return model, tea.Batch(model.spinner.Tick, waitCommand)
}

// updateCreatingKey handles a key press while agents are being created.
func (model Model) updateCreatingKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc":
		if model.creating.finished {
			return model.leaveCreating()
		}
		model.creating.cancel()
		model.status = "cancelling; instances and key pairs created so far are being removed..."
		return model, nil
	case "enter":
		return model.leaveCreating()
	}
	return model, nil
}

// leaveCreating returns to the agents screen and refreshes it.
func (model Model) leaveCreating() (tea.Model, tea.Cmd) {
	model.screen = screenAgents
	model.agents.refreshing = true
	model.setStatus(nil)
	return model, tea.Batch(model.spinner.Tick, model.refreshAgents())
}

// applyCreateProgress records one step, returns to the agents screen once every instance has launched, and waits for the next message.
func (model Model) applyCreateProgress(message createProgressMsg) (tea.Model, tea.Cmd) {
	model.creating.steps[message.name] = message.step
	commands := []tea.Cmd{waitFor(model.creating.channel)}
	if message.step == agent.StepWaitRunning {
		model.agents.refreshing = true
		commands = append(commands, model.spinner.Tick, model.refreshAgents())
	}
	if model.screen == screenCreating && model.creating.allLaunched() {
		model.screen = screenAgents
		model.setStatus(nil)
	}
	return model, tea.Batch(commands...)
}

// applyCreateFinished records the outcome of every agent.
func (model Model) applyCreateFinished(message createFinishedMsg) (tea.Model, tea.Cmd) {
	model.creating.finished = true
	model.creating.active = false
	model.creating.cancel()
	model.status = ""
	failures := []string{}
	for _, result := range message.results {
		if result.Err != nil {
			model.creating.errors[result.Name] = result.Err
			failures = append(failures, result.Name+": "+result.Err.Error())
		} else {
			model.creating.steps[result.Name] = agent.StepDone
		}
	}
	if len(failures) > 0 && model.screen != screenCreating {
		model.status = "creation failed for " + strings.Join(failures, "; ")
	}
	model.agents.refreshing = true
	return model, tea.Batch(model.spinner.Tick, model.refreshAgents())
}

// viewCreate renders the create form.
func (model Model) viewCreate() string {
	state := model.create
	var builder strings.Builder
	if state.batch {
		builder.WriteString(headerStyle.Render("Create a batch of agents"))
		builder.WriteString("\n")
		builder.WriteString(dimStyle.Render("Agents are named <name>-1 .. <name>-<count>, each with its own key pair."))
	} else {
		builder.WriteString(headerStyle.Render("Create an agent"))
	}
	builder.WriteString("\n\n")
	for index := 0; index < state.visibleFields(); index++ {
		label := fmt.Sprintf("%-*s", fieldLabelSize, fieldLabels[index])
		if index == state.focus {
			builder.WriteString(selectedStyle.Render("> " + label))
		} else {
			builder.WriteString(labelStyle.Render("  " + label))
		}
		builder.WriteString(state.inputs[index].View())
		builder.WriteString("\n")
	}
	return builder.String()
}

// viewCreating renders one progress row per agent.
func (model Model) viewCreating() string {
	state := model.creating
	var builder strings.Builder
	if state.finished {
		builder.WriteString(headerStyle.Render("Creation finished"))
	} else {
		builder.WriteString(headerStyle.Render("Creating agents"))
	}
	builder.WriteString("\n\n")
	names := append([]string{}, state.order...)
	sort.Strings(names)
	for _, name := range names {
		builder.WriteString(fmt.Sprintf("  %-24s ", name))
		switch {
		case state.errors[name] != nil:
			builder.WriteString(errorStyle.Render("failed: " + state.errors[name].Error()))
		case state.steps[name] == agent.StepDone:
			builder.WriteString(successStyle.Render(string(agent.StepDone)))
		case state.steps[name] == agent.StepCleanup && state.finished:
			builder.WriteString(dimStyle.Render("removed"))
		case state.finished:
			builder.WriteString(dimStyle.Render("not started"))
		default:
			builder.WriteString(model.spinner.View() + " " + string(state.steps[name]))
		}
		builder.WriteString("\n")
	}
	return builder.String()
}
