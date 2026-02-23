"""Defines the command router"""

# Standard libraries
import asyncio

# Third-party libraries
from fastapi import APIRouter

# Project libraries
from cloud_command.agent.agent_manager import agent_manager
from cloud_command.agent.command_model import AgentStatusModel, CommandModel, CommandResultModel

command_router = APIRouter()


@command_router.post("/agent/{name}/command/run", tags=["Commands"])
async def run_command(name: str, command_model: CommandModel) -> CommandResultModel:
    agent = agent_manager.get_agent(name=name)
    stdout, stderr, exit_code = await asyncio.to_thread(
        agent.run_command, command_model.command, sudo=command_model.sudo
    )
    return CommandResultModel(stdout=stdout, stderr=stderr, exit_code=exit_code)


@command_router.post("/agent/{name}/command/statistics", tags=["Commands"])
async def get_command_status(name: str) -> AgentStatusModel:
    agent = agent_manager.get_agent(name=name)
    return await asyncio.to_thread(agent.get_statistics)
