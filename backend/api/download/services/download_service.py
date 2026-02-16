"""Download service — handles file download logic."""

from datetime import datetime, timezone
from pathlib import Path

from api.files.repositories import files_repository


def get_file_for_download(code: str, filename: str) -> Path | None:
    """Validate and return the file path for download, or None if invalid."""
    file_record = files_repository.get_by_code(code)
    if not file_record:
        return None

    # Check filename matches
    if file_record.filename != filename:
        return None

    # Check expiry
    if file_record.expires_at:
        expires = file_record.expires_at
        if expires.tzinfo is None:
            expires = expires.replace(tzinfo=timezone.utc)
        if datetime.now(timezone.utc) > expires:
            return None

    # Check max downloads
    if file_record.max_downloads is not None:
        if file_record.download_count >= file_record.max_downloads:
            return None

    filepath = Path(files_repository.get_filepath_by_code(code))
    if not filepath.exists():
        return None

    return filepath


def record_download(code: str) -> None:
    """Increment download counter."""
    files_repository.increment_download(code)
