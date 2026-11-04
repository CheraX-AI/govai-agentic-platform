"""Tests for app/storage/config.py: reading and validating the object storage settings."""

import pytest

from app.storage.config import REQUIRED_VARIABLES, load_storage_settings
from app.storage.exceptions import StorageConfigError
from tests.helpers import write_env_file

TEST_SECRET_KEY = "test-secret-key-not-real"


# Helper functions

def make_storage_env(**overrides):
    """A complete, valid set of storage .env values, with some replaced by overrides."""

    values = {
        "OBJECT_STORAGE_ENDPOINT": "http://localhost:8333",
        "OBJECT_STORAGE_BUCKET": "govai-knowledge",
        "OBJECT_STORAGE_REGION": "ap-south-1",
        "OBJECT_STORAGE_ACCESS_KEY": "test_access_key",
        "OBJECT_STORAGE_SECRET_KEY": TEST_SECRET_KEY,
    }
    values.update(overrides)

    return values


def clear_storage_variables(monkeypatch):

    # Remove real settings so a test never reads the developer's environment
    for name in REQUIRED_VARIABLES:
        monkeypatch.delenv(name, raising=False)


# Reading settings

def test_every_setting_is_read_from_env_values():

    settings = load_storage_settings(make_storage_env())

    assert settings.endpoint == "http://localhost:8333"
    assert settings.bucket == "govai-knowledge"
    assert settings.region == "ap-south-1"
    assert settings.access_key.get_secret_value() == "test_access_key"
    assert settings.secret_key.get_secret_value() == TEST_SECRET_KEY


def test_settings_are_read_from_the_env_file(tmp_path, monkeypatch):

    # Without explicit values, the given .env file is read
    clear_storage_variables(monkeypatch)
    env_file = write_env_file(tmp_path / ".env", make_storage_env(OBJECT_STORAGE_BUCKET="from-file"))

    settings = load_storage_settings(env_file=env_file)

    assert settings.bucket == "from-file"


def test_environment_variables_override_the_env_file(tmp_path, monkeypatch):

    # A real environment variable wins over the file, e.g. in CI
    clear_storage_variables(monkeypatch)
    env_file = write_env_file(tmp_path / ".env", make_storage_env())
    monkeypatch.setenv("OBJECT_STORAGE_ENDPOINT", "http://seaweedfs:8333")

    settings = load_storage_settings(env_file=env_file)

    assert settings.endpoint == "http://seaweedfs:8333"


# Invalid settings

@pytest.mark.parametrize("name", REQUIRED_VARIABLES)
def test_each_missing_setting_is_reported_by_name(name):

    values = make_storage_env()
    values[name] = "   "

    with pytest.raises(StorageConfigError, match=name):
        load_storage_settings(values)


@pytest.mark.parametrize("bucket", ["Govai-Knowledge", "ab", "govai_knowledge", "-govai"])
def test_invalid_bucket_name_is_rejected(bucket):

    # S3 bucket names allow only lowercase letters, digits, and hyphens
    with pytest.raises(StorageConfigError, match="bucket"):
        load_storage_settings(make_storage_env(OBJECT_STORAGE_BUCKET=bucket))


def test_endpoint_must_be_a_url():

    with pytest.raises(StorageConfigError, match="endpoint"):
        load_storage_settings(make_storage_env(OBJECT_STORAGE_ENDPOINT="localhost:8333"))


def test_secret_key_is_never_shown():

    # Neither the error message nor the settings' text may contain the secret
    with pytest.raises(StorageConfigError) as error:
        load_storage_settings(make_storage_env(OBJECT_STORAGE_BUCKET="X"))

    settings = load_storage_settings(make_storage_env())

    assert TEST_SECRET_KEY not in str(error.value)
    assert TEST_SECRET_KEY not in repr(settings)
