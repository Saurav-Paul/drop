"""Upload Data Transfer Objects."""

from datetime import datetime

from pydantic import BaseModel


class UploadResponse(BaseModel):
    url: str
    code: str
    filename: str
    size: int
    expires_at: datetime | None = None
    max_downloads: int | None = None
