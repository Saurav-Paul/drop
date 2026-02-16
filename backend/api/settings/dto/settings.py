"""Settings Data Transfer Objects."""

from pydantic import BaseModel


class SettingsResponse(BaseModel):
    default_expiry: str = "24h"
    max_expiry: str = "7d"
    max_file_size: str = "100MB"
    storage_limit: str = "1GB"
    upload_api_key: str = ""


class SettingsUpdate(BaseModel):
    default_expiry: str | None = None
    max_expiry: str | None = None
    max_file_size: str | None = None
    storage_limit: str | None = None
    upload_api_key: str | None = None
