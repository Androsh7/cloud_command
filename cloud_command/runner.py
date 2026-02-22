"""Creates an entry-point for the API"""

# Standard libraries
import argparse
import sys

# Third-party libraries
import uvicorn
from stdiomask import getpass

# Project libraries
from cloud_command.app import app
from cloud_command.auth import password_exists, write_password_hash
from cloud_command.constants import VERSION


def update_api_password() -> None:
    password = getpass("New API password: ")
    if not password:
        print("Password cannot be empty.")
        sys.exit(1)

    confirm_password = getpass("Confirm API password: ")
    if password != confirm_password:
        print("Passwords do not match. Try again.")
        sys.exit(1)

    write_password_hash(password)


def main():
    parser = argparse.ArgumentParser(prog="CloudCommand", description="GUI for commanding multiple AWS assets")
    parser.add_argument("--version", action="version", version=f"CloudCommand v{VERSION}")

    # Add uvicorn arguments
    uvicorn_arguments = parser.add_argument_group("uvicorn options")
    uvicorn_arguments.add_argument("--host", default="127.0.0.1", help="Uvicorn server IP, default: 127.0.0.1")
    uvicorn_arguments.add_argument("--port", default=8080, help="Uvicorn server Port, default: 8080")

    # API password setup
    api_arguments = parser.add_argument_group("API options")
    api_arguments.add_argument("--update-password", action="store_true", help="Update API password")

    args = parser.parse_args()
    if not password_exists() or args.update_password:
        update_api_password()

    uvicorn.run(app=app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
