"""Tests for app/api/conversations.py: listing, reading and deleting a citizen's conversations."""

import pytest
from fastapi.testclient import TestClient

from app.api.config import AgentAPISettings
from app.api.conversations import CONVERSATIONS_PATH
from app.api.main import INVALID_QUERY_DETAIL, create_app
from app.conversations.service import ConversationService
from app.memory.errors import MemoryUnavailableError
from fake_checkpointer import FakeCheckpointer
from fake_conversations import FakeConversationStore

TEST_TOKEN = "synthetic-service-token-0123456789abcdef"
CITIZEN = "11111111-2222-4333-8444-555555555555"
OTHER_CITIZEN = "66666666-7777-4888-9999-000000000000"
UNKNOWN_ID = "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"


# Fixtures

class FakeGraph:

    def invoke(self, state, config=None, **kwargs):

        raise AssertionError("history routes never run the graph")


@pytest.fixture
def conversations():

    service = ConversationService(FakeConversationStore(), FakeCheckpointer(), 6, 90, 600)

    return service


@pytest.fixture
def client(conversations):

    settings = AgentAPISettings(auth_mode="bearer", token=TEST_TOKEN)
    app = create_app(graph=FakeGraph(), settings=settings, memory_graph=None, conversations=conversations)

    with TestClient(app) as created:
        yield created


def headers(citizen=CITIZEN):

    return {"Authorization": f"Bearer {TEST_TOKEN}", "X-GovAI-Citizen": citizen}


def conversation_with(conversations, questions, citizen=CITIZEN):

    # A conversation with one answered turn per question
    conversation_id = None
    for question in questions:
        started = conversations.start(citizen, conversation_id, question)
        conversations.complete(started, f"Answer to: {question}")
        conversation_id = started.conversation_id
        conversations.store.advance(1)

    return conversation_id


# Listing

def test_list_shows_only_the_citizens_conversations_newest_first(client, conversations):

    older = conversation_with(conversations, ["Older question?"])
    newer = conversation_with(conversations, ["Newer question?"])
    conversation_with(conversations, ["Someone else's question?"], citizen=OTHER_CITIZEN)

    response = client.get(CONVERSATIONS_PATH, headers=headers())

    body = response.json()
    assert response.status_code == 200
    assert [item["id"] for item in body["conversations"]] == [newer, older]
    assert body["conversations"][0]["title"] == "Newer question?"
    assert body["next_offset"] is None


def test_list_pages_with_offset(client, conversations):

    for number in range(3):
        conversation_with(conversations, [f"Question {number}?"])

    first = client.get(CONVERSATIONS_PATH, params={"limit": 2}, headers=headers()).json()
    second = client.get(CONVERSATIONS_PATH, params={"limit": 2, "offset": first["next_offset"]}, headers=headers()).json()

    assert len(first["conversations"]) == 2
    assert first["next_offset"] == 2
    assert len(second["conversations"]) == 1
    assert second["next_offset"] is None


# Reading turns

def test_turns_are_paged_oldest_first(client, conversations):

    conversation_id = conversation_with(conversations, ["Q1?", "Q2?", "Q3?"])
    path = f"{CONVERSATIONS_PATH}/{conversation_id}/turns"

    newest = client.get(path, params={"limit": 2}, headers=headers()).json()
    older = client.get(path, params={"limit": 2, "before": newest["next_before"]}, headers=headers()).json()

    assert [turn["question"] for turn in newest["turns"]] == ["Q2?", "Q3?"]
    assert newest["turns"][0]["answer"] == "Answer to: Q2?"
    assert [turn["question"] for turn in older["turns"]] == ["Q1?"]
    assert older["next_before"] is None


def test_another_citizens_turns_are_404_like_an_unknown_conversation(client, conversations):

    conversation_id = conversation_with(conversations, ["Private question?"])

    theirs = client.get(f"{CONVERSATIONS_PATH}/{conversation_id}/turns", headers=headers(OTHER_CITIZEN))
    unknown = client.get(f"{CONVERSATIONS_PATH}/{UNKNOWN_ID}/turns", headers=headers())

    assert theirs.status_code == 404
    assert unknown.status_code == 404
    assert theirs.json() == unknown.json()
    assert "Private" not in theirs.text


