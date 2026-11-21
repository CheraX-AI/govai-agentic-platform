"""Conversation memory settings, read from .env."""

import base64
import binascii
import os
from pathlib import Path
from urllib.parse import urlsplit

from dotenv import dotenv_values
from pydantic import BaseModel, ConfigDict, Field, SecretBytes, SecretStr

ENV_FILE = Path(__file__).resolve().parents[2] / ".env"

# AES-256 key length, the same as core-service's message key
KEY_SIZE = 32

DEFAULT_MAX_TURNS = 6
MAX_TURNS_LIMIT = 20


# Errors

class MemoryConfigError(ValueError):
    """The memory settings are invalid. The message never holds the URL or the key."""


# Models

class MemorySettings(BaseModel):
    """Where memory is stored, the key that encrypts it, and how many turns are kept."""

    model_config = ConfigDict(frozen=True)

    database_url: SecretStr
    data_key: SecretBytes
    max_turns: int = Field(default=DEFAULT_MAX_TURNS, ge=1, le=MAX_TURNS_LIMIT)


# Public functions

def load_memory_settings(environ=None, env_file=ENV_FILE):
    """Build MemorySettings from .env, or return None when memory is off (no database URL).

    When environ is given (as in tests), only that mapping is used and no file is read.
    """

    # Read the .env file; real environment variables take priority over it
    if environ is None:
        environ = {**dotenv_values(env_file), **os.environ}

    # No database URL: the API answers every question without memory
    database_url = (environ.get("AGENT_MEMORY_DATABASE_URL") or "").strip()
    if not database_url:
        return None

    # Check the URL and the key without ever putting their values in an error
    check_database_url(database_url)
    data_key = decode_key((environ.get("AGENT_MEMORY_DATA_KEY") or "").strip())
    max_turns = read_max_turns((environ.get("AGENT_MEMORY_MAX_TURNS") or "").strip())

    settings = MemorySettings(database_url=database_url, data_key=data_key, max_turns=max_turns)

    return settings


# Helper functions

def check_database_url(database_url):

    # A postgres:// URL with a host and a login
    try:
        url = urlsplit(database_url)
        host = url.hostname

    except ValueError:
        raise MemoryConfigError("AGENT_MEMORY_DATABASE_URL is not a valid URL") from None

    if url.scheme not in ("postgres", "postgresql") or not host or not url.username:
        raise MemoryConfigError("AGENT_MEMORY_DATABASE_URL must be postgres://user:password@host:port/database")


def decode_key(value):

    # Base64 of exactly 32 random bytes
    try:
        key = base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError):
        raise MemoryConfigError("AGENT_MEMORY_DATA_KEY must be base64") from None

    if len(key) != KEY_SIZE:
        raise MemoryConfigError(f"AGENT_MEMORY_DATA_KEY must be base64 of {KEY_SIZE} bytes")

    return key


def read_max_turns(value):

    # Blank means the default
    if not value:
        return DEFAULT_MAX_TURNS

    try:
        max_turns = int(value)
    except ValueError:
        raise MemoryConfigError("AGENT_MEMORY_MAX_TURNS must be a whole number") from None

    if not 1 <= max_turns <= MAX_TURNS_LIMIT:
        raise MemoryConfigError(f"AGENT_MEMORY_MAX_TURNS must be 1-{MAX_TURNS_LIMIT}")

    return max_turns
