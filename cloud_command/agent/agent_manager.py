"""Defines the AgentManager object."""

# Standard libraries
import asyncio
import threading
from http import HTTPStatus

# Third-party libraries
from attrs import define, field
from fastapi import HTTPException
from loguru import logger

# Project libraries
from cloud_command.agent.agent import Agent, Ec2Config
from cloud_command.constants import AGENT_CONFIG_FILENAME, AGENT_DIRECTORY


@define
class AgentManager:
    agent_list: list[Agent] = field(factory=list, init=False)
    _lock: threading.Lock = field(init=False, repr=False)

    def __attrs_post_init__(self) -> None:
        self._lock = threading.Lock()

    def load_all_agents(self) -> None:
        with self._lock:
            self.agent_list.clear()
            for config_path in AGENT_DIRECTORY.glob(f"*/{AGENT_CONFIG_FILENAME}"):
                try:
                    agent = Agent.from_config(config_path)
                    self.agent_list.append(agent)
                    logger.debug(f"Loaded agent with name {agent.name}")
                except Exception:
                    error_log = f"Failed to load agent config from {config_path}"
                    logger.error(error_log)
                    raise HTTPException(status_code=HTTPStatus.INTERNAL_SERVER_ERROR, detail=error_log)

    def get_agent(self, name: str) -> Agent:
        with self._lock:
            for agent in self.agent_list:
                if agent.name == name:
                    return agent
        raise HTTPException(status_code=HTTPStatus.NOT_FOUND, detail=f"Could not find agent with name {name}")

    async def delete_agent(self, name: str) -> None:
        """Destroy the selected agent."""
        agent = self.get_agent(name)
        await asyncio.to_thread(agent.destroy)
        agent.delete_on_exit = False
        with self._lock:
            self.agent_list = [existing_agent for existing_agent in self.agent_list if existing_agent.name != name]

    async def update_all(self) -> None:
        """Update the status of all agents."""
        await asyncio.gather(*(asyncio.to_thread(agent.get_state) for agent in self.agent_list))

    async def create_ec2_agent(self, name: str, instance_type: str, region: str) -> Agent:
        """Create and persist a new EC2-backed agent."""
        with self._lock:
            if any(existing_agent.name == name for existing_agent in self.agent_list):
                raise HTTPException(status_code=HTTPStatus.CONFLICT, detail=f"Agent with name {name} already exists")

        agent = Agent(name=name, config=Ec2Config(instance_type=instance_type, region=region))
        await asyncio.to_thread(agent.build)
        agent.dump_config()

        with self._lock:
            self.agent_list.append(agent)
        return agent

agent_manager = AgentManager()
