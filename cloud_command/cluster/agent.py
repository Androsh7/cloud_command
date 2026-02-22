"""Defines the agent class"""

# Standard libraries
import re

# Third-party libraries
from attrs import define, field, validators

# Project libraries
from cluster.aws.compute_abstraction import RemoteCompute

@define
class Agent:
    remote_compute: RemoteCompute = field(validator=validators.instance_of(RemoteCompute))

    async def get_statistics(self) -> str:
        with self.remote_compute.connection() as conn:
            # Calculate disk usage
            disk_usage_result = re.sub(r' +','',conn.run("df / | tail -n 1").stdout.strip()).split(" ")
            disk_used_bytes = disk_usage_result[2]
            disk_available_bytes = disk_usage_result[3]

        return f'Statistics:\nDisk usage: {disk_used_bytes}/{disk_used_bytes + disk_available_bytes}'
