"""Setting ORM model."""

from sqlalchemy import Column, String

from database import Base


class SettingModel(Base):
    __tablename__ = "settings"

    key = Column(String, primary_key=True)
    value = Column(String, nullable=False, default="")
