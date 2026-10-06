package ec2

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/smithy-go"
)

const (
	notFoundErrorCodeSuffix = ".NotFound"
	nameTagKey              = "Name"
)

// InstanceInfo is the subset of an EC2 instance description the application needs.
type InstanceInfo struct {
	Id          string
	State       string
	PublicIp    string
	KeyPairName string
	Name        string
}

// IsNotFoundError reports whether an AWS error says the resource does not exist.
func IsNotFoundError(err error) bool {
	var apiError smithy.APIError
	if !errors.As(err, &apiError) {
		return false
	}
	return strings.HasSuffix(apiError.ErrorCode(), notFoundErrorCodeSuffix)
}

// DescribeInstance returns the current description of one instance.
func DescribeInstance(ctx context.Context, region string, instanceId string) (InstanceInfo, error) {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return InstanceInfo{}, err
	}
	output, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceId}})
	if err != nil {
		return InstanceInfo{}, fmt.Errorf("describing instance %q in region %q: %w", instanceId, region, err)
	}
	return instanceInfoFromOutput(output, instanceId, region)
}

// WaitForInstanceRunning blocks until the instance reports the running state and returns its description.
func WaitForInstanceRunning(ctx context.Context, region string, instanceId string, maxWait time.Duration) (InstanceInfo, error) {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return InstanceInfo{}, err
	}
	waiter := ec2.NewInstanceRunningWaiter(client)
	output, err := waiter.WaitForOutput(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{instanceId}}, maxWait)
	if err != nil {
		return InstanceInfo{}, fmt.Errorf("waiting for instance %q to run: %w", instanceId, err)
	}
	return instanceInfoFromOutput(output, instanceId, region)
}

// TerminateInstance terminates one instance, treating an unknown instance as already gone.
func TerminateInstance(ctx context.Context, region string, instanceId string) error {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return err
	}
	_, err = client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{instanceId}})
	if err != nil && !IsNotFoundError(err) {
		return fmt.Errorf("terminating instance %q in region %q: %w", instanceId, region, err)
	}
	return nil
}

// DeleteKeyPair removes one key pair, treating an unknown key pair as already gone.
func DeleteKeyPair(ctx context.Context, region string, keyPairName string) error {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return err
	}
	_, err = client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{KeyName: aws.String(keyPairName)})
	if err != nil && !IsNotFoundError(err) {
		return fmt.Errorf("deleting key pair %q in region %q: %w", keyPairName, region, err)
	}
	return nil
}

// instanceInfoFromOutput extracts the first instance from a describe output.
func instanceInfoFromOutput(output *ec2.DescribeInstancesOutput, instanceId string, region string) (InstanceInfo, error) {
	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return InstanceInfo{}, fmt.Errorf("instance %q not found in region %q", instanceId, region)
	}
	instance := output.Reservations[0].Instances[0]
	info := InstanceInfo{
		Id:          aws.ToString(instance.InstanceId),
		PublicIp:    aws.ToString(instance.PublicIpAddress),
		KeyPairName: aws.ToString(instance.KeyName),
	}
	if instance.State != nil {
		info.State = string(instance.State.Name)
	}
	for _, tag := range instance.Tags {
		if aws.ToString(tag.Key) == nameTagKey {
			info.Name = aws.ToString(tag.Value)
		}
	}
	return info, nil
}
