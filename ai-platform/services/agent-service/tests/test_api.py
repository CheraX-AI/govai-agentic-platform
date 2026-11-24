"""Tests for app/api: the chat API that core-service calls, with fake graphs and a fake conversation store.

Only synthetic questions and answers are used.
"""

import logging

import pytest
from fastapi.testclient import TestClient
from langchain_core.messages import AIMessage

from app.agents.knowledge_agent.validation import LONG_QUESTION_REPLY, InvalidQuestionError
from app.agents.knowledge_agent.schemas import KnowledgeAnswer
from app.api.config import AgentAPIConfigError, AgentAPISettings, load_api_settings
from app.api.conversations import CONVERSATIONS_PATH
from app.api.main import CHAT_PATH, INVALID_REQUEST_DETAIL, create_app
from app.conversations.service import ConversationService
from app.graph.constants import FAILURE_INVALID_QUESTION, FAILURE_UNAVAILABLE
from app.graph.workflow import build_graph
from app.llm.exceptions import LLMTimeoutError
from fake_checkpointer import FakeCheckpointer, make_serializer
from fake_conversations import FakeConversationStore

TEST_TOKEN = "synthetic-service-token-0123456789abcdef"
CITIZEN = "11111111-2222-4333-8444-555555555555"
OTHER_CITIZEN = "66666666-7777-4888-9999-000000000000"
AUTH = {"Authorization": f"Bearer {TEST_TOKEN}", "X-GovAI-Citizen": CITIZEN}
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
        "reply": AIMessage(content=answer),
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


# Memory off

def test_without_memory_no_conversation_id_is_returned():

    graph = FakeGraph(answered())

    response = post(make_client(graph), body={"message": QUESTION, "conversation_id": CONVERSATION_ID})

    # Nothing is stored, so there is no conversation to name
    assert response.status_code == 200
    assert response.json() == {"answer": "Hand in form TF-1."}
    assert graph.calls[0]["state"] == {"messages": graph.calls[0]["state"]["messages"]}
    assert len(graph.calls[0]["state"]["messages"]) == 1
    assert "configurable" not in graph.calls[0]["config"]


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

    result = {"messages": [AIMessage(content=LONG_QUESTION_REPLY)], "reply": AIMessage(content=LONG_QUESTION_REPLY), "knowledge_answer": None, "failure": FAILURE_INVALID_QUESTION}

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

    graph = build_graph(FakeKnowledgeAgent(error=LLMTimeoutError("Azure OpenAI did not respond")))

    response = post(make_client(graph))

    assert response.status_code == 503
    assert "Azure" not in response.text


def test_unexpected_graph_error_is_503_without_details():

    response = post(make_client(FakeGraph(error=RuntimeError("secret internal detail"))))

    assert response.status_code == 503
    assert "secret" not in response.text


def test_empty_answer_is_503():

    response = post(make_client(FakeGraph(answered(answer=""))))

    assert response.status_code == 503


def test_unavailable_failure_flag_is_503():

    result = {"messages": [AIMessage(content="down")], "reply": AIMessage(content="down"), "knowledge_answer": None, "failure": FAILURE_UNAVAILABLE}

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


@pytest.mark.parametrize(
    "citizen",
    [None, "", "has space", "x" * 256],
)
def test_missing_or_invalid_citizen_is_401_and_the_graph_is_not_run(citizen):

    graph = FakeGraph(answered())
    headers = {"Authorization": f"Bearer {TEST_TOKEN}"}
    if citizen is not None:
        headers["X-GovAI-Citizen"] = citizen

    response = post(make_client(graph), headers=headers)

    assert response.status_code == 401
    assert graph.calls == []


def test_citizen_is_never_trusted_without_the_service_token():

    graph = FakeGraph(answered())

    response = post(make_client(graph), headers={"X-GovAI-Citizen": CITIZEN})

    assert response.status_code == 401
    assert graph.calls == []


def test_auth_mode_none_needs_no_token_but_still_a_citizen():

    client = make_client(FakeGraph(answered()), auth_mode="none")

    assert post(client, headers={"X-GovAI-Citizen": CITIZEN}).status_code == 200
    assert post(client, headers={}).status_code == 401


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

