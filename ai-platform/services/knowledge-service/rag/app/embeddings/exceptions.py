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

def get_status_code(error):
    """Read the HTTP status code from an NVIDIA error, or None when it has none."""

    # The NVIDIA client puts the HTTP status at the start of the message
    match = STATUS_CODE_PATTERN.match(str(error))

    # No status code, e.g. a timeout or a dropped connection
    if not match:
        return None

    status_code = int(match.group(1))

    return status_code


def is_auth_error(status_code):

    # 401 and 403 mean NVIDIA rejected the API key
    auth_error = status_code in AUTH_STATUS_CODES

    return auth_error


def is_retryable(status_code, error=None):
    """Decide whether a failed request is worth sending again."""

    # With a status code: retry timeouts, rate limits, and server errors
    if status_code is not None:
        retryable = status_code in RETRYABLE_STATUS_CODES or 500 <= status_code <= 599
        return retryable

    # Without a status code there is nothing to judge by
    if error is None:
        return False

    # Only network errors are retried; anything else would fail the same way again
    retryable = isinstance(error, (TimeoutError, ConnectionError))

    return retryable


def format_validation_error(error):
    """Turn a Pydantic error into one line of field names and reasons."""

    # Show the field and the reason, but never the input value (it may be the API key)
    messages = []
    for detail in error.errors():

        # Join the field path, e.g. "records.0.embedding"
        parts = []
        for part in detail["loc"]:
            parts.append(str(part))
        field = ".".join(parts) or "settings"

        messages.append(f"{field}: {detail['msg']}")

    # Put every problem on one line
    message = "; ".join(messages)

    return message


def short_message(error):

    # Join the lines; provider errors can be long multi-line dumps
    message = " ".join(str(error).split())

    # Keep the message short
    message = message[:MAX_ERROR_MESSAGE_LENGTH]

    return message
