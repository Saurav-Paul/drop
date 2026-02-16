"""Upload service — handles file upload logic."""

import re
import secrets
import shutil
import tempfile
from datetime import datetime, timedelta, timezone

from fastapi import Request

from config import FILES_DIR
from api.files.repositories import files_repository
from api.settings.services import settings_service
from api.upload.dto.upload import UploadResponse


def generate_code(length: int = 6) -> str:
    """Generate a unique random code for the upload."""
    while True:
        code = secrets.token_urlsafe(length)[:length]
        if not files_repository.code_exists(code):
            return code


def parse_expiry(expiry_str: str) -> datetime | None:
    """Parse expiry string like '30m', '2h', '3d' into a datetime."""
    if not expiry_str:
        return None

    match = re.match(r"^(\d+)([mhdw])$", expiry_str.strip().lower())
    if not match:
        return None

    value = int(match.group(1))
    unit = match.group(2)

    deltas = {
        "m": timedelta(minutes=value),
        "h": timedelta(hours=value),
        "d": timedelta(days=value),
        "w": timedelta(weeks=value),
    }

    return datetime.now(timezone.utc) + deltas[unit]


def parse_size(size_str: str) -> int:
    """Parse size string like '100MB', '1GB' into bytes."""
    if not size_str:
        return 0

    match = re.match(r"^(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB)$", size_str.strip().upper())
    if not match:
        return 0

    value = float(match.group(1))
    unit = match.group(2)

    multipliers = {
        "B": 1,
        "KB": 1024,
        "MB": 1024**2,
        "GB": 1024**3,
        "TB": 1024**4,
    }

    return int(value * multipliers[unit])


async def save_upload(
    request: Request,
    filename: str,
    expires: str | None = None,
    max_downloads: int | None = None,
    is_admin: bool = False,
) -> UploadResponse:
    """Stream request body to disk, validate limits, create DB record."""
    settings = settings_service.get_all()

    # Determine expiry
    if expires:
        expires_at = parse_expiry(expires)
    else:
        expires_at = parse_expiry(settings.default_expiry)

    # Enforce max expiry for non-admin uploads
    if not is_admin and expires_at:
        max_expires_at = parse_expiry(settings.max_expiry)
        if max_expires_at and expires_at > max_expires_at:
            expires_at = max_expires_at

    # Check storage limit
    max_file_size = parse_size(settings.max_file_size)
    storage_limit = parse_size(settings.storage_limit)
    current_storage = files_repository.get_total_storage()

    # Stream body to temp file
    tmp = tempfile.NamedTemporaryFile(delete=False, dir=str(FILES_DIR))
    try:
        size = 0
        async for chunk in request.stream():
            size += len(chunk)
            if max_file_size and size > max_file_size and not is_admin:
                tmp.close()
                import os
                os.unlink(tmp.name)
                raise ValueError(
                    f"File exceeds max size of {settings.max_file_size}"
                )
            if storage_limit and (current_storage + size) > storage_limit and not is_admin:
                tmp.close()
                import os
                os.unlink(tmp.name)
                raise ValueError(
                    f"Storage limit of {settings.storage_limit} would be exceeded"
                )
            tmp.write(chunk)
        tmp.close()

        # Generate code and move to final location
        code = generate_code()
        file_dir = FILES_DIR / code
        file_dir.mkdir(parents=True, exist_ok=True)
        final_path = file_dir / filename
        shutil.move(tmp.name, str(final_path))

        # Check upload API key
        upload_key = settings.upload_api_key
        if upload_key:
            provided_key = request.headers.get("X-Upload-Key", "")
            if not is_admin and provided_key != upload_key:
                # Clean up
                final_path.unlink(missing_ok=True)
                file_dir.rmdir()
                raise PermissionError("Invalid upload API key")

        # Create DB record
        file_record = files_repository.create(
            code=code,
            filename=filename,
            filepath=str(final_path),
            size=size,
            max_downloads=max_downloads,
            expires_at=expires_at,
        )

        # Build download URL
        base_url = str(request.base_url).rstrip("/")
        url = f"{base_url}/{code}/{filename}"

        return UploadResponse(
            url=url,
            code=code,
            filename=filename,
            size=size,
            expires_at=expires_at,
            max_downloads=max_downloads,
        )
    except (ValueError, PermissionError):
        raise
    except Exception:
        import os
        if os.path.exists(tmp.name):
            os.unlink(tmp.name)
        raise