@pytest.mark.parametrize(
    "path, params",
    [
        (f"{CONVERSATIONS_PATH}/not-a-uuid/turns", {}),
        (f"{CONVERSATIONS_PATH}/{UNKNOWN_ID.upper()}/turns", {}),
        (f"{CONVERSATIONS_PATH}/{UNKNOWN_ID}/turns", {"limit": 51}),
        (f"{CONVERSATIONS_PATH}/{UNKNOWN_ID}/turns", {"before": 0}),
        (CONVERSATIONS_PATH, {"offset": -1}),
    ],
)
def test_invalid_ids_and_paging_are_422_without_echoing(client, path, params):

    response = client.get(path, params=params, headers=headers())

    assert response.status_code == 422
    assert response.json() == {"detail": INVALID_QUERY_DETAIL}


# Deleting

def test_delete_removes_the_conversation(client, conversations):

    conversation_id = conversation_with(conversations, ["Question?"])

    response = client.delete(f"{CONVERSATIONS_PATH}/{conversation_id}", headers=headers())
    again = client.delete(f"{CONVERSATIONS_PATH}/{conversation_id}", headers=headers())

    assert response.status_code == 204
    assert again.status_code == 404
    assert client.get(f"{CONVERSATIONS_PATH}/{conversation_id}/turns", headers=headers()).status_code == 404


def test_another_citizen_cannot_delete(client, conversations):

    conversation_id = conversation_with(conversations, ["Question?"])

    response = client.delete(f"{CONVERSATIONS_PATH}/{conversation_id}", headers=headers(OTHER_CITIZEN))

    assert response.status_code == 404
    assert conversations.store.owns(CITIZEN, conversation_id)


def test_delete_all_removes_only_the_citizens_conversations(client, conversations):

    conversation_with(conversations, ["Mine?"])
    theirs = conversation_with(conversations, ["Theirs?"], citizen=OTHER_CITIZEN)

    response = client.delete(CONVERSATIONS_PATH, headers=headers())

    assert response.status_code == 204
    assert client.get(CONVERSATIONS_PATH, headers=headers()).json()["conversations"] == []
    assert conversations.store.owns(OTHER_CITIZEN, theirs)


def test_delete_with_memory_failure_is_503_and_deletes_nothing(client, conversations):

    conversation_id = conversation_with(conversations, ["Question?"])
    conversations.checkpointer.errors["delete_thread"] = RuntimeError("secret database detail")

    response = client.delete(f"{CONVERSATIONS_PATH}/{conversation_id}", headers=headers())

    assert response.status_code == 503
    assert "secret" not in response.text
    assert conversations.store.owns(CITIZEN, conversation_id)


def test_delete_all_with_memory_failure_is_503_and_deletes_nothing(client, conversations):

    conversation_id = conversation_with(conversations, ["Question?"])
    conversations.checkpointer.errors["prune"] = RuntimeError("secret database detail")

    response = client.delete(CONVERSATIONS_PATH, headers=headers())

    assert response.status_code == 503
    assert conversations.store.owns(CITIZEN, conversation_id)


# Access and failures

@pytest.mark.parametrize(
    "request_headers",
    [{}, {"X-GovAI-Citizen": CITIZEN}, {"Authorization": f"Bearer {TEST_TOKEN}"}, {"Authorization": "Bearer wrong", "X-GovAI-Citizen": CITIZEN}],
)
def test_every_route_needs_the_service_token_and_a_citizen(client, request_headers):

    responses = [
        client.get(CONVERSATIONS_PATH, headers=request_headers),
        client.get(f"{CONVERSATIONS_PATH}/{UNKNOWN_ID}/turns", headers=request_headers),
        client.delete(f"{CONVERSATIONS_PATH}/{UNKNOWN_ID}", headers=request_headers),
        client.delete(CONVERSATIONS_PATH, headers=request_headers),
    ]

    for response in responses:
        assert response.status_code == 401


def test_database_failure_is_503_without_details(client, conversations):

    conversations.store.errors["conversations"] = MemoryUnavailableError("secret database detail")

    response = client.get(CONVERSATIONS_PATH, headers=headers())

    assert response.status_code == 503
    assert "secret" not in response.text


def test_memory_off_is_503():

    settings = AgentAPISettings(auth_mode="bearer", token=TEST_TOKEN)

    with TestClient(create_app(graph=FakeGraph(), settings=settings)) as client:
        response = client.get(CONVERSATIONS_PATH, headers=headers())

    assert response.status_code == 503
