"""Tests for app/embeddings/config.py: reading and validating the .env settings."""

import pytest

from app.embeddings.config import REQUIRED_VARIABLES, load_settings
from app.embeddings.exceptions import EmbeddingConfigError
from tests.helpers import TEST_NVIDIA_KEY, make_env, write_env_file


# Helper functions

def clear_embedding_variables(monkeypatch):

    # Remove real settings so a test never reads the developer's environment
    for name in REQUIRED_VARIABLES:
        monkeypatch.delenv(name, raising=False)


# Reading settings

def test_every_setting_is_read_from_env_values():

    # All model details come from .env, none from code defaults
    settings = load_settings(make_env())

    assert settings.api_key.get_secret_value() == TEST_NVIDIA_KEY
    assert settings.model == "nvidia/test-embed-model"
    assert settings.batch_size == 20
    assert settings.max_retries == 3
    assert settings.retry_delay_seconds == 0.5
    assert settings.truncate == "NONE"


def test_settings_are_read_from_the_env_file(tmp_path, monkeypatch):

    # Without explicit values, load_settings reads the given .env file
    clear_embedding_variables(monkeypatch)
    env_file = write_env_file(tmp_path / ".env", make_env(EMBEDDING_MODEL="nvidia/from-file"))

    settings = load_settings(env_file=env_file)

    assert settings.model == "nvidia/from-file"
    assert settings.api_key.get_secret_value() == TEST_NVIDIA_KEY


def test_environment_variables_override_the_env_file(tmp_path, monkeypatch):

    # A real environment variable wins over the file, e.g. in CI
    clear_embedding_variables(monkeypatch)
    env_file = write_env_file(tmp_path / ".env", make_env(EMBEDDING_BATCH_SIZE="10"))
    monkeypatch.setenv("EMBEDDING_BATCH_SIZE", "5")

    settings = load_settings(env_file=env_file)

    assert settings.batch_size == 5


def test_missing_env_file_reports_the_first_setting(tmp_path, monkeypatch):

    # No file and no environment variables: the key is reported first
    clear_embedding_variables(monkeypatch)

    with pytest.raises(EmbeddingConfigError, match="NVIDIA_API_KEY is required in .env"):
        load_settings(env_file=tmp_path / "missing.env")


def test_values_are_trimmed():

    # Spaces around a value in .env are ignored
    settings = load_settings(make_env(EMBEDDING_MODEL="  nvidia/test-embed-model  "))

    assert settings.model == "nvidia/test-embed-model"


def test_truncate_is_case_insensitive():

    settings = load_settings(make_env(EMBEDDING_TRUNCATE="end"))

    assert settings.truncate == "END"


# Rejecting settings

@pytest.mark.parametrize("name", REQUIRED_VARIABLES)
def test_missing_or_blank_setting_is_named(name):

    # A missing value is reported by name
    missing = make_env()
    del missing[name]

    with pytest.raises(EmbeddingConfigError, match=f"{name} is required in .env"):
        load_settings(missing)

    # A blank value is reported the same way
    with pytest.raises(EmbeddingConfigError, match=f"{name} is required in .env"):
        load_settings(make_env(**{name: "   "}))


@pytest.mark.parametrize(
    "overrides, expected_message",
    [
        ({"NVIDIA_API_KEY": "sk-wrong-provider-key"}, "must start with 'nvapi-'"),
        ({"EMBEDDING_BATCH_SIZE": "0"}, "batch_size"),
        ({"EMBEDDING_BATCH_SIZE": "51"}, "batch_size"),
        ({"EMBEDDING_BATCH_SIZE": "many"}, "batch_size"),
        ({"EMBEDDING_MAX_RETRIES": "-1"}, "max_retries"),
        ({"EMBEDDING_MAX_RETRIES": "6"}, "max_retries"),
        ({"EMBEDDING_RETRY_DELAY_SECONDS": "-1"}, "retry_delay_seconds"),
        ({"EMBEDDING_TRUNCATE": "MIDDLE"}, "truncate"),
    ],
)
def test_invalid_settings_are_rejected(overrides, expected_message):

    # Each invalid value raises a config error that names the problem
    with pytest.raises(EmbeddingConfigError, match=expected_message):
        load_settings(make_env(**overrides))


@pytest.mark.parametrize("batch_size", ["1", "50"])
def test_batch_size_limits_are_accepted(batch_size):

    # The smallest and largest allowed batch sizes are valid
    settings = load_settings(make_env(EMBEDDING_BATCH_SIZE=batch_size))

    assert settings.batch_size == int(batch_size)


# Keeping the key secret

def test_api_key_never_appears_in_errors_or_repr():

    # A wrong key must not be echoed back in the error message
    wrong_key = "sk-secret-value-123456"
    with pytest.raises(EmbeddingConfigError) as error:
        load_settings(make_env(NVIDIA_API_KEY=wrong_key))

    assert wrong_key not in str(error.value)

    # A valid key is hidden when the settings are printed
    settings = load_settings(make_env())

    assert TEST_NVIDIA_KEY not in repr(settings)
    assert TEST_NVIDIA_KEY not in str(settings)
    assert TEST_NVIDIA_KEY not in settings.model_dump_json()
