"""Defines the abstract class for an Agent"""

# Standard libraries
from abc import ABC, abstractmethod
from pathlib import Path

# Third-party libraries
from fabric import Connection


class AbstractAgent(ABC):
    @classmethod
    @abstractmethod
    def from_config(cls, config_path: Path):
        """Create object from a config file"""
        pass

    @abstractmethod
    def dump_config() -> Path:
        """Dump config to a file"""
        pass

    @abstractmethod
    def destroy():
        """Destroy the associated resources"""
        pass

    @abstractmethod
    def build():
        """Provision the resources to create the agent"""
        pass

    @abstractmethod
    def connection() -> Connection:
        """Return a fabric connection object"""
        pass
