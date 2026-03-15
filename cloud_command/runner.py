"""Creates an entry-point for the API"""

# Standard libraries
import argparse
import shutil
import sys
from pathlib import Path

# Third-party libraries
import boto3
import uvicorn
from loguru import logger

from cloud_command.agent.aws import list_all_ec2s

# Project libraries
from cloud_command.app import app
from cloud_command.constants import AGENT_DIRECTORY, GENERATED_SSL_CERTIFICATE, GENERATED_SSL_KEY, VERSION
from cloud_command.ssl_cert import generate_self_signed_cert


def main():
    parser = argparse.ArgumentParser(prog="CloudCommand", description="GUI for commanding multiple AWS assets")
    parser.add_argument("--version", action="version", version=f"CloudCommand v{VERSION}")

    # Add uvicorn arguments
    uvicorn_arguments = parser.add_argument_group("uvicorn options")
    uvicorn_arguments.add_argument(
        "--host", type=str, default="127.0.0.1", help="Uvicorn server IP, default: 127.0.0.1"
    )
    uvicorn_arguments.add_argument("--port", type=int, default=8080, help="Uvicorn server Port, default: 8080")
    uvicorn_arguments.add_argument(
        "--private-key-path", type=Path, default=None, help="Path to private key for SSH access to agents"
    )
    uvicorn_arguments.add_argument(
        "--public-key-path", type=Path, default=None, help="Path to public key for SSH access to agents"
    )

    # Add management arguments
    management_arguments = parser.add_argument_group("management options")
    management_arguments.add_argument(
        "--list-all", action="store_true", help="Lists all EC2s with a 'cloud_command' tag"
    )
    management_arguments.add_argument(
        "--delete-all", action="store_true", help="Destroys all EC2s with a 'cloud_command' tag"
    )

    args = parser.parse_args()

    if args.list_all or args.delete_all:
        detached_list = list_all_ec2s()
        for detached in detached_list:
            if args.delete_all:
                logger.info(f"Destroying EC2 {detached.id} in region {detached.region}")
                boto3_client = boto3.client("ec2", region_name=detached.region)
                boto3_client.terminate_instances(InstanceIds=[detached.id])
                if detached.key_pair is not None:
                    boto3_client.delete_key_pair(KeyName=detached.key_pair)
            else:
                logger.info(f"Found EC2 {detached.id} in region {detached.region}")
        if args.delete_all:
            shutil.rmtree(AGENT_DIRECTORY)
        sys.exit(0)

    # Validate ssl path
    if args.private_key_path and args.public_key_path:
        pass
    elif args.private_key_path or args.public_key_path:
        parser.error("Both the private and public key must be specified")
    elif GENERATED_SSL_KEY.exists() and GENERATED_SSL_CERTIFICATE.exists():
        logger.warning("No SSL certificate specified, using existing self-signed certificate")
        args.private_key_path = GENERATED_SSL_KEY
        args.public_key_path = GENERATED_SSL_CERTIFICATE
    else:
        logger.warning("No SSL certificate specified, generating self-signed certificate")
        generate_self_signed_cert()
        args.private_key_path = GENERATED_SSL_KEY
        args.public_key_path = GENERATED_SSL_CERTIFICATE

    uvicorn.run(
        app=app,
        host=args.host,
        port=args.port,
        ssl_keyfile=args.private_key_path,
        ssl_certfile=args.public_key_path,
        server_header=False,
        date_header=False,
        headers={
            ("server", "CloudCommand"),
        },
    )


if __name__ == "__main__":
    main()
