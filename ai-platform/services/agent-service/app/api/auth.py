"""Service token check for the agent API.

Only core-service calls the API. It sends the shared token as a bearer token;
the token is compared in constant time and never logged.
"""

import hmac

from fastapi import HTTPException, Request

from app.api.config import AUTH_MODE_NONE

UNAUTHORIZED_DETAIL = "Missing or invalid service token."


# Public functions

def require_service_token(request: Request):
    """FastAPI dependency: reject the request unless it has the service token."""

    settings = request.app.state.settings

    # Local development without a token (port bound to 127.0.0.1 only)
    if settings.auth_mode == AUTH_MODE_NONE:
        return

    # Expect exactly "Bearer <token>"
    scheme, _, token = request.headers.get("Authorization", "").partition(" ")
    expected = settings.token.get_secret_value()

    # compare_digest takes the same time whichever character differs
    if scheme.lower() != "bearer" or not hmac.compare_digest(token.encode(), expected.encode()):
        raise HTTPException(status_code=401, detail=UNAUTHORIZED_DETAIL, headers={"WWW-Authenticate": "Bearer"})
