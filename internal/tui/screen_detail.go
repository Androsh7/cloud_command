package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/pkg/agent"
)

// detailState holds the agent shown on the detail screen.
type detailState struct {
	agent *agent.Agent
}

// openDetail switches to the detail screen for one agent.
func (model Model) openDetail(target *agent.Agent) Model {
	model.detail.agent = target
	model.screen = screenDetail
	model.setStatus(nil)
	return model
}

// updateDetailKey handles a key press on the detail screen.
func (model Model) updateDetailKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	target := model.detail.agent
	if target == nil {
		model.screen = screenAgents
		return model, nil
	}
	switch message.String() {
	case "esc", "q":
		model.screen = screenAgents
		return model, nil
	case "c":
		if err := model.checkReady([]*agent.Agent{target}); err != nil {
			model.setStatus(err)
			return model, nil
		}
		return model.openShell([]*agent.Agent{target}, screenDetail)
	case "J":
		if err := model.checkReady([]*agent.Agent{target}); err != nil {
			model.setStatus(err)
			return model, nil
		}
		return model.openJobEditor(target, screenDetail), nil
	case "j":
		return model.openJobs(target)
	case "r":
		model.agents.refreshing = true
		return model, tea.Batch(model.spinner.Tick, model.refreshAgents())
	case "t":
		return model.openConfirm(
			fmt.Sprintf("Terminate agent %q (instance %s)?", target.Name(), target.Record.InstanceId),
			screenDetail,
			func(model Model) (Model, tea.Cmd) {
				model.closeShell(target.Name())
				model.agents.busy = "terminating " + target.Name()
				model.screen = screenAgents
				return model, tea.Batch(model.spinner.Tick, destroyAgent(model.ctx, target))
			},
		), nil
	}
	return model, nil
}

// viewDetail renders one agent's record.
func (model Model) viewDetail() string {
	target := model.detail.agent
	if target == nil {
		return ""
	}
	record := target.Record
	info := model.agents.infos[target.Name()]
	rows := [][2]string{
		{"Name", record.Name},
		{"State", info.State},
		{"Region", record.Region},
		{"Instance", record.InstanceId},
		{"Type", record.InstanceType},
		{"Architecture", string(record.Architecture)},
		{"AMI", record.AmiId},
		{"Public IP", record.PublicIp},
		{"SSH", fmt.Sprintf("%s@%s:%d", record.SshUser, record.PublicIp, record.SshPort)},
		{"Key pair", record.KeyPairName},
		{"Private key", target.PrivateKeyPath()},
		{"Security group", record.SecurityGroupId},
		{"Subnet", record.SubnetId},
		{"Created", record.CreatedAt.Local().Format("2006-01-02 15:04:05")},
		{"Jobs", fmt.Sprintf("%d", len(record.Jobs))},
	}
	var builder strings.Builder
	builder.WriteString(headerStyle.Render("Agent " + record.Name))
	builder.WriteString("\n\n")
	for _, row := range rows {
		builder.WriteString(labelStyle.Render(fmt.Sprintf("  %-16s", row[0])))
		builder.WriteString(row[1])
		builder.WriteString("\n")
	}
	if model.agents.refreshing {
		builder.WriteString("\n" + model.spinner.View() + " Refreshing...\n")
	}
	return builder.String()
}