class RecordingKnowledgeAgent:
    """Answers every question (or raises the queued errors in turn) and records the history it was given."""

    def __init__(self, errors=()):

        self.errors = list(errors)
        self.histories = []

    def answer(self, question, request_id=None, history=()):

        self.histories.append([message.content for message in history])
        error = self.errors.pop(0) if self.errors else None
        if error is not None:
            raise error

        answer = KnowledgeAnswer(answer=f"Answer to: {question}", scope="govai_documents", prompt_version="v2")

        return answer


class MemorySetup:
    """The API with the real graph and conversation service, the fake store and the in-memory checkpointer.

    Pass the store and checkpointer of another setup to act as a second worker or a restarted API.
    """

    def __init__(self, agent=None, max_turns=6, store=None, checkpointer=None):

        self.agent = agent if agent is not None else RecordingKnowledgeAgent()
        self.store = store if store is not None else FakeConversationStore()
        self.checkpointer = checkpointer if checkpointer is not None else FakeCheckpointer()

        memory_graph = build_graph(self.agent, self.checkpointer, max_turns)
        self.conversations = ConversationService(self.store, self.checkpointer, max_turns, 90, 600)
        settings = AgentAPISettings(auth_mode="bearer", token=TEST_TOKEN)
        self.app = create_app(
            graph=build_graph(self.agent),
            settings=settings,
            memory_graph=memory_graph,
            conversations=self.conversations,
        )
        self.client = TestClient(self.app)

    def listed(self, citizen=CITIZEN):

        # The conversation IDs the citizen's history list shows
        response = self.client.get(CONVERSATIONS_PATH, headers={**AUTH, "X-GovAI-Citizen": citizen})
        ids = []
        for conversation in response.json()["conversations"]:
            ids.append(conversation["id"])

        return ids

    def ask(self, question=QUESTION, conversation_id=None, citizen=CITIZEN):

        body = {"message": question}
        if conversation_id is not None:
            body["conversation_id"] = conversation_id

        response = self.client.post(CHAT_PATH, json=body, headers={**AUTH, "X-GovAI-Citizen": citizen})

        return response


@pytest.fixture
def setup():

    created = MemorySetup()
    with created.client:
        yield created


def test_first_message_creates_a_conversation_and_returns_its_id(setup):

    response = setup.ask("First question?")

    conversation_id = response.json()["conversation_id"]
    assert response.status_code == 200
    assert response.json()["answer"] == "Answer to: First question?"
    assert setup.store.owns(CITIZEN, conversation_id)
    assert setup.store.statuses(conversation_id) == ["completed"]


def test_greeting_gets_a_friendly_reply_and_a_real_conversation(setup):

    response = setup.ask("Hi")

    conversation_id = response.json()["conversation_id"]
    assert response.status_code == 200
    assert response.json()["answer"].startswith("Hello! 👋 Welcome to GovAI.")
    assert setup.agent.histories == []
    assert setup.store.statuses(conversation_id) == ["completed"]
    assert setup.listed() == [conversation_id]


def test_thanks_after_an_answer_is_answered_politely(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]

    response = setup.ask("Thank you!", conversation_id)

    assert response.json()["answer"].startswith("You're welcome!")
    assert len(setup.agent.histories) == 1


def test_failed_first_question_leaves_no_ghost_conversation():

    failing = MemorySetup(agent=RecordingKnowledgeAgent(errors=[LLMTimeoutError("down")]))
    with failing.client:
        response = failing.ask("First question?")
        listed = failing.listed()

    assert response.status_code == 503
    assert listed == []


def test_invalid_first_question_is_hidden_until_one_is_answered():

    invalid = MemorySetup(agent=RecordingKnowledgeAgent(errors=[InvalidQuestionError(LONG_QUESTION_REPLY)]))
    with invalid.client:
        conversation_id = invalid.ask("Too long?").json()["conversation_id"]
        hidden = invalid.listed()
        invalid.ask("A shorter question?", conversation_id)
        shown = invalid.listed()

    assert hidden == []
    assert shown == [conversation_id]


