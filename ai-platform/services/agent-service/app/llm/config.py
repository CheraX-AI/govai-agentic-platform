"""Azure OpenAI chat model settings, read from .env."""

import os
from pathlib import Path
from urllib.parse import urlparse

from dotenv import dotenv_values
from pydantic import BaseModel, ConfigDict, Field, SecretStr, ValidationError, field_validator

from app.llm.exceptions import LLMConfigError, format_validation_error

ENV_FILE = Path(__file__).resolve().parents[2] / ".env"

# Limits the .env values are checked against
MAX_TEMPERATURE = 1
MAX_TOKENS_LIMIT = 4096
MAX_TIMEOUT_SECONDS = 120

# The OpenAI-compatible v1 API of an Azure OpenAI or Azure AI Foundry resource
ENDPOINT_PATH = "/openai/v1"

# Every setting must be present in .env (or the environment)
REQUIRED_VARIABLES = [
    "AZURE_OPENAI_ENDPOINT",
    "AZURE_OPENAI_API_KEY",
    "LLM_MODEL",
    "LLM_TEMPERATURE",
    "LLM_MAX_TOKENS",
    "LLM_TIMEOUT_SECONDS",
]


# Models

class LLMSettings(BaseModel):
    """Chat model settings. The API key is a SecretStr so it is never printed or logged.

    Each field's alias is its .env name, so validation errors name the variable to fix.
    """

    model_config = ConfigDict(populate_by_name=True)

    endpoint: str = Field(alias="AZURE_OPENAI_ENDPOINT")
    api_key: SecretStr = Field(alias="AZURE_OPENAI_API_KEY")
    # The Azure deployment name, e.g. gpt-4.1-mini-1
    model: str = Field(alias="LLM_MODEL", min_length=1)
    # Low values keep answers close to the documents
    temperature: float = Field(alias="LLM_TEMPERATURE", ge=0, le=MAX_TEMPERATURE, allow_inf_nan=False)
    max_tokens: int = Field(alias="LLM_MAX_TOKENS", ge=1, le=MAX_TOKENS_LIMIT)
    timeout_seconds: float = Field(alias="LLM_TIMEOUT_SECONDS", gt=0, le=MAX_TIMEOUT_SECONDS, allow_inf_nan=False)

    @field_validator("endpoint")
    @classmethod
    def check_endpoint(cls, endpoint):

        # Questions may hold personal details, so they are only sent over HTTPS
        endpoint = endpoint.rstrip("/")
        url = urlparse(endpoint)
        if url.scheme != "https" or not url.netloc:
            raise ValueError("must be an https:// URL")

        # Other paths are older Azure APIs that need an api-version
        if url.path != ENDPOINT_PATH:
            raise ValueError(f"must end with '{ENDPOINT_PATH}'")

        return endpoint


# Public functions

def load_llm_settings(environ=None, env_file=ENV_FILE):
    """Build LLMSettings from the .env file.

    When environ is given (as in tests), only that mapping is used and no
    file is read.
    """

    # Read the .env file; real environment variables take priority over it
    if environ is None:
        environ = {**dotenv_values(env_file), **os.environ}

    # Every setting must be present and not blank
    values = {}
    for name in REQUIRED_VARIABLES:
        value = (environ.get(name) or "").strip()
        if not value:
            raise LLMConfigError(f"{name} is required in .env")
        values[name] = value

    try:
        # Check the values and build the settings
        settings = LLMSettings.model_validate(values)

    except ValidationError as error:
        # "from None" drops the Pydantic error, whose input values include the API key
        raise LLMConfigError(format_validation_error(error)) from None

    return settings
