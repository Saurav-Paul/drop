"""Admin authentication — headers, cookies, and Basic Auth."""

import hashlib
import hmac
import secrets

from fastapi import Request

from config import ADMIN_ENABLED, DROP_ADMIN_PASS, DROP_ADMIN_USER

COOKIE_NAME = "drop_session"


def _make_token() -> str:
    """Create HMAC token from current credentials."""
    key = f"{DROP_ADMIN_USER}:{DROP_ADMIN_PASS}".encode()
    return hmac.new(key, b"drop_session", hashlib.sha256).hexdigest()


def create_session_cookie() -> str:
    return _make_token()


def _verify_token(token: str) -> bool:
    return secrets.compare_digest(token, _make_token())


def is_admin(request: Request) -> bool:
    """Check admin via headers or session cookie."""
    if not ADMIN_ENABLED:
        return True

    # Check header auth (API / curl)
    user = request.headers.get("X-Admin-User", "")
    password = request.headers.get("X-Admin-Pass", "")
    if user and password:
        return (
            secrets.compare_digest(user, DROP_ADMIN_USER)
            and secrets.compare_digest(password, DROP_ADMIN_PASS)
        )

    # Check session cookie (browser)
    session = request.cookies.get(COOKIE_NAME)
    if session and _verify_token(session):
        return True

    return False
