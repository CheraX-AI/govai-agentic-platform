"""Live tests: conversations and short-term memory (LangGraph checkpoints) in the real PostgreSQL database.

Skipped unless agent-service/.env (or the environment) has the AGENT_MEMORY_* settings and
the database is set up (README, Conversations and memory). Each Worker is one API process
with its own pools, so two Workers act as two uvicorn workers or a restarted API. Each test
uses new random citizens and deletes everything after. The reconnect test drops every
govai-agent connection of the runtime login, so a running agent-api reconnects too.
"""

import threading
import time
import uuid

import pytest
from fastapi.testclient import TestClient
from psycopg import errors as database_errors

from app.agents.knowledge_agent.schemas import KnowledgeAnswer
from app.api.config import AgentAPISettings
from app.api.conversations import CONVERSATIONS_PATH
from app.api.main import CHAT_PATH, create_app
from app.conversations.models import COMPLETED, START_REOPEN, ConversationBusyError, ConversationNotFoundError
from app.conversations.service import ConversationService
from app.conversations.store import open_conversation_store
from app.graph.workflow import build_graph
from app.memory.checkpointer import close_checkpointer, open_checkpointer
from app.memory.config import MemoryConfigError, load_memory_settings
from app.memory.encryption import CIPHER_PREFIX, KeyRing, build_key_ring, create_serializer
from app.memory.errors import MemoryUnavailableError

TEST_TOKEN = "synthetic-service-token-0123456789abcdef"
CHECKPOINT_TABLES = ("checkpoints", "checkpoint_blobs", "checkpoint_writes")


# Fakes

class RecordingKnowledgeAgent:
    """Answers every question, optionally slowly, and records the history it was given."""

    def __init__(self, delay_seconds=0):

        self.delay_seconds = delay_seconds
        self.histories = []
        self.lock = threading.Lock()

    def answer(self, question, request_id=None, history=()):

        time.sleep(self.delay_seconds)
        with self.lock:
            self.histories.append([message.content for message in history])

        answer = KnowledgeAnswer(answer=f"Answer to: {question}", scope="govai_documents", prompt_version="v2")

        return answer


class Worker:
    """One API process: its own store pool, checkpointer pool and event loop, as uvicorn builds them."""

    def __init__(self, settings, agent):

        self.agent = agent
        self.conversations = open_conversations(settings)
        self.store = self.conversations.store
        self.checkpointer = self.conversations.checkpointer
        self.closed = False
        self.app = create_app(
            graph=build_graph(agent),
            settings=AgentAPISettings(auth_mode="bearer", token=TEST_TOKEN),
            memory_graph=build_graph(agent, self.checkpointer, settings.max_turns),
            conversations=self.conversations,
        )
        self.client = TestClient(self.app)
        self.client.__enter__()

    def ask(self, citizen, question, conversation_id=None):

        body = {"message": question}
        if conversation_id is not None:
            body["conversation_id"] = conversation_id

        headers = {"Authorization": f"Bearer {TEST_TOKEN}", "X-GovAI-Citizen": citizen}
        response = self.client.post(CHAT_PATH, json=body, headers=headers)

        return response

    def delete(self, citizen, conversation_id):

        headers = {"Authorization": f"Bearer {TEST_TOKEN}", "X-GovAI-Citizen": citizen}
        response = self.client.delete(f"{CONVERSATIONS_PATH}/{conversation_id}", headers=headers)

        return response

    def close(self):

        # The API's shutdown closes the store; the test closes the checkpointer it opened
        if not self.closed:
            self.closed = True
            self.client.__exit__(None, None, None)
            close_checkpointer(self.checkpointer)


def open_conversations(settings):

    # The store and the checkpointer, as the API's startup opens them
    ring = build_key_ring(settings)
    store = open_conversation_store(settings, ring)
    try:
        checkpointer = open_checkpointer(settings, create_serializer(ring))
    except MemoryUnavailableError:
        store.close()
        raise

    service = ConversationService(store, checkpointer, settings.max_turns, settings.in_flight_seconds, settings.replay_seconds)

    return service


