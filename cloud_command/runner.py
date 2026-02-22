"""Creates an entry-point for the API"""

# Standard libraries
import argparse
import getpass

# Third-party libraries
import uvicorn

# Project libraries
from cloud_command.app import app
from cloud_command.auth import password_exists, write_password_hash
from cloud_command.constants import VERSION


def ensure_api_password() -> None:
    if password_exists():
        return

    print("No API password configured. Create a new password.")
    while True:
        password = getpass.getpass("New API password: ")
        if not password:
            print("Password cannot be empty.")
            continue

        confirm_password = getpass.getpass("Confirm API password: ")
        if password != confirm_password:
            print("Passwords do not match. Try again.")
            continue

        write_password_hash(password)
        print("API password saved.")
        return


def main():
    parser = argparse.ArgumentParser(prog="CloudCommand", description="GUI for commanding multiple AWS assets")
    parser.add_argument("--version", action="version", version=f"CloudCommand v{VERSION}")

    # Add uvicorn arguments
    uvicorn_arguments = parser.add_argument_group("uvicorn options")
    uvicorn_arguments.add_argument("--host", default="127.0.0.1", help="Uvicorn server IP, default: 127.0.0.1")
    uvicorn_arguments.add_argument("--port", default=8080, help="Uvicorn server Port, default: 8080")

    args = parser.parse_args()
    ensure_api_password()

    uvicorn.run(app=app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
