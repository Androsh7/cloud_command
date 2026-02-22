"""Defines the command functions"""

# Third-party libraries
import asyncio
from pydantic import BaseModel, Field

# Project libraries
from cloud_command.agent.agent import Agent


class CommandModel(BaseModel):
    command: str = Field(examples=["ls -la"])
    sudo: bool = Field(default=False, examples=[True, False])

class CommandResultModel(BaseModel):
    stdout: str = Field(examples=["total 0\ndrwxr-xr-x  2 user user 4096 Jan 1 00:00 .\ndrwxr-xr-x 18 user user 4096 Jan 1 00:00 .."])
    stderr: str = Field(examples=["File not found /tmp/file.txt"])
    exit_code: int = Field(examples=[0])
