"""Defines the MCP server for CloudCommand"""

# Third-party libraries
from mcp.server.fastmcp import FastMCP

# Project libraries
from cloud_command.router.agent import get_cluster_list, get_agent_state, create_ec2_agent, delete_agent
from cloud_command.router.command import agent_run_command, agent_command_status

mcp = FastMCP(name="CloudCommand", streamable_http_path="/", instructions="""\
You are an agent for creating, managing, and utilizing short-lived EC2 instances

Rules:
1. Minimize the EC2 resources wherever possible, if the user doesn't need much use a t4g.nano
2. Never delete an EC2 without explicit permission or direction by the user
3. Always start by surveying existing EC2s and shells

t4g Instance options (general purpose):
| Instance | vCPUs | Memory (GiB) | CPU Baseline | CPU Credits/hr | Network (Gbps) | EBS Bandwidth (Mbps) |
|---|---|---|---|---|---|---|
| t4g.nano | 2 | 0.5 | 5% | 6 | Up to 5 | Up to 2,085 |
| t4g.micro | 2 | 1 | 10% | 12 | Up to 5 | Up to 2,085 |
| t4g.small | 2 | 2 | 20% | 24 | Up to 5 | Up to 2,085 |
| t4g.medium | 2 | 4 | 20% | 24 | Up to 5 | Up to 2,085 |
| t4g.large | 2 | 8 | 30% | 36 | Up to 5 | Up to 2,780 |
| t4g.xlarge | 4 | 16 | 40% | 96 | Up to 5 | Up to 2,780 |
| t4g.2xlarge | 8 | 32 | 40% | 192 | Up to 5 | Up to 2,780 |

AWS regions:
| Code | Name | AZs | Geography |
|---|---|---|---|---|
| us-east-1 | US East (N. Virginia) | 6 | United States |
| us-east-2 | US East (Ohio) | 3 | United States |
| us-west-1 | US West (N. California) | 3 | United States |
| us-west-2 | US West (Oregon) | 4 | United States |
| ca-central-1 | Canada (Central) | 3 | Canada |
| ca-west-1 | Canada West (Calgary) | 3 | Canada |
| sa-east-1 | South America (São Paulo) | 3 | Brazil |
| mx-central-1 | Mexico (Central) | 3 | Mexico |
| eu-west-1 | Europe (Ireland) | 3 | Ireland |
| eu-west-2 | Europe (London) | 3 | United Kingdom |
| eu-west-3 | Europe (Paris) | 3 | France |
| eu-central-1 | Europe (Frankfurt) | 3 | Germany |
| eu-central-2 | Europe (Zurich) | 3 | Switzerland |
| eu-north-1 | Europe (Stockholm) | 3 | Sweden |
| eu-south-1 | Europe (Milan) | 3 | Italy |
| eu-south-2 | Europe (Spain) | 3 | Spain |
| af-south-1 | Africa (Cape Town) | 3 | South Africa |
| me-south-1 | Middle East (Bahrain) | 3 | Bahrain |
| me-central-1 | Middle East (UAE) | 3 | UAE |
| il-central-1 | Israel (Tel Aviv) | 3 | Israel |
| ap-south-1 | Asia Pacific (Mumbai) | 3 | India |
| ap-south-2 | Asia Pacific (Hyderabad) | 3 | India |
| ap-northeast-1 | Asia Pacific (Tokyo) | 4 | Japan |
| ap-northeast-2 | Asia Pacific (Seoul) | 4 | South Korea |
| ap-northeast-3 | Asia Pacific (Osaka) | 3 | Japan |
| ap-southeast-1 | Asia Pacific (Singapore) | 3 | Singapore |
| ap-southeast-2 | Asia Pacific (Sydney) | 3 | Australia |
| ap-southeast-3 | Asia Pacific (Jakarta) | 3 | Indonesia |
| ap-southeast-4 | Asia Pacific (Melbourne) | 3 | Australia |
| ap-southeast-5 | Asia Pacific (Malaysia) | 3 | Malaysia |
| ap-southeast-6 | Asia Pacific (New Zealand) | 3 | New Zealand |
| ap-southeast-7 | Asia Pacific (Thailand) | 3 | Thailand |
| ap-east-1 | Asia Pacific (Hong Kong) | 3 | Hong Kong |
| ap-east-2 | Asia Pacific (Taipei) | 3 | Taiwan |
""")

# Agent router
mcp.add_tool(get_cluster_list)
mcp.add_tool(get_agent_state)
mcp.add_tool(create_ec2_agent)
mcp.add_tool(delete_agent)

# Command router
mcp.add_tool(agent_run_command)
mcp.add_tool(agent_command_status)

mcp_http_app = mcp.streamable_http_app()