def test_follow_up_sees_the_earlier_turn_from_the_checkpoint(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]
    setup.store.errors["recent_turns"] = AssertionError("the turns must not be read while the checkpoint is current")

    response = setup.ask("What about it?", conversation_id)

    assert response.json() == {"answer": "Answer to: What about it?", "conversation_id": conversation_id}
    assert setup.agent.histories == [[], ["First question?", "Answer to: First question?"]]


def test_lost_checkpoint_is_restored_from_the_turns(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]
    setup.ask("Second question?", conversation_id)
    setup.checkpointer.delete_thread(conversation_id)

    setup.ask("Third question?", conversation_id)

    assert setup.agent.histories[-1] == ["First question?", "Answer to: First question?", "Second question?", "Answer to: Second question?"]


def test_another_worker_or_a_restarted_api_continues_the_conversation(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]
    setup.store.errors["recent_turns"] = AssertionError("the checkpoint is shared, so the turns are not read")

    # Same database, new process: nothing is kept in the API's memory
    other = MemorySetup(agent=setup.agent, store=setup.store, checkpointer=setup.checkpointer)
    with other.client:
        response = other.ask("What about it?", conversation_id)

    assert response.status_code == 200
    assert setup.agent.histories[-1] == ["First question?", "Answer to: First question?"]


def test_only_the_latest_checkpoint_is_kept(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]
    for question in ("Second question?", "Third question?"):
        setup.ask(question, conversation_id)

    assert setup.checkpointer.checkpoint_count(conversation_id) == 1


def test_memory_keeps_only_the_last_answered_turns():

    short = MemorySetup(max_turns=2)
    with short.client:
        conversation_id = short.ask("Question 1?").json()["conversation_id"]
        for number in range(2, 5):
            short.ask(f"Question {number}?", conversation_id)

    assert short.agent.histories[-1] == ["Question 2?", "Answer to: Question 2?", "Question 3?", "Answer to: Question 3?"]


def test_checkpoint_read_failure_is_503_and_the_retry_works(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]
    setup.checkpointer.errors["get_tuple"] = RuntimeError("secret database detail")

    failed = setup.ask("What about it?", conversation_id)
    setup.checkpointer.errors.clear()
    retried = setup.ask("What about it?", conversation_id)

    assert failed.status_code == 503
    assert "secret" not in failed.text
    assert retried.status_code == 200
    assert setup.store.statuses(conversation_id) == ["completed", "completed"]
    assert setup.agent.histories[-1] == ["First question?", "Answer to: First question?"]


def test_checkpoint_save_failure_is_503_and_the_turn_fails(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]
    setup.checkpointer.errors["put"] = RuntimeError("synthetic database error")

    response = setup.ask("What about it?", conversation_id)

    assert response.status_code == 503
    assert setup.store.statuses(conversation_id) == ["completed", "failed"]


def test_unreadable_checkpoint_is_rebuilt_from_the_turns(setup):

    # A checkpoint sealed with a key that is no longer in the key ring
    serde = setup.checkpointer.serde
    setup.checkpointer.serde = make_serializer(key_id=2, key=bytes(32))
    conversation_id = setup.ask("First question?").json()["conversation_id"]
    setup.checkpointer.serde = serde

    response = setup.ask("What about it?", conversation_id)

    assert response.status_code == 200
    assert setup.agent.histories[-1] == ["First question?", "Answer to: First question?"]


def test_conversations_do_not_mix(setup):

    first = setup.ask("First conversation?").json()["conversation_id"]
    second = setup.ask("Second conversation?").json()["conversation_id"]

    setup.ask("Follow-up?", second)

    assert first != second
    assert setup.agent.histories[-1] == ["Second conversation?", "Answer to: Second conversation?"]


def test_another_citizens_conversation_is_404_and_the_graph_is_not_run(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]

    response = setup.ask("Let me in?", conversation_id, citizen=OTHER_CITIZEN)

    assert response.status_code == 404
    assert len(setup.agent.histories) == 1


def test_unknown_conversation_is_404(setup):

    response = setup.ask("First question?", CONVERSATION_ID)

    assert response.status_code == 404
    assert setup.agent.histories == []


