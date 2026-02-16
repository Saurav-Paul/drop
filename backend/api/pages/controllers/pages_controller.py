"""Pages controller — HTML routes for the web UI."""

from datetime import datetime, timezone
from pathlib import Path

from fastapi import APIRouter, Form, Request
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates

from auth import is_admin
from api.files.services import files_service
from api.settings.dto.settings import SettingsUpdate
from api.settings.services import settings_service

router = APIRouter(tags=["Pages"])

TEMPLATES_DIR = Path(__file__).parent.parent.parent.parent.parent / "templates"
templates = Jinja2Templates(directory=TEMPLATES_DIR)


def _timeago(dt: datetime) -> str:
    now = datetime.now(timezone.utc)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    seconds = (now - dt).total_seconds()
    if seconds < 60:
        return "just now"
    if seconds < 3600:
        m = int(seconds // 60)
        return f"{m}m ago"
    if seconds < 86400:
        h = int(seconds // 3600)
        return f"{h}h ago"
    if seconds < 604800:
        d = int(seconds // 86400)
        return f"{d}d ago"
    return dt.strftime("%Y-%m-%d")


def _filesize(size: int) -> str:
    if size < 1024:
        return f"{size} B"
    if size < 1024**2:
        return f"{size / 1024:.1f} KB"
    if size < 1024**3:
        return f"{size / 1024**2:.1f} MB"
    return f"{size / 1024**3:.1f} GB"


templates.env.filters["timeago"] = _timeago
templates.env.filters["filesize"] = _filesize


@router.get("/", response_class=HTMLResponse)
async def dashboard(request: Request):
    if not is_admin(request):
        return templates.TemplateResponse("unauthorized.html", {"request": request}, status_code=401)

    files = files_service.list_files()
    stats = files_service.get_stats()
    return templates.TemplateResponse("dashboard.html", {
        "request": request,
        "files": files,
        "stats": stats,
    })


@router.get("/settings", response_class=HTMLResponse)
async def settings_page(request: Request):
    if not is_admin(request):
        return templates.TemplateResponse("unauthorized.html", {"request": request}, status_code=401)

    settings = settings_service.get_all()
    return templates.TemplateResponse("settings.html", {
        "request": request,
        "settings": settings,
    })


@router.post("/settings")
async def settings_submit(
    request: Request,
    default_expiry: str = Form("24h"),
    max_expiry: str = Form("7d"),
    max_file_size: str = Form("100MB"),
    storage_limit: str = Form("1GB"),
    upload_api_key: str = Form(""),
):
    if not is_admin(request):
        return templates.TemplateResponse("unauthorized.html", {"request": request}, status_code=401)

    settings_service.update(SettingsUpdate(
        default_expiry=default_expiry,
        max_expiry=max_expiry,
        max_file_size=max_file_size,
        storage_limit=storage_limit,
        upload_api_key=upload_api_key,
    ))
    return RedirectResponse(url="/settings", status_code=303)


@router.delete("/api/files/{code}/htmx")
async def htmx_delete_file(request: Request, code: str):
    """HTMX endpoint — delete file and return updated stats partial."""
    if not is_admin(request):
        return HTMLResponse("Unauthorized", status_code=401)

    files_service.delete_file(code)
    stats = files_service.get_stats()
    return templates.TemplateResponse("_stats.html", {
        "request": request,
        "stats": stats,
    })
