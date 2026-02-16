"""Download controller — handles file downloads."""

import mimetypes

from fastapi import APIRouter, HTTPException
from fastapi.responses import StreamingResponse

from api.download.services import download_service

router = APIRouter(tags=["Download"])

CHUNK_SIZE = 1024 * 1024  # 1MB


@router.get("/{code}/{filename:path}")
async def download_file(code: str, filename: str):
    """Stream a file download."""
    filepath = download_service.get_file_for_download(code, filename)
    if not filepath:
        raise HTTPException(status_code=404, detail="File not found or expired")

    # Record the download
    download_service.record_download(code)

    # Determine content type
    content_type, _ = mimetypes.guess_type(filename)
    if not content_type:
        content_type = "application/octet-stream"

    file_size = filepath.stat().st_size

    def iterfile():
        with open(filepath, "rb") as f:
            while chunk := f.read(CHUNK_SIZE):
                yield chunk

    return StreamingResponse(
        iterfile(),
        media_type=content_type,
        headers={
            "Content-Disposition": f'attachment; filename="{filename}"',
            "Content-Length": str(file_size),
        },
    )
