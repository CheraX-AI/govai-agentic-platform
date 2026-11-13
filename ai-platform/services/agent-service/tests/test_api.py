"""Tests for app/api: the chat API that core-service calls, with a fake graph.

Only synthetic questions and answers are used.
"""

import logging

import pytest
from fastapi.testclient import TestClient
from langchain_core.messages import AIMessage

from app.agents.knowledge_agent.validation import LONG_QUESTION_REPLY, InvalidQuestionError
from app.agents.knowledge_agent.schemas import KnowledgeAnswer, SourceReference
from app.api.config import AgentAPIConfigError, AgentAPISettings, load_api_settings
from app.api.main import CHAT_PATH, INVALID_REQUEST_DETAIL, create_app
from app.graph.workflow import FAILURE_INVALID_QUESTION, FAILURE_UNAVAILABLE, build_graph
from app.llm.exceptions import LLMTimeoutError

TEST_TOKEN = "synthetic-service-token-0123456789abcdef"
AUTH = {"Authorization": f"Bearer {TEST_TOKEN}"}
QUESTION = "What do I hand in at the test passport counter?"


# Fakes

class FakeGraph:
    """Returns a fixed graph result, or raises, and records each run."""

    def __init__(self, result=None, error=None):

        self.result = result
        self.error = error
        self.calls = []

    def invoke(self, state, config=None):

        self.calls.append({"state": state, "config": config})
        if self.error is not None:
            raise self.error

        return self.result


class FakeKnowledgeAgent:
    """Answers like KnowledgeAgent.answer, or raises the given error."""

    def __init__(self, answer=None, error=None):

        self.answer_value = answer
        self.error = error

    def answer(self, question, request_id=None):

        if self.error is not None:
            raise self.error

        return self.answer_value


def answered(answer="Hand in form TF-1 [1].", sources=None):

    # A successful graph result with the given answer and cited sources
    if sources is None:
        sources = [{"number": 1, "document_title": "Synthetic guide", "source_file": "guide.pdf", "page_number": 2}]

    result = {
        "messages": [AIMessage(content=answer)],
        "knowledge_answer": {"answer": answer, "sources": sources, "scope": "govai_documents", "prompt_version": "v1"},
        "failure": None,
    }

    return result


def make_client(graph, auth_mode="bearer"):

    # Settings built directly, without reading .env
    token = TEST_TOKEN if auth_mode == "bearer" else None
    settings = AgentAPISettings(auth_mode=auth_mode, token=token)
    client = TestClient(create_app(graph=graph, settings=settings))

    return client


def post(client, body=None, headers=AUTH, **kwargs):

    with client:
        response = client.post(CHAT_PATH, json={"message": QUESTION} if body is None else body, headers=headers, **kwargs)

    return response


# Success

def test_answer_with_sources_matches_the_core_service_contract():

    response = post(make_client(FakeGraph(answered())))

    assert response.status_code == 200
    assert response.json() == {
        "answer": "Hand in form TF-1 [1].",
        "sources": [{"number": 1, "title": "Synthetic guide", "page": 2}],
    }


def test_only_the_question_reaches_the_graph():

    graph = FakeGraph(answered())

    post(make_client(graph), headers={**AUTH, "X-Request-ID": "abc123"})

    messages = graph.calls[0]["state"]["messages"]
    assert len(messages) == 1
    assert messages[0].content == QUESTION
    assert graph.calls[0]["config"]["metadata"]["request_id"] == "abc123"


def test_request_id_is_returned_and_unsafe_ids_are_replaced():

    response = post(make_client(FakeGraph(answered())), headers={**AUTH, "X-Request-ID": "bad id\x01"})

    assert response.headers["X-Request-ID"] != "bad id\x01"
    assert len(response.headers["X-Request-ID"]) == 32


def test_missing_page_is_left_out():

    sources = [{"number": 3, "document_title": "Synthetic guide", "source_file": None, "page_number": None}]

    response = post(make_client(FakeGraph(answered(sources=sources))))

    assert response.json()["sources"] == [{"number": 3, "title": "Synthetic guide"}]


def test_title_falls_back_to_file_name_then_number():

    sources = [
        {"number": 1, "document_title": None, "source_file": "guide.pdf", "page_number": None},
        {"number": 2, "document_title": "  ", "source_file": None, "page_number": None},
    ]

    response = post(make_client(FakeGraph(answered(sources=sources))))

    titles = [source["title"] for source in response.json()["sources"]]
    assert titles == ["guide.pdf", "Document 2"]


def test_long_multilingual_title_is_cut_to_512_bytes():

    # Sinhala letters take 3 bytes each in UTF-8
    sources = [{"number": 1, "document_title": "ශ්‍රී" * 300, "source_file": None, "page_number": None}]

    response = post(make_client(FakeGraph(answered(sources=sources))))

    title = response.json()["sources"][0]["title"]
    assert 0 < len(title.encode("utf-8")) <= 512


def test_no_documents_found_is_an_answer_without_sources():

    response = post(make_client(FakeGraph(answered(answer="I could not find this information.", sources=[]))))

    assert response.status_code == 200
    assert response.json() == {"answer": "I could not find this information.", "sources": []}


