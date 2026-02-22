"""Defines the agent router."""

# Third-party libraries
from fastapi import APIRouter
from pydantic import BaseModel, Field

# Project libraries
from cloud_command.agents.agent_manager import agent_manager

cluster_router = APIRouter()


class AgentModel(BaseModel):
    name: str = Field(examples=["agent-1"])
    state: str = Field(examples=["pending", "running"])
    public_ip_address: str | None = Field(default=None, examples=["127.0.0.1"])
    config: dict | None = Field(default=None, examples=[{"instance_type": "t4g.nano", "region": "us-east-2"}])


@cluster_router.get("/agent/list", tags=["Agents"])
async def get_cluster_list() -> list[AgentModel]:
    return [
        AgentModel(
            name=agent.name,
            state=agent.instance_state,
            ip_address=str(agent.public_ip_address) if agent.public_ip_address else None,
            config=agent.config.to_dict(),
        )
        for agent in agent_manager.agent_list
    ]


class CreateEC2AgentModel(BaseModel):
    name: str = Field(examples=["ec2-1"])
    region: str = Field(examples=["us-east-2"])
    instance_type: str = Field(examples=["t4g.nano"])


@cluster_router.post("/agent/create/ec2", tags=["Agents"])
async def create_ec2_agent(create_ec2_model: CreateEC2AgentModel) -> None:
    await agent_manager.create_ec2_agent(
        name=create_ec2_model.name,
        instance_type=create_ec2_model.instance_type,
        region=create_ec2_model.region,
    )


@cluster_router.delete("/agent/delete", tags=["Agents"])
async def delete_agent(name: str) -> None:
    await agent_manager.delete_agent(name=name)
