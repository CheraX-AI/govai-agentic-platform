"""Agent API settings, read from .env."""

import os
from pathlib import Path
from typing import Literal

from dotenv import dotenv_values
from pydantic import BaseModel, ConfigDict, Field, SecretStr, ValidationError, model_validator

from app.llm.exceptions import format_validation_error

ENV_FILE = Path(__file__).resolve().parents[2] / ".env"

AUTH_MODE_BEARER = "bearer"
AUTH_MODE_NONE = "none"

# Long enough to be unguessable; core-service accepts at most 8192 characters
MIN_TOKEN_LENGTH = 32
MAX_TOKEN_LENGTH = 8192


# Errors

class AgentAPIConfigError(ValueError):
    """The agent API settings are missing or invalid."""


# Models

class AgentAPISettings(BaseModel):
    """How callers authenticate. The token is a SecretStr so it is never printed or logged.

    bearer: every chat request needs "Authorization: Bearer <AGENT_API_TOKEN>".
    none: no token; only for local development with the port bound to 127.0.0.1.
    """

    model_config = ConfigDict(populate_by_name=True)

    auth_mode: Literal["bearer", "none"] = Field(alias="AGENT_API_AUTH_MODE")
    token: SecretStr | None = Field(default=None, alias="AGENT_API_TOKEN")

    @model_validator(mode="after")
    def check_token(self):

        token = self.token.get_secret_value() if self.token is not None else ""

        if self.auth_mode == AUTH_MODE_NONE:
            # A token that is never checked would give a false sense of protection
            if token:
                raise ValueError("AGENT_API_TOKEN must be empty when AGENT_API_AUTH_MODE is none")
            return self

        if not MIN_TOKEN_LENGTH <= len(token) <= MAX_TOKEN_LENGTH:
            raise ValueError(f"AGENT_API_TOKEN must be {MIN_TOKEN_LENGTH}-{MAX_TOKEN_LENGTH} characters")

        # The same characters core-service allows in a bearer token
        for character in token:
            if not 33 <= ord(character) <= 126:
                raise ValueError("AGENT_API_TOKEN must be printable ASCII without spaces")

        return self


# Public functions

def load_api_settings(environ=None, env_file=ENV_FILE):
    """Build AgentAPISettings from the .env file.

    When environ is given (as in tests), only that mapping is used and no
    file is read.
    """

    # Read the .env file; real environment variables take priority over it
    if environ is None:
        environ = {**dotenv_values(env_file), **os.environ}

    # The mode must be chosen on purpose; there is no default
    auth_mode = (environ.get("AGENT_API_AUTH_MODE") or "").strip()
    if not auth_mode:
        raise AgentAPIConfigError("AGENT_API_AUTH_MODE is required in .env (bearer or none)")

    values = {"AGENT_API_AUTH_MODE": auth_mode}
    token = (environ.get("AGENT_API_TOKEN") or "").strip()
    if token:
        values["AGENT_API_TOKEN"] = token

    try:
        settings = AgentAPISettings.model_validate(values)

    except ValidationError as error:
        # "from None" drops the Pydantic error, whose input values include the token
        raise AgentAPIConfigError(format_validation_error(error)) from None

    return settings
