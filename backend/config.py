"""Application configuration."""

import os
from pathlib import Path

DATA_DIR = Path(os.environ.get("DATA_DIR", str(Path(__file__).parent.parent / "data")))
DATA_DIR.mkdir(parents=True, exist_ok=True)

FILES_DIR = DATA_DIR / "files"
FILES_DIR.mkdir(parents=True, exist_ok=True)

# Admin authentication
DROP_ADMIN_USER = os.environ.get("DROP_ADMIN_USER", "").strip()
DROP_ADMIN_PASS = os.environ.get("DROP_ADMIN_PASS", "").strip()
ADMIN_ENABLED = bool(DROP_ADMIN_USER and DROP_ADMIN_PASS)
