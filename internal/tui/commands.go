package tui

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/ec2"
	"github.com/androsh7/cloud_command/pkg/sshx"
)

// waitFor returns a command that delivers the next message from a channel, or nothing when it is closed.
func waitFor(channel <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		message, open := <-channel
		if !open {
			return nil
		}
		return message
	}
}

// createOptions builds the timeouts from the settings.
func (model Model) createOptions() agent.CreateOptions {
	return agent.CreateOptions{
		SshTimeout:   model.deps.Settings.SshTimeout,
		ReadyTimeout: model.deps.Settings.ReadyTimeout,
		PollInterval: model.deps.Settings.PollInterval,
	}
}

// refreshAgents loads every stored agent and describes its instance.
func (model Model) refreshAgents() tea.Cmd {
	ctx := model.ctx
	store := model.deps.Store
	sshTimeout := model.deps.Settings.SshTimeout
	return func() tea.Msg {
		agents, err := store.List()
		if err != nil {
			return agentsLoadedMsg{err: err}
		}
		infos := make(map[string]ec2.InstanceInfo, len(agents))
		var infosLock sync.Mutex
		var waitGroup sync.WaitGroup
		for _, storedAgent := range agents {
			storedAgent.SetSshOptions(agent.SshOptions{Timeout: sshTimeout})
			if storedAgent.Record.InstanceId == "" {
				continue
			}
			waitGroup.Add(1)
			go func(storedAgent *agent.Agent) {
				defer waitGroup.Done()
				info, err := storedAgent.Refresh(ctx)
				if err != nil {
					info = ec2.InstanceInfo{Id: storedAgent.Record.InstanceId, State: "unknown"}
				}
				infosLock.Lock()
				infos[storedAgent.Name()] = info
				infosLock.Unlock()
			}(storedAgent)
		}
		waitGroup.Wait()
		return agentsLoadedMsg{agents: agents, infos: infos}
	}
}

// scanDetached searches every region for tagged instances.
func (model Model) scanDetached() tea.Cmd {
	ctx := model.ctx
	threads := model.deps.Settings.RegionSearchThreads
	return func() tea.Msg {
		detached, err := ec2.ListAllEc2s(ctx, ec2.AgentTypeTagValue, threads)
		return detachedLoadedMsg{detached: detached, err: err}
	}
}

// startCreation launches agent creation in the background and streams progress through a channel.
func (model Model) startCreation(ctx context.Context, spec agent.Spec, count int) (<-chan tea.Msg, tea.Cmd) {
	store := model.deps.Store
	options := model.createOptions()
	channel := make(chan tea.Msg, 64)
	reporter := func(name string, step agent.Step) {
		select {
		case channel <- createProgressMsg{name: name, step: step}:
		default:
			slog.Warn("dropped creation progress message", "agent", name, "step", step)
		}
	}
	go func() {
		defer close(channel)
		var results []agent.BatchResult
		if count <= 1 {
			createdAgent, err := agent.Create(ctx, store, spec, options, reporter)
			results = []agent.BatchResult{{Name: spec.Name, Agent: createdAgent, Err: err}}
		} else {
			results = agent.CreateBatch(ctx, store, spec, count, options, reporter)
		}
		channel <- createFinishedMsg{results: results}
	}()
	return channel, waitFor(channel)
}

// runCommandOn runs one command on several agents at once.
func runCommandOn(ctx context.Context, targets []*agent.Agent, command sshx.Command) tea.Cmd {
	return func() tea.Msg {
		return commandFinishedMsg{results: agent.RunOnMany(ctx, targets, command, 0)}
	}
}

// startJobOn starts a persistent job on one agent.
func startJobOn(ctx context.Context, target *agent.Agent, command sshx.Command) tea.Cmd {
	return func() tea.Msg {
		job, err := target.StartJob(ctx, command)
		return jobStartedMsg{agentName: target.Name(), job: job, err: err}
	}
}

// loadJobStatuses fetches a summary for every job on one agent.
func loadJobStatuses(ctx context.Context, target *agent.Agent) tea.Cmd {
	jobs := append([]agent.Job{}, target.Record.Jobs...)
	return func() tea.Msg {
		summaries := make(map[string]string, len(jobs))
		for _, job := range jobs {
			status, err := target.JobStatus(ctx, job)
			if err != nil {
				return jobStatusesMsg{agentName: target.Name(), err: err}
			}
			summaries[job.Id] = status.Summary()
		}
		return jobStatusesMsg{agentName: target.Name(), summaries: summaries}
	}
}

// loadJobDetail fetches the status and output of one job.
func loadJobDetail(ctx context.Context, target *agent.Agent, job agent.Job, generation int) tea.Cmd {
	return func() tea.Msg {
		status, err := target.JobStatus(ctx, job)
		if err != nil {
			return jobDetailMsg{generation: generation, err: err}
		}
		stdout, stderr, err := target.JobOutput(ctx, job)
		return jobDetailMsg{generation: generation, status: status, stdout: stdout, stderr: stderr, err: err}
	}
}

// pollJobAfter schedules the next job detail refresh.
func pollJobAfter(interval time.Duration, generation int) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg { return jobPollMsg{generation: generation} })
}

// stopJobOn stops one job.
func stopJobOn(ctx context.Context, target *agent.Agent, job agent.Job) tea.Cmd {
	return func() tea.Msg { return jobActionMsg{err: target.StopJob(ctx, job)} }
}

// removeJobOn stops one job, cleans up its files and forgets it.
func removeJobOn(ctx context.Context, target *agent.Agent, job agent.Job) tea.Cmd {
	return func() tea.Msg { return jobActionMsg{err: target.RemoveJob(ctx, job)} }
}

// destroyAgent terminates a managed agent and removes its directory.
func destroyAgent(ctx context.Context, target *agent.Agent) tea.Cmd {
	return func() tea.Msg { return terminateFinishedMsg{name: target.Name(), err: target.Destroy(ctx)} }
}

// terminateDetached terminates a tagged instance that has no local record, deleting its key pair when this tool created it.
func terminateDetached(ctx context.Context, detached ec2.DetachedAgent) tea.Cmd {
	return func() tea.Msg {
		if err := ec2.TerminateInstance(ctx, detached.Region, detached.Id); err != nil {
			return terminateFinishedMsg{name: detached.Id, err: err}
		}
		if strings.HasPrefix(detached.KeyPair, agent.KeyPairNamePrefix) {
			if err := ec2.DeleteKeyPair(ctx, detached.Region, detached.KeyPair); err != nil {
				return terminateFinishedMsg{name: detached.Id, err: err}
			}
		}
		return terminateFinishedMsg{name: detached.Id}
	}
}