# Fixtures

@pytest.fixture(scope="module")
def settings():

    # The memory settings must be in .env
    try:
        loaded = load_memory_settings()
    except MemoryConfigError as error:
        pytest.skip(f"memory settings are invalid ({error})")

    if loaded is None:
        pytest.skip("AGENT_MEMORY_DATABASE_URL is not set")

    return loaded


@pytest.fixture
def open_worker(settings):

    # Opens workers on demand and closes every one after the test
    opened = []

    def open_one(agent=None):
        try:
            worker = Worker(settings, agent if agent is not None else RecordingKnowledgeAgent())
        except MemoryUnavailableError as error:
            pytest.skip(str(error))
        opened.append(worker)
        return worker

    yield open_one

    for worker in opened:
        worker.close()


@pytest.fixture
def worker(open_worker):

    return open_worker()


@pytest.fixture
def conversations(worker):

    return worker.conversations


@pytest.fixture
def new_citizen(settings):

    # Fresh citizens; everything they created is deleted afterwards
    created = []

    def make():
        subject = f"live-test-{uuid.uuid4()}"
        created.append(subject)
        return subject

    yield make

    if not created:
        return

    cleaner = open_conversations(settings)
    try:
        for subject in created:
            cleaner.delete_all(subject)
    finally:
        cleaner.close()
        close_checkpointer(cleaner.checkpointer)


@pytest.fixture
def citizen(new_citizen):

    return new_citizen()


def answered(conversations, citizen, question, conversation_id=None):

    started = conversations.start(citizen, conversation_id, question)
    conversations.complete(started, f"Answer to: {question}")

    return started.conversation_id


def checkpoint_rows(store, table, conversation_id):

    with store.pool.connection() as connection:
        rows = connection.execute(f"SELECT * FROM {table} WHERE thread_id = %s", (conversation_id,)).fetchall()

    return rows


def block_turn_reads(monkeypatch, store):

    # Proves an answer came from the checkpoint: reading the turns for a restore fails the test
    def fail(*args, **kwargs):
        raise AssertionError("the turns must not be read while the checkpoint is current")

    monkeypatch.setattr(store, "recent_turns", fail)


# PostgreSQL: the same turn rules as the fake store

def test_turn_rules_hold_in_the_real_database(conversations, citizen):

    started = conversations.start(citizen, None, "Synthetic first question?")
    conversation_id = started.conversation_id

    with pytest.raises(ConversationBusyError):
        conversations.start(citizen, conversation_id, "Synthetic first question?")

    conversations.fail(started)
    reopened = conversations.start(citizen, conversation_id, "Synthetic first question?")
    conversations.complete(reopened, "Synthetic answer.")
    replayed = conversations.start(citizen, conversation_id, "Synthetic first question?")

    assert reopened.kind == START_REOPEN
    assert reopened.turn_id == started.turn_id
    assert replayed.replayed_answer == "Synthetic answer."
    assert replayed.last_answered_turn_id == started.turn_id


def test_restore_and_history_are_in_order(conversations, citizen):

    conversation_id = answered(conversations, citizen, "Q1?")
    for question in ("Q2?", "Q3?"):
        answered(conversations, citizen, question, conversation_id)

    restored = conversations.restore(conversation_id)
    newest, before = conversations.turns(citizen, conversation_id, None, 2)
    older, _ = conversations.turns(citizen, conversation_id, before, 2)

    assert [message.content for message in restored][-2:] == ["Q3?", "Answer to: Q3?"]
    assert [turn.question for turn in newest] == ["Q2?", "Q3?"]
    assert [turn.question for turn in older] == ["Q1?"]


