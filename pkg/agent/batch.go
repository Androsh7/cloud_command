package agent

import (
	"context"
	"fmt"
	"sync"
)

// BatchResult is the outcome of creating one agent in a batch.
type BatchResult struct {
	Name  string
	Agent *Agent
	Err   error
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
