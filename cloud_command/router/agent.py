"""Defines the agent router."""

# Standard libraries
import asyncio
from http import HTTPStatus
from pathlib import Path

# Third-party libraries
from fastapi import APIRouter
from pydantic import BaseModel, Field

# Project libraries
from cloud_command.agent.agent_manager import agent_manager
from cloud_command.router.error_model import ErrorResponse

cluster_router = APIRouter()


class SshKeyPairModel(BaseModel):
    private_key: Path = Field(examples=["/path/to/private/key.pem"])
    public_key: Path = Field(examples=["/path/to/public/key.pub"])


class AgentConfigModel(BaseModel):
    instance_type: str = Field(examples=["t4g.nano"])
    region: str = Field(examples=["us-east-2"])
    ami_id: str = Field(examples=["ami-0c55b159cbfafe1f0"])
    instance_id: str = Field(examples=["i-1234567890abcdef0"])
    vpc_id: str = Field(examples=["vpc-12345678"])
    subnet_id: str = Field(examples=["subnet-12345678"])
    security_group_id: str = Field(examples=["sg-12345678"])
    key_pair_name: str = Field(examples=["my-key-pair"])
    key_pair: SshKeyPairModel

    @classmethod
    def from_dict(cls, config: dict):
        return cls(
            instance_type=config["instance_type"],
            region=config["region"],
            ami_id=config.get("ami_id"),
            instance_id=config.get("instance_id"),
            vpc_id=config.get("vpc_id"),
            subnet_id=config.get("subnet_id"),
            security_group_id=config.get("security_group_id"),
            key_pair_name=config.get("key_pair_name"),
            key_pair=SshKeyPairModel(
                private_key=Path(config["key_pair"]["private_key"]),
                public_key=Path(config["key_pair"]["public_key"]),
            ),
        )


class AgentModel(BaseModel):
    name: str = Field(examples=["agent-1"])
    state: str = Field(examples=["pending", "running"])
    public_ip_address: str | None = Field(default=None, examples=["127.0.0.1"])
    config: AgentConfigModel


@cluster_router.get("/agent/list", tags=["Agents"])
async def get_cluster_list() -> list[AgentModel]:
    return [
        AgentModel(
            name=agent.name,
            state=agent.instance_state,
            public_ip_address=str(agent.public_ip_address) if agent.public_ip_address else None,
            config=AgentConfigModel.from_dict(agent.config.to_dict()),
        )
        for agent in agent_manager.agent_list
    ]


class CreateEC2AgentModel(BaseModel):
    region: str = Field(examples=["us-east-2"])
    instance_type: str = Field(examples=["t4g.nano"])


@cluster_router.post(
    "/agent/{name}/create",
    tags=["Agents"],
    responses={
        HTTPStatus.CONFLICT: {"model": ErrorResponse},
    },
)
async def create_ec2_agent(name: str, create_ec2_model: CreateEC2AgentModel):
    await agent_manager.create_ec2_agent(
        name=name,
        instance_type=create_ec2_model.instance_type,
        region=create_ec2_model.region,
    )


@cluster_router.delete("/agent/{name}/delete", tags=["Agents"])
async def delete_agent(name: str):
    await agent_manager.delete_agent(name=name)


@cluster_router.get(
    "/agent/{name}/state",
    tags=["Agents"],
    responses={
        HTTPStatus.NOT_FOUND: {"model": ErrorResponse},
    },
)
async def get_agent_state(name: str) -> str:
    agent = agent_manager.get_agent(name=name)
    return await asyncio.to_thread(agent.get_state)
