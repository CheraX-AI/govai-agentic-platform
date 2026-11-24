"""LLM errors and helpers to summarise them."""

MAX_ERROR_MESSAGE_LENGTH = 200


# Errors

class LLMError(Exception):
    """Base class for LLM errors."""


class LLMConfigError(LLMError):
    """The LLM settings are missing or invalid, or the client could not be created."""


class LLMAuthError(LLMError):
    """Azure OpenAI rejected the API key."""


class LLMTimeoutError(LLMError):
    """Azure OpenAI did not answer within the timeout."""


class LLMUnavailableError(LLMError):
    """Azure OpenAI is not reachable, is rate limiting, or had a server error."""


class LLMRequestError(LLMError):
    """Azure OpenAI rejected the request, e.g. an unknown deployment or the content filter."""


# Public functions

def error_summary(error):
    """The first line of an error message, shortened.

    Only the first line is kept: the lines after it can repeat parts of the
    request, which holds the citizen's question.
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
