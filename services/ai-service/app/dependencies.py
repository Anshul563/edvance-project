from __future__ import annotations

from typing import Annotated

import jwt
from fastapi import Header, HTTPException, status
from jwt import InvalidTokenError

from app.config import get_settings

settings = get_settings()


def _get_secret() -> str:
    return settings["jwt_access_secret"] or ""


def decode_access_token(token: str) -> dict:
    secret = _get_secret()
    if not secret:
        raise ValueError("JWT_ACCESS_SECRET is not configured")
    return jwt.decode(token, secret, algorithms=["HS256"], options={"require": ["sub"]})


def get_current_user_id(
    authorization: Annotated[str | None, Header(default=None, alias="Authorization")] = None,
) -> str | None:
    if not authorization:
        return None
    scheme, _, token = authorization.partition(" ")
    if scheme.lower() != "bearer" or not token:
        return None
    try:
        claims = decode_access_token(token)
    except (ValueError, InvalidTokenError):
        return None
    return str(claims.get("sub") or claims.get("user_id") or "") or None


def get_token_claims(
    authorization: Annotated[str | None, Header(default=None, alias="Authorization")] = None,
) -> dict:
    if not authorization:
        return {}
    scheme, _, token = authorization.partition(" ")
    if scheme.lower() != "bearer" or not token:
        return {}
    try:
        return decode_access_token(token)
    except (ValueError, InvalidTokenError):
        return {}


def require_authentication(
    authorization: Annotated[str | None, Header(default=None, alias="Authorization")] = None,
) -> str:
    user_id = get_current_user_id(authorization)
    if not user_id:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="authentication required")
    return user_id


def require_internal_token(authorization: str | None = None) -> str:
    if not authorization:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="missing internal token")
    scheme, _, token = authorization.partition(" ")
    expected = settings["internal_service_token"]
    if scheme.lower() != "bearer" or token != expected:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="invalid internal token")
    return token
