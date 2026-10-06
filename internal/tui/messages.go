package tui

import (
	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/ec2"
	"github.com/androsh7/cloud_command/pkg/sshx"
	"github.com/androsh7/cloud_command/pkg/systemd"
)

// agentsLoadedMsg carries the locally stored agents and their current AWS state.
type agentsLoadedMsg struct {
	agents []*agent.Agent
	infos  map[string]ec2.InstanceInfo
	err    error
}

// detachedLoadedMsg carries tagged instances found in AWS without a local record.
type detachedLoadedMsg struct {
	detached []ec2.DetachedAgent
	err      error
}

// createProgressMsg reports one creation step for one agent.
type createProgressMsg struct {
	name string
	step agent.Step
}

// createFinishedMsg carries the outcome of a creation run.
type createFinishedMsg struct {
	results []agent.BatchResult
}

// shellOpenedMsg reports that an agent's shell connected, or failed to.
type shellOpenedMsg struct {
	agentName string
	shell     *sshx.Shell
	err       error
}

// shellEventMsg carries one streamed event from an agent's shell.
type shellEventMsg struct {
	agentName string
	event     sshx.ShellEvent
}

// jobStartedMsg reports a newly started persistent job.
type jobStartedMsg struct {
	agentName string
	job       agent.Job
	err       error
}

// jobStatusesMsg carries the summaries of every job on one agent.
type jobStatusesMsg struct {
	agentName string
	summaries map[string]string
	err       error
}

// jobDetailMsg carries the status and output of one job.
type jobDetailMsg struct {
	generation int
	status     systemd.UnitStatus
	stdout     string
	stderr     string
	err        error
}

// jobPollMsg asks the job detail screen to refresh.
type jobPollMsg struct {
	generation int
}

// jobActionMsg reports the result of stopping or removing a job.
type jobActionMsg struct {
	err error
}

// terminateFinishedMsg reports the result of destroying an agent or detached instance.
type terminateFinishedMsg struct {
	name string
	err  error
}

// clearMessageMsg asks the footer to drop its status and notice when nothing newer replaced them.
type clearMessageMsg struct {
	generation int
}

// errorMsg carries an error to show in the footer.
type errorMsg struct {
	err error
}
