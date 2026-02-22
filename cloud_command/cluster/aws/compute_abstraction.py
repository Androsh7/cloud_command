"""Defines the abstract RemoteCompute class"""

# Standard libraries
from abc import ABC, abstractmethod

# Third-party libraries
from fabric import Connection

class RemoteCompute(ABC):
    @abstractmethod
    def connection() -> Connection:
        """Returns a Fabric connection object"""
        pass

    @abstractmethod
    def is_built() -> bool:
        """Returns True if the instance is built, otherwise it returns False"""
        pass
