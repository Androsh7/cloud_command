"""Defines the command functions"""

# Third-party libraries
from pydantic import BaseModel, Field


class CommandModel(BaseModel):
    command: str = Field(examples=["ls -la"])
    executable: str = Field(default="/bin/bash", examples=["/bin/bash", "/usr/bin/python3"])
    sudo: bool = Field(default=False, examples=[True, False])


class CommandResultModel(BaseModel):
    stdout: str = Field(
        examples=["total 0\ndrwxr-xr-x  2 user user 4096 Jan 1 00:00 .\ndrwxr-xr-x 18 user user 4096 Jan 1 00:00 .."]
    )
    stderr: str = Field(examples=["File not found /tmp/file.txt"])
    exit_code: int = Field(examples=[0])


class AgentStatusModel(BaseModel):
    uptime_seconds: float = Field(examples=[586.4])
    disk_usage: str = Field(examples=["1.1GB/10.11GB"])
    ram_usage: str = Field(examples=["0.5GB/4.0GB"])
    cpu_usage: str = Field(examples=["15%"])
