"""Tests for app/embeddings/exceptions.py: classifying NVIDIA API errors."""

import pytest
from pydantic import ValidationError

from app.embeddings.config import EmbeddingSettings
from app.embeddings.exceptions import (
    MAX_ERROR_MESSAGE_LENGTH,
    format_validation_error,
    get_status_code,
    is_auth_error,
    is_retryable,
    short_message,
)


# Status codes

@pytest.mark.parametrize(
    "error, expected_status",
    [
        (Exception("[401] Unauthorized"), 401),
        (Exception("[503] Service Unavailable\nRequestID: abc-123"), 503),
        (Exception("Request failed with [401]"), None),
        (Exception("no status code"), None),
        (TimeoutError("read timed out"), None),
    ],
)
def test_get_status_code_reads_the_nvidia_message(error, expected_status):

    # The NVIDIA client puts the status only at the start of the message
    status_code = get_status_code(error)

    assert status_code == expected_status


@pytest.mark.parametrize("status_code, expected", [(401, True), (403, True), (400, False), (429, False), (None, False)])
def test_is_auth_error(status_code, expected):
    assert is_auth_error(status_code) is expected


# Retry decisions

@pytest.mark.parametrize("status_code", [408, 429, 500, 503, 599])
def test_temporary_status_codes_are_retryable(status_code):
    assert is_retryable(status_code) is True


@pytest.mark.parametrize("status_code", [400, 401, 403, 404, 422])
def test_client_status_codes_are_not_retryable(status_code):
    assert is_retryable(status_code) is False


@pytest.mark.parametrize("error", [TimeoutError("read timed out"), ConnectionError("connection reset")])
def test_network_errors_without_status_are_retryable(error):
    assert is_retryable(None, error) is True


@pytest.mark.parametrize("error", [ValueError("unexpected response"), Exception("unknown"), None])
def test_other_errors_without_status_are_not_retryable(error):
    assert is_retryable(None, error) is False


# Error messages

def test_format_validation_error_hides_input_values():

    # Pydantic errors include the rejected input; the formatted message must not
    with pytest.raises(ValidationError) as error:
        EmbeddingSettings(
            api_key="sk-secret-value-123456",
            model="nvidia/test-embed-model",
            batch_size=0,
            max_retries=1,
            retry_delay_seconds=0,
            truncate="NONE",
        )

    message = format_validation_error(error.value)

    assert "api_key" in message
    assert "batch_size" in message
    assert "sk-secret-value-123456" not in message


def test_short_message_joins_lines():

    # Multi-line provider errors become one line
    message = short_message(Exception("[400] Bad Request\n\n   detail:  too long"))

    assert message == "[400] Bad Request detail: too long"


def test_short_message_is_cut_to_the_limit():

    message = short_message(Exception("x" * 1000))

    assert len(message) == MAX_ERROR_MESSAGE_LENGTH
