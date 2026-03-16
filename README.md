# Cloud Command

Cloud Command is a web-based GUI for creating and managing multiple AWS EC2 instances. It takes roughly a minute and 30 seconds to create an instance and start the first interactive shell.

The built-in MCP server allows LLMs to connect to Cloud Command to create/destroy EC2s and run commands on them.

![Main UI image](/docs/main_ui.png)

![Shell image](/docs/shell.png)

# Features

- Create and destroy EC2 instances
- Interactive terminal via the Web UI
- File upload/download via SFTP
- MCP server for AI agent integration

# Setup

Cloud Command requires that the [aws cli](https://aws.amazon.com/cli/) be installed and connected to an AWS account.

# Args

```
usage: cloud_command [-h] [--version] [--host HOST] [--port PORT]
                     [--private-key-path PRIVATE_KEY_PATH]
                     [--public-key-path PUBLIC_KEY_PATH]
                     [--list-all] [--delete-all]

GUI for commanding multiple AWS assets

options:
  -h, --help            show this help message and exit
  --version             show program's version number and exit

uvicorn options:
  --host HOST           Uvicorn server IP, default: 127.0.0.1
  --port PORT           Uvicorn server port, default: 8080
  --private-key-path PRIVATE_KEY_PATH
                        Path to SSL private key file
  --public-key-path PUBLIC_KEY_PATH
                        Path to SSL certificate file

management options:
  --list-all            Lists all EC2s with a 'cloud_command' tag
  --delete-all          Destroys all EC2s with a 'cloud_command' tag
```

# MCP integration

The MCP server is hosted at `/api/mcp`, these are the configs for claude:

```json
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

**Linux:** Download the `.bin` file, make it executable, then move it to `/usr/bin/cloud_command` to add it to your PATH:

```bash
chmod +x cloud_command.bin
sudo mv cloud_command.bin /usr/bin/cloud_command
```

**Windows:** Download the `.exe` file, move it to a permanent location (e.g. `%APPDATA%\CloudCommand\`), then add that directory to your system PATH:

1. Open **Settings** → **System** → **About** → **Advanced system settings**
2. Click **Environment Variables**
3. Under **System variables**, select **Path** and click **Edit**
4. Click **New** and add the path to the directory containing `cloud_command.exe`
5. Click OK to save

## Build from source

```bash
# Create virtual environment
python3 -m venv .venv

# Activate (Linux)
source .venv/bin/activate
# Activate (Windows)
.venv/Scripts/activate.ps1

# Install dependencies
pip install .[dev]

# Run normally
python3 cloud_command/runner.py

# Build standalone executable
python3 build.py --help
```