def test_same_question_still_pending_is_409_with_retry_after(setup):

    started = setup.conversations.start(CITIZEN, None, "First question?")

    response = setup.ask("First question?", started.conversation_id)

    assert response.status_code == 409
    assert response.headers["Retry-After"] == "2"
    assert setup.agent.histories == []


def test_quick_repeat_gets_the_stored_answer_without_the_model(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]

    response = setup.ask("First question?", conversation_id)

    assert response.json() == {"answer": "Answer to: First question?", "conversation_id": conversation_id}
    assert len(setup.agent.histories) == 1
    assert setup.store.statuses(conversation_id) == ["completed"]


def test_failed_answer_is_503_and_the_retry_reuses_the_turn():

    failing = MemorySetup(agent=RecordingKnowledgeAgent(errors=[LLMTimeoutError("down"), None]))
    with failing.client:
        started = failing.conversations.start(CITIZEN, None, "Opening question?")
        failing.conversations.complete(started, "Opening answer.")
        conversation_id = started.conversation_id

        failed = failing.ask("First question?", conversation_id)
        retried = failing.ask("First question?", conversation_id)

    assert failed.status_code == 503
    assert retried.json()["answer"] == "Answer to: First question?"
    assert failing.store.statuses(conversation_id) == ["completed", "completed"]
    assert failing.agent.histories[-1] == ["Opening question?", "Opening answer."]


def test_invalid_question_advice_is_shown_but_not_kept():

    invalid = MemorySetup(agent=RecordingKnowledgeAgent(errors=[InvalidQuestionError(LONG_QUESTION_REPLY)]))
    with invalid.client:
        response = invalid.ask("Too long?")

    conversation_id = response.json()["conversation_id"]
    assert response.json() == {"answer": LONG_QUESTION_REPLY, "conversation_id": conversation_id}
    assert invalid.store.statuses(conversation_id) == ["failed"]
    assert invalid.conversations.restore(conversation_id) == []


def test_answer_that_cannot_be_saved_is_503_and_deletes_the_checkpoint(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]
    setup.store.errors["complete_turn"] = RuntimeError("synthetic database error")

    response = setup.ask("Second question?", conversation_id)

    assert response.status_code == 503
    assert setup.checkpointer.get_tuple({"configurable": {"thread_id": conversation_id}}) is None


def test_unsaved_answer_is_never_remembered_even_when_its_delete_failed(setup):

    conversation_id = setup.ask("First question?").json()["conversation_id"]
    setup.store.errors["complete_turn"] = RuntimeError("synthetic database error")
    setup.checkpointer.errors["delete_thread"] = RuntimeError("synthetic database error")
    setup.ask("Unsaved question?", conversation_id)
    setup.store.errors.clear()
    setup.checkpointer.errors.clear()

    setup.ask("Next question?", conversation_id)

    # The checkpoint still holds the unsaved turn, but its tag is not the newest answered turn
    assert setup.agent.histories[-1] == ["First question?", "Answer to: First question?"]


def test_conversation_store_down_is_503_without_details(setup):

    setup.store.errors["start_turn"] = RuntimeError("secret database detail")

    response = setup.ask("First question?")

    assert response.status_code == 503
    assert "secret" not in response.text
    assert setup.agent.histories == []


def test_graph_crash_fails_the_turn():

    crashing = MemorySetup()
    with crashing.client:
        crashing.app.state.memory_graph = FakeGraph(error=RuntimeError("secret internal detail"))
        response = crashing.ask("First question?")

    assert response.status_code == 503
    conversation_ids = list(crashing.store.conversations_by_id)
    assert crashing.store.statuses(conversation_ids[0]) == ["failed"]


def test_conversations_are_closed_when_the_api_stops():

    created = MemorySetup()
    with created.client:
        created.client.get("/health")

    assert created.store.closed


def test_no_content_or_citizen_is_logged(caplog, setup):

    with caplog.at_level(logging.DEBUG):
        conversation_id = setup.ask("Synthetic secret question?").json()["conversation_id"]
        setup.ask("Synthetic secret follow-up?", conversation_id)

    assert "Synthetic secret" not in caplog.text
    assert CITIZEN not in caplog.text


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
