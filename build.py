"""Build the executable with nuitka"""

# Standard libraries
import argparse
import subprocess
from pathlib import Path

PARENT_DIRECTORY = Path(__file__).parent


def build_npm():
    subprocess.run(
        "docker rm --force npm-builder",
        cwd=PARENT_DIRECTORY,
        shell=True,
        check=False,
        capture_output=True,
    )
    subprocess.run(
        "docker build -t npm-builder:latest -f npm_build.Dockerfile . "
        "&& docker run --name npm-builder npm-builder:latest "
        "&& docker cp npm-builder:/src/dist . "
        "&& docker rm npm-builder "
        "&& docker rmi npm-builder:latest",
        cwd=PARENT_DIRECTORY / "frontend",
        shell=True,
        check=True,
    )


def build_linux(architecture: str, libc: str):
    subprocess.run(
        "docker rmi --force nuitka-compiler:latest",
        cwd=PARENT_DIRECTORY,
        shell=True,
        check=False,
        capture_output=True,
    )
    subprocess.run(
        "docker build -t nuitka-compiler:latest -f nuitka_build.Dockerfile "
        f"--build-arg architecture={architecture} --build-arg libc={libc} . "
        "&& docker run --name nuitka-compiler nuitka-compiler:latest "
        "&& docker cp nuitka-compiler:/src/cloud_command.bin cloud_command.bin "
        "&& docker rm nuitka-compiler "
        "&& docker rmi nuitka-compiler:latest",
        cwd=PARENT_DIRECTORY,
        shell=True,
        check=True,
    )


def build_windows():
    subprocess.run(
        "python3 boto_clean.py",
        cwd=PARENT_DIRECTORY,
        capture_output=False,
        shell=True,
        check=True,
    )
    subprocess.run(
        "nuitka "
        "--assume-yes-for-downloads "
        "--standalone "
        "--onefile "
        "--include-data-file=version.txt=version.txt "
        "--include-data-dir=frontend/dist=frontend/dist "
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
    parser.add_argument(
        "--architecture", type=str, default="x86_64", help="Architecture for Linux executable, default: x86_64"
    )
    parser.add_argument("--libc", type=str, default="glibc-2.28", help="The libc and version, default: glibc-2.28")
    parser.add_argument("--build-all", action="store_true", help="Build the Windows and Linux executable")
    parser.add_argument("--skip-npm", action="store_true", help="Skip the npm build step (use pre-built frontend/dist)")
    args = parser.parse_args()

    if (args.windows or args.linux or args.build_all) and not args.skip_npm:
        build_npm()
    if args.windows or args.build_all:
        build_windows()
    if args.linux or args.build_all:
        build_linux(libc=args.libc, architecture=args.architecture)
    if not args.linux and not args.windows and not args.build_all:
        parser.print_help()


if __name__ == "__main__":
    main()
