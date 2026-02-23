"""Build the executable with nuitka"""

# Standard libraries
import argparse
import subprocess
from pathlib import Path

PARENT_DIRECTORY = Path(__file__).parent


def build_linux():
    subprocess.run(
        "docker rm --force nuitka-compiler",
        cwd=PARENT_DIRECTORY,
        shell=True,
        check=False,
    )
    subprocess.run(
        "docker run --name nuitka-compiler --detach androsh7/nuitka-compiler:latest-x86_64-glibc-2.28-py3.13 sleep infinity "
        "&& docker cp cloud_command nuitka-compiler:/src/cloud_command "
        "&& docker cp pyproject.toml nuitka-compiler:/src/pyproject.toml "
        "&& docker cp version.txt nuitka-compiler:/src/version.txt "
        "&& docker exec nuitka-compiler python3 -m pip install .[dev] "
        "&& docker exec nuitka-compiler python3 -m nuitka "
        "   --standalone "
        "   --onefile "
        "   --output-filename=/src/cloud_command.bin "
        "   --onefile-tempdir-spec={HOME}/.cloud_command "
        "   --include-data-file=version.txt=version.txt "
        "   --include-module=cloud_command.app "
        "   /src/cloud_command/runner.py "
        "&& docker cp nuitka-compiler:/src/cloud_command.bin cloud_command.bin "
        "&& docker rm --force nuitka-compiler",
        cwd=PARENT_DIRECTORY,
        shell=True,
        check=True,
    )


def build_windows():
    subprocess.run(
        "nuitka "
        "--assume-yes-for-downloads "
        "--standalone "
        "--onefile "
        "--include-data-file=version.txt=version.txt "
        "--output-filename=cloud_command.exe "
        "--onefile-tempdir-spec={HOME}/.cloud_command "
        "cloud_command/runner.py",
        cwd=PARENT_DIRECTORY,
        shell=True,
        check=True,
    )


def main():
    """Main logic"""
    parser = argparse.ArgumentParser(prog="build.py")
    parser.add_argument("--windows", action="store_true", help="Build the Windows executable")
    parser.add_argument("--linux", action="store_true", help="Build the Linux executable")
    parser.add_argument("--build-all", action="store_true", help="Build the Windows and Linux executable")
    args = parser.parse_args()

    if args.windows or args.build_all:
        build_windows()
    if args.linux or args.build_all:
        build_linux()
    if not args.linux and not args.windows and not args.build_all:
        parser.print_help()


if __name__ == "__main__":
    main()
