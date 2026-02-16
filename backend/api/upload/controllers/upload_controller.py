"""Upload controller — handles file uploads via PUT."""

from fastapi import APIRouter, HTTPException, Request

from auth import is_admin
from api.upload.services import upload_service
from api.upload.dto.upload import UploadResponse

router = APIRouter(tags=["Upload"])


@router.put("/{filename:path}", response_model=UploadResponse)
async def upload_file(request: Request, filename: str):
    """Upload a file via streaming PUT request."""
    admin = is_admin(request)

    expires = request.headers.get("X-Expires")
    max_downloads_str = request.headers.get("X-Max-Downloads")
    max_downloads = int(max_downloads_str) if max_downloads_str else None

    try:
        return await upload_service.save_upload(
            request=request,
            filename=filename,
            expires=expires,
            max_downloads=max_downloads,
            is_admin=admin,
        )
    except ValueError as e:
        raise HTTPException(status_code=413, detail=str(e))
    except PermissionError as e:
        raise HTTPException(status_code=403, detail=str(e))
