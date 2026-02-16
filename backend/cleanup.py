"""Background cleanup — removes expired and max-downloaded files."""

import asyncio
import shutil
from pathlib import Path

from api.files.repositories import files_repository
from config import FILES_DIR


async def cleanup_loop(interval: int = 600):
    """Run cleanup every `interval` seconds (default 10 minutes)."""
    while True:
        try:
            _run_cleanup()
        except Exception as e:
            print(f"Cleanup error: {e}")
        await asyncio.sleep(interval)


def _run_cleanup():
    """Delete expired files, max-downloaded files, and orphaned directories."""
    # Expired files
    for file in files_repository.get_expired():
        _delete_file(file.code)

    # Max downloads reached
    for file in files_repository.get_max_downloads_reached():
        _delete_file(file.code)

    # Orphaned directories (on disk but not in DB)
    if FILES_DIR.exists():
        for entry in FILES_DIR.iterdir():
            if entry.is_dir() and not files_repository.code_exists(entry.name):
                shutil.rmtree(entry, ignore_errors=True)


def _delete_file(code: str):
    """Remove file from DB and disk."""
    filepath = files_repository.get_filepath_by_code(code)
    files_repository.delete_by_code(code)
    if filepath:
        file_path = Path(filepath)
        if file_path.parent.exists():
            shutil.rmtree(file_path.parent, ignore_errors=True)
