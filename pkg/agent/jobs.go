package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/androsh7/cloud_command/pkg/sshx"
	"github.com/androsh7/cloud_command/pkg/systemd"
)

var ErrJobNotFound = errors.New("job not found")

// jobSpec converts a stored job back into the systemd spec that describes it.
func (managedAgent *Agent) jobSpec(job Job) systemd.JobSpec {
	user := managedAgent.Record.SshUser
	if job.Sudo {
		user = systemd.RootUser
	}
	return systemd.JobSpec{Id: job.Id, Script: job.Script, Executable: systemd.DefaultExecutable, User: user}
}

// runRaw runs an already-built command line, base64-wrapped like every other command so quoting never leaks.
func (managedAgent *Agent) runRaw(ctx context.Context, line string) (sshx.CommandResponse, error) {
	host, err := managedAgent.SshHost()
	if err != nil {
		return sshx.CommandResponse{}, err
	}
	response, err := host.RunCommandContext(ctx, sshx.Command{Script: line}.Line())
	if err != nil {
		return sshx.CommandResponse{}, fmt.Errorf("agent %q: %w", managedAgent.Name(), err)
	}
	return response, nil
}

// StartJob starts a persistent systemd job and records it.
func (managedAgent *Agent) StartJob(ctx context.Context, command sshx.Command) (Job, error) {
	job := Job{
		Id:        uuid.NewString(),
		Script:    command.Script,
		Sudo:      command.Sudo,
		StartedAt: time.Now().UTC(),
	}
	spec := managedAgent.jobSpec(job)
	job.StdoutPath = spec.StdoutPath()
	job.StderrPath = spec.StderrPath()

	response, err := managedAgent.runRaw(ctx, spec.RunCommand())
	if err != nil {
		return Job{}, err
	}
	if response.ReturnCode != 0 {
		return Job{}, fmt.Errorf("starting job on agent %q failed with exit code %d: %s",
			managedAgent.Name(), response.ReturnCode, strings.TrimSpace(response.Stderr))
	}
	err = managedAgent.updateStored(func(record *Record) error {
		record.Jobs = append(record.Jobs, job)
		return nil
	})
	if err != nil {
		return Job{}, err
	}
	slog.Info("started job", "agent", managedAgent.Name(), "job", job.Id, "unit", spec.UnitName())
	return job, nil
}

// JobStatus returns the systemd state of one job.
func (managedAgent *Agent) JobStatus(ctx context.Context, job Job) (systemd.UnitStatus, error) {
	response, err := managedAgent.runRaw(ctx, systemd.ShowCommand(managedAgent.jobSpec(job).UnitName()))
	if err != nil {
		return systemd.UnitStatus{}, err
	}
	status, err := systemd.ParseShow(response.Stdout)
	if err != nil {
		return systemd.UnitStatus{}, fmt.Errorf("job %q on agent %q: %w", job.Id, managedAgent.Name(), err)
	}
	return status, nil
}

// JobOutput returns the last lines of the standard output and standard error captured so far.
func (managedAgent *Agent) JobOutput(ctx context.Context, job Job, lines int) (string, string, error) {
	stdoutResponse, err := managedAgent.runRaw(ctx, systemd.ReadOutputCommand(job.StdoutPath, lines))
	if err != nil {
		return "", "", err
	}
	stderrResponse, err := managedAgent.runRaw(ctx, systemd.ReadOutputCommand(job.StderrPath, lines))
	if err != nil {
		return "", "", err
	}
	return stdoutResponse.Stdout, stderrResponse.Stdout, nil
}

// StopJob stops the unit of one job, leaving its record and output in place.
func (managedAgent *Agent) StopJob(ctx context.Context, job Job) error {
	response, err := managedAgent.runRaw(ctx, systemd.StopCommand(managedAgent.jobSpec(job).UnitName()))
	if err != nil {
		return err
	}
	if response.ReturnCode != 0 {
		return fmt.Errorf("stopping job %q on agent %q failed with exit code %d: %s",
			job.Id, managedAgent.Name(), response.ReturnCode, strings.TrimSpace(response.Stderr))
	}
	slog.Info("stopped job", "agent", managedAgent.Name(), "job", job.Id)
	return nil
}

// RemoveJob stops the unit, deletes its output files and forgets the job.
func (managedAgent *Agent) RemoveJob(ctx context.Context, job Job) error {
	spec := managedAgent.jobSpec(job)
	if _, err := managedAgent.runRaw(ctx, systemd.StopCommand(spec.UnitName())); err != nil {
		return err
	}
	if _, err := managedAgent.runRaw(ctx, systemd.CleanupCommand(spec)); err != nil {
		return err
	}
	return managedAgent.ForgetJob(job)
}

// ForgetJob drops a job from the record without touching the remote host.
func (managedAgent *Agent) ForgetJob(job Job) error {
	return managedAgent.updateStored(func(record *Record) error {
		remaining := make([]Job, 0, len(record.Jobs))
		found := false
		for _, storedJob := range record.Jobs {
			if storedJob.Id == job.Id {
				found = true
				continue
			}
			remaining = append(remaining, storedJob)
		}
		if !found {
			return fmt.Errorf("%w: %q on agent %q", ErrJobNotFound, job.Id, managedAgent.Name())
		}
		record.Jobs = remaining
		return nil
	})
}
