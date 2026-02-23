"""Defines the command router"""

# Standard libraries
import asyncio
from io import BytesIO

# Third-party libraries
from fastapi import APIRouter, File, UploadFile
from fastapi.responses import StreamingResponse

# Project libraries
from cloud_command.agent.agent_manager import agent_manager
from cloud_command.agent.command_model import AgentStatusModel, CommandModel, CommandResultModel
from cloud_command.agent.file_model import FileUploadModel

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


@command_router.post("/agent/{name}/command/upload", tags=["Commands"])
async def upload_file(name: str, path: str, file: UploadFile = File(...)) -> FileUploadModel:
    agent = agent_manager.get_agent(name=name)
    return await asyncio.to_thread(agent.upload_file, file=file, destination_path=path)


@command_router.get("/agent/{name}/command/download", tags=["Commands"])
async def download_file(name: str, path: str) -> StreamingResponse:
    agent = agent_manager.get_agent(name=name)
    payload, filename, mime_type = await asyncio.to_thread(agent.download_file, source_path=path)
    return StreamingResponse(
        content=BytesIO(payload),
        media_type=mime_type,
        headers={"Content-Disposition": f'attachment; filename="{filename}"'},
    )
