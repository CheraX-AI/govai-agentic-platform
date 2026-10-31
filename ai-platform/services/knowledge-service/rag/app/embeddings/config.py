"""NVIDIA embedding settings, read from the rag/.env file.

All model settings and the API key live in .env; nothing is hardcoded here
except the safety limits the values are checked against. Real environment
variables override the file, which is useful in CI.
"""

import os
from pathlib import Path
from typing import Literal

from dotenv import dotenv_values
from pydantic import BaseModel, Field, SecretStr, ValidationError, field_validator

from app.embeddings.exceptions import EmbeddingConfigError, format_validation_error

ENV_FILE = Path(__file__).resolve().parents[2] / ".env"

# Limits the .env values are checked against
MAX_BATCH_SIZE = 50
MAX_RETRIES = 5
NVIDIA_KEY_PREFIX = "nvapi-"

# Every setting must be present in .env (or the environment)
REQUIRED_VARIABLES = [
    "NVIDIA_API_KEY",
    "EMBEDDING_MODEL",
    "EMBEDDING_BATCH_SIZE",
    "EMBEDDING_MAX_RETRIES",
    "EMBEDDING_RETRY_DELAY_SECONDS",
    "EMBEDDING_TRUNCATE",
]


# Models

class EmbeddingSettings(BaseModel):
    """Embedding settings. The API key is a SecretStr so it is never printed or logged."""

    api_key: SecretStr
    model: str = Field(min_length=1)
    batch_size: int = Field(ge=1, le=MAX_BATCH_SIZE)
    max_retries: int = Field(ge=0, le=MAX_RETRIES)
    retry_delay_seconds: float = Field(ge=0)
    truncate: Literal["NONE", "START", "END"]

    @field_validator("api_key")
    @classmethod
    def check_api_key(cls, api_key):

        # NVIDIA keys always start with "nvapi-"
        if not api_key.get_secret_value().startswith(NVIDIA_KEY_PREFIX):
            raise ValueError(f"NVIDIA_API_KEY must start with '{NVIDIA_KEY_PREFIX}'")

        return api_key


# Public functions

def load_settings(environ=None, env_file=ENV_FILE):
    """Build EmbeddingSettings from the .env file.

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
            raise EmbeddingConfigError(f"{name} is required in .env")
        values[name] = value

    # Validate; the message lists only field names and reasons, never the key itself
    try:
        return EmbeddingSettings(
            api_key=values["NVIDIA_API_KEY"],
            model=values["EMBEDDING_MODEL"],
            batch_size=values["EMBEDDING_BATCH_SIZE"],
            max_retries=values["EMBEDDING_MAX_RETRIES"],
            retry_delay_seconds=values["EMBEDDING_RETRY_DELAY_SECONDS"],
            truncate=values["EMBEDDING_TRUNCATE"].upper(),
        )
    except ValidationError as error:
        raise EmbeddingConfigError(format_validation_error(error)) from None
