"""Tests for app/llm/config.py: reading and validating the LLM .env settings."""

import pytest

from app.llm.config import REQUIRED_VARIABLES, load_llm_settings
from app.llm.exceptions import LLMConfigError
from helpers import TEST_MODEL, TEST_NVIDIA_KEY, make_env, write_env_file


# Helper functions

def clear_llm_variables(monkeypatch):

    # Remove real settings so a test never reads the developer's environment
    for name in REQUIRED_VARIABLES:
        monkeypatch.delenv(name, raising=False)


# Reading settings

def test_every_setting_is_read_from_env_values():

    # All model details come from .env, none from code defaults
    settings = load_llm_settings(make_env())

    assert settings.api_key.get_secret_value() == TEST_NVIDIA_KEY
    assert settings.model == TEST_MODEL
    assert settings.temperature == 0.1
    assert settings.max_tokens == 512
    assert settings.timeout_seconds == 15


def test_settings_are_read_from_the_env_file(tmp_path, monkeypatch):

    # Without explicit values, the given .env file is read
    clear_llm_variables(monkeypatch)
    env_file = write_env_file(tmp_path / ".env", make_env(LLM_MODEL="nvidia/from-file"))

    settings = load_llm_settings(env_file=env_file)

    assert settings.model == "nvidia/from-file"


def test_environment_variables_override_the_env_file(tmp_path, monkeypatch):

    # A real environment variable wins over the file, e.g. in CI
    clear_llm_variables(monkeypatch)
    env_file = write_env_file(tmp_path / ".env", make_env(LLM_MAX_TOKENS="512"))
    monkeypatch.setenv("LLM_MAX_TOKENS", "256")

    settings = load_llm_settings(env_file=env_file)

    assert settings.max_tokens == 256


def test_values_are_trimmed():

    # Spaces around a value in .env are ignored
    settings = load_llm_settings(make_env(LLM_MODEL=f"  {TEST_MODEL}  "))

    assert settings.model == TEST_MODEL


def test_missing_env_file_reports_the_first_setting(tmp_path, monkeypatch):

    # No file and no environment variables: the key is reported first
    clear_llm_variables(monkeypatch)

    with pytest.raises(LLMConfigError, match="NVIDIA_API_KEY is required in .env"):
        load_llm_settings(env_file=tmp_path / "missing.env")


# Missing settings

@pytest.mark.parametrize("name", REQUIRED_VARIABLES)
def test_each_missing_setting_is_named(name):

    # Leave one variable out
    env = make_env()
    del env[name]

    with pytest.raises(LLMConfigError, match=f"{name} is required in .env"):
        load_llm_settings(env)


@pytest.mark.parametrize("name", REQUIRED_VARIABLES)
def test_blank_setting_counts_as_missing(name):

    # A variable of only spaces is not a value
    with pytest.raises(LLMConfigError, match=f"{name} is required in .env"):
        load_llm_settings(make_env(**{name: "   "}))


# Invalid settings

@pytest.mark.parametrize(
    "name, value",
    [
        ("LLM_TEMPERATURE", "-0.1"),
        ("LLM_TEMPERATURE", "1.5"),
        ("LLM_TEMPERATURE", "nan"),
        ("LLM_TEMPERATURE", "warm"),
        ("LLM_MAX_TOKENS", "0"),
        ("LLM_MAX_TOKENS", "100000"),
        ("LLM_MAX_TOKENS", "1.5"),
        ("LLM_TIMEOUT_SECONDS", "0"),
        ("LLM_TIMEOUT_SECONDS", "121"),
        ("LLM_TIMEOUT_SECONDS", "inf"),
    ],
)
def test_out_of_range_values_are_rejected_by_name(name, value):

    # The error names the .env variable to fix
    with pytest.raises(LLMConfigError, match=name):
        load_llm_settings(make_env(**{name: value}))


def test_key_without_nvidia_prefix_is_rejected():

    with pytest.raises(LLMConfigError, match="NVIDIA_API_KEY: .*nvapi-"):
        load_llm_settings(make_env(NVIDIA_API_KEY="sk-wrong-provider-key"))


def test_errors_never_show_the_api_key():

    # A wrong key and a wrong value at once: neither value is in the message
    secret = "sk-secret-value-9876"

    with pytest.raises(LLMConfigError) as error:
        load_llm_settings(make_env(NVIDIA_API_KEY=secret, LLM_TEMPERATURE="5"))

    assert secret not in str(error.value)
    assert error.value.__cause__ is None


def test_settings_hide_the_api_key_when_printed():

    # SecretStr keeps the key out of repr and str, and so out of logs
    settings = load_llm_settings(make_env())

    assert TEST_NVIDIA_KEY not in repr(settings)
    assert TEST_NVIDIA_KEY not in str(settings)
