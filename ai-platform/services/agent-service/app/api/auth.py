"""Checks the service token that core-service sends, and reads the citizen it vouches for."""

import hmac

from fastapi import HTTPException, Request

from app.api.config import AUTH_MODE_NONE

UNAUTHORIZED_DETAIL = "Missing or invalid service token."
UNAUTHORIZED_CITIZEN_DETAIL = "Missing or invalid citizen."

# core-service sends the Keycloak subject of the token it verified
CITIZEN_HEADER = "X-GovAI-Citizen"
MAX_SUBJECT_LENGTH = 255


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


def require_citizen(request: Request):
    """FastAPI dependency: the service token, then the citizen subject. Returns the subject.

    The subject is trusted only because the service token proves core-service sent
    it (option B in the memory plan); it is never read from the body.
    """

    require_service_token(request)

    subject = request.headers.get(CITIZEN_HEADER, "")
    if not valid_subject(subject):
        raise HTTPException(status_code=401, detail=UNAUTHORIZED_CITIZEN_DETAIL)

    return subject


# Helper functions

def valid_subject(subject):

    # 1-255 printable ASCII characters without spaces, like a Keycloak subject
    if not 1 <= len(subject) <= MAX_SUBJECT_LENGTH:
        return False

    for character in subject:
        if not 33 <= ord(character) <= 126:
            return False

    return True
