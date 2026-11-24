"""Conversation memory settings, read from .env."""

import base64
import binascii
import os
from pathlib import Path
from urllib.parse import urlsplit

from dotenv import dotenv_values
from psycopg.rows import dict_row
from pydantic import BaseModel, ConfigDict, Field, SecretBytes, SecretStr

ENV_FILE = Path(__file__).resolve().parents[2] / ".env"

# AES-256 key length
KEY_SIZE = 32
MAX_KEY_ID = 255

DEFAULT_MAX_TURNS = 6
MAX_TURNS_LIMIT = 20

# core-service waits up to a minute, so a pending turn older than this was abandoned
DEFAULT_IN_FLIGHT_SECONDS = 90
IN_FLIGHT_RANGE = (30, 600)

# A repeated question this soon after its answer gets the stored answer (core-service used 10 min)
DEFAULT_REPLAY_SECONDS = 600
REPLAY_RANGE = (0, 3600)

# Every memory database connection: short limits so a slow database fails a request instead of hanging it
CONNECTION_OPTIONS = {
    "autocommit": True,
    "prepare_threshold": 0,
    "row_factory": dict_row,
    "connect_timeout": 5,
    "application_name": "govai-agent",
    "options": "-c statement_timeout=5000 -c lock_timeout=3000",
}


# Errors

class MemoryConfigError(ValueError):
    """The memory settings are invalid. The message never holds a URL or a key."""


# Models

class MemorySettings(BaseModel):
    """Where conversations are stored, the keys that encrypt them, and the short-term memory limits."""

    model_config = ConfigDict(frozen=True)

    database_url: SecretStr
    data_key: SecretBytes
    data_key_id: int = Field(default=1, ge=1, le=MAX_KEY_ID)
    old_data_keys: dict[int, SecretBytes] = {}
    max_turns: int = Field(default=DEFAULT_MAX_TURNS, ge=1, le=MAX_TURNS_LIMIT)
    in_flight_seconds: int = DEFAULT_IN_FLIGHT_SECONDS
    replay_seconds: int = DEFAULT_REPLAY_SECONDS


# Public functions

def load_memory_settings(environ=None, env_file=ENV_FILE):
    """Build MemorySettings from .env, or return None when memory is off (no database URL).

    When environ is given (as in tests), only that mapping is used and no file is read.
    """

    # Read the .env file; real environment variables take priority over it
    if environ is None:
        environ = {**dotenv_values(env_file), **os.environ}

    # No database URL: the API answers every question without conversations or memory
    database_url = read(environ, "AGENT_MEMORY_DATABASE_URL")
    if not database_url:
        return None

    # Check every value without ever putting a URL or key in an error
    check_database_url(database_url)
    data_key_id = read_number(environ, "AGENT_MEMORY_DATA_KEY_ID", 1, (1, MAX_KEY_ID))
    data_key = decode_key(read(environ, "AGENT_MEMORY_DATA_KEY"), "AGENT_MEMORY_DATA_KEY")
    old_data_keys = read_old_keys(read(environ, "AGENT_MEMORY_OLD_DATA_KEYS"), data_key_id)

    settings = MemorySettings(
        database_url=database_url,
        data_key=data_key,
        data_key_id=data_key_id,
        old_data_keys=old_data_keys,
        max_turns=read_number(environ, "AGENT_MEMORY_MAX_TURNS", DEFAULT_MAX_TURNS, (1, MAX_TURNS_LIMIT)),
        in_flight_seconds=read_number(environ, "AGENT_MEMORY_IN_FLIGHT_SECONDS", DEFAULT_IN_FLIGHT_SECONDS, IN_FLIGHT_RANGE),
        replay_seconds=read_number(environ, "AGENT_MEMORY_REPLAY_SECONDS", DEFAULT_REPLAY_SECONDS, REPLAY_RANGE),
    )

    return settings


# Helper functions

def read(environ, name):

    value = (environ.get(name) or "").strip()

    return value


def check_database_url(database_url):

    # A postgres:// URL with a host and a login
    url = split_url(database_url, "AGENT_MEMORY_DATABASE_URL")

    if url.scheme not in ("postgres", "postgresql") or not url.hostname or not url.username:
        raise MemoryConfigError("AGENT_MEMORY_DATABASE_URL must be postgres://user:password@host:port/database")


def split_url(value, name):

    # urlsplit raises for a bad port; its message may hold the URL, so it is replaced
    try:
        url = urlsplit(value)
        url.port

    except ValueError:
        raise MemoryConfigError(f"{name} is not a valid URL") from None

    return url


def decode_key(value, name):

    # Base64 of exactly 32 random bytes
    try:
        key = base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError):
        raise MemoryConfigError(f"{name} must be base64") from None

    if len(key) != KEY_SIZE:
        raise MemoryConfigError(f"{name} must be base64 of {KEY_SIZE} bytes")

    return key


def read_old_keys(value, current_id):

    # "id:base64,id:base64": keys that still decrypt older values
    old_keys = {}
    if not value:
        return old_keys

    for entry in value.split(","):
        key_id_text, separator, key_text = entry.strip().partition(":")
        if not separator or not key_id_text.isdigit():
            raise MemoryConfigError("AGENT_MEMORY_OLD_DATA_KEYS must be id:base64 entries separated by commas")

        key_id = int(key_id_text)
        if not 1 <= key_id <= MAX_KEY_ID or key_id == current_id or key_id in old_keys:
            raise MemoryConfigError("AGENT_MEMORY_OLD_DATA_KEYS ids must be 1-255, unique, and not the current key ID")

        old_keys[key_id] = decode_key(key_text.strip(), "AGENT_MEMORY_OLD_DATA_KEYS")

    return old_keys


def read_number(environ, name, default, limits):

    # Blank means the default
    value = read(environ, name)
    if not value:
        return default

    try:
        number = int(value)
    except ValueError:
        raise MemoryConfigError(f"{name} must be a whole number") from None

    low, high = limits
    if not low <= number <= high:
        raise MemoryConfigError(f"{name} must be {low}-{high}")

    return number
