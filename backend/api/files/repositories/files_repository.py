"""Files repository — data access layer."""

from datetime import datetime, timezone

from sqlalchemy import func

from database import SessionLocal
from api.files.orm.file_model import FileModel
from api.files.dto.file import FileResponse


def _get_session():
    return SessionLocal()


def _model_to_dto(model: FileModel) -> FileResponse:
    return FileResponse(
        id=model.id,
        code=model.code,
        filename=model.filename,
        size=model.size or 0,
        max_downloads=model.max_downloads,
        download_count=model.download_count or 0,
        expires_at=model.expires_at,
        created_at=model.created_at,
    )


def list_all() -> list[FileResponse]:
    with _get_session() as session:
        models = session.query(FileModel).order_by(FileModel.created_at.desc()).all()
        return [_model_to_dto(m) for m in models]


def get_by_code(code: str) -> FileResponse | None:
    with _get_session() as session:
        model = session.query(FileModel).filter_by(code=code).first()
        return _model_to_dto(model) if model else None


def get_filepath_by_code(code: str) -> str | None:
    with _get_session() as session:
        model = session.query(FileModel).filter_by(code=code).first()
        return model.filepath if model else None


def create(
    code: str,
    filename: str,
    filepath: str,
    size: int,
    max_downloads: int | None = None,
    expires_at: datetime | None = None,
) -> FileResponse:
    with _get_session() as session:
        model = FileModel(
            code=code,
            filename=filename,
            filepath=filepath,
            size=size,
            max_downloads=max_downloads,
            expires_at=expires_at,
        )
        session.add(model)
        session.commit()
        session.refresh(model)
        return _model_to_dto(model)


def code_exists(code: str) -> bool:
    with _get_session() as session:
        return session.query(FileModel).filter_by(code=code).first() is not None


def increment_download(code: str) -> None:
    with _get_session() as session:
        model = session.query(FileModel).filter_by(code=code).first()
        if model:
            model.download_count = (model.download_count or 0) + 1
            session.commit()


def delete_by_code(code: str) -> bool:
    with _get_session() as session:
        model = session.query(FileModel).filter_by(code=code).first()
        if not model:
            return False
        session.delete(model)
        session.commit()
        return True


def get_expired() -> list[FileResponse]:
    now = datetime.now(timezone.utc)
    with _get_session() as session:
        models = (
            session.query(FileModel)
            .filter(FileModel.expires_at.isnot(None), FileModel.expires_at <= now)
            .all()
        )
        return [_model_to_dto(m) for m in models]


def get_max_downloads_reached() -> list[FileResponse]:
    with _get_session() as session:
        models = (
            session.query(FileModel)
            .filter(
                FileModel.max_downloads.isnot(None),
                FileModel.download_count >= FileModel.max_downloads,
            )
            .all()
        )
        return [_model_to_dto(m) for m in models]


def get_total_storage() -> int:
    with _get_session() as session:
        total = session.query(func.sum(FileModel.size)).scalar()
        return total or 0
