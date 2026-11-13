"""Creates the NVIDIA chat model and sends messages to it."""

import logging

import requests
from langchain_core.language_models import BaseChatModel
from pydantic import BaseModel, ConfigDict

from app.llm.config import ENV_FILE, LLMSettings, load_llm_settings
from app.llm.exceptions import (
    LLMAuthError,
    LLMConfigError,
    LLMError,
    LLMRequestError,
    LLMTimeoutError,
    LLMUnavailableError,
    error_summary,
    get_status_code,
)

logger = logging.getLogger(__name__)

AUTH_STATUS_CODES = {401, 403}
TIMEOUT_STATUS_CODES = {408, 504}
RATE_LIMIT_STATUS_CODE = 429


# Models

class LLMProvider(BaseModel):
    """A chat model together with the settings used to create it."""

    model_config = ConfigDict(arbitrary_types_allowed=True)

    chat_model: BaseChatModel
    settings: LLMSettings


# Public functions

def create_llm_provider(settings):
    """Create the NVIDIA chat client from the settings.

    A model in the NVIDIA library's model table is created without calling the
    API. Any other model name is looked up at NVIDIA first, so that lookup can
    fail with an authentication or connection error.
    """

    # Imported here so the client library loads only when the LLM is used
    from langchain_nvidia_ai_endpoints import ChatNVIDIA

    try:
        # Build the NVIDIA client with the generation settings
        chat_model = ChatNVIDIA(
            model=settings.model,
            api_key=settings.api_key.get_secret_value(),
            temperature=settings.temperature,
            max_completion_tokens=settings.max_tokens,
            timeout=settings.timeout_seconds,
            # Do not send usage statistics to NVIDIA from a government service
            usage_telemetry_enabled=False,
        )

    except Exception as error:
        llm_error = to_llm_error(error)

        # Errors with no clear cause mean the client itself could not be built
        if type(llm_error) is LLMError:
            raise LLMConfigError(f"LLM client could not be created: {error_summary(error)}") from None

        raise llm_error from None

    # Keep the client together with its settings
    provider = LLMProvider(chat_model=chat_model, settings=settings)

    return provider


def load_llm_provider(env_file=ENV_FILE):
    """Read the LLM settings from .env and create the provider."""

    # Read and check the settings
    settings = load_llm_settings(env_file=env_file)

    # Create the NVIDIA client
    provider = create_llm_provider(settings)

    return provider


def invoke_llm(provider, messages, config=None):
    """Send chat messages to the model and return the answer text.

    config is the LangChain run config, e.g. the LangSmith run name, tags, and
    metadata from build_trace_config(). Every failure raises an LLMError subclass.
    """

    try:
        # Send the messages to NVIDIA
        reply = provider.chat_model.invoke(messages, config=config)

    except Exception as error:
        raise to_llm_error(error) from None

    # An empty answer cannot be shown to a citizen
    answer = str(reply.text).strip()
    if not answer:
        raise LLMError("The model returned an empty answer")

    # Log the outcome without the question or answer text, which may hold personal details
    logger.info(
        "llm answer generated",
        extra={"model": provider.settings.model, "answer_length": len(answer)},
    )

    return answer


def to_llm_error(error):
    """Turn an NVIDIA client error into an LLMError subclass with a short message."""

    # Already classified
    if isinstance(error, LLMError):
        return error

    # Timeouts: the request timeout, or the NVIDIA polling timeout
    if isinstance(error, (requests.exceptions.Timeout, TimeoutError)):
        llm_error = LLMTimeoutError("NVIDIA did not respond in time")
        return llm_error

    # Network errors: NVIDIA cannot be reached
    if isinstance(error, (requests.exceptions.ConnectionError, ConnectionError)):
        llm_error = LLMUnavailableError("NVIDIA is not reachable")
        return llm_error

    # HTTP errors: the NVIDIA client puts the status code at the start of the message
    status_code = get_status_code(error)
    summary = error_summary(error)

    if status_code in AUTH_STATUS_CODES:
        llm_error = LLMAuthError("NVIDIA rejected the API key")

    elif status_code in TIMEOUT_STATUS_CODES:
        llm_error = LLMTimeoutError("NVIDIA did not respond in time")

    elif status_code == RATE_LIMIT_STATUS_CODE:
        llm_error = LLMUnavailableError("NVIDIA rate limit reached")

    elif status_code is not None and status_code >= 500:
        llm_error = LLMUnavailableError(f"NVIDIA server error: {summary}")

    elif status_code is not None and status_code >= 400:
        llm_error = LLMRequestError(f"NVIDIA rejected the request: {summary}")

    else:
        llm_error = LLMError(f"LLM request failed: {summary}")

    return llm_error
