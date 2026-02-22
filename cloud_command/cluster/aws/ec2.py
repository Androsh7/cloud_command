"""Defines the Ec2 class"""

# Standard libraries
import os
import shutil
import sys
import time
from ipaddress import IPv4Address
from pathlib import Path

# Third-party libraries
import boto3
from attrs import define, field, validators
from fabric import Connection
from loguru import logger

from cluster.aws.aws import (
    create_ec2,
    create_ec2_key_pair,
    create_security_group,
    get_ami_id,
    get_default_vpc_id,
    get_vpc_subnet_id,
)
from cluster.utils import create_ssh_key_pair, SshKeyPair
from cloud_command.constants import AGENT_STATES, BUILD_DIR, SSH_TIMEOUT
from cluster.aws.compute_abstraction import RemoteCompute

@define
class Ec2(RemoteCompute):
    # Required parameters
    name: str = field(validator=validators.instance_of(str))
    instance_type: str = field(validator=validators.instance_of(str))
    region: str = field(validator=validators.instance_of(str))
    delete_on_exit: bool = field(default=True, validator=validators.instance_of(bool))

    # Build configs
    ami_id: str = field(validator=validators.instance_of(str), init=False)
    setup_dir: Path = field(validator=validators.instance_of(Path), init=False)
    instance_id: str = field(validator=validators.instance_of(str), init=False)
    vpc_id: str = field(validator=validators.instance_of(str), init=False)
    subnet_id: str = field(validator=validators.instance_of(str), init=False)
    security_group_id: str = field(validator=validators.instance_of(str), init=False)
    key_pair_name: str = field(validator=validators.instance_of(str), init=False)
    key_pair: SshKeyPair = field(validator=validators.instance_of(SshKeyPair), init=False)

    # Runtime data
    instance_state: AGENT_STATES = field(
        validator=validators.and_(validators.instance_of(str), validators.in_(AGENT_STATES)), init=False
    )
    public_ip_address: IPv4Address = field(converter=IPv4Address, init=False)

    def __del__(self):
        if sys.meta_path is None:
            return
        if not getattr(self, "delete_on_exit", False):
            return
        if not hasattr(self, "instance_id"):
            return
        logger.debug(f"Destroying EC2 {self.name}")
        boto3_client = boto3.client("ec2", region_name=self.region)

        # Destroy EC2
        logger.debug(f"Destroying instance {self.instance_id}")
        boto3_client.terminate_instances(InstanceIds=[self.instance_id])

        # Destroy key pair
        logger.debug(f"Destroying key pair {self.key_pair_name}")
        boto3_client.delete_key_pair(KeyName=self.key_pair_name)

        # Delete build folder
        logger.debug(f"Destroying setup dir {self.setup_dir}")
        shutil.rmtree(self.setup_dir)

    def __attrs_post_init__(self):
        # Build setup dir
        self.setup_dir = BUILD_DIR / self.name
        os.makedirs(self.setup_dir, mode=500, exist_ok=True)

        # Build EC2
        logger.debug(f"Building ec2 {self.name}")
        self.key_pair_name = f"ssh-key-{self.name}"
        self.key_pair = create_ssh_key_pair(output_dir=self.setup_dir)
        create_ec2_key_pair(
            ui=self.ui, region=self.region, key_pair_name=self.key_pair_name, public_key_path=self.key_pair.public_key
        )
        self.ami_id = get_ami_id(region=self.region)
        self.vpc_id = get_default_vpc_id(region=self.region)
        self.security_group_id = create_security_group(region=self.region, vpc_id=self.vpc_id)
        self.subnet_id = get_vpc_subnet_id(region=self.region, vpc_id=self.vpc_id)
        self.instance_id = create_ec2(
            region=self.region,
            ami_id=self.ami_id,
            instance_type=self.instance_type,
            key_pair_name=self.key_pair_name,
            subnet_id=self.subnet_id,
            security_group_id=self.security_group_id,
            name=self.name,
        )

    def connection(self) -> Connection:
        """Create a fabric connection object"""
        return Connection(
            host=str(self.public_ip_address),
            user="ec2-user",
            port=22,
            connect_timeout=SSH_TIMEOUT,
            connect_kwargs={
                "key_filename": str(self.key_pair.private_key),
            },
        )

    def cloudinit_status(self, retries: int = 2) -> bool:
        for attempt in range(retries + 1):
            try:
                with self.connection() as conn:
                    cloud_init_status = conn.run("cloud-init status", hide="both").stdout.strip().split(" ")[1]
                if cloud_init_status != "done":
                    logger.debug(f"{self.name} cloud-init status is {cloud_init_status}")
                    return False
                return True
            except (OSError, EOFError) as ex:
                logger.warning(f"Attempt to status {self.name} cloud-init status failed with error: {ex}")
                if attempt >= retries:
                    raise
                time.sleep(0.5 * (attempt + 1))

    def is_built(self) -> bool:
        """Returns True if the instance is built"""
        boto3_client = boto3.client("ec2", region_name=self.region)

        # Describe instance
        instance_describe_dict = boto3_client.describe_instances(InstanceIds=[self.instance_id])["Reservations"][0][
            "Instances"
        ][0]
        self.instance_state = instance_describe_dict["State"]["Name"]
        if self.instance_state == "pending":
            logger.debug(f'{self.name} instance state is "pending"')
            return False
        if self.instance_state not in ["pending", "running"]:
            raise KeyError(f"Unexpected instance state {self.instance_state}")

        # Get instance status
        instance_status_dict = boto3_client.describe_instance_status(
            InstanceIds=[self.instance_id],
            IncludeAllInstances=True,
        )
        status = instance_status_dict.get("InstanceStatuses")
        if status is None:
            logger.debug(f"{self.name} instance status is not present")
            return False
        system_status = status[0]["SystemStatus"]["Status"]
        instance_status = status[0]["InstanceStatus"]["Status"]
        if system_status != "ok" or instance_status != "ok":
            logger.debug(f'{self.name} instance_status="{instance_status}", system_status="{instance_status}"')
            return False

        # Set public and private IP address
        if instance_describe_dict.get("PrivateIpAddress") is not None:
            self.private_ip_address = IPv4Address(instance_describe_dict["PrivateIpAddress"])
        if instance_describe_dict.get("PublicIpAddress") is not None:
            self.public_ip_address = IPv4Address(instance_describe_dict["PublicIpAddress"])

        # Validate cloud-init status
        return self.cloudinit_status()

    def get_state(self) -> str:
        """Returns the EC2 state"""
        boto3_client = boto3.client("ec2", region_name=self.region)
        instance_dict = boto3_client.describe_instances(InstanceIds=[self.instance_id])["Reservations"][0]["Instances"][
            0
        ]
        self.instance_state = instance_dict["State"]["Name"]
        return self.instance_state
