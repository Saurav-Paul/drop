"""Settings repository — data access layer."""

from database import SessionLocal
from api.settings.orm.settings_model import SettingModel


def _get_session():
    return SessionLocal()


def get_all() -> dict[str, str]:
    with _get_session() as session:
        settings = session.query(SettingModel).all()
        return {s.key: s.value for s in settings}


def get(key: str) -> str | None:
    with _get_session() as session:
        setting = session.query(SettingModel).filter_by(key=key).first()
        return setting.value if setting else None


def set(key: str, value: str) -> None:
    with _get_session() as session:
        setting = session.query(SettingModel).filter_by(key=key).first()
        if setting:
            setting.value = value
        else:
            session.add(SettingModel(key=key, value=value))
        session.commit()


def set_many(data: dict[str, str]) -> None:
    with _get_session() as session:
        for key, value in data.items():
            setting = session.query(SettingModel).filter_by(key=key).first()
            if setting:
                setting.value = value
            else:
                session.add(SettingModel(key=key, value=value))
        session.commit()
