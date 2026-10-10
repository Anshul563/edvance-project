from __future__ import annotations

from typing import Annotated

from fastapi import Header, HTTPException, status

from app.config import get_settings

settings = get_settings()


def get_internal_token() -> str:
    return settings["internal_service_token"]


def require_internal_token(authorization: Annotated[str | None, Header(...)] = None) -> str:
    if not authorization:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED, detail="missing internal token"
        )
    scheme, _, token = authorization.partition(" ")
    if scheme.lower() != "bearer" or token != get_internal_token():
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED, detail="invalid internal token"
        )
    return token


def get_current_user_id(
    authorization: Annotated[str | None, Header(default=None, alias="Authorization")] = None,
) -> str | None:
    if not authorization:
        return None
    scheme, _, token = authorization.partition(" ")
    if scheme.lower() != "bearer" or not token:
        return None
    if settings["jwt_access_secret"] and token.count(".") != 2:
        return None
    return token
