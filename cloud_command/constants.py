"""Defines constants"""

# Standard libraries
import sys
from pathlib import Path

# Directories
if getattr(sys, "frozen", False):
    PARENT_DIRECTORY = Path(sys.argv[0]).resolve().parent.parent
else:
    PARENT_DIRECTORY = Path(__file__).resolve().parent.parent
AGENT_DIRECTORY = PARENT_DIRECTORY / "agents"
AGENT_DIRECTORY.mkdir(exist_ok=True)
AGENT_CONFIG_FILENAME = "agent_config.json"
REACT_FILE_PATH = PARENT_DIRECTORY / "frontend" / "dist"

# Status Trackers
STATUS_TRACKER_UPDATE_INTERVAL = 15

# Version
with open(file=PARENT_DIRECTORY / "version.txt", encoding="utf-8") as version_file:
    VERSION = version_file.read()

# Shell
SHELL_SESSION_POOL_SIZE = 5

# SSH
SSH_TIMEOUT = 10

# SSL
GENERATED_SSL_CERTIFICATE = PARENT_DIRECTORY / "frontend" / "self-signed.crt"
GENERATED_SSL_KEY = PARENT_DIRECTORY / "frontend" / "self-signed.key"

# AWS
AWS_EC2_STATES = ("pending", "running", "shutting-down", "terminated", "stopping", "stopped")
