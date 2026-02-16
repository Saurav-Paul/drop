"""Files controller — API routes for file management."""

from fastapi import APIRouter, HTTPException, Request, status

from auth import is_admin
from api.files.dto.file import FileResponse
from api.files.services import files_service

router = APIRouter(prefix="/api/files", tags=["Files"])


@router.get("", response_model=list[FileResponse])
async def list_files(request: Request):
    if not is_admin(request):
        raise HTTPException(status_code=401, detail="Admin access required")
    return files_service.list_files()


@router.get("/{code}", response_model=FileResponse)
async def get_file(request: Request, code: str):
    if not is_admin(request):
        raise HTTPException(status_code=401, detail="Admin access required")
    file = files_service.get_file(code)
    if not file:
        raise HTTPException(status_code=404, detail="File not found")
    return file


@router.delete("/{code}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_file(request: Request, code: str):
    if not is_admin(request):
        raise HTTPException(status_code=401, detail="Admin access required")
    if not files_service.delete_file(code):
        raise HTTPException(status_code=404, detail="File not found")