def test_list_shows_only_conversations_with_an_answer(conversations, citizen):

    ghost = conversations.start(citizen, None, "Synthetic failed question?")
    conversations.fail(ghost)
    pending = conversations.start(citizen, None, "Synthetic pending question?")
    answered_id = answered(conversations, citizen, "Synthetic answered question?")

    listed, _ = conversations.conversations(citizen, 0, 20)

    assert [summary.id for summary in listed] == [answered_id]
    assert conversations.store.owns(citizen, ghost.conversation_id)
    assert conversations.store.owns(citizen, pending.conversation_id)

    # A retry that is answered makes the conversation appear
    retried = conversations.start(citizen, ghost.conversation_id, "Synthetic failed question?")
    conversations.complete(retried, "Synthetic answer.")
    listed, _ = conversations.conversations(citizen, 0, 20)

    assert {summary.id for summary in listed} == {answered_id, ghost.conversation_id}


def test_rows_hold_ciphertext_only(conversations, citizen):

    conversation_id = answered(conversations, citizen, "Synthetic secret question?")

    with conversations.store.pool.connection() as connection:
        turn = connection.execute(
            "SELECT question_ciphertext, answer_ciphertext, status FROM conversation_turns WHERE conversation_id = %s",
            (conversation_id,),
        ).fetchone()
        title = connection.execute("SELECT title_ciphertext FROM conversations WHERE id = %s", (conversation_id,)).fetchone()

    assert turn["status"] == COMPLETED
    for value in (turn["question_ciphertext"], turn["answer_ciphertext"], title["title_ciphertext"]):
        assert b"Synthetic secret" not in bytes(value)


def test_runtime_login_cannot_rewrite_a_question(conversations, citizen):

    conversation_id = answered(conversations, citizen, "Synthetic question?")

    with pytest.raises(database_errors.InsufficientPrivilege):
        with conversations.store.pool.connection() as connection:
            connection.execute(
                "UPDATE conversation_turns SET question_ciphertext = question_ciphertext WHERE conversation_id = %s",
                (conversation_id,),
            )


def test_runtime_login_cannot_change_the_checkpoint_schema(conversations):

    with pytest.raises(database_errors.InsufficientPrivilege):
        with conversations.store.pool.connection() as connection:
            connection.execute("ALTER TABLE checkpoints ADD COLUMN live_test integer")


# Short-term memory across requests

def test_follow_ups_remember_the_conversation(worker, citizen, monkeypatch):

    first = worker.ask(citizen, "Synthetic first question?")
    conversation_id = first.json()["conversation_id"]
    block_turn_reads(monkeypatch, worker.store)

    second = worker.ask(citizen, "Synthetic follow-up?", conversation_id)
    worker.ask(citizen, "Synthetic third question?", conversation_id)

    assert first.status_code == second.status_code == 200
    assert worker.agent.histories[-1] == [
        "Synthetic first question?",
        "Answer to: Synthetic first question?",
        "Synthetic follow-up?",
        "Answer to: Synthetic follow-up?",
    ]


def test_memory_keeps_the_last_turns_in_one_checkpoint(worker, citizen, settings):

    conversation_id = worker.ask(citizen, "Synthetic question 0?").json()["conversation_id"]
    for number in range(1, settings.max_turns + 2):
        worker.ask(citizen, f"Synthetic question {number}?", conversation_id)

    history = worker.agent.histories[-1]
    assert len(history) == 2 * settings.max_turns
    assert history[0] == "Synthetic question 1?"
    assert len(checkpoint_rows(worker.store, "checkpoints", conversation_id)) == 1


def test_memory_survives_an_api_restart(open_worker, citizen, monkeypatch):

    agent = RecordingKnowledgeAgent()
    first_run = open_worker(agent)
    conversation_id = first_run.ask(citizen, "Synthetic first question?").json()["conversation_id"]
    first_run.close()

    restarted = open_worker(agent)
    block_turn_reads(monkeypatch, restarted.store)
    response = restarted.ask(citizen, "Synthetic follow-up?", conversation_id)

    assert response.status_code == 200
    assert agent.histories[-1] == ["Synthetic first question?", "Answer to: Synthetic first question?"]


