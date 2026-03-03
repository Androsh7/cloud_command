"""Defines file upload and download models"""

# Third-party libraries
from pydantic import BaseModel


class FileUploadModel(BaseModel):
    destination_path: str
    file_size_bytes: int
    mime_type: str
