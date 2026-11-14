// Package ec2 defines the AWS queries used to provision and find agents.
package ec2

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go"
)

const (
	AgentTypeTagValue             = "cloud-command"
	SshSecurityGroupName          = "cloud-command-ssh-only"
	SshSecurityGroupDescription   = "Allow ssh access (cloud command)"
	SshPort                       = 22
	AnyIpv4Cidr                   = "0.0.0.0/0"
	DuplicateGroupErrorCode       = "InvalidGroup.Duplicate"
	DuplicatePermissionErrorCode  = "InvalidPermission.Duplicate"
	tcpProtocol                   = "tcp"
	allProtocols                  = "-1"
	amazonLinuxAmiParameterFormat = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-%s"
)

// Architecture is the CPU architecture an AMI targets.
type Architecture string

const (
	ArchitectureX86_64 Architecture = "x86_64"
	ArchitectureArm64  Architecture = "arm64"
)

// DetachedAgent describes an EC2 instance found in AWS that no agent object currently manages.
type DetachedAgent struct {
	Id      string
	KeyPair string
	Region  string
	State   string
}

var (
	awsConfigCache sync.Map // region -> aws.Config
	regionsLock    sync.Mutex
	cachedRegions  []string
)

// GetAwsConfig returns a cached AWS config for one region. An empty region uses the default region.
func GetAwsConfig(ctx context.Context, region string) (aws.Config, error) {
	if cached, found := awsConfigCache.Load(region); found {
		return cached.(aws.Config), nil
	}
	options := []func(*config.LoadOptions) error{}
	if region != "" {
		options = append(options, config.WithRegion(region))
	}
	awsConfig, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading aws config for region %q: %w", region, err)
	}
	awsConfigCache.Store(region, awsConfig)
	return awsConfig, nil
}

// GetEc2Client returns an EC2 client for one region. An empty region uses the default region.
func GetEc2Client(ctx context.Context, region string) (*ec2.Client, error) {
	awsConfig, err := GetAwsConfig(ctx, region)
	if err != nil {
		return nil, err
	}
	return ec2.NewFromConfig(awsConfig), nil
}

// GetSsmClient returns an SSM client for one region. An empty region uses the default region.
func GetSsmClient(ctx context.Context, region string) (*ssm.Client, error) {
	awsConfig, err := GetAwsConfig(ctx, region)
	if err != nil {
		return nil, err
	}
	return ssm.NewFromConfig(awsConfig), nil
}

// GetEc2Regions returns every AWS region that offers EC2, sorted by name, asking the given region for the list.
// An empty region uses the default region. A successful result is cached; a failure is retried on the next call.
func GetEc2Regions(ctx context.Context, region string) ([]string, error) {
	regionsLock.Lock()
	defer regionsLock.Unlock()
	if cachedRegions != nil {
		return cachedRegions, nil
	}
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return nil, err
	}
	output, err := client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, fmt.Errorf("describing regions: %w", err)
	}
	names := make([]string, 0, len(output.Regions))
	for _, described := range output.Regions {
		names = append(names, aws.ToString(described.RegionName))
	}
	sort.Strings(names)
	cachedRegions = names
	return cachedRegions, nil
}

// IsGoneState reports whether an instance state means the instance is terminated or on its way there.
func IsGoneState(state string) bool {
	return state == string(types.InstanceStateNameTerminated) || state == string(types.InstanceStateNameShuttingDown)
}

// FindEc2InRegion returns every instance that is not terminated or shutting down in one region carrying the given Type tag.
func FindEc2InRegion(ctx context.Context, region string, nameTag string) ([]DetachedAgent, error) {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return nil, err
	}
	slog.Debug("searching region for tagged instances", "region", region, "tag", nameTag)
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{{Name: aws.String("tag:Type"), Values: []string{nameTag}}},
	})
	foundAgents := []DetachedAgent{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing instances in region %q: %w", region, err)
		}
		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				state := ""
				if instance.State != nil {
					state = string(instance.State.Name)
				}
				if IsGoneState(state) {
					continue
				}
				foundAgents = append(foundAgents, DetachedAgent{
					Id:      aws.ToString(instance.InstanceId),
					KeyPair: aws.ToString(instance.KeyName),
					Region:  region,
					State:   state,
				})
			}
		}
	}
	return foundAgents, nil
}

