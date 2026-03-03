"""Defines the agent class."""

# Standard libraries
import json
import shutil
import sys
from http import HTTPStatus
from ipaddress import IPv4Address
from pathlib import Path
from typing import Any

# Third-party libraries
import boto3
from attrs import define, field, validators
from fabric import Connection
from loguru import logger

# Project libraries
from cloud_command.agent.agent_abstract import AbstractAgent
from cloud_command.agent.aws import (
    create_ec2,
    create_ec2_key_pair,
    create_security_group,
    get_ami_id,
    get_default_vpc_id,
    get_vpc_subnet_id,
)
from cloud_command.constants import AGENT_CONFIG_FILENAME, AGENT_DIRECTORY, AWS_EC2_STATES, SSH_TIMEOUT
from cloud_command.router.error_model import ServerError
from cloud_command.utils import SshKeyPair, create_ssh_key_pair


@define
class Ec2Config:
    instance_type: str = field(validator=validators.instance_of(str))
    region: str = field(validator=validators.instance_of(str))
    ami_id: str | None = field(default=None, validator=validators.optional(validators.instance_of(str)))
    instance_id: str | None = field(default=None, validator=validators.optional(validators.instance_of(str)))
    vpc_id: str | None = field(default=None, validator=validators.optional(validators.instance_of(str)))
    subnet_id: str | None = field(default=None, validator=validators.optional(validators.instance_of(str)))
    security_group_id: str | None = field(default=None, validator=validators.optional(validators.instance_of(str)))
    key_pair_name: str | None = field(default=None, validator=validators.optional(validators.instance_of(str)))
    key_pair: SshKeyPair | None = field(default=None, validator=validators.optional(validators.instance_of(SshKeyPair)))

    @classmethod
    def from_dict(cls, config: dict[str, Any]):
        key_pair = None
        if config.get("key_pair"):
            key_pair = SshKeyPair(
                private_key=Path(config["key_pair"]["private_key"]),
                public_key=Path(config["key_pair"]["public_key"]),
            )

        return cls(
            instance_type=config["instance_type"],
            region=config["region"],
            ami_id=config.get("ami_id"),
            instance_id=config.get("instance_id"),
            vpc_id=config.get("vpc_id"),
            subnet_id=config.get("subnet_id"),
            security_group_id=config.get("security_group_id"),
            key_pair_name=config.get("key_pair_name"),
            key_pair=key_pair,
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "instance_type": self.instance_type,
            "region": self.region,
            "ami_id": self.ami_id,
            "instance_id": self.instance_id,
            "vpc_id": self.vpc_id,
            "subnet_id": self.subnet_id,
            "security_group_id": self.security_group_id,
            "key_pair_name": self.key_pair_name,
            "key_pair": (
                {
                    "private_key": str(self.key_pair.private_key),
                    "public_key": str(self.key_pair.public_key),
                }
                if self.key_pair
                else None
            ),
        }


