package ec2

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestAllowsSshIngressAcceptsOpenSshRule(t *testing.T) {
	permissions := []types.IpPermission{{
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int32(22),
		ToPort:     aws.Int32(22),
		IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
	}}

	if !AllowsSshIngress(permissions) {
		t.Fatal("expected the ssh rule to be recognised")
	}
}

func TestAllowsSshIngressRejectsGroupWithoutRules(t *testing.T) {
	if AllowsSshIngress(nil) {
		t.Fatal("a group without rules does not allow ssh")
	}
}

func TestAllowsSshIngressRejectsOtherPort(t *testing.T) {
	permissions := []types.IpPermission{{
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int32(443),
		ToPort:     aws.Int32(443),
		IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
	}}

	if AllowsSshIngress(permissions) {
		t.Fatal("a rule for another port does not allow ssh")
	}
}

func TestPickSubnetSkipsZonesWithoutTheInstanceType(t *testing.T) {
	subnets := []types.Subnet{
		{SubnetId: aws.String("subnet-a"), AvailabilityZone: aws.String("us-east-1a")},
		{SubnetId: aws.String("subnet-b"), AvailabilityZone: aws.String("us-east-1b")},
	}

	subnetId, found := pickSubnet(subnets, map[string]bool{"us-east-1b": true})

	if !found || subnetId != "subnet-b" {
		t.Fatalf("got %q, %t; want subnet-b", subnetId, found)
	}
}

func TestPickSubnetReportsNoMatch(t *testing.T) {
	subnets := []types.Subnet{{SubnetId: aws.String("subnet-a"), AvailabilityZone: aws.String("us-east-1a")}}

	_, found := pickSubnet(subnets, map[string]bool{"us-east-1b": true})

	if found {
		t.Fatal("expected no subnet to match")
	}
}
