"""Run ruff checks and mypy"""

# Standard libraries
import re
import subprocess


def test_ruff_formatting():
    assert subprocess.run("ruff format . --check", shell=True, capture_output=False, check=True)


def test_ruff_check():
    result = subprocess.run("ruff check . --show-fixes", shell=True, capture_output=False, check=False)
    assert re.search(r"\d+ fixable with the", str(result.stdout)) is None


def test_mypy():
    assert subprocess.run("mypy cloud_command", shell=True, capture_output=False, check=True)
