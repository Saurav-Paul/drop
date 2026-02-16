"""Pages controller — HTML routes for the web UI."""

from datetime import datetime, timezone
from pathlib import Path

from fastapi import APIRouter, Form, Request
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates

from fastapi import UploadFile, File

from auth import COOKIE_NAME, create_session_cookie, is_admin
from config import ADMIN_ENABLED, DROP_ADMIN_PASS, DROP_ADMIN_USER, FILES_DIR
from api.files.repositories import files_repository
from api.files.services import files_service
from api.settings.dto.settings import SettingsUpdate
from api.settings.repositories import settings_repository
from api.settings.services import settings_service
from api.upload.services import upload_service
from cleanup import run_cleanup

import secrets

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


def _require_admin(request: Request):
    """Return a redirect to /login if not admin, or None if authorized."""
    if is_admin(request):
        return None
    return RedirectResponse(url="/login", status_code=302)


@router.get("/login", response_class=HTMLResponse)
async def login_page(request: Request):
    if not ADMIN_ENABLED or is_admin(request):
        return RedirectResponse(url="/")
    return templates.TemplateResponse("login.html", {"request": request, "error": None})


@router.post("/login")
async def login_submit(
    request: Request,
    username: str = Form(...),
    password: str = Form(...),
):
    if not ADMIN_ENABLED:
        return RedirectResponse(url="/", status_code=303)

    if (
        secrets.compare_digest(username, DROP_ADMIN_USER)
        and secrets.compare_digest(password, DROP_ADMIN_PASS)
    ):
        response = RedirectResponse(url="/", status_code=303)
        response.set_cookie(
            COOKIE_NAME,
            create_session_cookie(),
            httponly=True,
            samesite="lax",
            max_age=60 * 60 * 24 * 30,  # 30 days
        )
        return response

    return templates.TemplateResponse(
        "login.html",
        {"request": request, "error": "Invalid username or password"},
        status_code=401,
    )


@router.get("/logout")
async def logout(request: Request):
    response = RedirectResponse(url="/login", status_code=302)
    response.delete_cookie(COOKIE_NAME)
    return response


@router.get("/", response_class=HTMLResponse)
async def dashboard(request: Request):
    redirect = _require_admin(request)
    if redirect:
        return redirect

    files = files_service.list_files()
    stats = files_service.get_stats()
    last_cleanup_raw = settings_repository.get("last_cleanup")
    last_cleanup = None
    if last_cleanup_raw:
        try:
            last_cleanup = datetime.fromisoformat(last_cleanup_raw)
        except ValueError:
            last_cleanup = None
    return templates.TemplateResponse("dashboard.html", {
        "request": request,
        "files": files,
        "stats": stats,
        "last_cleanup": last_cleanup,
    })


@router.get("/settings", response_class=HTMLResponse)
async def settings_page(request: Request):
    redirect = _require_admin(request)
    if redirect:
        return redirect

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
    redirect = _require_admin(request)
    if redirect:
        return redirect

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
    """HTMX endpoint — delete file and return empty row + OOB stats update."""
    if not is_admin(request):
        return HTMLResponse("Unauthorized", status_code=401)

    files_service.delete_file(code)
    stats = files_service.get_stats()
    stats_html = templates.get_template("_stats.html").render(
        request=request, stats=stats,
    )
    return HTMLResponse(f'<div id="stats-container" hx-swap-oob="innerHTML">{stats_html}</div>')


@router.post("/cleanup", response_class=HTMLResponse)
async def run_cleanup_action(request: Request):
    """HTMX endpoint — trigger cleanup and return updated dashboard."""
    if not is_admin(request):
        return HTMLResponse("Unauthorized", status_code=401)

    count = run_cleanup()
    files = files_service.list_files()
    stats = files_service.get_stats()
    last_cleanup_raw = settings_repository.get("last_cleanup")
    last_cleanup = None
    if last_cleanup_raw:
        try:
            last_cleanup = datetime.fromisoformat(last_cleanup_raw)
        except ValueError:
            last_cleanup = None
    return templates.TemplateResponse("_dashboard_content.html", {
        "request": request,
        "files": files,
        "stats": stats,
        "last_cleanup": last_cleanup,
        "cleanup_message": f"Cleaned up {count} file{'s' if count != 1 else ''}",
    })


@router.post("/upload", response_class=HTMLResponse)
async def manual_upload(
    request: Request,
    file: UploadFile = File(...),
    expiry: str = Form(""),
    max_downloads: str = Form(""),
):
    """Manual file upload from the dashboard."""
    if not is_admin(request):
        return HTMLResponse("Unauthorized", status_code=401)

    # Save file to disk
    filename = file.filename
    code = upload_service.generate_code()
    file_dir = FILES_DIR / code
    file_dir.mkdir(parents=True, exist_ok=True)
    final_path = file_dir / filename

    size = 0
    with open(final_path, "wb") as f:
        while chunk := await file.read(1024 * 1024):
            size += len(chunk)
            f.write(chunk)

    expires_at = upload_service.parse_expiry(expiry) if expiry else upload_service.parse_expiry(
        settings_service.get_all().default_expiry
    )
    max_dl = int(max_downloads) if max_downloads else None

    files_repository.create(
        code=code,
        filename=filename,
        filepath=str(final_path),
        size=size,
        max_downloads=max_dl,
        expires_at=expires_at,
    )

    return RedirectResponse(url="/", status_code=303)
