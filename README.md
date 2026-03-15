# Cloud Command

<ENTER DESCRIPTION HERE>

# Setup

Cloud Command requires that the [aws cli](https://aws.amazon.com/cli/) be setup with an account that has sufficient permissions to create and destroy EC2s.

# Args

```
usage: cloud_command [-h] [--version] [--host HOST] [--port PORT] [--private-key-path PRIVATE_KEY_PATH]
 [--public-key-path PUBLIC_KEY_PATH] [--list-all] [--delete-all] GUI for commanding multiple AWS assets options:
 -h, --help show this help message and exit
 --version show program's version number and exit uvicorn options:
 --host HOST Uvicorn server IP, default: 127.0.0.1
 --port PORT Uvicorn server Port, default: 8080
 --private-key-path PRIVATE_KEY_PATH
Path to private key for SSH access to agents
--public-key-path PUBLIC_KEY_PATH
Path to public key for SSH access to agents

management options:
--list-all Lists all EC2s with a 'cloud_command' tag
--delete-all Destroys all EC2s with a 'cloud_command' tag
```

# MCP integration

The MCP server is hosted at `/api/mcp`, these are the configs for claude:

```
{
    "mcpServers": {
        "cloud-command": {
            "type": "http",
            "url": "https://127.0.0.1:8080/api/mcp/"
        }
    }
}
```

NOTE: you may need to disable SSL verification when using self-signed certificates

## Deployment

Download .bin, chmod it, then move it to /usr/bin/cloud_command (This adds it to the path)

Download .exe, move it to %APPDATA% somewhere, add it to path (NEEDS MORE INSTRUCTIONS)

## Build from source

```
# Create virtual environment
python3 -m venv .venv

# Set env (linux)
source .venv/bin/activate
# Set env (windows)
.venv/Scripts/activate.ps1

# Install dependencies
pip install .[dev]

# Run normally
python3 cloud_command/runner.py
# Build
python3 build.py --help
```
