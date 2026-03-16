"""Defines the shell router"""

# Standard libraries
import asyncio
from http import HTTPStatus
from uuid import UUID, uuid4

# Third-party libraries
from fastapi import APIRouter, Response
from pydantic import BaseModel, Field

# Project libraries
from cloud_command.agent.agent_manager import agent_manager
from cloud_command.command.commands import AgentStatusModel
from cloud_command.command.shell_session import ShellSession, TmuxSendKeys
from cloud_command.router.error_model import ErrorResponse, ServerError

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
    for shell_session in agent_manager.get_session_list():
        out_list.append(ShellSessionModel.from_shell_session(shell_session))
    return out_list


class CreateShellSessionModel(BaseModel):
    agent: str


@shell_router.post(
    "/shell/create",
    status_code=HTTPStatus.CREATED,
    responses={
        HTTPStatus.NOT_FOUND: {"model": ErrorResponse},
        HTTPStatus.BAD_REQUEST: {"model": ErrorResponse},
        HTTPStatus.CONFLICT: {"model": ErrorResponse},
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
    },
)
async def create_shell_session(request: CreateShellSessionModel) -> UUID:
    agent = agent_manager.get_agent(request.agent)
    if agent.public_ip_address is None:
        raise ServerError(
            status_code=HTTPStatus.SERVICE_UNAVAILABLE,
            error="EC2 unavailable",
            details="EC2 is still pending an IP address assignment",
        )
    return await asyncio.to_thread(agent_manager.create_session, request.agent)


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
    await asyncio.to_thread(agent_manager.delete_session, uuid)
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
    shell_session = agent_manager.get_session(uuid=uuid)
    await asyncio.to_thread(shell_session.session_send_command, command)
    return Response(status_code=HTTPStatus.CREATED)


@shell_router.post(
    "/shell/{uuid}/send_command_ctrl_c",
    responses={
        HTTPStatus.CREATED: {"detail": "Sent command", "content": {}},
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
        HTTPStatus.NOT_FOUND: {"model": ErrorResponse},
    },
    status_code=HTTPStatus.CREATED,
)
async def run_command_ctrl_c_shell_session(uuid: UUID) -> Response:
    shell_session = agent_manager.get_session(uuid=uuid)
    await asyncio.to_thread(shell_session.session_send_ctrl_c)
    return Response(status_code=HTTPStatus.CREATED)


@shell_router.get(
    "/shell/{uuid}/get_output",
    responses={
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
        HTTPStatus.NOT_FOUND: {"model": ErrorResponse},
    },
)
async def get_output_shell_session(uuid: UUID, show_existing: bool = False) -> str | None:
    shell_session = agent_manager.get_session(uuid=uuid)
    has_change, content = await asyncio.to_thread(shell_session.session_get_output)
    if has_change or show_existing:
        return content
    return None


@shell_router.post(
    "/shell/{uuid}/get_statistics",
    responses={
        HTTPStatus.SERVICE_UNAVAILABLE: {"model": ErrorResponse},
        HTTPStatus.INTERNAL_SERVER_ERROR: {"model": ErrorResponse},
        HTTPStatus.NOT_FOUND: {"model": ErrorResponse},
    },
)
async def get_statistics_shell_session(uuid: UUID) -> AgentStatusModel:
    shell_session = agent_manager.get_session(uuid=uuid)
    return await asyncio.to_thread(shell_session.session_get_statistics)
