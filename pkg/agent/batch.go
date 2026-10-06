package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/androsh7/cloud_command/pkg/sshx"
)

// BatchResult is the outcome of creating one agent in a batch.
type BatchResult struct {
	Name  string
	Agent *Agent
	Err   error
}

// CommandResult is the outcome of running one command on one agent.
type CommandResult struct {
	AgentName string
	Response  sshx.CommandResponse
	Err       error
}

// BatchNames returns "<base>-1" through "<base>-<count>".
func BatchNames(baseName string, count int) []string {
	names := make([]string, 0, max(count, 0))
	for number := 1; number <= count; number++ {
		names = append(names, fmt.Sprintf("%s-%d", baseName, number))
	}
	return names
}

// CreateBatch creates several agents at once from one spec and returns one result per name, in order.
func CreateBatch(ctx context.Context, store *Store, baseSpec Spec, count int, options CreateOptions, reporter Reporter) []BatchResult {
	names := BatchNames(baseSpec.Name, count)
	results := make([]BatchResult, len(names))
	var waitGroup sync.WaitGroup
	for index, name := range names {
		waitGroup.Add(1)
		go func(index int, name string) {
			defer waitGroup.Done()
			spec := baseSpec
			spec.Name = name
			createdAgent, err := Create(ctx, store, spec, options, reporter)
			results[index] = BatchResult{Name: name, Agent: createdAgent, Err: err}
		}(index, name)
	}
	waitGroup.Wait()
	return results
}

// RunOnMany runs one command on every agent at once and returns results in input order.
// concurrency limits how many agents run at the same time; zero means all of them.
func RunOnMany(ctx context.Context, agents []*Agent, command sshx.Command, concurrency int) []CommandResult {
	if concurrency <= 0 {
		concurrency = max(len(agents), 1)
	}
	results := make([]CommandResult, len(agents))
	semaphore := make(chan struct{}, concurrency)
	var waitGroup sync.WaitGroup
	for index, targetAgent := range agents {
		waitGroup.Add(1)
		go func(index int, targetAgent *Agent) {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			response, err := targetAgent.RunCommand(ctx, command)
			results[index] = CommandResult{AgentName: targetAgent.Name(), Response: response, Err: err}
		}(index, targetAgent)
	}
	waitGroup.Wait()
	return results
}
