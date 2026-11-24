"""Tests for app/memory/config.py: memory settings, and errors that never show secrets."""

import base64

import pytest

from app.memory.config import (
    DEFAULT_IN_FLIGHT_SECONDS,
    DEFAULT_MAX_TURNS,
    DEFAULT_REPLAY_SECONDS,
    MemoryConfigError,
    load_memory_settings,
)

PASSWORD = "synthetic-secret-password"
KEY = base64.b64encode(bytes(range(32))).decode()
OLD_KEY = base64.b64encode(bytes(range(1, 33))).decode()
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


def test_valid_settings_load_with_the_defaults():

    settings = load_memory_settings(environ=env())

    assert settings.database_url.get_secret_value() == URL
    assert settings.data_key.get_secret_value() == bytes(range(32))
    assert settings.data_key_id == 1
    assert settings.old_data_keys == {}
    assert settings.max_turns == DEFAULT_MAX_TURNS
    assert settings.in_flight_seconds == DEFAULT_IN_FLIGHT_SECONDS
    assert settings.replay_seconds == DEFAULT_REPLAY_SECONDS


def test_every_limit_can_be_set():

    settings = load_memory_settings(environ=env(
        AGENT_MEMORY_MAX_TURNS="3",
        AGENT_MEMORY_IN_FLIGHT_SECONDS="120",
        AGENT_MEMORY_REPLAY_SECONDS="0",
    ))

    assert settings.max_turns == 3
    assert settings.in_flight_seconds == 120
    assert settings.replay_seconds == 0


def test_key_ring_settings_load():

    settings = load_memory_settings(environ=env(AGENT_MEMORY_DATA_KEY_ID="2", AGENT_MEMORY_OLD_DATA_KEYS=f"1:{OLD_KEY}"))

    assert settings.data_key_id == 2
    assert settings.old_data_keys[1].get_secret_value() == bytes(range(1, 33))


@pytest.mark.parametrize(
    "url",
    [
        f"postgres://govai_agent_app:{PASSWORD}@postgres:5432/govai_agent",
        f"postgresql://govai_agent_app:{PASSWORD}@localhost/govai_agent",
        f"postgres://govai_agent_app:{PASSWORD}@10.0.0.5:5432/govai_agent?sslmode=disable",
    ],
)
def test_allowed_database_urls(url):

    assert load_memory_settings(environ=env(AGENT_MEMORY_DATABASE_URL=url)) is not None


# Rejected settings

@pytest.mark.parametrize(
    "overrides",
    [
        {"AGENT_MEMORY_DATABASE_URL": f"mysql://u:{PASSWORD}@127.0.0.1/db?sslmode=disable"},
        {"AGENT_MEMORY_DATABASE_URL": "postgres://127.0.0.1/db?sslmode=disable"},
        {"AGENT_MEMORY_DATABASE_URL": "postgres://u:pw@127.0.0.1:notaport/db"},
        {"AGENT_MEMORY_DATA_KEY": ""},
        {"AGENT_MEMORY_DATA_KEY": "not base64!"},
        {"AGENT_MEMORY_DATA_KEY": base64.b64encode(b"short").decode()},
        {"AGENT_MEMORY_DATA_KEY_ID": "0"},
        {"AGENT_MEMORY_DATA_KEY_ID": "256"},
        {"AGENT_MEMORY_OLD_DATA_KEYS": OLD_KEY},
        {"AGENT_MEMORY_OLD_DATA_KEYS": f"1:{OLD_KEY}"},
        {"AGENT_MEMORY_OLD_DATA_KEYS": f"2:{OLD_KEY},2:{OLD_KEY}"},
        {"AGENT_MEMORY_OLD_DATA_KEYS": "2:short"},
        {"AGENT_MEMORY_MAX_TURNS": "0"},
        {"AGENT_MEMORY_MAX_TURNS": "21"},
        {"AGENT_MEMORY_MAX_TURNS": "six"},
        {"AGENT_MEMORY_IN_FLIGHT_SECONDS": "10"},
        {"AGENT_MEMORY_REPLAY_SECONDS": "3601"},
    ],
)
def test_invalid_settings_are_rejected(overrides):

    with pytest.raises(MemoryConfigError):
        load_memory_settings(environ=env(**overrides))


def test_errors_never_show_a_password_or_key():

    errors = []
    for overrides in (
        {"AGENT_MEMORY_DATABASE_URL": f"mysql://u:{PASSWORD}@db.example.org/db"},
        {"AGENT_MEMORY_DATABASE_URL": f"postgres://u:{PASSWORD}@db.example.org:notaport/db"},
        {"AGENT_MEMORY_DATA_KEY": KEY[:-4]},
        {"AGENT_MEMORY_OLD_DATA_KEYS": f"2:{OLD_KEY[:-4]}"},
    ):
        with pytest.raises(MemoryConfigError) as error:
            load_memory_settings(environ=env(**overrides))
        errors.append(str(error.value))

    for message in errors:
        assert PASSWORD not in message
        assert KEY[:-4] not in message
        assert OLD_KEY[:-4] not in message


def test_settings_never_print_a_password_or_key():

    settings = load_memory_settings(environ=env(AGENT_MEMORY_DATA_KEY_ID="2", AGENT_MEMORY_OLD_DATA_KEYS=f"1:{OLD_KEY}"))

    assert PASSWORD not in repr(settings)
    assert PASSWORD not in str(settings)
    assert "\\x00\\x01\\x02" not in repr(settings)