def test_invalid_question_returns_the_advice_as_the_answer():

    result = {"messages": [AIMessage(content=LONG_QUESTION_REPLY)], "knowledge_answer": None, "failure": FAILURE_INVALID_QUESTION}

    response = post(make_client(FakeGraph(result)))

    assert response.status_code == 200
    assert response.json() == {"answer": LONG_QUESTION_REPLY, "sources": []}


def test_real_graph_with_a_fake_agent():

    answer = KnowledgeAnswer(
        answer="Hand in form TF-1 [1].",
        sources=[SourceReference(number=1, document_title="Synthetic guide", page_number=1)],
        scope="govai_documents",
        prompt_version="v1",
    )

    response = post(make_client(build_graph(FakeKnowledgeAgent(answer=answer))))

    assert response.status_code == 200
    assert response.json()["sources"] == [{"number": 1, "title": "Synthetic guide", "page": 1}]


def test_real_graph_invalid_question_is_not_an_outage():

    graph = build_graph(FakeKnowledgeAgent(error=InvalidQuestionError(LONG_QUESTION_REPLY)))

    response = post(make_client(graph))

    assert response.status_code == 200
    assert response.json()["answer"] == LONG_QUESTION_REPLY


# Failures

def test_rag_or_llm_failure_is_503_not_an_answer():

    graph = build_graph(FakeKnowledgeAgent(error=LLMTimeoutError("NVIDIA did not respond")))

    response = post(make_client(graph))

    assert response.status_code == 503
    assert "NVIDIA" not in response.text


def test_unexpected_graph_error_is_503_without_details():

    response = post(make_client(FakeGraph(error=RuntimeError("secret internal detail"))))

    assert response.status_code == 503
    assert "secret" not in response.text


def test_empty_answer_is_503():

    response = post(make_client(FakeGraph(answered(answer=""))))

    assert response.status_code == 503


def test_unavailable_failure_flag_is_503():

    result = {"messages": [AIMessage(content="down")], "knowledge_answer": None, "failure": FAILURE_UNAVAILABLE}

    response = post(make_client(FakeGraph(result)))

    assert response.status_code == 503


# Authentication

@pytest.mark.parametrize(
    "headers",
    [
        {},
        {"Authorization": "Bearer wrong-token"},
        {"Authorization": f"Basic {TEST_TOKEN}"},
        {"Authorization": TEST_TOKEN},
        {"Authorization": f"Bearer {TEST_TOKEN}x"},
    ],
)
def test_wrong_or_missing_token_is_401_and_the_graph_is_not_run(headers):

    graph = FakeGraph(answered())

    response = post(make_client(graph), headers=headers)

    assert response.status_code == 401
    assert graph.calls == []


def test_token_is_checked_before_the_body():

    # An attacker without the token learns nothing about the body rules
    response = post(make_client(FakeGraph(answered())), body={"wrong": 1}, headers={})

    assert response.status_code == 401


def test_auth_mode_none_needs_no_token():

    response = post(make_client(FakeGraph(answered()), auth_mode="none"), headers={})

    assert response.status_code == 200


# Request validation

@pytest.mark.parametrize(
    "body",
    [
        {},
        {"message": ""},
        {"message": 123},
        {"message": "x" * 4001},
        {"message": QUESTION, "user_id": "someone"},
        [QUESTION],
    ],
)
def test_invalid_body_is_422_without_echoing_it(body):

    graph = FakeGraph(answered())

    response = post(make_client(graph), body=body)

    assert response.status_code == 422
    assert response.json() == {"detail": INVALID_REQUEST_DETAIL}
    assert graph.calls == []


def test_question_and_answer_are_not_logged(caplog):

    with caplog.at_level(logging.DEBUG):
        post(make_client(FakeGraph(answered(answer="Synthetic secret answer"))), body={"message": "Synthetic secret question"})
        post(make_client(FakeGraph(answered())), body={"message": "Synthetic secret question", "extra": 1})

    assert "Synthetic secret" not in caplog.text


def test_health_needs_no_token():

    with make_client(FakeGraph(answered())) as client:
        response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


# Settings

def test_bearer_settings_load():

    settings = load_api_settings(environ={"AGENT_API_AUTH_MODE": "bearer", "AGENT_API_TOKEN": TEST_TOKEN})

    assert settings.token.get_secret_value() == TEST_TOKEN


@pytest.mark.parametrize(
    "environ",
    [
        {},
        {"AGENT_API_AUTH_MODE": "basic", "AGENT_API_TOKEN": TEST_TOKEN},
        {"AGENT_API_AUTH_MODE": "bearer"},
        {"AGENT_API_AUTH_MODE": "bearer", "AGENT_API_TOKEN": "too-short"},
        {"AGENT_API_AUTH_MODE": "bearer", "AGENT_API_TOKEN": "has a space " + "x" * 32},
        {"AGENT_API_AUTH_MODE": "none", "AGENT_API_TOKEN": TEST_TOKEN},
    ],
)
def test_invalid_settings_are_rejected(environ):

    with pytest.raises(AgentAPIConfigError):
        load_api_settings(environ=environ)


def test_settings_errors_never_hold_the_token():

    with pytest.raises(AgentAPIConfigError) as error:
        load_api_settings(environ={"AGENT_API_AUTH_MODE": "none", "AGENT_API_TOKEN": TEST_TOKEN})

    assert TEST_TOKEN not in str(error.value)
