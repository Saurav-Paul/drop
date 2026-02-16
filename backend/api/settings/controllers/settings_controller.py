"""Settings controller — API routes for settings management."""

from fastapi import APIRouter, HTTPException, Request

from auth import is_admin
from api.settings.dto.settings import SettingsResponse, SettingsUpdate
from api.settings.services import settings_service

router = APIRouter(prefix="/api/settings", tags=["Settings"])


@router.get("", response_model=SettingsResponse)
async def get_settings(request: Request):
    if not is_admin(request):
        raise HTTPException(status_code=401, detail="Admin access required")
    return settings_service.get_all()


@router.put("", response_model=SettingsResponse)
async def update_settings(request: Request, data: SettingsUpdate):
    if not is_admin(request):
        raise HTTPException(status_code=401, detail="Admin access required")
    return settings_service.update(data)
