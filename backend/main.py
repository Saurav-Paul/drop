"""Drop — Main application entry point."""

from pathlib import Path

from alembic import command
from alembic.config import Config
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

from api.files.controllers.files_controller import router as files_router
from api.settings.controllers.settings_controller import router as settings_router
from api.pages.controllers.pages_controller import router as pages_router
from api.download.controllers.download_controller import router as download_router
from api.upload.controllers.upload_controller import router as upload_router


def run_migrations():
    """Run Alembic migrations on startup."""
    try:
        alembic_ini = Path(__file__).parent / "alembic.ini"
        alembic_cfg = Config(str(alembic_ini))
        alembic_cfg.set_main_option(
            "script_location", str(Path(__file__).parent / "db_migrations")
        )
        command.upgrade(alembic_cfg, "head")
    except Exception as e:
        print(f"Warning: Migration failed: {e}")
        from database import init_db

        init_db()


app = FastAPI(title="Drop", version="0.1.0")

# Run database migrations
run_migrations()

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Static files
STATIC_DIR = Path(__file__).parent.parent / "static"
app.mount("/static", StaticFiles(directory=STATIC_DIR), name="static")

# Router registration order matters:
# 1. Health check (before catch-all routes)
@app.get("/api/health")
async def health():
    return {"status": "ok"}


# 2. API routers (prefixed — match first)
app.include_router(files_router)
app.include_router(settings_router)

# 3. Pages (exact match)
app.include_router(pages_router)

# 4. Download (catch-all GET /{code}/{filename})
app.include_router(download_router)

# 5. Upload (catch-all PUT /{filename})
app.include_router(upload_router)
