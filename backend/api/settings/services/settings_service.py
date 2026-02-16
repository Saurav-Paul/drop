"""Settings service — business logic."""

from api.settings.dto.settings import SettingsResponse, SettingsUpdate
from api.settings.repositories import settings_repository


SETTINGS_DEFAULTS = {
    "default_expiry": "24h",
    "max_expiry": "7d",
    "max_file_size": "100MB",
    "storage_limit": "1GB",
    "upload_api_key": "",
}


def get_all() -> SettingsResponse:
    raw = settings_repository.get_all()
    data = {field: raw.get(field, default) for field, default in SETTINGS_DEFAULTS.items()}
    return SettingsResponse(**data)


def update(data: SettingsUpdate) -> SettingsResponse:
    updates = {k: v for k, v in data.model_dump().items() if v is not None}
    if updates:
        settings_repository.set_many(updates)
    return get_all()
