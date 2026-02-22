"""Defines constants"""

# Standard libraries
from enum import Enum
from pathlib import Path

# Directories
BUILD_DIR = Path().home() / ".cluster_lib"

# PARENT_DIRECTORY = Path(__file__).parent 
# ADD NUITKA PATH SUPPORT

# SSH
SSH_TIMEOUT = 10

# AWS
AWS_STARTUP_SCRIPT="""\
#!/bin/bash
set -euo pipefail

sudo dnf -y install git make gcc tmux\
"""

class AGENT_STATES(Enum):
    BUILDING = 0
    STARTING = 1
    RUNNING = 2
    STOPPED = 3
    DELETED = 4
    ERROR = 5