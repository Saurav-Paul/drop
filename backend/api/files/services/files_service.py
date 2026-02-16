"""Files service — business logic for file management."""

import shutil
from pathlib import Path

from api.files.dto.file import FileResponse
from api.files.repositories import files_repository


def list_files() -> list[FileResponse]:
    return files_repository.list_all()


def get_file(code: str) -> FileResponse | None:
    return files_repository.get_by_code(code)


def delete_file(code: str) -> bool:
    """Delete file from DB and disk."""
    filepath = files_repository.get_filepath_by_code(code)
    if not filepath:
        return False

    # Delete from DB
    files_repository.delete_by_code(code)

    # Delete from disk
    file_path = Path(filepath)
    if file_path.exists():
        # Remove the code directory (contains the file)
        shutil.rmtree(file_path.parent, ignore_errors=True)

    return True


def get_stats() -> dict:
    files = files_repository.list_all()
    total_storage = files_repository.get_total_storage()
    return {
        "total_files": len(files),
        "total_storage": total_storage,
        "total_downloads": sum(f.download_count for f in files),
    }