// ListAllEc2s returns every non-terminated instance across all regions carrying the given Type tag.
// homeRegion is the region asked for the list of regions; an empty one uses the default region.
// searchThreads limits how many regions are searched at once; zero searches them all at once.
func ListAllEc2s(ctx context.Context, homeRegion string, nameTag string, searchThreads int) ([]DetachedAgent, error) {
	regions, err := GetEc2Regions(ctx, homeRegion)
	if err != nil {
		return nil, err
	}
	if searchThreads <= 0 {
		searchThreads = len(regions)
	}

	type regionResult struct {
		agents []DetachedAgent
		err    error
	}
	results := make([]regionResult, len(regions))
	semaphore := make(chan struct{}, searchThreads)
	var waitGroup sync.WaitGroup
	for index, region := range regions {
		waitGroup.Add(1)
		go func(index int, region string) {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			agents, err := FindEc2InRegion(ctx, region, nameTag)
			results[index] = regionResult{agents: agents, err: err}
		}(index, region)
	}
	waitGroup.Wait()

	foundAgents := []DetachedAgent{}
	for _, result := range results {
		if result.err != nil {
			return nil, result.err
		}
		foundAgents = append(foundAgents, result.agents...)
	}
	return foundAgents, nil
}

// CreateEc2KeyPair registers a public key with EC2, replacing any key pair that already has the name.
func CreateEc2KeyPair(ctx context.Context, region string, keyPairName string, publicKeyPath string) error {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return err
	}
	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("reading public key %q: %w", publicKeyPath, err)
	}
	if err := DeleteKeyPair(ctx, region, keyPairName); err != nil {
		return err
	}
	_, err = client.ImportKeyPair(ctx, &ec2.ImportKeyPairInput{
		KeyName:           aws.String(keyPairName),
		PublicKeyMaterial: publicKey,
	})
	if err != nil {
		return fmt.Errorf("importing key pair %q: %w", keyPairName, err)
	}
	return nil
}

// GetDefaultVpcId returns the default VPC for a region.
func GetDefaultVpcId(ctx context.Context, region string) (string, error) {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return "", err
	}
	output, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []types.Filter{{Name: aws.String("isDefault"), Values: []string{"true"}}},
	})
	if err != nil {
		return "", fmt.Errorf("describing vpcs in region %q: %w", region, err)
	}
	if len(output.Vpcs) == 0 {
		return "", fmt.Errorf("no default vpc in region %q", region)
	}
	return aws.ToString(output.Vpcs[0].VpcId), nil
}

// GetVpcSubnetId returns a subnet of a VPC that sits in an availability zone offering the instance type.
// When the offerings cannot be read, the first subnet is returned and the launch decides.
func GetVpcSubnetId(ctx context.Context, region string, vpcId string, instanceType string) (string, error) {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return "", err
	}
	output, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcId}}},
	})
	if err != nil {
		return "", fmt.Errorf("describing subnets for vpc %q: %w", vpcId, err)
	}
	if len(output.Subnets) == 0 {
		return "", fmt.Errorf("no subnets in vpc %q", vpcId)
	}
	subnets := output.Subnets
	sort.Slice(subnets, func(left int, right int) bool {
		return aws.ToString(subnets[left].AvailabilityZone) < aws.ToString(subnets[right].AvailabilityZone)
	})
	zones, err := instanceTypeZones(ctx, client, instanceType)
	if err != nil {
		slog.Warn("could not read instance type offerings, using the first subnet", "region", region, "instance_type", instanceType, "error", err)
		return aws.ToString(subnets[0].SubnetId), nil
	}
	subnetId, found := pickSubnet(subnets, zones)
	if !found {
		return "", fmt.Errorf("no subnet of vpc %q is in an availability zone that offers instance type %q in region %q", vpcId, instanceType, region)
	}
	return subnetId, nil
}

// instanceTypeZones returns the availability zones of the client's region that offer an instance type.
func instanceTypeZones(ctx context.Context, client *ec2.Client, instanceType string) (map[string]bool, error) {
	paginator := ec2.NewDescribeInstanceTypeOfferingsPaginator(client, &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: types.LocationTypeAvailabilityZone,
		Filters:      []types.Filter{{Name: aws.String("instance-type"), Values: []string{instanceType}}},
	})
	zones := map[string]bool{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing offerings of instance type %q: %w", instanceType, err)
		}
		for _, offering := range page.InstanceTypeOfferings {
			zones[aws.ToString(offering.Location)] = true
		}
	}
	return zones, nil
}

// pickSubnet returns the first subnet whose availability zone is one of the given zones.
func pickSubnet(subnets []types.Subnet, zones map[string]bool) (string, bool) {
	for _, subnet := range subnets {
		if zones[aws.ToString(subnet.AvailabilityZone)] {
			return aws.ToString(subnet.SubnetId), true
		}
	}
	return "", false
}

// GetAmiId returns the latest Amazon Linux 2023 AMI for an architecture in a region.
func GetAmiId(ctx context.Context, region string, architecture Architecture) (string, error) {
	client, err := GetSsmClient(ctx, region)
	if err != nil {
		return "", err
	}
	parameterName := fmt.Sprintf(amazonLinuxAmiParameterFormat, architecture)
	output, err := client.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(parameterName)})
	if err != nil {
		return "", fmt.Errorf("getting ssm parameter %q: %w", parameterName, err)
	}
	return aws.ToString(output.Parameter.Value), nil
}

