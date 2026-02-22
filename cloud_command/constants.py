"""Defines constants"""

# Standard libraries
import sys
from enum import Enum
from pathlib import Path

# Directories
if getattr(sys, "frozen", False):
    PARENT_DIRECTORY = Path(sys.argv[0]).resolve().parent.parent
else:
    PARENT_DIRECTORY = Path(__file__).resolve().parent.parent
AGENT_DIRECTORY = PARENT_DIRECTORY / "agents"
AGENT_DIRECTORY.mkdir(exist_ok=True)

# Version
with open(file=PARENT_DIRECTORY / "version.txt", encoding="utf-8") as version_file:
    VERSION = version_file.read()

# SSH
SSH_TIMEOUT = 10

# AWS
AWS_STARTUP_SCRIPT = """\
#!/bin/bash
set -euo pipefail

sudo dnf -y install update || sudo yum -y install update || sudo apt-get -y update
sudo dnf -y install tmux || sudo yum -y install tmux || sudo apt-get -y install tmux\
"""


class AGENT_STATES(Enum):
    BUILDING = 0
    STARTING = 1
    RUNNING = 2
    STOPPED = 3
    DELETED = 4
    ERROR = 5


class AGENT_TYPE(Enum):
    EC2 = 0
