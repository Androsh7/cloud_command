"""Defines the command functions"""

# Third-party libraries
from pydantic import BaseModel, Field

# Project libraries


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


class AgentLocationModel(BaseModel):
    public_ip_address: str = Field(examples=["127.0.0.1"])
    hostname: str = Field(examples=["ip-127-0-0-1.ec2.internal"])
    city: str = Field(examples=["Seattle"])
    region: str = Field(examples=["New Jersey"])
    country: str = Field(examples=["United States"])
    loc: str = Field(examples=["10.0111, -20.0222"])
    org: str = Field(examples=["Amazon.com, Inc."])
    postal: str = Field(examples=["98101"])
    timezone: str = Field(examples=["America/Los_Angeles"])

    @classmethod
    def from_ipinfo_dict(cls, ipinfo_dict: dict):
        """Create an AgentLocationModel from an ipinfo.io response dictionary"""
        return cls(
            public_ip_address=ipinfo_dict.get("ip", ""),
            hostname=ipinfo_dict.get("hostname", ""),
            city=ipinfo_dict.get("city", ""),
            region=ipinfo_dict.get("region", ""),
            country=ipinfo_dict.get("country", ""),
            loc=ipinfo_dict.get("loc", ""),
            org=ipinfo_dict.get("org", ""),
            postal=ipinfo_dict.get("postal", ""),
            timezone=ipinfo_dict.get("timezone", ""),
        )


class AgentStatusModel(BaseModel):
    uptime_seconds: float = Field(examples=[586.4])
    location: AgentLocationModel
    disk_usage: str = Field(examples=["1.1GB/10.11GB"])
    ram_usage: str = Field(examples=["0.5GB/4.0GB"])
    cpu_usage: str = Field(examples=["15%"])
