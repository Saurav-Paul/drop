"""File Data Transfer Objects."""

from datetime import datetime

from pydantic import BaseModel


class FileResponse(BaseModel):
    id: int
    code: str
    filename: str
    size: int
    max_downloads: int | None = None
    download_count: int
    expires_at: datetime | None = None
    created_at: datetime
