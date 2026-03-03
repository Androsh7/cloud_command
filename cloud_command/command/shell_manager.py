"""Defines the shell manager class"""

# Standard libraries
import threading
from http import HTTPStatus
from typing import Literal
from uuid import UUID, uuid4

# Third-party libraries
from attrs import define, field, validators
from fabric import Connection
from loguru import logger
from pydantic import BaseModel, Field

# Project libraries
from cloud_command.agent.agent import Agent
from cloud_command.agent.agent_manager import agent_manager
from cloud_command.command.commands import (
    AgentStatusModel,
    get_statistics,
)
from cloud_command.constants import SHELL_SESSION_POOL_SIZE
from cloud_command.router.error_model import ServerError


class TmuxSendKeys(BaseModel):
    send_command: str = Field(examples=["ls -lisah"])


@define
class ShellSession:
    agent: Agent = field(validator=validators.instance_of(Agent))
    uuid: UUID = field(validator=validators.instance_of(UUID), init=False)
    tmux_exists: bool = field(validator=validators.instance_of(bool), init=False)
    command_connection: Connection = field(validator=validators.instance_of(Connection), init=False)
    command_lock: threading.Lock = field(validator=validators.instance_of(threading.Lock), init=False)
    polling_connection: Connection = field(validator=validators.instance_of(Connection), init=False)
    polling_lock: threading.Lock = field(validator=validators.instance_of(threading.Lock), init=False)

    def __attrs_post_init__(self):
        self.uuid = uuid4()
        self.command_lock = threading.Lock()
        self.command_connection = self.agent.connection()
        self.command_connection.run(
            "sudo yum install -y tmux || sudo dnf install -y tmux || sudo apt-get install -y tmux", hide="both"
        )
        self.tmux_exists = False
        self.polling_lock = threading.Lock()
        self.polling_connection = self.agent.connection()

    def auto_create_tmux_session(self) -> Connection:
        if not self.tmux_exists:
            self.command_connection.run(f"tmux new-session -d -s {self.uuid}")
            self.tmux_exists = True
        return self.command_connection

    def auto_renew_connection(self, connection_name: Literal["polling", "command"], force: bool = False) -> Connection:
        current_connection = getattr(self, f"{connection_name}_connection")
        if current_connection.is_connected and not force:
            return current_connection

        # Renew connection
        setattr(self, f"{connection_name}_connection", self.agent.connection())
        return getattr(self, f"{connection_name}_connection")

    def session_send_command(self, request: TmuxSendKeys):
        with self.command_lock:
            conn = self.auto_create_tmux_session()
            send_keys = request.send_command.replace('"', '\\"').replace("$", "\\$")
            conn.run(f'tmux send-keys -t {self.uuid} "{send_keys}" C-m')

    def session_send_ctrl_c(self):
        with self.command_lock:
            conn = self.auto_create_tmux_session()
            conn.run(f'tmux send-keys -t {self.uuid} C-c')

    def session_get_output(self) -> str:
        with self.command_lock:
            conn = self.auto_create_tmux_session()
            result = conn.run(f"tmux capture-pane -t {self.uuid} -p -S -", hide=True)
            return result.stdout.strip()

    def session_get_statistics(self) -> AgentStatusModel:
        with self.polling_lock:
            return get_statistics(conn=self.auto_renew_connection(connection_name="polling"))

    def session_destroy(self):
        self.command_connection.run(f"tmux kill-session -t {self.uuid}", warn=True)
        self.command_connection.close()
        self.polling_connection.close()

    def __del__(self):
        self.session_destroy()


@define
class ShellSessionManager:
    lock: threading.Lock = field(validator=validators.instance_of(threading.Lock), init=False)
    pool_size: int = field(validator=validators.and_(validators.instance_of(int), validators.ge(1)))
    shell_sessions: list[ShellSession] = field(
        validator=validators.deep_iterable(
            member_validator=validators.instance_of(ShellSession), iterable_validator=validators.instance_of(list)
        ),
        init=False,
    )

    def __attrs_post_init__(self):
        self.lock = threading.Lock()
        self.shell_sessions = []

    def create_session(self, agent_name: str) -> UUID:
        with self.lock:
            agent = agent_manager.get_agent(agent_name)
            if len(self.shell_sessions) >= self.pool_size:
                raise ServerError(
                    status_code=HTTPStatus.CONFLICT,
                    error="Resource constraint",
                    details=f"Too many sessions exist, current limit is {self.pool_size}",
                )
            shell_session = ShellSession(agent=agent)
            logger.debug(f"Created shell session {shell_session.uuid}")
            self.shell_sessions.append(shell_session)
            return shell_session.uuid

    def get_session(self, uuid: UUID) -> ShellSession:
        for session in self.shell_sessions:
            if session.uuid == uuid:
                return session
        raise ServerError(
            status_code=HTTPStatus.NOT_FOUND, error="Key Error", details=f"No session found with UUID: {uuid}"
        )

    def delete_session(self, uuid: UUID):
        with self.lock:
            for index, session in enumerate(self.shell_sessions):
                if session.uuid == uuid:
                    del self.shell_sessions[index]
                    return None
        raise ServerError(
            status_code=HTTPStatus.NOT_FOUND, error="Key Error", details=f"No session found with UUID: {uuid}"
        )


shell_session_manager = ShellSessionManager(pool_size=SHELL_SESSION_POOL_SIZE)
