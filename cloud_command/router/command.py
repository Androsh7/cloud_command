"""Defines the command router"""

# Standard libraries
import asyncio

# Third-party libraries
from fastapi import APIRouter, Depends

# Project libraries
from cloud_command.agent.agent_manager import agent_manager
from cloud_command.agent.command_model import CommandModel, CommandResultModel
from cloud_command.auth import require_api_token

command_router = APIRouter(dependencies=[Depends(require_api_token)])


@command_router.post("/agent/{name}/command/run", tags=["Commands"])
async def run_command(name: str, command_model: CommandModel) -> CommandResultModel:
    agent = agent_manager.get_agent(name=name)
    stdout, stderr, exit_code = await asyncio.to_thread(
        agent.run_command, command_model.command, sudo=command_model.sudo
    )
    return CommandResultModel(stdout=stdout, stderr=stderr, exit_code=exit_code)
