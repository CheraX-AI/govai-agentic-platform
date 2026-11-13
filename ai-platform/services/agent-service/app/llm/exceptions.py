"""LLM errors and helpers to summarise them."""

import re

MAX_ERROR_MESSAGE_LENGTH = 200

# The NVIDIA client raises a plain Exception whose message starts with "[401] ..."
STATUS_CODE_PATTERN = re.compile(r"^\[(\d{3})\]")


# Errors

class LLMError(Exception):
    """Base class for LLM errors."""


class LLMConfigError(LLMError):
    """The LLM settings are missing or invalid, or the client could not be created."""


class LLMAuthError(LLMError):
    """NVIDIA rejected the API key."""


class LLMTimeoutError(LLMError):
    """NVIDIA did not answer within the timeout."""


class LLMUnavailableError(LLMError):
    """NVIDIA is not reachable, is rate limiting, or had a server error."""


class LLMRequestError(LLMError):
    """NVIDIA rejected the request, e.g. an unknown model or a prompt that is too long."""


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


def error_summary(error):
    """The first line of an error message, shortened.

    Only the first line ("[400] Bad Request") is kept: the lines after it can
    repeat parts of the request, which holds the citizen's question.
    """

    # Keep the first non-empty line
    summary = ""
    for line in str(error).splitlines():
        if line.strip():
            summary = line.strip()
            break

    # Fall back to the error type when there is no message
    if not summary:
        summary = type(error).__name__

    # Keep the message short
    summary = summary[:MAX_ERROR_MESSAGE_LENGTH]

    return summary


def format_validation_error(error):
    """Turn a Pydantic error into one line of field names and reasons."""

    # Show the field and the reason, but never the input value (it may be the API key)
    messages = []
    for detail in error.errors():

        # Join the field path, e.g. "LLM_TEMPERATURE"
        parts = []
        for part in detail["loc"]:
            parts.append(str(part))
        field = ".".join(parts) or "settings"

        messages.append(f"{field}: {detail['msg']}")

    # Put every problem on one line
    message = "; ".join(messages)

    return message
