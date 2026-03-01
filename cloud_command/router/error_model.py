"""Defines error models"""

# Third-party libraries
from pydantic import BaseModel, Field


class ServerError(Exception):
    """Base error that triggers the ErrorResponse model"""

    def __init__(self, status_code: int, error: str, details: str):
        self.status_code = status_code
        self.error = error
        self.details = details


class ErrorResponse(BaseModel):
    error: str = Field(examples=["Error name"])
    details: str = Field(examples=['Error description'])
