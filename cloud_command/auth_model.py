"""Pydantic models for authentication requests/responses."""

# Third-party libraries
from pydantic import BaseModel, Field


class LoginModel(BaseModel):
    password: str = Field(min_length=1, examples=["my-secret-password"])


class LoginResultModel(BaseModel):
    authenticated: bool = Field(examples=[True])
    token: str = Field(examples=["5nQf91Pq1YfG8QZ6pXyEDxw7jG8j7E5V8ldE3n3CzjA"])
    token_type: str = Field(default="bearer", examples=["bearer"])
