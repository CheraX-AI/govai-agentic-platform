"""Shared test helpers: LLM settings builders and synthetic documents.

Tests send only the synthetic text built here, never real government
documents or citizen data.
"""

import pytest

from app.llm.config import LLMSettings, load_llm_settings
from app.llm.exceptions import LLMConfigError

# Has the NVIDIA key format, so settings accept it, but it is not a real key: NVIDIA rejects it
TEST_NVIDIA_KEY = "nvapi-test-key-not-real-1234"

# A model in the NVIDIA library's model table, so creating the client makes no API call
TEST_MODEL = "meta/llama-3.3-70b-instruct"

# A clearly fictional document, so no real passport rule is ever stated by a test
SYNTHETIC_SOURCE = {
    "chunk_id": "synthetic-doc:0",
    "text": (
        "SYNTHETIC TEST DOCUMENT - not real government information. "
        "At the test passport counter, applicants hand in the completed test form TF-1."
    ),
    "document_title": "Synthetic passport test guide",
    "source_file": "synthetic_passport_test_guide.pdf",
    "page_number": 1,
    "department_id": "TEST",
    "service_id": "TEST_PASSPORT",
    "similarity": 0.9,
}

SYNTHETIC_QUESTION = "What do I hand in at the test passport counter?"


# Settings builders

def make_env(**overrides):
    """A complete set of LLM .env values, with some replaced."""

    # Valid values for every required variable
    env = {
        "NVIDIA_API_KEY": TEST_NVIDIA_KEY,
        "LLM_MODEL": TEST_MODEL,
        "LLM_TEMPERATURE": "0.1",
        "LLM_MAX_TOKENS": "512",
        "LLM_TIMEOUT_SECONDS": "15",
    }

    # Replace the values the test is about
    env.update(overrides)

    return env


def make_settings(**overrides):

    # Settings built from the test .env values, without reading a file
    settings = LLMSettings.model_validate(make_env(**overrides))

    return settings


def write_env_file(path, values):

    # Write the values as KEY=value lines
    lines = []
    for name, value in values.items():
        lines.append(f"{name}={value}")

    path.write_text("\n".join(lines) + "\n", encoding="utf-8")

    return path


# Real NVIDIA

def real_settings():
    """The settings from agent-service/.env. Skips the test when they are missing."""

    # Load the real settings; without them the test cannot reach NVIDIA
    try:
        settings = load_llm_settings()
    except LLMConfigError as error:
        pytest.skip(f"real NVIDIA LLM settings are not available in agent-service/.env ({error})")

    return settings
