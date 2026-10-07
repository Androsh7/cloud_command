// Package mcpserver exposes the agent operations to MCP clients over stdio.
package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/androsh7/cloud_command/internal/control"
	"github.com/androsh7/cloud_command/internal/version"
)

const (
	ServerName = "cloud-command"

	instructions = `You manage short-lived AWS EC2 agents for the user through cloud-command.

Rules:
1. Start by listing existing agents and reuse them whenever possible.
2. Keep resources small: when the user has no stated need, use the default instance type (t4g.nano).
3. Never terminate an agent or remove a job without explicit permission from the user.
4. Use run_command for commands that finish within a couple of minutes. Use start_job for anything longer, then poll job_status.
5. Commands run as the SSH user (ec2-user by default); set sudo for root.

t4g instance options (arm64, general purpose):
| Instance | vCPUs | Memory (GiB) |
|---|---|---|
| t4g.nano | 2 | 0.5 |
| t4g.micro | 2 | 1 |
| t4g.small | 2 | 2 |
| t4g.medium | 2 | 4 |
| t4g.large | 2 | 8 |
| t4g.xlarge | 4 | 16 |
| t4g.2xlarge | 8 | 32 |
Use the x86_64 architecture only when the software requires it; t3 instances are the x86_64 counterpart.`
)

// ListAgentsInput selects what list_agents returns.
type ListAgentsInput struct {
	IncludeDetached bool `json:"include_detached,omitempty" jsonschema:"also scan every AWS region for tagged instances that have no local record"`
}

// ListAgentsOutput is the result of list_agents.
type ListAgentsOutput struct {
	Agents   []control.AgentSummary    `json:"agents"`
	Detached []control.DetachedSummary `json:"detached,omitempty"`
}

// AgentNameInput names one agent.
type AgentNameInput struct {
	Name string `json:"name" jsonschema:"the agent name"`
}

// CreateAgentInput describes one agent to create.
type CreateAgentInput struct {
	Name         string `json:"name" jsonschema:"agent name"`
	Region       string `json:"region,omitempty" jsonschema:"AWS region, default from settings"`
	InstanceType string `json:"instance_type,omitempty" jsonschema:"EC2 instance type, default from settings (t4g.nano)"`
	Architecture string `json:"architecture,omitempty" jsonschema:"arm64 or x86_64, default from settings"`
	Wait         *bool  `json:"wait,omitempty" jsonschema:"wait until the agents accept SSH before returning, default true; set false to return right after launch and poll with get_agent"`
}

// RunCommandInput describes a script to run on one agent.
type RunCommandInput struct {
	Agent          string `json:"agent" jsonschema:"name of the agent to run the script on"`
	Script         string `json:"script" jsonschema:"bash script to run"`
	Sudo           bool   `json:"sudo,omitempty" jsonschema:"run as root"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"timeout in seconds, default 120"`
}

// StartJobInput describes a persistent job to start.
type StartJobInput struct {
	Agent  string `json:"agent" jsonschema:"agent name"`
	Script string `json:"script" jsonschema:"bash script to run as a systemd unit"`
	Sudo   bool   `json:"sudo,omitempty" jsonschema:"run as root"`
}

// JobInput names one job on one agent.
type JobInput struct {
	Agent       string `json:"agent" jsonschema:"agent name"`
	JobId       string `json:"job_id" jsonschema:"job id, or an unambiguous prefix of it"`
	OutputLines int    `json:"output_lines,omitempty" jsonschema:"how many trailing lines of stdout and stderr to return, default 200"`
}

// EmptyOutput is returned by tools that have nothing to report beyond success.
type EmptyOutput struct {
	Ok bool `json:"ok"`
}

