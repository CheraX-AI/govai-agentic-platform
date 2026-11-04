"""Tests for app/tools/knowledge_rag_client.py with a fake HTTP transport; no service is called."""

import json

import httpx
import pytest

from app.agents.knowledge_agent.schemas import KnowledgeSource
from app.tools.knowledge_rag_client import (
    REQUIRED_VARIABLES,
    KnowledgeRagClient,
    KnowledgeServiceConfigError,
    KnowledgeServiceError,
    KnowledgeServiceRequestError,
    KnowledgeServiceUnavailableError,
    create_rag_client,
    load_rag_client_settings,
)
from helpers import write_env_file

BASE_URL = "http://knowledge.test"

RESULT = {
    "rank": 1,
    "chunk_id": "doc:0",
    "document_id": "doc",
    "text": "Synthetic chunk text.",
    "document_title": "Synthetic guide",
    "source_file": "synthetic.pdf",
    "page_number": 2,
    "department_id": "TEST",
    "service_id": "TEST_SERVICE",
    "object_key": "documents/TEST/TEST_SERVICE/doc.pdf",
    "distance": 0.2,
    "similarity": 0.8,
}


# Helper functions

def make_env(**overrides):

    # Valid values for every knowledge service variable, with some replaced
    env = {
        "KNOWLEDGE_SERVICE_URL": BASE_URL,
        "KNOWLEDGE_TOP_K": "3",
        "KNOWLEDGE_TIMEOUT_SECONDS": "10",
    }
    env.update(overrides)

    return env


def make_client(handler):
    """A client whose requests go to handler instead of the network; returns the client and the request log."""

    requests = []

    def record(request):
        requests.append(request)
        return handler(request)

    settings = load_rag_client_settings(make_env())
    http_client = httpx.Client(base_url=BASE_URL, transport=httpx.MockTransport(record))
    client = KnowledgeRagClient(settings, http_client=http_client)

    return client, requests


def reply(status_code, body):

    # A fake knowledge service response
    def handler(request):
        return httpx.Response(status_code, json=body)

    return handler


# Settings

def test_every_setting_is_read_from_env_values():

    settings = load_rag_client_settings(make_env())

    assert settings.base_url == BASE_URL
    assert settings.top_k == 3
    assert settings.timeout_seconds == 10


def test_settings_are_read_from_the_env_file(tmp_path, monkeypatch):

    # Without explicit values, the given .env file is read
    for name in REQUIRED_VARIABLES:
        monkeypatch.delenv(name, raising=False)
    env_file = write_env_file(tmp_path / ".env", make_env(KNOWLEDGE_TOP_K="7"))

    client = create_rag_client(env_file=env_file)

    assert client.settings.top_k == 7


@pytest.mark.parametrize("name", REQUIRED_VARIABLES)
def test_each_missing_setting_is_named(name):

    env = make_env()
    del env[name]

    with pytest.raises(KnowledgeServiceConfigError, match=f"{name} is required in .env"):
        load_rag_client_settings(env)


@pytest.mark.parametrize(
    "name, value",
    [
        ("KNOWLEDGE_SERVICE_URL", "knowledge.test"),
        ("KNOWLEDGE_SERVICE_URL", "ftp://knowledge.test"),
        ("KNOWLEDGE_TOP_K", "0"),
        ("KNOWLEDGE_TOP_K", "51"),
        ("KNOWLEDGE_TIMEOUT_SECONDS", "0"),
        ("KNOWLEDGE_TIMEOUT_SECONDS", "121"),
    ],
)
def test_invalid_values_are_rejected_by_name(name, value):

    with pytest.raises(KnowledgeServiceConfigError, match=name):
        load_rag_client_settings(make_env(**{name: value}))


# Retrieving

def test_results_become_knowledge_sources():

    client, _ = make_client(reply(200, {"results": [RESULT]}))

    sources = client.retrieve("Synthetic question?")

    assert sources == [KnowledgeSource.model_validate(RESULT)]
    assert sources[0].page_number == 2


def test_no_results_is_an_empty_list():

    client, _ = make_client(reply(200, {"results": []}))

    sources = client.retrieve("Synthetic question?")

    assert sources == []


def test_request_has_the_question_top_k_and_filters():

    client, requests = make_client(reply(200, {"results": []}))

    client.retrieve("Synthetic question?", department_id="TEST", service_id="TEST_SERVICE")

    request = requests[0]
    assert request.method == "POST"
    assert request.url.path == "/api/v1/retrieve"
    assert json.loads(request.content) == {
        "question": "Synthetic question?",
        "top_k": 3,
        "department_id": "TEST",
        "service_id": "TEST_SERVICE",
    }


def test_no_filters_searches_every_document():

    client, requests = make_client(reply(200, {"results": []}))

    client.retrieve("Synthetic question?")

    body = json.loads(requests[0].content)
    assert body["department_id"] is None
    assert body["service_id"] is None


def test_request_id_is_passed_on():

    client, requests = make_client(reply(200, {"results": []}))

    client.retrieve("Synthetic question?", request_id="abc123")

    assert requests[0].headers["X-Request-ID"] == "abc123"


# Errors

def test_rejected_request_is_a_request_error_with_the_reason():

    client, _ = make_client(reply(422, {"detail": "question: question is empty"}))

    with pytest.raises(KnowledgeServiceRequestError, match="question is empty"):
        client.retrieve("Synthetic question?")


def test_server_error_is_unavailable():

    client, _ = make_client(reply(503, {"detail": "ChromaDB is not reachable"}))

    with pytest.raises(KnowledgeServiceUnavailableError, match="ChromaDB is not reachable"):
        client.retrieve("Synthetic question?")


def test_error_without_detail_shows_the_status():

    client, _ = make_client(reply(500, {"error": "unexpected"}))

    with pytest.raises(KnowledgeServiceUnavailableError, match="HTTP 500"):
        client.retrieve("Synthetic question?")


def test_connection_failure_is_unavailable():

    def refuse(request):
        raise httpx.ConnectError("connection refused")

    client, _ = make_client(refuse)

    with pytest.raises(KnowledgeServiceUnavailableError, match="not reachable"):
        client.retrieve("Synthetic question?")


def test_timeout_is_unavailable():

    def time_out(request):
        raise httpx.ReadTimeout("timed out")

    client, _ = make_client(time_out)

    with pytest.raises(KnowledgeServiceUnavailableError, match="did not respond in time"):
        client.retrieve("Synthetic question?")


@pytest.mark.parametrize("body", [{"no_results": []}, {"results": [{"chunk_id": "doc:0"}]}, ["not", "a", "dict"]])
def test_invalid_response_is_an_error(body):

    client, _ = make_client(reply(200, body))

    with pytest.raises(KnowledgeServiceError, match="invalid response"):
        client.retrieve("Synthetic question?")
