"""Cleanup — removes expired and max-downloaded files.

Run standalone: python cleanup.py
Called by cron every 10 minutes inside the container.
"""

import shutil
from pathlib import Path

from api.files.repositories import files_repository
from config import FILES_DIR


def run_cleanup():
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


if __name__ == "__main__":
    run_cleanup()
