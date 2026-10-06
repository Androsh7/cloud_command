package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/androsh7/cloud_command/pkg/ec2"
	"github.com/androsh7/cloud_command/pkg/sshx"
)

// Step identifies a phase of agent creation for progress reporting.
type Step string

const (
	StepDirectory   Step = "creating directory"
	StepKeys        Step = "generating ssh keys"
	StepImportKey   Step = "importing key pair"
	StepAmi         Step = "resolving ami"
	StepNetwork     Step = "resolving network"
	StepLaunch      Step = "launching instance"
	StepWaitRunning Step = "waiting for instance"
	StepWaitSsh     Step = "waiting for ssh"
	StepCleanup     Step = "cleaning up"
	StepDone        Step = "ready"
)

const cleanupTimeout = 2 * time.Minute

// Spec is what the user supplies to create an agent.
type Spec struct {
	Name         string
	Region       string
	InstanceType string
	Architecture ec2.Architecture
	SshUser      string
	SshPort      int
}

// CreateOptions controls the timeouts used while waiting for a new agent.
type CreateOptions struct {
	SshTimeout   time.Duration
	ReadyTimeout time.Duration
	PollInterval time.Duration
}

// Reporter receives creation progress. A nil reporter is allowed.
type Reporter func(agentName string, step Step)

// report calls the reporter when one is set.
func (reporter Reporter) report(agentName string, step Step) {
	if reporter != nil {
		reporter(agentName, step)
	}
}

// Create provisions one agent and blocks until it accepts SSH connections.
// When provisioning fails or the context is cancelled, everything created so far is removed again.
func Create(ctx context.Context, store *Store, spec Spec, options CreateOptions, reporter Reporter) (*Agent, error) {
	reporter.report(spec.Name, StepDirectory)
	directory, err := store.NewAgentDirectory(spec.Name)
	if err != nil {
		return nil, err
	}
	newAgent := &Agent{
		Record: Record{
			Name:         spec.Name,
			Region:       spec.Region,
			InstanceType: spec.InstanceType,
			Architecture: spec.Architecture,
			SshUser:      spec.SshUser,
			SshPort:      spec.SshPort,
			CreatedAt:    time.Now().UTC(),
			Jobs:         []Job{},
		},
		Directory: directory,
		store:     store,
		sshConfig: SshOptions{Timeout: options.SshTimeout},
	}

	if err := newAgent.provision(ctx, reporter); err != nil {
		newAgent.rollback(reporter)
		return nil, err
	}
	if err := newAgent.WaitUntilReady(ctx, options, reporter); err != nil {
		if ctx.Err() != nil {
			newAgent.rollback(reporter)
			return nil, fmt.Errorf("creation of agent %q cancelled: %w", spec.Name, err)
		}
		return newAgent, err
	}
	reporter.report(spec.Name, StepDone)
	return newAgent, nil
}

// provision performs every step up to and including launching the instance.
func (managedAgent *Agent) provision(ctx context.Context, reporter Reporter) error {
	name := managedAgent.Name()
	region := managedAgent.Record.Region

	reporter.report(name, StepKeys)
	keyPair, err := sshx.GenerateRsaKeyPair(managedAgent.Directory)
	if err != nil {
		return err
	}
	managedAgent.Record.PrivateKeyFile = sshx.PrivateKeyFileName
	managedAgent.Record.PublicKeyFile = sshx.PublicKeyFileName
	if err := managedAgent.Save(); err != nil {
		return err
	}

	reporter.report(name, StepImportKey)
	keyPairName := KeyPairNamePrefix + uuid.NewString()
	if err := ec2.CreateEc2KeyPair(ctx, region, keyPairName, keyPair.PublicKeyPath); err != nil {
		return err
	}
	managedAgent.Record.KeyPairName = keyPairName
	if err := managedAgent.Save(); err != nil {
		return err
	}

	reporter.report(name, StepAmi)
	amiId, err := ec2.GetAmiId(ctx, region, managedAgent.Record.Architecture)
	if err != nil {
		return err
	}
	managedAgent.Record.AmiId = amiId

	reporter.report(name, StepNetwork)
	vpcId, err := ec2.GetDefaultVpcId(ctx, region)
	if err != nil {
		return err
	}
	securityGroupId, err := ec2.CreateSecurityGroup(ctx, region, vpcId)
	if err != nil {
		return err
	}
	subnetId, err := ec2.GetVpcSubnetId(ctx, region, vpcId)
	if err != nil {
		return err
	}
	managedAgent.Record.VpcId = vpcId
	managedAgent.Record.SecurityGroupId = securityGroupId
	managedAgent.Record.SubnetId = subnetId
	if err := managedAgent.Save(); err != nil {
		return err
	}

	reporter.report(name, StepLaunch)
	instanceId, err := ec2.CreateEc2(ctx, region, amiId, managedAgent.Record.InstanceType, keyPairName, subnetId, securityGroupId, name)
	if err != nil {
		return err
	}
	managedAgent.Record.InstanceId = instanceId
	slog.Info("launched instance", "agent", name, "instance_id", instanceId, "region", region)
	return managedAgent.Save()
}

// WaitUntilReady blocks until the instance is running and accepts SSH connections.
func (managedAgent *Agent) WaitUntilReady(ctx context.Context, options CreateOptions, reporter Reporter) error {
	name := managedAgent.Name()
	readyCtx, cancel := context.WithTimeout(ctx, options.ReadyTimeout)
	defer cancel()

	reporter.report(name, StepWaitRunning)
	info, err := ec2.WaitForInstanceRunning(readyCtx, managedAgent.Record.Region, managedAgent.Record.InstanceId, options.ReadyTimeout)
	if err != nil {
		return err
	}
	managedAgent.Record.PublicIp = info.PublicIp
	if err := managedAgent.Save(); err != nil {
		return err
	}

	reporter.report(name, StepWaitSsh)
	host, err := managedAgent.SshHost()
	if err != nil {
		return err
	}
	if err := host.WaitForSsh(readyCtx, options.PollInterval); err != nil {
		return fmt.Errorf("agent %q: %w", name, err)
	}
	slog.Info("agent ready", "agent", name, "public_ip", info.PublicIp)
	return nil
}

// rollback terminates the instance, deletes the key pair and removes the directory, whichever of them exist.
// It uses its own context so a cancelled creation can still clean up.
func (managedAgent *Agent) rollback(reporter Reporter) {
	reporter.report(managedAgent.Name(), StepCleanup)
	cleanupCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	if err := managedAgent.Destroy(cleanupCtx); err != nil {
		slog.Warn("rollback left resources behind", "agent", managedAgent.Name(),
			"instance_id", managedAgent.Record.InstanceId, "key_pair", managedAgent.Record.KeyPairName, "error", err)
		return
	}
	slog.Info("rolled back agent", "agent", managedAgent.Name())
}
