"""Tests for app/llm/provider.py without calling Azure OpenAI."""

import json

import httpx2
import openai
import pytest
from langchain_core.language_models.fake_chat_models import FakeListChatModel
from langchain_core.load import dumpd
from langchain_core.messages import AIMessage, HumanMessage
from langchain_core.outputs import ChatGeneration, ChatResult

from app.llm.exceptions import (
    LLMAuthError,
    LLMConfigError,
    LLMError,
    LLMRequestError,
    LLMTimeoutError,
    LLMUnavailableError,
)
from app.llm.provider import MAX_RETRIES, LLMProvider, create_llm_provider, invoke_llm, load_llm_provider, to_llm_error
from helpers import TEST_API_KEY, TEST_ENDPOINT, TEST_MODEL, make_env, make_settings, write_env_file

MESSAGES = [HumanMessage(content="Synthetic test question")]

REQUEST = httpx2.Request("POST", f"{TEST_ENDPOINT}/chat/completions")


# Fakes

class FailingChatModel(FakeListChatModel):
    """A chat model whose every call raises the given error."""

    error: Exception

    def invoke(self, messages, config=None, **kwargs):

        raise self.error


class FilteredChatModel(FakeListChatModel):
    """A chat model whose answer Azure's content filter stopped part way."""

    def _generate(self, messages, stop=None, run_manager=None, **kwargs):

        message = AIMessage(content="Partial answer", response_metadata={"finish_reason": "content_filter"})

        return ChatResult(generations=[ChatGeneration(message=message)])


@pytest.fixture(autouse=True)
def block_network(monkeypatch):

    # Any HTTP request fails the test, so "no API call" is checked, not assumed
    def fail_request(*arguments, **keyword_arguments):
        raise AssertionError("a test tried to call the network")

    monkeypatch.setattr(httpx2.Client, "send", fail_request)


def make_provider(responses):

    # A provider around the fake chat model, with the test settings
    provider = LLMProvider(chat_model=FakeListChatModel(responses=responses), settings=make_settings())

    return provider


def status_error(status_code, code=None):

    # An openai HTTP error as the SDK raises it, with Azure's error code in the body
    response = httpx2.Response(status_code, request=REQUEST)
    body = {"code": code, "message": "Synthetic error"} if code else None
    error = openai.APIStatusError(f"Error code: {status_code}", response=response, body=body)

    return error


# Creating the provider

def test_provider_uses_every_setting():

    # The client gets the endpoint, deployment, and generation settings from .env
    provider = create_llm_provider(make_settings())
    chat_model = provider.chat_model

    assert chat_model.model_name == TEST_MODEL
    assert chat_model.openai_api_base == TEST_ENDPOINT
    assert chat_model.openai_api_key.get_secret_value() == TEST_API_KEY
    assert chat_model.temperature == 0.1
    assert chat_model.max_tokens == 512
    assert chat_model.root_client.timeout == 15
    assert chat_model.root_client.max_retries == MAX_RETRIES


def test_provider_uses_chat_completions():

    provider = create_llm_provider(make_settings())

    assert provider.chat_model.use_responses_api is False


def test_provider_keeps_its_settings():

    settings = make_settings()

    provider = create_llm_provider(settings)

    assert provider.settings == settings


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

    with pytest.raises(LLMConfigError, match="AZURE_OPENAI_ENDPOINT is required"):
        load_llm_provider(env_file=tmp_path / "missing.env")


def test_api_key_is_never_in_trace_data():

    # LangSmith records the serialized model and its parameters; neither holds the key
    chat_model = create_llm_provider(make_settings()).chat_model

    assert TEST_API_KEY not in json.dumps(dumpd(chat_model))
    assert TEST_API_KEY not in str(chat_model._get_invocation_params())
    assert TEST_API_KEY not in str(chat_model._get_ls_params())
    assert TEST_API_KEY not in repr(chat_model)


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


def test_filtered_answer_is_not_returned():

    # A partial answer cut by the content filter is never shown to the citizen
    provider = LLMProvider(chat_model=FilteredChatModel(responses=["unused"]), settings=make_settings())

    with pytest.raises(LLMRequestError, match="content filter blocked the answer"):
        invoke_llm(provider, MESSAGES)


def test_invoke_turns_provider_errors_into_llm_errors():

    # A rejected key during the call becomes LLMAuthError
    chat_model = FailingChatModel(responses=["unused"], error=status_error(401))
    provider = LLMProvider(chat_model=chat_model, settings=make_settings())

    with pytest.raises(LLMAuthError, match="Azure OpenAI rejected the API key"):
        invoke_llm(provider, MESSAGES)


def test_real_client_errors_become_llm_errors(monkeypatch):

    # The real client with the network down: the openai error is classified
    def no_network(*arguments, **keyword_arguments):
        raise httpx2.ConnectError("connection refused")

    monkeypatch.setattr(httpx2.Client, "send", no_network)
    provider = create_llm_provider(make_settings())

    with pytest.raises(LLMUnavailableError, match="Azure OpenAI is not reachable"):
        invoke_llm(provider, MESSAGES)


# Classifying errors

@pytest.mark.parametrize(
    "error, error_class",
    [
        (status_error(401), LLMAuthError),
        (status_error(403), LLMAuthError),
        (status_error(408), LLMTimeoutError),
        (status_error(504), LLMTimeoutError),
        (status_error(429), LLMUnavailableError),
        (status_error(500), LLMUnavailableError),
        (status_error(503), LLMUnavailableError),
        (status_error(400), LLMRequestError),
        (status_error(404, "DeploymentNotFound"), LLMRequestError),
        (openai.APITimeoutError(request=REQUEST), LLMTimeoutError),
        (openai.APIConnectionError(request=REQUEST), LLMUnavailableError),
        (TimeoutError("timed out"), LLMTimeoutError),
        (ConnectionError("reset"), LLMUnavailableError),
    ],
)
def test_errors_are_classified(error, error_class):

    llm_error = to_llm_error(error)

    assert type(llm_error) is error_class


def test_content_filter_is_named():

    llm_error = to_llm_error(status_error(400, "content_filter"))

    assert type(llm_error) is LLMRequestError
    assert "content filter blocked the request" in str(llm_error)


def test_unknown_deployment_points_to_the_setting():

    llm_error = to_llm_error(status_error(404, "DeploymentNotFound"))

    assert "LLM_MODEL" in str(llm_error)
    assert "DeploymentNotFound" in str(llm_error)


def test_unknown_errors_become_the_base_error():

    llm_error = to_llm_error(ValueError("something odd"))

    assert type(llm_error) is LLMError
    assert "something odd" in str(llm_error)


def test_llm_errors_are_returned_unchanged():

    error = LLMTimeoutError("already classified")

    assert to_llm_error(error) is error


def test_http_error_body_is_never_in_the_message():

    # The error body can repeat the request, which holds the citizen's question
    response = httpx2.Response(400, request=REQUEST)
    body = {"code": "invalid_request", "message": "Input: my NIC is 200012345678"}
    error = openai.BadRequestError("Error code: 400 - Input: my NIC is 200012345678", response=response, body=body)

    llm_error = to_llm_error(error)

    assert "invalid_request" in str(llm_error)
    assert "200012345678" not in str(llm_error)


def test_other_error_messages_keep_only_the_first_line():

    error = Exception("Something failed\nInput: my NIC is 200012345678")

    llm_error = to_llm_error(error)

    assert "Something failed" in str(llm_error)
    assert "200012345678" not in str(llm_error)
