package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/systemd"
)

const (
	scriptPreviewLength = 40
	timestampLayout     = "2006-01-02 15:04:05"
)

// jobsState holds the job list of the detail agent.
type jobsState struct {
	cursor    int
	summaries map[string]string
	loading   bool
	busy      bool
}

// jobDetailState holds one job's live status and output.
type jobDetailState struct {
	job        agent.Job
	status     systemd.UnitStatus
	stdout     string
	stderr     string
	showStderr bool
	viewport   viewport.Model
	generation int
	loading    bool
}

// openJobs switches to the jobs list and loads each job's status.
func (model Model) openJobs(target *agent.Agent) (tea.Model, tea.Cmd) {
	model.detail.agent = target
	model.jobs = jobsState{summaries: map[string]string{}, loading: len(target.Record.Jobs) > 0}
	model.screen = screenJobs
	model.setStatus(nil)
	if !model.jobs.loading {
		return model, nil
	}
	return model, tea.Batch(model.spinner.Tick, loadJobStatuses(model.ctx, target))
}

// cursorJob returns the job under the cursor, or false when there is none.
func (model Model) cursorJob() (agent.Job, bool) {
	jobs := model.detail.agent.Record.Jobs
	if model.jobs.cursor < len(jobs) {
		return jobs[model.jobs.cursor], true
	}
	return agent.Job{}, false
}

// updateJobsKey handles a key press on the jobs list.
func (model Model) updateJobsKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	target := model.detail.agent
	if target == nil {
		model.screen = screenAgents
		return model, nil
	}
	jobs := target.Record.Jobs
	switch message.String() {
	case "esc", "q":
		model.screen = screenDetail
		return model, nil
	case "up", "k":
		if model.jobs.cursor > 0 {
			model.jobs.cursor--
		}
	case "down", "j":
		if model.jobs.cursor < len(jobs)-1 {
			model.jobs.cursor++
		}
	case "r":
		if model.jobs.busy || len(jobs) == 0 {
			return model, nil
		}
		model.jobs.loading = true
		return model, tea.Batch(model.spinner.Tick, loadJobStatuses(model.ctx, target))
	case "enter":
		if job, found := model.cursorJob(); found {
			return model.openJobDetail(job)
		}
	case "s":
		if job, found := model.cursorJob(); found && !model.jobs.busy {
			return model.openConfirm(
				fmt.Sprintf("Stop job %s on %q?", shortId(job.Id), target.Name()),
				screenJobs,
				func(model Model) (Model, tea.Cmd) {
					model.jobs.busy = true
					return model, tea.Batch(model.spinner.Tick, stopJobOn(model.ctx, target, job))
				},
			), nil
		}
	case "d":
		if job, found := model.cursorJob(); found && !model.jobs.busy {
			return model.openConfirm(
				fmt.Sprintf("Stop and remove job %s on %q, deleting its output files?", shortId(job.Id), target.Name()),
				screenJobs,
				func(model Model) (Model, tea.Cmd) {
					model.jobs.busy = true
					return model, tea.Batch(model.spinner.Tick, removeJobOn(model.ctx, target, job))
				},
			), nil
		}
	}
	return model, nil
}

// applyJobStatuses stores the loaded summaries.
func (model Model) applyJobStatuses(message jobStatusesMsg) (tea.Model, tea.Cmd) {
	model.jobs.loading = false
	if message.err != nil {
		model.setStatus(message.err)
		return model, nil
	}
	if model.detail.agent != nil && model.detail.agent.Name() == message.agentName {
		model.jobs.summaries = message.summaries
	}
	return model, nil
}

// applyJobAction refreshes the jobs list after a stop or removal.
func (model Model) applyJobAction(message jobActionMsg) (tea.Model, tea.Cmd) {
	model.jobs.busy = false
	model.setStatus(message.err)
	if model.detail.agent == nil {
		model.screen = screenAgents
		return model, nil
	}
	if model.jobs.cursor >= len(model.detail.agent.Record.Jobs) {
		model.jobs.cursor = max(len(model.detail.agent.Record.Jobs)-1, 0)
	}
	if model.screen == screenJobDetail {
		return model.refreshJobDetail()
	}
	if len(model.detail.agent.Record.Jobs) == 0 {
		return model, nil
	}
	model.jobs.loading = true
	return model, tea.Batch(model.spinner.Tick, loadJobStatuses(model.ctx, model.detail.agent))
}

// shortId returns the first block of a UUID.
func shortId(id string) string {
	if index := strings.Index(id, "-"); index > 0 {
		return id[:index]
	}
	return id
}

// previewScript returns the first line of a script, shortened for a list row.
func previewScript(script string) string {
	firstLine, _, _ := strings.Cut(script, "\n")
	if len(firstLine) > scriptPreviewLength {
		return firstLine[:scriptPreviewLength-3] + "..."
	}
	return firstLine
}

