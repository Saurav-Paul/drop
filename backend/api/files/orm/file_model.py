"""File ORM model."""

from sqlalchemy import Column, DateTime, Integer, String, func

from database import Base


class FileModel(Base):
    __tablename__ = "files"

    id = Column(Integer, primary_key=True, autoincrement=True)
    code = Column(String, unique=True, nullable=False, index=True)
    filename = Column(String, nullable=False)
    filepath = Column(String, nullable=False)
    size = Column(Integer, default=0)
    max_downloads = Column(Integer, nullable=True)
    download_count = Column(Integer, default=0)
    expires_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, default=func.now())
