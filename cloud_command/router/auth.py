"""Defines the authentication router."""

# Third-party libraries
from fastapi import APIRouter, HTTPException, status

# Project libraries
from cloud_command.auth import create_auth_token, password_exists, verify_password
from cloud_command.auth_model import LoginModel, LoginResultModel

auth_router = APIRouter()


@auth_router.post("/auth/login", tags=["Auth"])
async def login(login_model: LoginModel) -> LoginResultModel:
    if not password_exists():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Password is not configured",
        )

    if not verify_password(login_model.password):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Authentication failed",
        )

    return LoginResultModel(authenticated=True, token=create_auth_token())
