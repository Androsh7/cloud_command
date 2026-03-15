"""Defines AWS queries"""

# Standard libraries
from pathlib import Path
from typing import Literal, Optional

# Third-party libraries
import boto3
from attrs import define, field, validators
from tqdm import tqdm
from botocore.exceptions import ClientError
from loguru import logger


def load_regions() -> list[str]:
    """Returns a list of AWS regions"""
    logger.info("Loading AWS regions")
    boto3_client = boto3.client("ec2")

    # Describe regions
    logger.debug("Running boto3 describe_regions")
    region_dict = boto3_client.describe_regions()
    out_list = []
    for region_subdict in region_dict["Regions"]:
        out_list.append(region_subdict["RegionName"])

    return sorted(set(out_list))

@define
class DetachedAgent:
    id: str = field(validator=validators.instance_of(str))
    key_pair: Optional[str] = field(validator=validators.optional(validators.instance_of(str)))
    region: str = field(validator=validators.instance_of(str))

def list_all_ec2s() -> list[DetachedAgent]:
    out_list = []
    regions = load_regions()
    for region in tqdm(regions, desc="Searching by region", unit="region"):
        boto3_client = boto3.client("ec2", region_name=region)
        paginator = boto3_client.get_paginator("describe_instances")
        for page in paginator.paginate(Filters=[{"Name": "tag:Type", "Values": ["cloud_command"]}]):
            for reservation in page["Reservations"]:
                for instance in reservation["Instances"]:
                    if instance["State"]["Name"] != "terminated":
                        out_list.append(
                            DetachedAgent(id=instance["InstanceId"], key_pair=instance.get("KeyName"), region=region)
                        )
    return out_list

def create_ec2_key_pair(region: str, key_pair_name: str, public_key_path: Path):
    """Create an SSH key pair, automatically replaces existing key pairs with the same name

    Args:
        ui: The ui object
        region: The aws region to create the key in
        key_pair_name: The new name for the key pair
        public_key_path: Path to the public key file
    """
    boto3_ec2_client = boto3.client("ec2", region_name=region)
    with open(file=public_key_path, encoding="utf-8") as public_key_file:
        public_key = public_key_file.read()
    try:
        boto3_ec2_client.delete_key_pair(KeyName=key_pair_name)
        logger.debug(f"Deleted existing key pair {key_pair_name}")
    except ClientError as ex:
        if ex.response["Error"]["Code"] != "InvalidKeyPair.NotFound":
            raise
    boto3_ec2_client.import_key_pair(
        KeyName=key_pair_name,
        PublicKeyMaterial=public_key,
    )
    logger.debug(f'Creating key pair: public_path="{public_key_path}", KeyName="{key_pair_name}"')


def get_default_vpc_id(region: str) -> str:
    """Provides the default VPC for a given region

    Args:
        region: The AWS region to search

    Returns:
        The default VPC ID
    """
    return boto3.client("ec2", region_name=region).describe_vpcs(Filters=[{"Name": "isDefault", "Values": ["true"]}])[
        "Vpcs"
    ][0]["VpcId"]


def get_vpc_subnet_id(region: str, vpc_id: str) -> str:
    """Provides the subnet ID attached to a VPC

    Args:
        region: The AWS region to search in
        vpc_id: The VPC id to search

    Returns:
        The subnet ID for the specified VPC
    """
    return boto3.client("ec2", region_name=region).describe_subnets(Filters=[{"Name": "vpc-id", "Values": [vpc_id]}])[
        "Subnets"
    ][0]["SubnetId"]


def get_ami_id(region: str, architecture: Literal["arm", "x86"] = "arm") -> str:
    """Returns the AMI ID for the amazon linux 2023 in the region with the specified architecture

    Args:
        region: The AWS region to search in
        architecture: The device architecture

    Returns:
        Returns the AMI ID
    """
    ssm_arch = "arm64" if architecture == "arm" else "x86_64"
    return boto3.client("ssm", region_name=region).get_parameter(
        Name=f"/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-{ssm_arch}"
    )["Parameter"]["Value"]


def create_ec2(
    region: str,
    ami_id: str,
    instance_type: str,
    key_pair_name: str,
    subnet_id: str,
    security_group_id: str,
    name: str,
) -> str:
    """Creates an EC2 and returns the instance ID

    Args:
        region: The AWS region to build the instance in
        ami_id: The AMI ID for the instance
        instance_type: The AWS device type for the instance, I.E: "t4g.nano"
        key_pair_name: The AWS key pair name
        subnet_id: The subnet ID
        security_group_id: The security group ID
        name: The name of the scanner

    Returns:
        The instance ID
    """
    return boto3.client("ec2", region_name=region).run_instances(
        ImageId=ami_id,
        InstanceType=instance_type,
        KeyName=key_pair_name,
        MinCount=1,
        MaxCount=1,
        NetworkInterfaces=[
            {
                "DeviceIndex": 0,
                "SubnetId": subnet_id,
                "Groups": [security_group_id],
                "AssociatePublicIpAddress": True,
            }
        ],
        TagSpecifications=[
            {
                "ResourceType": "instance",
                "Tags": [
                    {"Key": "Name", "Value": name},
                    {"Key": "Type", "Value": "cloud_command"},
                ],
            }
        ],
    )["Instances"][0]["InstanceId"]


def create_security_group(region: str, vpc_id: str) -> str:
    """Creates a security group and allows ssh access, reuses existing security groups if one already exists

    Args:
        ui: The ui object
        region: The AWS region to create the security group in
        vpc_id: The ID of the VPC

    Returns:
        The security group ID
    """
    boto3_ec2_client = boto3.client("ec2", region_name=region)
    security_group_query_response = boto3_ec2_client.describe_security_groups(
        Filters=[
            {"Name": "group-name", "Values": ["scan-cue-ssh-only"]},
            {"Name": "vpc-id", "Values": [vpc_id]},
        ]
    )
    if security_group_query_response["SecurityGroups"]:
        security_group_id = security_group_query_response["SecurityGroups"][0]["GroupId"]
        logger.debug(f"Using existing security group: {security_group_id}")
    else:
        security_group_build_response = boto3_ec2_client.create_security_group(
            GroupName="scan-cue-ssh-only",
            Description="Allow ssh access (Scan Cue)",
            VpcId=vpc_id,
        )
        security_group_id = security_group_build_response["GroupId"]

        # Add ssh access to the security group
        logger.debug(f"Created security group: {security_group_id}")
        boto3_ec2_client.authorize_security_group_ingress(
            GroupId=security_group_id,
            IpPermissions=[
                {
                    "IpProtocol": "tcp",
                    "FromPort": 22,
                    "ToPort": 22,
                    "IpRanges": [{"CidrIp": "0.0.0.0/0"}],
                }
            ],
        )
        logger.debug(f"Added ssh ingress rule to security group: {security_group_id}")
    return security_group_id
