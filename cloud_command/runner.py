"""Creates an entry-point for the API"""

# Standard libraries
import argparse

# Third-party libraries
import uvicorn

# Project libraries
from cloud_command.app import app
from cloud_command.constants import VERSION


def main():
    parser = argparse.ArgumentParser(prog="CloudCommand", description="GUI for commanding multiple AWS assets")
    parser.add_argument("--version", action="version", version=f"CloudCommand v{VERSION}")

    # Add uvicorn arguments
    uvicorn_arguments = parser.add_argument_group("uvicorn options")
    uvicorn_arguments.add_argument("--host", default="127.0.0.1", help="Uvicorn server IP, default: 127.0.0.1")
    uvicorn_arguments.add_argument("--port", default=8080, help="Uvicorn server Port, default: 8080")

    args = parser.parse_args()

    uvicorn.run(app=app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
