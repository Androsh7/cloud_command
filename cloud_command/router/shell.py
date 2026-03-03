"""Defines the shell router"""

# Standard libraries
import asyncio
from http import HTTPStatus
from uuid import UUID, uuid4

# Third-party libraries
from fastapi import APIRouter, Response
from pydantic import BaseModel, Field

# Project libraries
from cloud_command.command.commands import AgentStatusModel
from cloud_command.command.shell_manager import ShellSession, TmuxSendKeys, shell_session_manager
from cloud_command.router.error_model import ErrorResponse

shell_router = APIRouter(tags=["Shell"])


class ShellSessionModel(BaseModel):
    agent_name: str = Field(examples=["agent-01"])
    uuid: UUID = Field(examples=[uuid4()])

    @classmethod
    def from_shell_session(cls, shell_session: ShellSession):
        return cls(
            agent_name=shell_session.agent.name,
            uuid=shell_session.uuid,
        )


@shell_router.get("/shell/list")
async def list_shell_sessions() -> list[ShellSessionModel]:
    out_list = []
    for shell_session in shell_session_manager.shell_sessions:
        out_list.append(ShellSessionModel.from_shell_session(shell_session))
    return out_list


class CreateShellSessionModel(BaseModel):
    agent: str


@shell_router.post(
    "/shell/create",
    status_code=HTTPStatus.CREATED,
    responses={
        HTTPStatus.BAD_REQUEST: {"model": ErrorResponse},
        HTTPStatus.CONFLICT: {"model": ErrorResponse},
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
    },
)
async def create_shell_session(request: CreateShellSessionModel) -> UUID:
    return shell_session_manager.create_session(agent_name=request.agent)


@shell_router.delete(
    "/shell/{uuid}",
    status_code=HTTPStatus.NO_CONTENT,
    responses={
        HTTPStatus.NO_CONTENT: {"description": "Shell session deleted", "content": {}},
        HTTPStatus.BAD_REQUEST: {"model": ErrorResponse},
        HTTPStatus.NOT_FOUND: {"model": ErrorResponse},
        HTTPStatus.CONFLICT: {"model": ErrorResponse},
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
    },
)
async def delete_shell_session(uuid: UUID):
    shell_session_manager.delete_session(uuid)
    return Response(status_code=HTTPStatus.NO_CONTENT)


@shell_router.post(
    "/shell/{uuid}/send_command",
    responses={
        HTTPStatus.CREATED: {"detail": "Sent command", "content": {}},
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
        HTTPStatus.NOT_FOUND: {"model": ErrorResponse},
    },
    status_code=HTTPStatus.CREATED,
)
async def run_command_shell_session(uuid: UUID, command: TmuxSendKeys) -> Response:
    shell_session = shell_session_manager.get_session(uuid=uuid)
    await asyncio.to_thread(shell_session.session_send_command, command)
    return Response(status_code=HTTPStatus.CREATED)


@shell_router.get(
    "/shell/{uuid}/get_output",
    responses={
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
        HTTPStatus.NOT_FOUND: {"model": ErrorResponse},
    },
)
async def get_output_shell_session(uuid: UUID) -> str:
    shell_session = shell_session_manager.get_session(uuid=uuid)
    return await asyncio.to_thread(shell_session.session_get_output)


@shell_router.post(
    "/shell/{uuid}/get_statistics",
    responses={
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
        HTTPStatus.NOT_FOUND: {"model": ErrorResponse},
    },
)
async def get_statistics_shell_session(uuid: UUID) -> AgentStatusModel:
    shell_session = shell_session_manager.get_session(uuid=uuid)
    return await asyncio.to_thread(shell_session.session_get_statistics)
