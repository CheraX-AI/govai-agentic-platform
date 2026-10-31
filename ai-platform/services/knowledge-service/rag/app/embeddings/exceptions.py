"""Embedding errors and the helpers that classify NVIDIA API failures."""

import re

MAX_ERROR_MESSAGE_LENGTH = 300

# 408 timeout, 429 rate limit, and 5xx server errors can succeed on a later attempt
RETRYABLE_STATUS_CODES = {408, 429}
AUTH_STATUS_CODES = {401, 403}

# The NVIDIA client raises a plain Exception whose message starts with "[401] ..."
STATUS_CODE_PATTERN = re.compile(r"^\[(\d{3})\]")


# Errors

class EmbeddingError(Exception):
    """Base class for embedding errors."""


class EmbeddingConfigError(EmbeddingError):
    """The embedding settings are missing or invalid."""


class ChunkFileError(EmbeddingError):
    """A chunk file is missing, unreadable, or has an invalid structure."""


class InvalidEmbeddingError(EmbeddingError):
    """The embedding model returned vectors that cannot be stored."""


class EmbeddingAuthError(EmbeddingError):
    """The provider rejected the credentials, so no further batch can succeed."""


# Public functions

def get_status_code(error: Exception) -> int | None:

    # The NVIDIA client puts the HTTP status at the start of the message
    match = STATUS_CODE_PATTERN.match(str(error))

    if match:
        return int(match.group(1))

    return None


def is_auth_error(status_code):
    return status_code in AUTH_STATUS_CODES


def is_retryable(status_code, error=None):
    if status_code is not None:
        return (
            status_code in RETRYABLE_STATUS_CODES
            or 500 <= status_code <= 599
        )

    if error is None:
        return False

    return isinstance(
        error,
        (TimeoutError, ConnectionError)
    )

def format_validation_error(error):

    # Show the field and the reason, but never the input value (it may be the API key)
    messages = []
    for detail in error.errors():
        field = ".".join(str(part) for part in detail["loc"]) or "settings"
        messages.append(f"{field}: {detail['msg']}")

    return "; ".join(messages)


def short_message(error):

    # Keep error messages short; provider errors can be long multi-line dumps
    message = " ".join(str(error).split())

    return message[:MAX_ERROR_MESSAGE_LENGTH]
