"""Tests for app/api: the chat API that core-service calls, with a fake graph.

Only synthetic questions and answers are used.
"""

import logging

import pytest
from fastapi.testclient import TestClient
from langchain_core.messages import AIMessage
from langgraph.checkpoint.memory import InMemorySaver

from app.agents.knowledge_agent.validation import LONG_QUESTION_REPLY, InvalidQuestionError
from app.agents.knowledge_agent.schemas import KnowledgeAnswer
from app.api.config import AgentAPIConfigError, AgentAPISettings, load_api_settings
from app.api.main import CHAT_PATH, INVALID_REQUEST_DETAIL, create_app
from app.graph.constants import FAILURE_INVALID_QUESTION, FAILURE_UNAVAILABLE
from app.graph.workflow import build_graph
from app.llm.exceptions import LLMTimeoutError
from app.memory.encryption import MemoryDataError

TEST_TOKEN = "synthetic-service-token-0123456789abcdef"
AUTH = {"Authorization": f"Bearer {TEST_TOKEN}"}
QUESTION = "What do I hand in at the test passport counter?"
CONVERSATION_ID = "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"


# Fakes

class FakeGraph:
    """Returns a fixed graph result, or raises, and records each run."""

    def __init__(self, result=None, error=None):

        self.result = result
        self.error = error
        self.calls = []

    def invoke(self, state, config=None, **kwargs):

        self.calls.append({"state": state, "config": config, **kwargs})
        if self.error is not None:
            raise self.error

        return self.result


class FakeKnowledgeAgent:
    """Answers like KnowledgeAgent.answer, or raises the given error."""

    def __init__(self, answer=None, error=None):

        self.answer_value = answer
        self.error = error

    def answer(self, question, request_id=None, history=()):

        if self.error is not None:
            raise self.error

        return self.answer_value


