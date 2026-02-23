"""Utility functions"""

# Standard libraries
import base64
from pathlib import Path

# Third-party libraries
from attrs import define, field, validators
from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa


@define
class SshKeyPair:
    private_key: Path = field(validator=validators.instance_of(Path))
    public_key: Path = field(validator=validators.instance_of(Path))


def create_ssh_key_pair(output_dir: Path) -> SshKeyPair:
    """Creates an ssh key pair

    Args:
        output_dir: The directory to write the private and public keys to

    Returns:
        SSH key pair object
    """
    key = rsa.generate_private_key(backend=default_backend(), public_exponent=65537, key_size=2048)

    # Serialize the private key to PEM format
    private_key = key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.TraditionalOpenSSL,
        encryption_algorithm=serialization.NoEncryption(),
    )

    # Serialize the public key to OpenSSH format
    public_key = key.public_key().public_bytes(
        encoding=serialization.Encoding.OpenSSH,
        format=serialization.PublicFormat.OpenSSH,
    )

    # Write the private key to a file
    private_key_path = output_dir / "private_rsa.key"
    with open(private_key_path, "wb") as private_key_file:
        private_key_file.write(private_key)

    # Write the public key to a file
    public_key_path = output_dir / "public_rsa.key"
    with open(public_key_path, "wb") as public_key_file:
        public_key_file.write(public_key)

    return SshKeyPair(private_key=private_key_path, public_key=public_key_path)


def encode_script(script: str, executable: str = "/bin/bash", sudo: bool = False) -> str:
    return f'echo "{base64.b64encode(script.encode("utf-8")).decode("utf-8")}" | base64 -d | {"sudo " if sudo else ""}{executable}'