def test_lost_checkpoint_is_restored_from_the_turns(worker, citizen):

    conversation_id = worker.ask(citizen, "Synthetic first question?").json()["conversation_id"]
    worker.checkpointer.delete_thread(conversation_id)

    worker.ask(citizen, "Synthetic follow-up?", conversation_id)

    assert worker.agent.histories[-1] == ["Synthetic first question?", "Answer to: Synthetic first question?"]


# Several workers and concurrent requests

def test_two_workers_continue_one_conversation(open_worker, citizen, monkeypatch):

    agent = RecordingKnowledgeAgent()
    first, second = open_worker(agent), open_worker(agent)
    conversation_id = first.ask(citizen, "Synthetic question A?").json()["conversation_id"]
    block_turn_reads(monkeypatch, first.store)
    block_turn_reads(monkeypatch, second.store)

    second.ask(citizen, "Synthetic question B?", conversation_id)
    first.ask(citizen, "Synthetic question C?", conversation_id)

    assert agent.histories[-1] == [
        "Synthetic question A?",
        "Answer to: Synthetic question A?",
        "Synthetic question B?",
        "Answer to: Synthetic question B?",
    ]


def test_concurrent_conversations_on_two_workers_stay_apart(open_worker, new_citizen):

    agent = RecordingKnowledgeAgent(delay_seconds=0.2)
    workers = [open_worker(agent), open_worker(agent)]
    results = {}

    def chat(index):
        worker = workers[index % 2]
        citizen = new_citizen()
        conversation_id = worker.ask(citizen, f"Synthetic opening {index}?").json()["conversation_id"]
        follow_up = workers[(index + 1) % 2].ask(citizen, f"Synthetic follow-up {index}?", conversation_id)
        results[index] = follow_up

    threads = [threading.Thread(target=chat, args=(index,)) for index in range(6)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join(timeout=60)

    assert len(results) == 6
    for response in results.values():
        assert response.status_code == 200

    # Each follow-up saw only its own conversation's opening and answer
    for history in agent.histories:
        assert len(history) in (0, 2)
        if history:
            assert history[1] == f"Answer to: {history[0]}"


def test_same_question_twice_at_once_runs_once(open_worker, citizen):

    agent = RecordingKnowledgeAgent(delay_seconds=0.5)
    first, second = open_worker(agent), open_worker(agent)
    conversation_id = first.ask(citizen, "Synthetic opening?").json()["conversation_id"]
    responses = []

    def ask(worker):
        responses.append(worker.ask(citizen, "Synthetic same question?", conversation_id))

    threads = [threading.Thread(target=ask, args=(worker,)) for worker in (first, second)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join(timeout=60)

    # The second gets 409 while the first runs, or the stored answer just after it
    statuses = sorted(response.status_code for response in responses)
    turns = first.store.recent_turns(conversation_id, 10)
    assert statuses in ([200, 409], [200, 200])
    assert [turn.question for turn in turns] == ["Synthetic opening?", "Synthetic same question?"]


def test_overlapping_questions_leave_no_turn_out_of_memory(open_worker, citizen):

    agent = RecordingKnowledgeAgent(delay_seconds=0.5)
    first, second = open_worker(agent), open_worker(agent)
    conversation_id = first.ask(citizen, "Synthetic opening?").json()["conversation_id"]

    # Two different questions at once, on two workers: both start from the same memory
    threads = [
        threading.Thread(target=first.ask, args=(citizen, "Synthetic question X?", conversation_id)),
        threading.Thread(target=second.ask, args=(citizen, "Synthetic question Y?", conversation_id)),
    ]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join(timeout=60)

    first.ask(citizen, "Synthetic last question?", conversation_id)

    # The newest checkpoint misses one of them, so the last question restores both from the turns
    history = agent.histories[-1]
    assert "Synthetic question X?" in history
    assert "Synthetic question Y?" in history


# Isolation and deleting

def test_another_citizen_gets_404_and_memory_is_untouched(worker, new_citizen):

    owner, other = new_citizen(), new_citizen()
    conversation_id = worker.ask(owner, "Synthetic private question?").json()["conversation_id"]
    before = checkpoint_rows(worker.store, "checkpoints", conversation_id)

    asked = worker.ask(other, "Let me in?", conversation_id)
    deleted = worker.delete(other, conversation_id)

    assert asked.status_code == 404
    assert deleted.status_code == 404
    assert len(worker.agent.histories) == 1
    assert checkpoint_rows(worker.store, "checkpoints", conversation_id) == before


def test_delete_removes_the_turns_and_every_checkpoint_row(worker, citizen):

    conversation_id = worker.ask(citizen, "Synthetic question?").json()["conversation_id"]
    worker.ask(citizen, "Synthetic follow-up?", conversation_id)

    response = worker.delete(citizen, conversation_id)

    assert response.status_code == 204
    assert not worker.store.owns(citizen, conversation_id)
    for table in CHECKPOINT_TABLES:
        assert checkpoint_rows(worker.store, table, conversation_id) == []


# Encryption

def test_checkpoint_tables_hold_ciphertext_only(worker, citizen, settings):

    conversation_id = worker.ask(citizen, "Synthetic secret question?").json()["conversation_id"]
    worker.ask(citizen, "Synthetic secret follow-up?", conversation_id)

    checkpoints = checkpoint_rows(worker.store, "checkpoints", conversation_id)
    blobs = checkpoint_rows(worker.store, "checkpoint_blobs", conversation_id)
    writes = checkpoint_rows(worker.store, "checkpoint_writes", conversation_id)

    assert checkpoints and blobs
    for row in checkpoints + blobs + writes:
        assert "Synthetic secret" not in str(row)
        assert "Answer to" not in str(row)

    # Every stored value names the current key; "empty" marks a channel with no value
    for row in blobs + writes:
        assert row["type"] == "empty" or row["type"].endswith(f"+{CIPHER_PREFIX}{settings.data_key_id}")


# Failures and recovery

def test_unsaved_answer_is_503_and_never_remembered(worker, citizen, monkeypatch):

    conversation_id = worker.ask(citizen, "Synthetic first question?").json()["conversation_id"]

    def refuse(*args, **kwargs):
        raise MemoryUnavailableError("synthetic database error")

    with monkeypatch.context() as patch:
        patch.setattr(worker.store, "complete_turn", refuse)
        failed = worker.ask(citizen, "Synthetic unsaved question?", conversation_id)

    worker.ask(citizen, "Synthetic next question?", conversation_id)

    assert failed.status_code == 503
    assert worker.agent.histories[-1] == ["Synthetic first question?", "Answer to: Synthetic first question?"]


def test_unreadable_checkpoint_is_rebuilt_from_the_turns(worker, citizen):

    # A checkpoint sealed with a key that is no longer in the key ring
    serde = worker.checkpointer.serde
    worker.checkpointer.serde = create_serializer(KeyRing(255, {255: bytes(32)}))
    conversation_id = worker.ask(citizen, "Synthetic first question?").json()["conversation_id"]
    worker.checkpointer.serde = serde

    response = worker.ask(citizen, "Synthetic follow-up?", conversation_id)

    assert response.status_code == 200
    assert worker.agent.histories[-1] == ["Synthetic first question?", "Answer to: Synthetic first question?"]


def test_api_reconnects_after_its_connections_are_dropped(worker, citizen):

    conversation_id = worker.ask(citizen, "Synthetic first question?").json()["conversation_id"]

    # As after a database restart: every connection of the pools is closed by the server
    with worker.store.pool.connection() as connection:
        connection.execute(
            "SELECT pg_terminate_backend(pid) FROM pg_stat_activity "
            "WHERE usename = current_user AND application_name = 'govai-agent' AND pid <> pg_backend_pid()"
        )

    response = worker.ask(citizen, "Synthetic follow-up?", conversation_id)

    assert response.status_code == 200
    assert worker.agent.histories[-1] == ["Synthetic first question?", "Answer to: Synthetic first question?"]
