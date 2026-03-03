"""Defines the command router"""

# Standard libraries
import asyncio
from http import HTTPStatus
from io import BytesIO

# Third-party libraries
from fastapi import APIRouter, File, UploadFile
from fastapi.responses import StreamingResponse

# Project libraries
from cloud_command.agent.agent_manager import agent_manager
from cloud_command.command.commands import (
    AgentStatusModel,
    CommandModel,
    CommandResultModel,
    download_file,
    get_statistics,
    run_command,
    upload_file,
)
from cloud_command.command.file_model import FileUploadModel
from cloud_command.router.error_model import ErrorResponse

command_router = APIRouter(tags=["Commands"])


@command_router.post(
    "/agent/{name}/command/run_command",
    responses={
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
    },
)
async def agent_run_command(name: str, command_model: CommandModel) -> CommandResultModel:
    agent = agent_manager.get_agent(name=name)
    with agent.connection() as conn:
        return await asyncio.to_thread(run_command, conn, command_model)


@command_router.get(
    "/agent/{name}/command/statistics",
    responses={
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
    },
)
async def agent_command_status(name: str) -> AgentStatusModel:
    agent = agent_manager.get_agent(name=name)
    with agent.connection() as conn:
        return await asyncio.to_thread(get_statistics, conn=conn)


@command_router.post(
    "/agent/{name}/command/upload",
    responses={
        HTTPStatus.BAD_REQUEST: {"model": ErrorResponse},
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
    },
)
async def agent_upload_file(name: str, path: str, file: UploadFile = File(...)) -> FileUploadModel:
    agent = agent_manager.get_agent(name=name)
    with agent.connection() as conn:
        return await asyncio.to_thread(upload_file, conn=conn, file=file, destination_path=path)


@command_router.get(
    "/agent/{name}/command/download",
    responses={
        HTTPStatus.BAD_REQUEST: {"model": ErrorResponse},
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
    },
)
async def agent_download_file(name: str, path: str) -> StreamingResponse:
    agent = agent_manager.get_agent(name=name)
    with agent.connection() as conn:
        payload, filename, mime_type = await asyncio.to_thread(download_file, conn=conn, source_path=path)
    return StreamingResponse(
        content=BytesIO(payload),
        media_type=mime_type,
        headers={"Content-Disposition": f'attachment; filename="{filename}"'},
    )
