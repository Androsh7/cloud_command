"""Defines the command functions"""

# Third-party libraries
from pydantic import BaseModel, Field

# Project libraries


class CommandModel(BaseModel):
    command: str = Field(examples=["ls -la"])
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
    status: str = Field(examples=["Agent is up 00:00:00 up 1 day,  2:34,  1 user,  load average: 0.00, 0.01, 0.05"])
    location: AgentLocationModel
    disk_usage: str = Field(
        examples=[
            "Filesystem      Size  Used Avail Use% Mounted on\n/dev/root        30G  1.5G   28G   5% /\ndevtmpfs        7.8G     0  7.8G   0% /dev\ntmpfs           7.8G     0  7.8G   0% /dev/shm\ntmpfs           7.8G   17M  7.8G   1% /run\ntmpfs           7.8G     0  7.8G   0% /sys/fs/cgroup"
        ]
    )
    ram_usage: str = Field(
        examples=[
            "              total        used        free      shared  buff/cache   available\nMem:           7.8G        1.2G        5.0G         17M        1.6G        6.3G\nSwap:          2.0G          0B        2.0G"
        ]
    )
    cpu_usage: str = Field(examples=["%Cpu(s):  1.0 us,  0.5 sy,  0.0 ni, 98.0 id,  0.5 wa,  0.0 hi,  0.0 si,  0.0 st"])
