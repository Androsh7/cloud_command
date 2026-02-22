"""Authentication helpers for password-hash API access."""

# Standard libraries
import hashlib
import hmac
import os
import secrets
from pathlib import Path

# Third-party libraries
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

# Project libraries
from cloud_command.constants import PARENT_DIRECTORY

PASSWORD_HASH_FILE: Path = PARENT_DIRECTORY / "api_password.sha256"
bearer_scheme = HTTPBearer(auto_error=False)
ACTIVE_TOKENS: set[str] = set()
SALT_BYTES = 16


def _hash_password(password: str) -> str:
    return hashlib.sha256(password.encode("utf-8")).hexdigest()


def _hash_password_with_salt(password: str, salt: bytes) -> str:
    return hashlib.sha256(salt + password.encode("utf-8")).hexdigest()


def password_exists() -> bool:
    return PASSWORD_HASH_FILE.exists()


def read_password_hash() -> str | None:
    if not password_exists():
        return None
    return PASSWORD_HASH_FILE.read_text(encoding="utf-8").strip()


def write_password_hash(password: str) -> None:
    salt = os.urandom(SALT_BYTES)
    salted_hash = _hash_password_with_salt(password, salt)
    # Format: <salt_hex>$<sha256(salt + password)_hex>
    PASSWORD_HASH_FILE.write_text(f"{salt.hex()}${salted_hash}", encoding="utf-8")


def verify_password(password: str) -> bool:
    expected_hash = read_password_hash()
    if not expected_hash:
        return False
    if "$" in expected_hash:
        salt_hex, stored_hash = expected_hash.split("$", maxsplit=1)
        try:
            salt = bytes.fromhex(salt_hex)
        except ValueError:
            return False
        given_hash = _hash_password_with_salt(password, salt)
        return hmac.compare_digest(given_hash, stored_hash)

    # Backward compatibility for existing unsalted password hash files.
    given_hash = _hash_password(password)
    return hmac.compare_digest(given_hash, expected_hash)


def create_auth_token() -> str:
    token = secrets.token_urlsafe(32)
    ACTIVE_TOKENS.add(token)
    return token


def verify_auth_token(token: str) -> bool:
    return token in ACTIVE_TOKENS


def require_api_token(
    credentials: HTTPAuthorizationCredentials | None = Depends(bearer_scheme),
) -> None:
    token = credentials.credentials if credentials and credentials.scheme.lower() == "bearer" else None
    if not token or not verify_auth_token(token):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Authentication failed",
        )