// viewJobs renders the jobs list.
func (model Model) viewJobs() string {
	target := model.detail.agent
	if target == nil {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(headerStyle.Render("Jobs on " + target.Name()))
	builder.WriteString("\n")
	switch {
	case model.jobs.busy:
		builder.WriteString(model.spinner.View() + " Working...\n")
	case model.jobs.loading:
		builder.WriteString(model.spinner.View() + " Loading job status...\n")
	}
	if len(target.Record.Jobs) == 0 {
		builder.WriteString(dimStyle.Render("  No jobs. Press esc then J to start one."))
		builder.WriteString("\n")
		return builder.String()
	}
	builder.WriteString(dimStyle.Render(fmt.Sprintf("  %-10s %-14s %-20s %-5s %s", "JOB", "STATUS", "STARTED", "SUDO", "SCRIPT")))
	builder.WriteString("\n")
	for index, job := range target.Record.Jobs {
		summary := model.jobs.summaries[job.Id]
		if summary == "" {
			summary = "..."
		}
		sudo := "no"
		if job.Sudo {
			sudo = "yes"
		}
		line := fmt.Sprintf("%-10s %-14s %-20s %-5s %s",
			shortId(job.Id), summary, job.StartedAt.Local().Format(timestampLayout), sudo, previewScript(job.Script))
		builder.WriteString(model.renderRow(line, index == model.jobs.cursor))
	}
	return builder.String()
}

// openJobDetail switches to the job detail screen and starts polling.
func (model Model) openJobDetail(job agent.Job) (tea.Model, tea.Cmd) {
	model.jobDetail = jobDetailState{
		job:        job,
		viewport:   viewport.New(model.width, max(model.contentHeight()-6, 3)),
		generation: model.jobDetail.generation + 1,
		loading:    true,
	}
	model.screen = screenJobDetail
	model.setStatus(nil)
	return model, tea.Batch(model.spinner.Tick, loadJobDetail(model.ctx, model.detail.agent, job, model.jobDetail.generation))
}

// refreshJobDetail reloads the current job immediately.
func (model Model) refreshJobDetail() (tea.Model, tea.Cmd) {
	model.jobDetail.generation++
	model.jobDetail.loading = true
	return model, tea.Batch(model.spinner.Tick, loadJobDetail(model.ctx, model.detail.agent, model.jobDetail.job, model.jobDetail.generation))
}

// updateJobDetailKey handles a key press on the job detail screen.
func (model Model) updateJobDetailKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	target := model.detail.agent
	switch message.String() {
	case "esc", "q":
		model.jobDetail.generation++
		return model.openJobs(target)
	case "o":
		model.jobDetail.showStderr = !model.jobDetail.showStderr
		model.jobDetail.viewport.SetContent(model.jobDetail.visibleOutput())
		return model, nil
	case "r":
		return model.refreshJobDetail()
	case "s":
		job := model.jobDetail.job
		return model.openConfirm(
			fmt.Sprintf("Stop job %s on %q?", shortId(job.Id), target.Name()),
			screenJobDetail,
			func(model Model) (Model, tea.Cmd) {
				model.jobs.busy = true
				return model, tea.Batch(model.spinner.Tick, stopJobOn(model.ctx, target, job))
			},
		), nil
	}
	var command tea.Cmd
	model.jobDetail.viewport, command = model.jobDetail.viewport.Update(message)
	return model, command
}

// visibleOutput returns whichever stream is being shown.
func (state jobDetailState) visibleOutput() string {
	if state.showStderr {
		return state.stderr
	}
	return state.stdout
}

// applyJobDetail stores a loaded status and schedules the next poll while the job runs.
func (model Model) applyJobDetail(message jobDetailMsg) (tea.Model, tea.Cmd) {
	if message.generation != model.jobDetail.generation || model.screen != screenJobDetail {
		return model, nil
	}
	model.jobDetail.loading = false
	if message.err != nil {
		model.setStatus(message.err)
		return model, nil
	}
	atBottom := model.jobDetail.viewport.AtBottom()
	model.jobDetail.status = message.status
	model.jobDetail.stdout = message.stdout
	model.jobDetail.stderr = message.stderr
	model.jobDetail.viewport.SetContent(model.jobDetail.visibleOutput())
	if atBottom {
		model.jobDetail.viewport.GotoBottom()
	}
	if message.status.IsRunning() {
		return model, pollJobAfter(model.deps.Settings.PollInterval, model.jobDetail.generation)
	}
	return model, nil
}

// applyJobPoll reloads the job when the poll is still current.
func (model Model) applyJobPoll(message jobPollMsg) (tea.Model, tea.Cmd) {
	if message.generation != model.jobDetail.generation || model.screen != screenJobDetail {
		return model, nil
	}
	return model, loadJobDetail(model.ctx, model.detail.agent, model.jobDetail.job, model.jobDetail.generation)
}

// viewJobDetail renders one job's status and output.
func (model Model) viewJobDetail() string {
	state := model.jobDetail
	var builder strings.Builder
	builder.WriteString(headerStyle.Render(fmt.Sprintf("Job %s on %s", shortId(state.job.Id), model.detail.agent.Name())))
	builder.WriteString("\n")
	statusText := state.status.Summary()
	if state.loading {
		statusText = model.spinner.View() + " " + statusText
	}
	builder.WriteString(labelStyle.Render("  Status   ") + statusText + "\n")
	builder.WriteString(labelStyle.Render("  Started  ") + state.job.StartedAt.Local().Format(timestampLayout) + "\n")
	builder.WriteString(labelStyle.Render("  Script   ") + previewScript(state.job.Script) + "\n")
	stream := "stdout"
	if state.showStderr {
		stream = "stderr"
	}
	builder.WriteString(dimStyle.Render(fmt.Sprintf("-- %s (%s) --", stream, state.job.StdoutPath)))
	builder.WriteString("\n")
	builder.WriteString(state.viewport.View())
	builder.WriteString("\n")
	return builder.String()
}