@define
class Agent(AbstractAgent):
    name: str = field(validator=validators.instance_of(str))
    config: Ec2Config = field(validator=validators.instance_of(Ec2Config))
    delete_on_exit: bool = field(default=True, validator=validators.instance_of(bool))
    config_dir: Path = field(init=False, validator=validators.instance_of(Path))
    instance_state: str = field(
        default="pending",
        validator=validators.and_(validators.instance_of(str), validators.in_(AWS_EC2_STATES)),
    )
    public_ip_address: IPv4Address | None = field(
        default=None,
        validator=validators.optional(validators.instance_of(IPv4Address)),
    )

    def __attrs_post_init__(self):
        self.config_dir = AGENT_DIRECTORY / self.name
        self.config_dir.mkdir(parents=True, exist_ok=True)

    @classmethod
    def from_config(cls, config_path: Path) -> "Agent":
        with open(file=config_path, encoding="utf-8") as config_file:
            payload = json.load(config_file)

        agent = cls(
            name=payload["name"],
            config=Ec2Config.from_dict(payload["config"]),
            delete_on_exit=payload.get("delete_on_exit", True),
            instance_state=payload.get("instance_state", "pending"),
        )
        if payload.get("config_dir"):
            agent.config_dir = Path(payload["config_dir"])
            agent.config_dir.mkdir(parents=True, exist_ok=True)
        if payload.get("public_ip_address"):
            agent.public_ip_address = IPv4Address(payload["public_ip_address"])
        return agent

    def destroy(self):
        """Destroy EC2 resources for this agent."""
        logger.debug(f"Destroying EC2 resources for {self.name}")
        if not self.config.instance_id:
            logger.debug(f"Agent {self.name} has no instance_id, skipping EC2 termination")
        else:
            boto3_client = boto3.client("ec2", region_name=self.config.region)
            logger.debug(f"Destroying instance {self.config.instance_id}")
            boto3_client.terminate_instances(InstanceIds=[self.config.instance_id])

            if self.config.key_pair_name:
                logger.debug(f"Destroying key pair {self.config.key_pair_name}")
                boto3_client.delete_key_pair(KeyName=self.config.key_pair_name)

        if self.config_dir.exists():
            logger.debug(f"Destroying config dir {self.config_dir}")
            shutil.rmtree(self.config_dir)

    def dump_config(self) -> Path:
        config_path = self.config_dir / AGENT_CONFIG_FILENAME
        with open(file=config_path, mode="w", encoding="utf-8") as config_file:
            json.dump(
                {
                    "name": self.name,
                    "config_dir": str(self.config_dir),
                    "public_ip_address": str(self.public_ip_address) if self.public_ip_address else None,
                    "instance_state": self.instance_state,
                    "delete_on_exit": self.delete_on_exit,
                    "config": self.config.to_dict(),
                },
                config_file,
                indent=4,
            )
        return config_path

    def __del__(self):
        if sys.meta_path is None:
            return
        if not getattr(self, "delete_on_exit", False):
            return
        if not getattr(self, "config", None):
            return
        if not self.config.instance_id:
            return
        self.destroy()

    def build(self):
        """Provision the EC2 instance and related resources."""
        logger.debug(f"Building EC2 for {self.name}")
        self.config.key_pair_name = f"ssh-key-{self.name}"
        self.config.key_pair = create_ssh_key_pair(output_dir=self.config_dir)
        create_ec2_key_pair(
            region=self.config.region,
            key_pair_name=self.config.key_pair_name,
            public_key_path=self.config.key_pair.public_key,
        )

        self.config.ami_id = get_ami_id(region=self.config.region)
        self.config.vpc_id = get_default_vpc_id(region=self.config.region)
        self.config.security_group_id = create_security_group(region=self.config.region, vpc_id=self.config.vpc_id)
        self.config.subnet_id = get_vpc_subnet_id(region=self.config.region, vpc_id=self.config.vpc_id)
        self.config.instance_id = create_ec2(
            region=self.config.region,
            ami_id=self.config.ami_id,
            instance_type=self.config.instance_type,
            key_pair_name=self.config.key_pair_name,
            subnet_id=self.config.subnet_id,
            security_group_id=self.config.security_group_id,
            name=self.name,
        )
        self.instance_state = "pending"

    def connection(self) -> Connection:
        """Create a fabric connection object."""
        if self.instance_state != "running" and self.get_state() != "running":
            raise ServerError(
                status_code=HTTPStatus.SERVICE_UNAVAILABLE,
                error="Agent error",
                detail=f"Agent {self.name} is not running. Current state: {self.get_state()}",
            )
        if not self.public_ip_address:
            raise ServerError(
                status_code=HTTPStatus.SERVICE_UNAVAILABLE,
                error="Agent error",
                detail=f"Agent {self.name} does not have a public IP address yet",
            )
        if not self.config.key_pair:
            raise ServerError(
                status_code=HTTPStatus.SERVICE_UNAVAILABLE,
                error="Agent error",
                detail=f"Agent {self.name} does not have an SSH key pair configured",
            )
        return Connection(
            host=str(self.public_ip_address),
            user="ec2-user",
            port=22,
            connect_timeout=SSH_TIMEOUT,
            connect_kwargs={
                "key_filename": str(self.config.key_pair.private_key),
            },
        )

    def get_state(self) -> str:
        """Returns the EC2 state"""
        boto3_client = boto3.client("ec2", region_name=self.config.region)
        instance_dict = boto3_client.describe_instances(InstanceIds=[self.config.instance_id])["Reservations"][0][
            "Instances"
        ][0]

        new_public_ip = IPv4Address(instance_dict["PublicIpAddress"]) if "PublicIpAddress" in instance_dict else None
        new_instance_state = instance_dict["State"]["Name"]
        if self.instance_state != new_instance_state or self.public_ip_address != new_public_ip:
            self.instance_state = new_instance_state
            self.public_ip_address = new_public_ip
            self.dump_config()
        return self.instance_state
