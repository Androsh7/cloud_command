// Package control exposes the agent operations as plain, JSON-friendly calls shared by the CLI and the MCP server.
package control

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/ec2"
	"github.com/androsh7/cloud_command/pkg/sshx"
)

const (
	DefaultCommandTimeout = 2 * time.Minute
	DefaultOutputLines    = 200
	MaxBatchCount         = 50
	stateUnknown          = "unknown"
	stateIncomplete       = "incomplete"
)

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrJobNotFound    = agent.ErrJobNotFound
)

// AgentSummary describes one managed agent for listings.
type AgentSummary struct {
	Name         string    `json:"name"`
	Region       string    `json:"region"`
	InstanceType string    `json:"instance_type"`
	Architecture string    `json:"architecture"`
	InstanceId   string    `json:"instance_id"`
	State        string    `json:"state"`
	PublicIp     string    `json:"public_ip"`
	JobCount     int       `json:"job_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// DetachedSummary describes a tagged instance that has no local record.
type DetachedSummary struct {
	InstanceId string `json:"instance_id"`
	Region     string `json:"region"`
	State      string `json:"state"`
	KeyPair    string `json:"key_pair"`
}

// JobSummary describes one persistent job and its current state.
type JobSummary struct {
	Id        string    `json:"id"`
	Script    string    `json:"script"`
	Sudo      bool      `json:"sudo"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"`
	ExitCode  int       `json:"exit_code"`
	Running   bool      `json:"running"`
}

// AgentDetail describes one agent together with its jobs.
type AgentDetail struct {
	AgentSummary
	SshCommand string       `json:"ssh_command"`
	Jobs       []JobSummary `json:"jobs"`
}

// CreateRequest describes agents to create. Empty fields fall back to the settings.
type CreateRequest struct {
	Name         string
	Count        int
	Region       string
	InstanceType string
	Architecture string
	Wait         bool
}

// CreateResult is the outcome for one agent of a create request.
type CreateResult struct {
	Name       string `json:"name"`
	InstanceId string `json:"instance_id"`
	PublicIp   string `json:"public_ip"`
	State      string `json:"state"`
	Error      string `json:"error,omitempty"`
}

// TerminateResult is the outcome for one agent of a terminate request.
type TerminateResult struct {
	Name  string `json:"name"`
	Error string `json:"error,omitempty"`
}