// CreateEc2 launches one EC2 instance with a public address and returns its instance ID.
func CreateEc2(
	ctx context.Context,
	region string,
	amiId string,
	instanceType string,
	keyPairName string,
	subnetId string,
	securityGroupId string,
	name string,
) (string, error) {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return "", err
	}
	output, err := client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:      aws.String(amiId),
		InstanceType: types.InstanceType(instanceType),
		KeyName:      aws.String(keyPairName),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		NetworkInterfaces: []types.InstanceNetworkInterfaceSpecification{{
			DeviceIndex:              aws.Int32(0),
			SubnetId:                 aws.String(subnetId),
			Groups:                   []string{securityGroupId},
			AssociatePublicIpAddress: aws.Bool(true),
		}},
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeInstance,
			Tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String(name)},
				{Key: aws.String("Type"), Value: aws.String(AgentTypeTagValue)},
			},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("running instance in region %q: %w", region, err)
	}
	if len(output.Instances) == 0 {
		return "", fmt.Errorf("run instances in region %q returned no instances", region)
	}
	return aws.ToString(output.Instances[0].InstanceId), nil
}

// CreateSecurityGroup returns the ID of the SSH-only security group in a VPC, creating it if it does not exist.
func CreateSecurityGroup(ctx context.Context, region string, vpcId string) (string, error) {
	client, err := GetEc2Client(ctx, region)
	if err != nil {
		return "", err
	}
	queryOutput, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{Name: aws.String("group-name"), Values: []string{SshSecurityGroupName}},
			{Name: aws.String("vpc-id"), Values: []string{vpcId}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("describing security groups in vpc %q: %w", vpcId, err)
	}
	if len(queryOutput.SecurityGroups) > 0 {
		existing := queryOutput.SecurityGroups[0]
		securityGroupId := aws.ToString(existing.GroupId)
		if AllowsSshIngress(existing.IpPermissions) {
			return securityGroupId, nil
		}
		slog.Warn("ssh security group has no ssh rule, adding it", "security_group", securityGroupId, "vpc", vpcId)
		if err := authorizeSshIngress(ctx, client, securityGroupId); err != nil {
			return "", err
		}
		return securityGroupId, nil
	}

	buildOutput, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(SshSecurityGroupName),
		Description: aws.String(SshSecurityGroupDescription),
		VpcId:       aws.String(vpcId),
	})
	if isDuplicateGroupError(err) {
		return CreateSecurityGroup(ctx, region, vpcId)
	}
	if err != nil {
		return "", fmt.Errorf("creating security group in vpc %q: %w", vpcId, err)
	}
	securityGroupId := aws.ToString(buildOutput.GroupId)
	if err := authorizeSshIngress(ctx, client, securityGroupId); err != nil {
		return "", err
	}
	return securityGroupId, nil
}

// AllowsSshIngress reports whether a set of ingress rules lets any IPv4 address reach the SSH port over TCP.
func AllowsSshIngress(permissions []types.IpPermission) bool {
	for _, permission := range permissions {
		protocol := aws.ToString(permission.IpProtocol)
		if protocol != tcpProtocol && protocol != allProtocols {
			continue
		}
		coversPort := protocol == allProtocols ||
			(aws.ToInt32(permission.FromPort) <= SshPort && aws.ToInt32(permission.ToPort) >= SshPort)
		if !coversPort {
			continue
		}
		for _, ipRange := range permission.IpRanges {
			if aws.ToString(ipRange.CidrIp) == AnyIpv4Cidr {
				return true
			}
		}
	}
	return false
}

// authorizeSshIngress opens the SSH port on a security group. A rule that already exists is not an error.
func authorizeSshIngress(ctx context.Context, client *ec2.Client, securityGroupId string) error {
	_, err := client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(securityGroupId),
		IpPermissions: []types.IpPermission{{
			IpProtocol: aws.String(tcpProtocol),
			FromPort:   aws.Int32(SshPort),
			ToPort:     aws.Int32(SshPort),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(AnyIpv4Cidr)}},
		}},
	})
	if err != nil && !hasErrorCode(err, DuplicatePermissionErrorCode) {
		return fmt.Errorf("authorizing ssh ingress on security group %q: %w", securityGroupId, err)
	}
	return nil
}

// hasErrorCode reports whether an error is an AWS API error with the given code.
func hasErrorCode(err error, code string) bool {
	var apiError smithy.APIError
	return errors.As(err, &apiError) && apiError.ErrorCode() == code
}

// isDuplicateGroupError reports whether AWS rejected a security group because another caller just created it.
func isDuplicateGroupError(err error) bool {
	return hasErrorCode(err, DuplicateGroupErrorCode)
}
