"""Creates the Azure OpenAI chat model and sends messages to it."""

import logging

import openai
from langchain_core.language_models import BaseChatModel
from langchain_openai import ChatOpenAI
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
)

logger = logging.getLogger(__name__)

# One retry for a dropped connection or a 429/5xx; more would keep the citizen waiting
MAX_RETRIES = 1

AUTH_STATUS_CODES = {401, 403}
TIMEOUT_STATUS_CODES = {408, 504}
RATE_LIMIT_STATUS_CODE = 429
NOT_FOUND_STATUS_CODE = 404
CONTENT_FILTER = "content_filter"


# Models

class LLMProvider(BaseModel):
    """A chat model together with the settings used to create it."""

    model_config = ConfigDict(arbitrary_types_allowed=True)

    chat_model: BaseChatModel
    settings: LLMSettings


# Public functions

def create_llm_provider(settings):
    """Create the Azure OpenAI chat client from the settings. No API call is made."""

    try:
        # The Azure v1 endpoint speaks the OpenAI API, so no api-version is needed
        chat_model = ChatOpenAI(
            model=settings.model,
            base_url=settings.endpoint,
            api_key=settings.api_key,
            temperature=settings.temperature,
            max_tokens=settings.max_tokens,
            timeout=settings.timeout_seconds,
            max_retries=MAX_RETRIES,
            # Chat Completions, which reports when the content filter cut an answer
            use_responses_api=False,
        )

    except Exception as error:
        raise LLMConfigError(f"LLM client could not be created: {error_summary(error)}") from None

    # Keep the client together with its settings
    provider = LLMProvider(chat_model=chat_model, settings=settings)

    return provider


def load_llm_provider(env_file=ENV_FILE):
    """Read the LLM settings from .env and create the provider."""

    # Read and check the settings
    settings = load_llm_settings(env_file=env_file)

    # Create the Azure OpenAI client
    provider = create_llm_provider(settings)

    return provider


def invoke_llm(provider, messages, config=None):
    """Send chat messages to the model and return the answer text.

    config is the LangChain run config, e.g. the LangSmith run name, tags, and
    metadata from build_trace_config(). Every failure raises an LLMError subclass.
    """

    try:
        # Send the messages to Azure OpenAI
        reply = provider.chat_model.invoke(messages, config=config)

    except Exception as error:
        raise to_llm_error(error) from None

    # Azure's content filter can stop an answer part way; a partial answer is not shown
    if reply.response_metadata.get("finish_reason") == CONTENT_FILTER:
        raise LLMRequestError("Azure OpenAI content filter blocked the answer")

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
    """Turn an Azure OpenAI client error into an LLMError subclass with a short message."""

    # Already classified
    if isinstance(error, LLMError):
        return error

    # Timeouts; APITimeoutError is a kind of APIConnectionError, so it is checked first
    if isinstance(error, (openai.APITimeoutError, TimeoutError)):
        llm_error = LLMTimeoutError("Azure OpenAI did not respond in time")
        return llm_error

    # Network errors: Azure cannot be reached
    if isinstance(error, (openai.APIConnectionError, ConnectionError)):
        llm_error = LLMUnavailableError("Azure OpenAI is not reachable")
        return llm_error

    # Anything else without an HTTP status
    if not isinstance(error, openai.APIStatusError):
        llm_error = LLMError(f"LLM request failed: {error_summary(error)}")
        return llm_error

    # HTTP errors: only the status and Azure's error code are kept, never the error body
    status_code = error.status_code
    error_code = error.code or "no error code"

    if status_code in AUTH_STATUS_CODES:
        llm_error = LLMAuthError("Azure OpenAI rejected the API key")

    elif status_code in TIMEOUT_STATUS_CODES:
        llm_error = LLMTimeoutError("Azure OpenAI did not respond in time")

    elif status_code == RATE_LIMIT_STATUS_CODE:
        llm_error = LLMUnavailableError("Azure OpenAI rate limit reached")

    elif status_code >= 500:
        llm_error = LLMUnavailableError(f"Azure OpenAI server error ({status_code} {error_code})")

    elif error_code == CONTENT_FILTER:
        llm_error = LLMRequestError("Azure OpenAI content filter blocked the request")

    elif status_code == NOT_FOUND_STATUS_CODE:
        llm_error = LLMRequestError(f"Azure OpenAI deployment not found, check LLM_MODEL ({error_code})")

    else:
        llm_error = LLMRequestError(f"Azure OpenAI rejected the request ({status_code} {error_code})")

    return llm_error
