"""Central ORM module — imports all models for Alembic metadata discovery."""

from api.files.orm import FileModel
from api.settings.orm import SettingModel

__all__ = [
    "FileModel",
    "SettingModel",
]
