"""Tests for app/memory/config.py: memory settings, and errors that never show secrets."""

import base64

import pytest

from app.memory.config import DEFAULT_MAX_TURNS, MemoryConfigError, load_memory_settings

PASSWORD = "synthetic-secret-password"
KEY = base64.b64encode(bytes(range(32))).decode()
URL = f"postgres://govai_agent_app:{PASSWORD}@127.0.0.1:5432/govai_agent?sslmode=disable"


def env(**overrides):

    # A complete set of memory values, with some replaced
    values = {"AGENT_MEMORY_DATABASE_URL": URL, "AGENT_MEMORY_DATA_KEY": KEY}
    values.update(overrides)

    return values


# Loading

def test_blank_url_turns_memory_off():

    assert load_memory_settings(environ={}) is None
    assert load_memory_settings(environ=env(AGENT_MEMORY_DATABASE_URL="  ")) is None


def test_valid_settings_load_with_the_default_turns():

    settings = load_memory_settings(environ=env())

    assert settings.database_url.get_secret_value() == URL
    assert settings.data_key.get_secret_value() == bytes(range(32))
    assert settings.max_turns == DEFAULT_MAX_TURNS


def test_max_turns_can_be_set():

    assert load_memory_settings(environ=env(AGENT_MEMORY_MAX_TURNS="3")).max_turns == 3


@pytest.mark.parametrize(
    "url",
    [
        f"postgres://govai_agent_app:{PASSWORD}@postgres:5432/govai_agent",
        f"postgresql://govai_agent_app:{PASSWORD}@localhost/govai_agent",
        f"postgres://govai_agent_app:{PASSWORD}@10.0.0.5:5432/govai_agent?sslmode=disable",
    ],
)
def test_allowed_urls(url):

    assert load_memory_settings(environ=env(AGENT_MEMORY_DATABASE_URL=url)) is not None


# Rejected settings

@pytest.mark.parametrize(
    "overrides",
    [
        {"AGENT_MEMORY_DATABASE_URL": f"mysql://u:{PASSWORD}@127.0.0.1/db?sslmode=disable"},
        {"AGENT_MEMORY_DATABASE_URL": "postgres://127.0.0.1/db?sslmode=disable"},
        {"AGENT_MEMORY_DATA_KEY": ""},
        {"AGENT_MEMORY_DATA_KEY": "not base64!"},
        {"AGENT_MEMORY_DATA_KEY": base64.b64encode(b"short").decode()},
        {"AGENT_MEMORY_MAX_TURNS": "0"},
        {"AGENT_MEMORY_MAX_TURNS": "21"},
        {"AGENT_MEMORY_MAX_TURNS": "six"},
    ],
)
def test_invalid_settings_are_rejected(overrides):

    with pytest.raises(MemoryConfigError):
        load_memory_settings(environ=env(**overrides))


def test_errors_never_show_the_password_or_key():

    bad_url = f"mysql://u:{PASSWORD}@db.example.org/db"

    with pytest.raises(MemoryConfigError) as url_error:
        load_memory_settings(environ=env(AGENT_MEMORY_DATABASE_URL=bad_url))

    with pytest.raises(MemoryConfigError) as key_error:
        load_memory_settings(environ=env(AGENT_MEMORY_DATA_KEY=KEY[:-4]))

    assert PASSWORD not in str(url_error.value)
    assert KEY[:-4] not in str(key_error.value)


def test_settings_never_print_the_password_or_key():

    settings = load_memory_settings(environ=env())

    assert PASSWORD not in repr(settings)
    assert PASSWORD not in str(settings)