// CommandResult is the captured output of one command on one agent.
type CommandResult struct {
	Agent    string `json:"agent"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Error    string `json:"error,omitempty"`
}

// JobOutput is the state and captured output of one job.
type JobOutput struct {
	Agent    string `json:"agent"`
	JobId    string `json:"job_id"`
	Status   string `json:"status"`
	ExitCode int    `json:"exit_code"`
	Running  bool   `json:"running"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// Controller performs agent operations against one store with one set of settings.
type Controller struct {
	Store    *agent.Store
	Settings config.Settings
}

// New returns a controller for a store.
func New(store *agent.Store, settings config.Settings) *Controller {
	return &Controller{Store: store, Settings: settings}
}

// createOptions builds the timeouts from the settings.
func (controller *Controller) createOptions() agent.CreateOptions {
	return agent.CreateOptions{
		SshTimeout:   controller.Settings.SshTimeout,
		ReadyTimeout: controller.Settings.ReadyTimeout,
		PollInterval: controller.Settings.PollInterval,
	}
}

// load returns one stored agent configured with the settings' SSH timeout.
func (controller *Controller) load(name string) (*agent.Agent, error) {
	loaded, err := controller.Store.Load(name)
	if err != nil {
		return nil, err
	}
	loaded.SetSshOptions(agent.SshOptions{Timeout: controller.Settings.SshTimeout})
	return loaded, nil
}

// summarize builds a summary from a record and its current instance state.
func summarize(loaded *agent.Agent, state string) AgentSummary {
	record := loaded.Record
	if record.InstanceId == "" {
		state = stateIncomplete
	}
	return AgentSummary{
		Name:         record.Name,
		Region:       record.Region,
		InstanceType: record.InstanceType,
		Architecture: string(record.Architecture),
		InstanceId:   record.InstanceId,
		State:        state,
		PublicIp:     record.PublicIp,
		JobCount:     len(record.Jobs),
		CreatedAt:    record.CreatedAt,
	}
}

// refreshState reads the instance state, reporting unknown when AWS cannot be reached.
func refreshState(ctx context.Context, loaded *agent.Agent) string {
	if loaded.Record.InstanceId == "" {
		return stateIncomplete
	}
	info, err := loaded.Refresh(ctx)
	if err != nil {
		return stateUnknown
	}
	return info.State
}

// ListAgents returns every stored agent with its current state, and optionally the tagged instances without a record.
func (controller *Controller) ListAgents(ctx context.Context, includeDetached bool) ([]AgentSummary, []DetachedSummary, error) {
	agents, err := controller.Store.List()
	if err != nil {
		return nil, nil, err
	}
	summaries := make([]AgentSummary, len(agents))
	var waitGroup sync.WaitGroup
	for index, loaded := range agents {
		loaded.SetSshOptions(agent.SshOptions{Timeout: controller.Settings.SshTimeout})
		waitGroup.Add(1)
		go func(index int, loaded *agent.Agent) {
			defer waitGroup.Done()
			summaries[index] = summarize(loaded, refreshState(ctx, loaded))
		}(index, loaded)
	}
	waitGroup.Wait()

	detached := []DetachedSummary{}
	if includeDetached {
		known := map[string]bool{}
		for _, loaded := range agents {
			known[loaded.Record.InstanceId] = true
		}
		found, err := ec2.ListAllEc2s(ctx, ec2.AgentTypeTagValue, controller.Settings.RegionSearchThreads)
		if err != nil {
			return summaries, nil, fmt.Errorf("scanning regions: %w", err)
		}
		for _, instance := range found {
			if !known[instance.Id] {
				detached = append(detached, DetachedSummary{InstanceId: instance.Id, Region: instance.Region, State: instance.State, KeyPair: instance.KeyPair})
			}
		}
	}
	return summaries, detached, nil
}

// GetAgent returns one agent with its jobs and their states.
func (controller *Controller) GetAgent(ctx context.Context, name string) (AgentDetail, error) {
	loaded, err := controller.load(name)
	if err != nil {
		return AgentDetail{}, err
	}
	detail := AgentDetail{AgentSummary: summarize(loaded, refreshState(ctx, loaded)), Jobs: []JobSummary{}}
	if loaded.Record.PublicIp != "" {
		detail.SshCommand = fmt.Sprintf("ssh -i %s -p %d %s@%s", loaded.PrivateKeyPath(), loaded.Record.SshPort, loaded.Record.SshUser, loaded.Record.PublicIp)
	}
	for _, job := range loaded.Record.Jobs {
		summary := JobSummary{Id: job.Id, Script: job.Script, Sudo: job.Sudo, StartedAt: job.StartedAt, Status: stateUnknown}
		if status, err := loaded.JobStatus(ctx, job); err == nil {
			summary.Status = status.Summary()
			summary.ExitCode = status.ExitCode
			summary.Running = status.IsRunning()
		}
		detail.Jobs = append(detail.Jobs, summary)
	}
	return detail, nil
}

// resolveSpec fills a request from the settings and validates it.
func (controller *Controller) resolveSpec(request CreateRequest) (agent.Spec, int, error) {
	settings := controller.Settings
	spec := agent.Spec{
		Name:         strings.TrimSpace(request.Name),
		Region:       firstNonEmpty(request.Region, settings.Region),
		InstanceType: firstNonEmpty(request.InstanceType, settings.InstanceType),
		Architecture: ec2.Architecture(firstNonEmpty(request.Architecture, string(settings.Architecture))),
		SshUser:      settings.SshUser,
		SshPort:      settings.SshPort,
	}
	if err := agent.ValidateName(spec.Name); err != nil {
		return agent.Spec{}, 0, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if spec.Architecture != ec2.ArchitectureX86_64 && spec.Architecture != ec2.ArchitectureArm64 {
		return agent.Spec{}, 0, fmt.Errorf("%w: architecture must be %s or %s", ErrInvalidRequest, ec2.ArchitectureX86_64, ec2.ArchitectureArm64)
	}
	count := request.Count
	if count == 0 {
		count = 1
	}
	if count < 1 || count > MaxBatchCount {
		return agent.Spec{}, 0, fmt.Errorf("%w: count must be between 1 and %d", ErrInvalidRequest, MaxBatchCount)
	}
	return spec, count, nil
}

// firstNonEmpty returns the first string that is not blank.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// CreateAgents creates one agent or a numbered batch, waiting for readiness unless asked not to.
func (controller *Controller) CreateAgents(ctx context.Context, request CreateRequest) ([]CreateResult, error) {
	spec, count, err := controller.resolveSpec(request)
	if err != nil {
		return nil, err
	}
	names := []string{spec.Name}
	if count > 1 {
		names = agent.BatchNames(spec.Name, count)
	}
	options := controller.createOptions()
	results := make([]CreateResult, len(names))
	var waitGroup sync.WaitGroup
	for index, name := range names {
		waitGroup.Add(1)
		go func(index int, name string) {
			defer waitGroup.Done()
			agentSpec := spec
			agentSpec.Name = name
			var created *agent.Agent
			var createErr error
			if request.Wait {
				created, createErr = agent.Create(ctx, controller.Store, agentSpec, options, nil)
			} else {
				created, createErr = agent.Launch(ctx, controller.Store, agentSpec, options, nil)
			}
			results[index] = describeCreate(ctx, name, created, createErr)
		}(index, name)
	}
	waitGroup.Wait()
	return results, nil
}

// describeCreate turns a creation outcome into a result row.
func describeCreate(ctx context.Context, name string, created *agent.Agent, err error) CreateResult {
	result := CreateResult{Name: name}
	if err != nil {
		result.Error = err.Error()
	}
	if created != nil {
		result.InstanceId = created.Record.InstanceId
		result.PublicIp = created.Record.PublicIp
		result.State = refreshState(ctx, created)
		result.PublicIp = created.Record.PublicIp
	}
	return result
}

// TerminateAgents destroys several agents at once.
func (controller *Controller) TerminateAgents(ctx context.Context, names []string) []TerminateResult {
	results := make([]TerminateResult, len(names))
	var waitGroup sync.WaitGroup
	for index, name := range names {
		waitGroup.Add(1)
		go func(index int, name string) {
			defer waitGroup.Done()
			results[index] = TerminateResult{Name: name}
			loaded, err := controller.load(name)
			if err == nil {
				err = loaded.Destroy(ctx)
			}
			if err != nil {
				results[index].Error = err.Error()
			}
		}(index, name)
	}
	waitGroup.Wait()
	return results
}

// RunCommand runs one script on several agents at once and returns the captured output of each.
func (controller *Controller) RunCommand(ctx context.Context, names []string, script string, sudo bool, timeout time.Duration) ([]CommandResult, error) {
	if strings.TrimSpace(script) == "" {
		return nil, fmt.Errorf("%w: script must not be empty", ErrInvalidRequest)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%w: at least one agent is required", ErrInvalidRequest)
	}
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}
	command := sshx.Command{Script: script, Sudo: sudo}
	results := make([]CommandResult, len(names))
	var waitGroup sync.WaitGroup
	for index, name := range names {
		waitGroup.Add(1)
		go func(index int, name string) {
			defer waitGroup.Done()
			results[index] = CommandResult{Agent: name}
			loaded, err := controller.load(name)
			if err != nil {
				results[index].Error = err.Error()
				return
			}
			runCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			response, err := loaded.RunCommand(runCtx, command)
			_ = loaded.Disconnect()
			if err != nil {
				results[index].Error = err.Error()
				return
			}
			results[index].ExitCode = response.ReturnCode
			results[index].Stdout = response.Stdout
			results[index].Stderr = response.Stderr
		}(index, name)
	}
	waitGroup.Wait()
	return results, nil
}

// StartJob starts a persistent job on one agent.
func (controller *Controller) StartJob(ctx context.Context, name string, script string, sudo bool) (JobSummary, error) {
	if strings.TrimSpace(script) == "" {
		return JobSummary{}, fmt.Errorf("%w: script must not be empty", ErrInvalidRequest)
	}
	loaded, err := controller.load(name)
	if err != nil {
		return JobSummary{}, err
	}
	defer loaded.Disconnect()
	job, err := loaded.StartJob(ctx, sshx.Command{Script: script, Sudo: sudo})
	if err != nil {
		return JobSummary{}, err
	}
	return JobSummary{Id: job.Id, Script: job.Script, Sudo: job.Sudo, StartedAt: job.StartedAt, Status: "running", Running: true}, nil
}

// findJob returns the stored job with an id, accepting the full id or an unambiguous prefix.
func findJob(loaded *agent.Agent, jobId string) (agent.Job, error) {
	matches := []agent.Job{}
	for _, job := range loaded.Record.Jobs {
		if job.Id == jobId {
			return job, nil
		}
		if strings.HasPrefix(job.Id, jobId) {
			matches = append(matches, job)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return agent.Job{}, fmt.Errorf("%w: job id %q is ambiguous", ErrInvalidRequest, jobId)
	}
	return agent.Job{}, fmt.Errorf("%w: %q on agent %q", ErrJobNotFound, jobId, loaded.Name())
}

// JobStatus returns the state of one job with the last lines of its output.
func (controller *Controller) JobStatus(ctx context.Context, name string, jobId string, outputLines int) (JobOutput, error) {
	loaded, err := controller.load(name)
	if err != nil {
		return JobOutput{}, err
	}
	defer loaded.Disconnect()
	job, err := findJob(loaded, jobId)
	if err != nil {
		return JobOutput{}, err
	}
	status, err := loaded.JobStatus(ctx, job)
	if err != nil {
		return JobOutput{}, err
	}
	stdout, stderr, err := loaded.JobOutput(ctx, job)
	if err != nil {
		return JobOutput{}, err
	}
	if outputLines <= 0 {
		outputLines = DefaultOutputLines
	}
	return JobOutput{
		Agent:    name,
		JobId:    job.Id,
		Status:   status.Summary(),
		ExitCode: status.ExitCode,
		Running:  status.IsRunning(),
		Stdout:   TailLines(stdout, outputLines),
		Stderr:   TailLines(stderr, outputLines),
	}, nil
}

// StopJob stops one job, leaving its record and output in place.
func (controller *Controller) StopJob(ctx context.Context, name string, jobId string) error {
	loaded, err := controller.load(name)
	if err != nil {
		return err
	}
	defer loaded.Disconnect()
	job, err := findJob(loaded, jobId)
	if err != nil {
		return err
	}
	return loaded.StopJob(ctx, job)
}

// RemoveJob stops one job, deletes its output and forgets it.
func (controller *Controller) RemoveJob(ctx context.Context, name string, jobId string) error {
	loaded, err := controller.load(name)
	if err != nil {
		return err
	}
	defer loaded.Disconnect()
	job, err := findJob(loaded, jobId)
	if err != nil {
		return err
	}
	return loaded.RemoveJob(ctx, job)
}

// TailLines returns the last count lines of text.
func TailLines(text string, count int) string {
	trimmed := strings.TrimRight(text, "\n")
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, "\n")
}
