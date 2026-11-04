"""Tests for app/vector_store/config.py: reading and validating the ChromaDB settings."""

import pytest

from app.vector_store.config import REQUIRED_VARIABLES, load_vector_store_settings
from app.vector_store.exceptions import VectorStoreConfigError
from tests.helpers import write_env_file


# Helper functions

def make_chroma_env(**overrides):
    """A complete, valid set of ChromaDB .env values, with some replaced by overrides."""

    values = {
        "CHROMA_HOST": "localhost",
        "CHROMA_PORT": "8001",
        "CHROMA_COLLECTION": "govai_knowledge",
    }
    values.update(overrides)

    return values


def clear_chroma_variables(monkeypatch):

    # Remove real settings so a test never reads the developer's environment
    for name in REQUIRED_VARIABLES:
        monkeypatch.delenv(name, raising=False)


# Reading settings

def test_every_setting_is_read_from_env_values():

    settings = load_vector_store_settings(make_chroma_env())

    assert settings.host == "localhost"
    assert settings.port == 8001
    assert settings.collection == "govai_knowledge"


def test_settings_are_read_from_the_env_file(tmp_path, monkeypatch):

    clear_chroma_variables(monkeypatch)
    env_file = write_env_file(tmp_path / ".env", make_chroma_env(CHROMA_COLLECTION="from_file"))

    settings = load_vector_store_settings(env_file=env_file)

    assert settings.collection == "from_file"


def test_environment_variables_override_the_env_file(tmp_path, monkeypatch):

    # Inside Docker the host is the service name
    clear_chroma_variables(monkeypatch)
    env_file = write_env_file(tmp_path / ".env", make_chroma_env())
    monkeypatch.setenv("CHROMA_HOST", "chromadb")
    monkeypatch.setenv("CHROMA_PORT", "8000")

    settings = load_vector_store_settings(env_file=env_file)

    assert settings.host == "chromadb"
    assert settings.port == 8000


# Invalid settings

@pytest.mark.parametrize("name", REQUIRED_VARIABLES)
def test_each_missing_setting_is_reported_by_name(name):

    values = make_chroma_env()
    values[name] = ""

    with pytest.raises(VectorStoreConfigError, match=name):
        load_vector_store_settings(values)


@pytest.mark.parametrize("port", ["0", "70000", "abc"])
def test_invalid_port_is_rejected(port):

    with pytest.raises(VectorStoreConfigError, match="port"):
        load_vector_store_settings(make_chroma_env(CHROMA_PORT=port))


@pytest.mark.parametrize("collection", ["ab", "govai knowledge", "_govai", "govai/knowledge"])
def test_invalid_collection_name_is_rejected(collection):

    with pytest.raises(VectorStoreConfigError, match="collection"):
        load_vector_store_settings(make_chroma_env(CHROMA_COLLECTION=collection))
