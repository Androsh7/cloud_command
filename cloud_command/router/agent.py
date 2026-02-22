"""Defines the cluster router"""

# Third-party libraries
from fastapi import APIRouter
from pydantic import BaseModel, Field

# Project libraries
from cloud_command.agents.agent_manager import agent_manager
from cloud_command.constants import AGENT_STATES, AGENT_TYPE

cluster_router = APIRouter()


class AgentModel(BaseModel):
    name: str = Field(examples=["agent-1"])
    type: str = Field(examples=[*AGENT_TYPE])
    state: str = Field(examples=[*AGENT_STATES])


@cluster_router.get("/agent/list", tags=["Agents"])
async def get_cluster_list() -> list[AgentModel]:
    out_list = []
    for agent in agent_manager.agent_list:
        out_list.append(
            AgentModel(
                name=agent.name,
                type=agent.type.name,
                state=agent.state.name,
            )
        )
    return out_list


class CreateEC2AgentModel(BaseModel):
    name: str = Field(examples=["ec2-1"])
    region: str = Field(examples=["us-east-2"])
    instance_type: str = Field(examples=["t4g.nano"])


@cluster_router.post("/agent/create/ec2", tags=["Agents"])
async def create_ec2_agent(create_ec2_model: CreateEC2AgentModel):
    await agent_manager.create_ec2_agent(
        name=create_ec2_model.name,
        instance_type=create_ec2_model.instance_type,
        region=create_ec2_model.region,
    )


@cluster_router.delete("/agent/delete", tags=["Agents"])
async def delete_agent(name: str):
    await agent_manager.delete_agent(name=name)
