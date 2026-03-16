"""Defines the shell session class"""

# Standard libraries
import threading
from typing import Literal
from uuid import UUID, uuid4

# Third-party libraries
from attrs import define, field, validators
from fabric import Connection
from pydantic import BaseModel, Field

# Project libraries
from cloud_command.agent.agent_abstract import AbstractAgent
from cloud_command.command.commands import (
    AgentStatusModel,
    get_statistics,
)

class TmuxSendKeys(BaseModel):
    """Model for sending commands to a tmux session"""
    send_command: str = Field(examples=["ls -lisah"])

class ShellSessionConfig(BaseModel):
    """Model for saving session configurations to a file"""
    uuid: UUID = field(validator=validators.instance_of(UUID))

    def to_dict(self) -> dict[str, str]:
        return {
            "uuid": str(self.uuid)
        }

@define
class ShellSession:
    agent: AbstractAgent = field(validator=validators.instance_of(AbstractAgent))
    uuid: UUID = field(factory=uuid4, validator=validators.instance_of(UUID))
    tmux_session_exists: bool = field(default=False, validator=validators.instance_of(bool))
    command_content: str = field(validator=validators.instance_of(str), init=False)
    command_connection: Connection = field(validator=validators.instance_of(Connection), init=False)
    command_lock: threading.Lock = field(validator=validators.instance_of(threading.Lock), init=False)
    polling_connection: Connection = field(validator=validators.instance_of(Connection), init=False)
    polling_lock: threading.Lock = field(validator=validators.instance_of(threading.Lock), init=False)

    def __attrs_post_init__(self):
        self.command_lock = threading.Lock()
        self.command_content = ""
        self.command_connection = self.agent.connection()
        if not self.tmux_session_exists:
            self.command_connection.run("sudo yum install -y tmux || sudo dnf install -y tmux || sudo apt-get install -y tmux", hide="both")
            self.command_connection.run(f"tmux new-session -d -s {self.uuid}", hide="both")
        self.polling_lock = threading.Lock()
        self.polling_connection = self.agent.connection()

    @classmethod
    def from_config(cls, agent: AbstractAgent, config: ShellSessionConfig):
        return cls(
            agent=agent,
            uuid=config.uuid,
            tmux_session_exists=True,
        )

    def to_config(self) -> ShellSessionConfig:
        return ShellSessionConfig(
            uuid=self.uuid
        )

    def auto_renew_connection(self, connection_name: Literal["polling", "command"], force: bool = False) -> Connection:
        current_connection = getattr(self, f"{connection_name}_connection")
        if current_connection.is_connected and not force:
            return current_connection

        # Renew connection
        setattr(self, f"{connection_name}_connection", self.agent.connection())
        return getattr(self, f"{connection_name}_connection")

    def session_send_command(self, request: TmuxSendKeys):
        with self.command_lock:
            conn = self.auto_renew_connection(connection_name='command')
            send_keys = request.send_command.replace('"', '\\"').replace("$", "\\$")
            conn.run(f'tmux send-keys -t {self.uuid} "{send_keys}" C-m')

    def session_send_ctrl_c(self):
        with self.command_lock:
            conn = self.auto_renew_connection(connection_name='command')
            conn.run(f"tmux send-keys -t {self.uuid} C-c")

    def session_get_output(self) -> tuple[bool, str]:
        """Returns a boolean for if the data has changed, then the content of stdout"""
        with self.command_lock:
            conn = self.auto_renew_connection(connection_name='command')
            result = conn.run(f"tmux capture-pane -t {self.uuid} -p -J -S -", hide=True).stdout.strip()
            has_change = not (result == self.command_content)
            self.command_content = result
            return has_change, result

    def session_get_statistics(self) -> AgentStatusModel:
        """Returns the agent statistics (cpu, ram, disk)"""
        with self.polling_lock:
            return get_statistics(conn=self.auto_renew_connection(connection_name="polling"))

    def session_destroy(self):
        if getattr(self, "command_connection", None) is not None and self.command_connection.is_connected:
            self.command_connection.run(f"tmux kill-session -t {self.uuid}", warn=True)
            self.command_connection.close()
        if getattr(self, "polling_connection", None) is not None and self.polling_connection.is_connected:
            self.polling_connection.close()

    def __del__(self):
        self.session_destroy()