def answered(answer="Hand in form TF-1."):

    # A successful graph result with the given answer
    result = {
        "messages": [AIMessage(content=answer)],
        "reply": answer,
        "knowledge_answer": {"answer": answer, "scope": "govai_documents", "prompt_version": "v2"},
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

def test_answer_matches_the_core_service_contract():

    response = post(make_client(FakeGraph(answered())))

    # Only the answer: the citizen is not shown which documents it came from
    assert response.status_code == 200
    assert response.json() == {"answer": "Hand in form TF-1."}


def test_only_the_question_reaches_the_graph():

    graph = FakeGraph(answered())

    post(make_client(graph), headers={**AUTH, "X-Request-ID": "abc123"})

    messages = graph.calls[0]["state"]["messages"]
    assert len(messages) == 1
    assert messages[0].content == QUESTION
    assert graph.calls[0]["config"]["metadata"]["request_id"] == "abc123"


# Conversation ID

def test_conversation_id_is_returned_unchanged():

    response = post(make_client(FakeGraph(answered())), body={"message": QUESTION, "conversation_id": CONVERSATION_ID})

    assert response.status_code == 200
    assert response.json() == {"answer": "Hand in form TF-1.", "conversation_id": CONVERSATION_ID}


def test_conversation_id_reaches_the_graph_only_as_metadata():

    graph = FakeGraph(answered())

    post(make_client(graph), body={"message": QUESTION, "conversation_id": CONVERSATION_ID})

    # Without memory the graph gets only the question
    call = graph.calls[0]
    assert call["state"] == {"messages": call["state"]["messages"]}
    assert len(call["state"]["messages"]) == 1
    assert call["config"]["metadata"]["conversation_id"] == CONVERSATION_ID


def test_request_without_conversation_id_still_works():

    graph = FakeGraph(answered())

    response = post(make_client(graph))

    # A legacy request: no conversation ID is invented, and none is returned
    assert response.status_code == 200
    assert response.json() == {"answer": "Hand in form TF-1."}
    assert "conversation_id" not in graph.calls[0]["config"]["metadata"]


def test_invalid_question_keeps_the_conversation_id():

    result = {"messages": [AIMessage(content=LONG_QUESTION_REPLY)], "reply": LONG_QUESTION_REPLY, "knowledge_answer": None, "failure": FAILURE_INVALID_QUESTION}

    response = post(make_client(FakeGraph(result)), body={"message": QUESTION, "conversation_id": CONVERSATION_ID})

    assert response.json() == {"answer": LONG_QUESTION_REPLY, "conversation_id": CONVERSATION_ID}


@pytest.mark.parametrize(
    "conversation_id",
    [
        "",
        CONVERSATION_ID.upper(),
        CONVERSATION_ID.replace("-", ""),
        "{" + CONVERSATION_ID + "}",
        " " + CONVERSATION_ID,
        "00000000-0000-0000-0000-000000000000",
        "3f2b8c1e-9a4d-0f6b-8c2e-1d7a5b9e0f34",
        "3f2b8c1e-9a4d-4f6b-0c2e-1d7a5b9e0f34",
        123,
    ],
)
def test_malformed_conversation_id_is_422(conversation_id):

    graph = FakeGraph(answered())

    response = post(make_client(graph), body={"message": QUESTION, "conversation_id": conversation_id})

    assert response.status_code == 422
    assert response.json() == {"detail": INVALID_REQUEST_DETAIL}
    assert graph.calls == []


def test_request_id_is_returned_and_unsafe_ids_are_replaced():

    response = post(make_client(FakeGraph(answered())), headers={**AUTH, "X-Request-ID": "bad id\x01"})

    assert response.headers["X-Request-ID"] != "bad id\x01"
    assert len(response.headers["X-Request-ID"]) == 32


def test_no_documents_found_is_an_answer():

    response = post(make_client(FakeGraph(answered(answer="I could not find this information."))))

    assert response.status_code == 200
    assert response.json() == {"answer": "I could not find this information."}


def test_invalid_question_returns_the_advice_as_the_answer():

    result = {"messages": [AIMessage(content=LONG_QUESTION_REPLY)], "reply": LONG_QUESTION_REPLY, "knowledge_answer": None, "failure": FAILURE_INVALID_QUESTION}

    response = post(make_client(FakeGraph(result)))

    assert response.status_code == 200
    assert response.json() == {"answer": LONG_QUESTION_REPLY}


def test_real_graph_with_a_fake_agent():

    answer = KnowledgeAnswer(answer="Hand in form TF-1.", scope="govai_documents", prompt_version="v2")

    response = post(make_client(build_graph(FakeKnowledgeAgent(answer=answer))))

    assert response.status_code == 200
    assert response.json() == {"answer": "Hand in form TF-1."}


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

    result = {"messages": [AIMessage(content="down")], "reply": "down", "knowledge_answer": None, "failure": FAILURE_UNAVAILABLE}

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


# Memory

class FakeMemory:
    """Records forget, compact and close, like ConversationMemory."""

    def __init__(self, compact_error=None):

        self.compact_error = compact_error
        self.forgotten = []
        self.compacted = []
        self.closed = False

    def forget(self, conversation_id):

        self.forgotten.append(conversation_id)

    def compact(self, conversation_id):

        self.compacted.append(conversation_id)
        if self.compact_error is not None:
            raise self.compact_error

    def close(self):

        self.closed = True


class UnreadableOnceGraph(FakeGraph):
    """Fails the first run as if its memory were saved with another key."""

    def invoke(self, state, config=None, **kwargs):

        if not self.calls:
            self.calls.append({"state": state, "config": config, **kwargs})
            raise MemoryDataError("Memory cannot be decrypted")

        return super().invoke(state, config=config, **kwargs)


class RecordingKnowledgeAgent:
    """Answers every question and records the history it was given."""

    def __init__(self):

        self.histories = []

    def answer(self, question, request_id=None, history=()):

        self.histories.append([message.content for message in history])
        answer = KnowledgeAnswer(answer=f"Answer to: {question}", scope="govai_documents", prompt_version="v2")

        return answer


def make_memory_client(memory_graph, memory, graph=None):

    # Both graphs and the memory passed in, without reading .env
    settings = AgentAPISettings(auth_mode="bearer", token=TEST_TOKEN)
    app = create_app(
        graph=graph if graph is not None else FakeGraph(answered("Stateless answer.")),
        settings=settings,
        memory_graph=memory_graph,
        memory=memory,
    )
    client = TestClient(app)

    return client


def ask_in(client, question=QUESTION, conversation_id=CONVERSATION_ID):

    response = client.post(CHAT_PATH, json={"message": question, "conversation_id": conversation_id}, headers=AUTH)

    return response


def test_conversation_uses_the_memory_graph_with_its_own_thread():

    memory_graph, memory = FakeGraph(answered()), FakeMemory()

    with make_memory_client(memory_graph, memory) as client:
        response = ask_in(client)

    call = memory_graph.calls[0]
    assert response.json() == {"answer": "Hand in form TF-1.", "conversation_id": CONVERSATION_ID}
    assert call["config"]["configurable"] == {"thread_id": CONVERSATION_ID}
    assert call["durability"] == "exit"
    assert memory.compacted == [CONVERSATION_ID]


def test_request_without_conversation_id_is_not_remembered():

    graph, memory_graph, memory = FakeGraph(answered("Stateless answer.")), FakeGraph(answered()), FakeMemory()

    response = post(make_memory_client(memory_graph, memory, graph=graph))

    assert response.json() == {"answer": "Stateless answer."}
    assert memory_graph.calls == []
    assert "configurable" not in graph.calls[0]["config"]


def test_unreadable_memory_is_forgotten_and_the_question_answered():

    memory_graph, memory = UnreadableOnceGraph(answered()), FakeMemory()

    with make_memory_client(memory_graph, memory) as client:
        response = ask_in(client)

    assert response.status_code == 200
    assert memory.forgotten == [CONVERSATION_ID]
    assert len(memory_graph.calls) == 2


def test_compact_failure_still_answers():

    memory = FakeMemory(compact_error=RuntimeError("synthetic database error"))

    with make_memory_client(FakeGraph(answered()), memory) as client:
        response = ask_in(client)

    assert response.status_code == 200


def test_memory_is_closed_when_the_api_stops():

    memory = FakeMemory()

    with make_memory_client(FakeGraph(answered()), memory) as client:
        client.get("/health")

    assert memory.closed


def test_real_memory_graph_remembers_the_conversation():

    agent = RecordingKnowledgeAgent()
    memory_graph = build_graph(agent, InMemorySaver())

    with make_memory_client(memory_graph, FakeMemory()) as client:
        ask_in(client, "First question?")
        second = ask_in(client, "What about it?")
        ask_in(client, "Another conversation?", conversation_id="9a8b7c6d-5e4f-4a3b-9c2d-1e0f9a8b7c6d")

    assert second.json()["answer"] == "Answer to: What about it?"
    assert agent.histories == [[], ["First question?", "Answer to: First question?"], []]


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
