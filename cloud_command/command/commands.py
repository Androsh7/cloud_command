"""Defines the command functions"""

# Standard libraries
import mimetypes
import re
from http import HTTPStatus
from pathlib import PurePosixPath

# Third-party libraries
from fabric import Connection
from fastapi import UploadFile
from loguru import logger
from pydantic import BaseModel, Field

# Project libraries
from cloud_command.command.file_model import FileUploadModel
from cloud_command.router.error_model import ServerError
from cloud_command.utils import encode_script


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


def run_command(conn: Connection, command: CommandModel) -> CommandResultModel:
    """Run a command on the EC2 instance and returns stdout, stderr, and exit code"""
    result = conn.run(
        encode_script(command.command, executable=command.executable, sudo=command.sudo), hide=True, warn=True
    )
    return CommandResultModel(stdout=result.stdout.strip(), stderr=result.stderr.strip(), exit_code=result.return_code)


def get_statistics(conn: Connection):
    """Get the current status of the agent"""
    # Get uptime
    uptime_seconds = float(conn.run("cat /proc/uptime", hide=True).stdout.strip().split(" ")[0])

    # Get disk usage
    disk_usage_output = conn.run("df / | tail -n 1", hide=True).stdout.strip()
    disk_usage_matches = re.search(r"(\d+) +(\d+) +(\d+) +(\d+)%", disk_usage_output)
    used_disk = int(disk_usage_matches.group(2))
    total_disk = int(disk_usage_matches.group(1))
    disk_usage = f"{used_disk / 1024 / 1024:.2f}GB/{total_disk / 1024 / 1024:.2f}GB"

    # Get RAM usage
    ram_usage_output = conn.run("cat /proc/meminfo", hide=True).stdout.strip()
    ram_usage_matches = re.search(r"MemTotal: +(\d+) \w+[\n.]+MemFree: +(\d+) \w+", ram_usage_output)
    ram_total = int(ram_usage_matches.group(1))
    ram_free = int(ram_usage_matches.group(2))
    ram_usage = f"{ram_free / 1024 / 1024:.2f}GB/{ram_total / 1024 / 1024:.2f}GB"

    # Get CPU usage
    cpu_usage_output = conn.run("top -bn1 | grep '%Cpu'", hide=True).stdout.strip()
    cpu_usage_matches = re.search(r"(\d+(\.\d+)?) id", cpu_usage_output)
    cpu_usage = f"{100 - float(cpu_usage_matches.group(1)):.2f}%"

    return AgentStatusModel(
        uptime_seconds=uptime_seconds,
        disk_usage=disk_usage,
        ram_usage=ram_usage,
        cpu_usage=cpu_usage,
    )


def upload_file(conn: Connection, file: UploadFile, destination_path: str) -> FileUploadModel:
    """Upload a file to the EC2 instance."""
    file.file.seek(0)

    # Fix upload path
    if not destination_path.startswith("/"):
        raise ServerError(
            status_code=HTTPStatus.BAD_REQUEST,
            error="Input error",
            detail=f"Destination path must be absolute. Received: {destination_path}",
        )
    if destination_path.endswith("/"):
        destination_path = f"{destination_path.rstrip('/')}/{file.filename}"

    try:
        conn.put(file.file, destination_path)
    except OSError as exc:
        raise ServerError(
            status_code=HTTPStatus.INTERNAL_SERVER_ERROR,
            error="Network error",
            detail=f"Upload failed for '{destination_path}': {exc}",
        ) from exc

    return FileUploadModel(
        destination_path=destination_path,
        file_size_bytes=file.size or 0,
        mime_type=file.content_type or "application/octet-stream",
    )


def download_file(conn: Connection, source_path: str) -> tuple[bytes, str, str]:
    """Download a file from the EC2 instance and return bytes, filename, and mime type."""
    if not source_path.startswith("/"):
        raise ServerError(
            status_code=HTTPStatus.BAD_REQUEST,
            error="Input error",
            detail=f"Source path must be absolute. Received: {source_path}",
        )
    if source_path.endswith("/"):
        raise ServerError(
            status_code=HTTPStatus.BAD_REQUEST,
            error="Input error",
            detail=f"Source path must reference a file. Received: {source_path}",
        )

    filename = PurePosixPath(source_path).name
    mime_type = mimetypes.guess_type(filename)[0] or "application/octet-stream"

    try:
        with conn.sftp() as sftp, sftp.open(source_path, "rb") as remote_file:
            payload = remote_file.read()
    except OSError as exc:
        raise ServerError(
            status_code=HTTPStatus.INTERNAL_SERVER_ERROR,
            error="Network error",
            detail=f"Download failed for '{source_path}': {exc}",
        ) from exc

    return payload, filename, mime_type