// New builds the MCP server with every tool registered against the controller.
func New(controller *control.Controller) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: ServerName, Title: "Cloud Command", Version: version.Version},
		&mcp.ServerOptions{Instructions: instructions},
	)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := &mcp.ToolAnnotations{DestructiveHint: boolPointer(true)}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_agents",
		Description: "List every agent with its instance state, public IP and job count.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListAgentsInput) (*mcp.CallToolResult, ListAgentsOutput, error) {
		agents, detached, err := controller.ListAgents(ctx, input.IncludeDetached)
		if err != nil {
			return nil, ListAgentsOutput{}, err
		}
		return nil, ListAgentsOutput{Agents: agents, Detached: detached}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_agent",
		Description: "Describe one agent, its ssh command and the state of each of its jobs.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input AgentNameInput) (*mcp.CallToolResult, control.AgentDetail, error) {
		return mustDetail(controller.GetAgent(ctx, input.Name))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_agent",
		Description: "Create one agent with its own instance and SSH key. Waits until it accepts SSH unless wait is false.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input CreateAgentInput) (*mcp.CallToolResult, control.CreateResult, error) {
		wait := input.Wait == nil || *input.Wait
		results, err := controller.CreateAgents(ctx, control.CreateRequest{
			Name: input.Name, Count: 1, Region: input.Region,
			InstanceType: input.InstanceType, Architecture: input.Architecture, Wait: wait,
		})
		if err != nil {
			return nil, control.CreateResult{}, err
		}
		if results[0].Error != "" {
			return nil, control.CreateResult{}, fmt.Errorf("creating agent %q: %s", input.Name, results[0].Error)
		}
		return nil, results[0], nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "terminate_agent",
		Description: "Terminate one agent: stop its instance, delete its SSH key pair and remove its local record. Ask the user first.",
		Annotations: destructive,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input AgentNameInput) (*mcp.CallToolResult, EmptyOutput, error) {
		result := controller.TerminateAgents(ctx, []string{input.Name})[0]
		if result.Error != "" {
			return nil, EmptyOutput{}, fmt.Errorf("terminating agent %q: %s", input.Name, result.Error)
		}
		return nil, EmptyOutput{Ok: true}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_command",
		Description: "Run a bash script on one agent and return its exit code, stdout and stderr. For long work use start_job.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RunCommandInput) (*mcp.CallToolResult, control.CommandResult, error) {
		results, err := controller.RunCommand(ctx, []string{input.Agent}, input.Script, input.Sudo, time.Duration(input.TimeoutSeconds)*time.Second)
		if err != nil {
			return nil, control.CommandResult{}, err
		}
		if results[0].Error != "" {
			return nil, control.CommandResult{}, fmt.Errorf("running command on %q: %s", input.Agent, results[0].Error)
		}
		return nil, results[0], nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "start_job",
		Description: "Start a script as a persistent systemd job on one agent and return its job id. Poll job_status for progress.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input StartJobInput) (*mcp.CallToolResult, control.JobSummary, error) {
		job, err := controller.StartJob(ctx, input.Agent, input.Script, input.Sudo)
		return nil, job, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "job_status",
		Description: "Return a job's state, exit code and the trailing lines of its stdout and stderr.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input JobInput) (*mcp.CallToolResult, control.JobOutput, error) {
		output, err := controller.JobStatus(ctx, input.Agent, input.JobId, input.OutputLines)
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stop_job",
		Description: "Stop a running job. Its record and captured output stay available.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input JobInput) (*mcp.CallToolResult, EmptyOutput, error) {
		if err := controller.StopJob(ctx, input.Agent, input.JobId); err != nil {
			return nil, EmptyOutput{}, err
		}
		return nil, EmptyOutput{Ok: true}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_job",
		Description: "Stop a job, delete its output files and forget it. Ask the user first.",
		Annotations: destructive,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input JobInput) (*mcp.CallToolResult, EmptyOutput, error) {
		if err := controller.RemoveJob(ctx, input.Agent, input.JobId); err != nil {
			return nil, EmptyOutput{}, err
		}
		return nil, EmptyOutput{Ok: true}, nil
	})

	return server
}

// ServeStdio runs the server on standard input and output until the client disconnects or the context ends.
func ServeStdio(ctx context.Context, server *mcp.Server) error {
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("serving mcp over stdio: %w", err)
	}
	return nil
}

// mustDetail adapts the two-value controller result to the handler's three values.
func mustDetail(detail control.AgentDetail, err error) (*mcp.CallToolResult, control.AgentDetail, error) {
	return nil, detail, err
}

// boolPointer returns a pointer to a bool for optional annotation fields.
func boolPointer(value bool) *bool {
	return &value
}
