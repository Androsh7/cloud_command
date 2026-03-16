"""Deletes unused data files bundled with boto3 and botocore"""

# Standard libraries
import os
import shutil
import sys
from pathlib import Path

# Third-party libraries
import boto3
import botocore

REQUIRED_DATA_FILES = ["ec2", "ssm", "sts"]

if sys.prefix == sys.base_prefix and not os.environ.get("CLEAN_BOTO_IGNORE_ENV", False):
    print(
        "ERROR: No virtual environment detected\n"
        "This script will delete parts of boto3 and botocore and should only be run with a virtual environment\n"
        'Set the environmental variable CLEAN_BOTO_IGNORE_ENV=True to bypass this warning'
    )
    sys.exit(1)
else:
    print("Virtual environment detected, proceeding to clean up boto3 and botocore")

botocore_path = Path(botocore.__file__).parent
for path in (botocore_path / "data").iterdir():
    if path.is_dir() and path.name not in REQUIRED_DATA_FILES:
        print(f"Deleting {path}")
        shutil.rmtree(path)

boto3_path = Path(boto3.__file__).parent
for path in (boto3_path / "data").iterdir():
    if path.is_dir() and path.name not in REQUIRED_DATA_FILES:
        print(f"Deleting {path}")
        shutil.rmtree(path)
