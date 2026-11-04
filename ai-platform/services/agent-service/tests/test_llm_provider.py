"""Tests for app/llm/provider.py without calling NVIDIA.

Network access is blocked in these tests, so creating the provider is checked
to make no API call. invoke_llm is tested with LangChain's fake chat model.
"""

import json

import pytest
import requests
from langchain_core.language_models.fake_chat_models import FakeListChatModel
from langchain_core.load import dumpd
from langchain_core.messages import HumanMessage

from app.llm.exceptions import (
    LLMAuthError,
    LLMConfigError,
    LLMError,
    LLMRequestError,
    LLMTimeoutError,
    LLMUnavailableError,
)
from app.llm.provider import LLMProvider, create_llm_provider, invoke_llm, load_llm_provider, to_llm_error
from tests.helpers import TEST_MODEL, TEST_NVIDIA_KEY, make_env, make_settings, write_env_file

MESSAGES = [HumanMessage(content="Synthetic test question")]


# Fakes

class FailingChatModel(FakeListChatModel):
    """A chat model whose every call raises the given error."""

    error: Exception

    def invoke(self, messages, config=None, **kwargs):

        raise self.error


@pytest.fixture(autouse=True)
def block_network(monkeypatch):

    # Any HTTP request fails the test, so "no API call" is checked, not assumed
    def fail_request(*arguments, **keyword_arguments):
        raise AssertionError("a test tried to call the network")

    monkeypatch.setattr(requests.Session, "send", fail_request)


def make_provider(responses):

    # A provider around the fake chat model, with the test settings
    provider = LLMProvider(chat_model=FakeListChatModel(responses=responses), settings=make_settings())

    return provider


# Creating the provider

def test_provider_uses_every_setting():

    # The NVIDIA client gets the model and generation settings from .env
    provider = create_llm_provider(make_settings())
    chat_model = provider.chat_model

    assert chat_model.model == TEST_MODEL
    assert chat_model.temperature == 0.1
    assert chat_model.max_tokens == 512
    assert chat_model._client.timeout == 15
    assert chat_model._client.api_key.get_secret_value() == TEST_NVIDIA_KEY


def test_provider_keeps_its_settings():

    settings = make_settings()

    provider = create_llm_provider(settings)

    assert provider.settings == settings


def test_usage_telemetry_is_turned_off():

    # No usage statistics are sent to NVIDIA
    provider = create_llm_provider(make_settings())

    assert provider.chat_model.usage_telemetry_enabled is False


def test_provider_is_loaded_from_the_env_file(tmp_path, monkeypatch):

    # load_llm_provider reads the settings file, then builds the client
    for name in make_env():
        monkeypatch.delenv(name, raising=False)
    env_file = write_env_file(tmp_path / ".env", make_env(LLM_MAX_TOKENS="256"))

    provider = load_llm_provider(env_file=env_file)

    assert provider.chat_model.max_tokens == 256


def test_missing_settings_stop_before_the_client_is_built(tmp_path, monkeypatch):

    for name in make_env():
        monkeypatch.delenv(name, raising=False)

    with pytest.raises(LLMConfigError, match="NVIDIA_API_KEY is required"):
        load_llm_provider(env_file=tmp_path / "missing.env")


def test_unknown_model_lookup_failure_is_an_llm_error(monkeypatch):

    # A model outside the library's table is looked up at NVIDIA; here the network is down
    def no_network(*arguments, **keyword_arguments):
        raise requests.exceptions.ConnectionError("connection refused")

    monkeypatch.setattr(requests.Session, "send", no_network)

    with pytest.raises(LLMUnavailableError, match="NVIDIA is not reachable"):
        create_llm_provider(make_settings(LLM_MODEL="nvidia/unknown-test-model"))


def test_api_key_is_never_in_trace_data():

    # LangSmith records the serialized model and its parameters; neither holds the key
    chat_model = create_llm_provider(make_settings()).chat_model

    assert TEST_NVIDIA_KEY not in json.dumps(dumpd(chat_model))
    assert TEST_NVIDIA_KEY not in str(chat_model._get_invocation_params())
    assert TEST_NVIDIA_KEY not in str(chat_model._get_ls_params())
    assert TEST_NVIDIA_KEY not in repr(chat_model)


# Calling the model

def test_invoke_returns_the_answer_text():

    provider = make_provider(["  The answer [1].  "])

    answer = invoke_llm(provider, MESSAGES)

    assert answer == "The answer [1]."


def test_invoke_accepts_a_trace_config():

    # The LangSmith run name, tags, and metadata are passed through
    provider = make_provider(["The answer."])
    config = {"run_name": "test_run", "tags": ["govai"], "metadata": {"prompt_version": "v1"}}

    answer = invoke_llm(provider, MESSAGES, config=config)

    assert answer == "The answer."


def test_empty_answer_is_an_error():

    provider = make_provider(["   "])

    with pytest.raises(LLMError, match="empty answer"):
        invoke_llm(provider, MESSAGES)


def test_invoke_turns_provider_errors_into_llm_errors():

    # A rejected key during the call becomes LLMAuthError
    chat_model = FailingChatModel(responses=["unused"], error=Exception("[401] Unauthorized\nPlease check your key."))
    provider = LLMProvider(chat_model=chat_model, settings=make_settings())

    with pytest.raises(LLMAuthError, match="NVIDIA rejected the API key"):
        invoke_llm(provider, MESSAGES)


# Classifying errors

@pytest.mark.parametrize(
    "error, error_class",
    [
        (Exception("[401] Unauthorized"), LLMAuthError),
        (Exception("[403] Forbidden"), LLMAuthError),
        (Exception("[408] Request Timeout"), LLMTimeoutError),
        (Exception("[504] Gateway Timeout"), LLMTimeoutError),
        (Exception("[429] Too Many Requests"), LLMUnavailableError),
        (Exception("[500] Internal Server Error"), LLMUnavailableError),
        (Exception("[503] Service Unavailable"), LLMUnavailableError),
        (Exception("[400] Bad Request"), LLMRequestError),
        (Exception("[404] Not Found"), LLMRequestError),
        (requests.exceptions.ReadTimeout("read timed out"), LLMTimeoutError),
        (TimeoutError("polling timed out"), LLMTimeoutError),
        (requests.exceptions.ConnectionError("refused"), LLMUnavailableError),
        (ConnectionError("reset"), LLMUnavailableError),
    ],
)
def test_errors_are_classified(error, error_class):

    llm_error = to_llm_error(error)

    assert type(llm_error) is error_class


def test_unknown_errors_become_the_base_error():

    llm_error = to_llm_error(ValueError("something odd"))

    assert type(llm_error) is LLMError
    assert "something odd" in str(llm_error)


def test_llm_errors_are_returned_unchanged():

    error = LLMTimeoutError("already classified")

    assert to_llm_error(error) is error


def test_error_message_keeps_only_the_first_line():

    # Lines after the first can repeat the request, which holds the citizen's question
    error = Exception("[400] Bad Request\nInput: my NIC is 200012345678")

    llm_error = to_llm_error(error)

    assert "Bad Request" in str(llm_error)
    assert "200012345678" not in str(llm_error)
