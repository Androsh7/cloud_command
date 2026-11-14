//go:build live_aws

package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/androsh7/cloud_command/pkg/ec2"
	"github.com/androsh7/cloud_command/pkg/sshx"
	"github.com/androsh7/cloud_command/pkg/systemd"
)

const (
	liveInstanceType = "t4g.nano"
	liveSshUser      = "ec2-user"
	liveSshPort      = 22
	liveSshTimeout   = 10 * time.Second
	liveReadyTimeout = 6 * time.Minute
	livePollInterval = 5 * time.Second
	liveJobTimeout   = 2 * time.Minute
	liveTestTimeout  = 10 * time.Minute
)

// liveRegion returns the region the default AWS config resolves to.
func liveRegion(t *testing.T, ctx context.Context) string {
	t.Helper()
	awsConfig, err := ec2.GetAwsConfig(ctx, "")
	if err != nil || awsConfig.Region == "" {
		t.Skipf("no default aws region: %v", err)
	}
	return awsConfig.Region
}

func TestLiveCancelledCreationCleansUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()
	store := NewStore(t.TempDir())
	region := liveRegion(t, ctx)
	spec := Spec{
		Name: "live-cancel-" + uuid.NewString()[:8], Region: region, InstanceType: liveInstanceType,
		Architecture: ec2.ArchitectureArm64, SshUser: liveSshUser, SshPort: liveSshPort,
	}
	options := CreateOptions{SshTimeout: liveSshTimeout, ReadyTimeout: liveReadyTimeout, PollInterval: livePollInterval}
	createCtx, cancelCreate := context.WithCancel(ctx)
	defer cancelCreate()
	var launched *Agent
	reporter := func(_ string, step Step) {
		if step == StepWaitRunning {
			loaded, err := store.Load(spec.Name)
			if err == nil {
				launched = loaded
			}
			cancelCreate()
		}
	}

	createdAgent, err := Create(createCtx, store, spec, options, reporter)

	if err == nil || createdAgent != nil {
		t.Fatalf("expected cancellation error without an agent, got %v %v", createdAgent, err)
	}
	if launched == nil || launched.Record.InstanceId == "" {
		t.Fatal("test never observed the launched instance")
	}
	if _, err := store.Load(spec.Name); err == nil {
		t.Fatal("agent directory still exists after cancellation")
	}
	info, err := ec2.DescribeInstance(ctx, region, launched.Record.InstanceId)
	if err != nil || !ec2.IsGoneState(info.State) {
		t.Fatalf("instance not terminated: %+v %v", info, err)
	}
	t.Cleanup(func() { _ = ec2.DeleteKeyPair(context.Background(), region, launched.Record.KeyPairName) })
}

func TestLiveAgentLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()
	store := NewStore(t.TempDir())
	spec := Spec{
		Name:         "live-" + uuid.NewString()[:8],
		Region:       liveRegion(t, ctx),
		InstanceType: liveInstanceType,
		Architecture: ec2.ArchitectureArm64,
		SshUser:      liveSshUser,
		SshPort:      liveSshPort,
	}
	options := CreateOptions{SshTimeout: liveSshTimeout, ReadyTimeout: liveReadyTimeout, PollInterval: livePollInterval}
	steps := []Step{}

	createdAgent, err := Create(ctx, store, spec, options, func(_ string, step Step) { steps = append(steps, step) })
	if createdAgent != nil {
		t.Cleanup(func() {
			if err := createdAgent.Destroy(context.Background()); err != nil {
				t.Errorf("Destroy returned error: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatalf("Create returned error after steps %v: %v", steps, err)
	}
	if steps[len(steps)-1] != StepDone {
		t.Fatalf("last step was %q, want %q", steps[len(steps)-1], StepDone)
	}

	response, err := createdAgent.RunCommand(ctx, sshx.Command{Script: "uname -a"})
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if response.ReturnCode != 0 || !strings.Contains(response.Stdout, "Linux") {
		t.Fatalf("unexpected uname response: %+v", response)
	}

	sudoResponse, err := createdAgent.RunCommand(ctx, sshx.Command{Script: "id -u", Sudo: true})
	if err != nil || strings.TrimSpace(sudoResponse.Stdout) != "0" {
		t.Fatalf("sudo command failed: %+v %v", sudoResponse, err)
	}

	job, err := createdAgent.StartJob(ctx, sshx.Command{Script: "echo started; sleep 15; echo done"})
	if err != nil {
		t.Fatalf("StartJob returned error: %v", err)
	}
	reloaded, err := store.Load(spec.Name)
	if err != nil || len(reloaded.Record.Jobs) != 1 || reloaded.Record.Jobs[0].Id != job.Id {
		t.Fatalf("job was not persisted: %v %+v", err, reloaded)
	}

	status, err := createdAgent.JobStatus(ctx, job)
	if err != nil {
		t.Fatalf("JobStatus returned error: %v", err)
	}
	if !status.IsRunning() {
		t.Fatalf("job should be running right after start, got %q", status.Summary())
	}

	deadline := time.Now().Add(liveJobTimeout)
	for status.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(livePollInterval)
		status, err = createdAgent.JobStatus(ctx, job)
		if err != nil {
			t.Fatalf("JobStatus returned error: %v", err)
		}
	}
	if status.Summary() != "exited (0)" {
		t.Fatalf("job finished with %q", status.Summary())
	}
	stdout, _, err := createdAgent.JobOutput(ctx, job, systemd.DefaultOutputLines)
	if err != nil || !strings.Contains(stdout, "done") {
		t.Fatalf("job output missing: %q %v", stdout, err)
	}

	longJob, err := createdAgent.StartJob(ctx, sshx.Command{Script: "sleep 600"})
	if err != nil {
		t.Fatalf("StartJob (long) returned error: %v", err)
	}
	if err := createdAgent.StopJob(ctx, longJob); err != nil {
		t.Fatalf("StopJob returned error: %v", err)
	}
	stoppedStatus, err := createdAgent.JobStatus(ctx, longJob)
	if err != nil || stoppedStatus.IsRunning() {
		t.Fatalf("job still running after stop: %+v %v", stoppedStatus, err)
	}
	if err := createdAgent.RemoveJob(ctx, longJob); err != nil {
		t.Fatalf("RemoveJob returned error: %v", err)
	}
	if len(createdAgent.Record.Jobs) != 1 {
		t.Fatalf("expected one job left, got %d", len(createdAgent.Record.Jobs))
	}
}
