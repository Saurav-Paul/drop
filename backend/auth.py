"""Admin authentication — header-based validation."""

import secrets

from fastapi import Request

from config import ADMIN_ENABLED, DROP_ADMIN_PASS, DROP_ADMIN_USER


def is_admin(request: Request) -> bool:
    """Check if request has valid admin credentials via headers."""
    if not ADMIN_ENABLED:
        return True

    user = request.headers.get("X-Admin-User", "")
    password = request.headers.get("X-Admin-Pass", "")

    if not user or not password:
        return False

    return (
        secrets.compare_digest(user, DROP_ADMIN_USER)
        and secrets.compare_digest(password, DROP_ADMIN_PASS)
    )
