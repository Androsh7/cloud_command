"""Defines the MCP server for CloudCommand"""

# Third-party libraries
from mcp.server.fastmcp import FastMCP

# Project libraries
from cloud_command.router.agent import get_cluster_list, get_agent_state, create_ec2_agent, delete_agent
from cloud_command.router.command import agent_run_command, agent_command_status, agent_download_file, agent_upload_file
from cloud_command.router.shell import create_shell_session, list_shell_sessions, run_command_shell_session, run_command_ctrl_c_shell_session, delete_shell_session, get_output_shell_session, get_statistics_shell_session

mcp = FastMCP(name="CloudCommand")

# Agent router
mcp.add_tool(get_cluster_list)
mcp.add_tool(get_agent_state)
mcp.add_tool(create_ec2_agent)
mcp.add_tool(delete_agent)

# Command router
mcp.add_tool(agent_run_command)
mcp.add_tool(agent_command_status)
mcp.add_tool(agent_upload_file)

# Shell router
mcp.add_tool(create_shell_session)
mcp.add_tool(list_shell_sessions)
mcp.add_tool(run_command_shell_session)
mcp.add_tool(run_command_ctrl_c_shell_session)
mcp.add_tool(delete_shell_session)
mcp.add_tool(get_output_shell_session)
mcp.add_tool(get_statistics_shell_session)

mcp_http_app = mcp.streamable_http_app()
