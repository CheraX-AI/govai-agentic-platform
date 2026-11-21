"""Live tests: conversation memory in the real PostgreSQL database, with a fake Knowledge Agent.

Skipped unless agent-service/.env (or the environment) has AGENT_MEMORY_DATABASE_URL and
AGENT_MEMORY_DATA_KEY, and the database is set up (README, Conversation memory).
Each test uses a new random conversation ID and forgets it afterwards.
"""

import uuid

import pytest
from langchain_core.messages import HumanMessage

from app.agents.knowledge_agent.schemas import KnowledgeAnswer
from app.graph.workflow import build_graph
from app.memory.checkpointer import MemoryUnavailableError, open_memory
from app.memory.config import MemoryConfigError, load_memory_settings


# Fakes

class RecordingKnowledgeAgent:
    """Answers every question and records the history it was given."""

    def __init__(self):

        self.histories = []

    def answer(self, question, request_id=None, history=()):

        self.histories.append([message.content for message in history])
        answer = KnowledgeAnswer(answer=f"Answer to: {question}", scope="govai_documents", prompt_version="v2")

        return answer


# Fixtures

@pytest.fixture(scope="module")
def memory():

    # The memory settings must be in .env and the database must be ready
    try:
        settings = load_memory_settings()
    except MemoryConfigError as error:
        pytest.skip(f"memory settings are invalid ({error})")

    if settings is None:
        pytest.skip("AGENT_MEMORY_DATABASE_URL is not set")

    try:
        opened = open_memory(settings)
    except MemoryUnavailableError as error:
        pytest.skip(str(error))

    yield opened

    opened.close()


@pytest.fixture
def conversation_id(memory):

    # A fresh conversation, forgotten after the test
    new_id = str(uuid.uuid4())

    yield new_id

    memory.forget(new_id)


def ask_in(graph, conversation_id, question):

    config = {"configurable": {"thread_id": conversation_id}}
    result = graph.invoke({"messages": [HumanMessage(content=question)]}, config=config, durability="exit")

    return result


# Tests

def test_follow_up_sees_the_earlier_turn(memory, conversation_id):

    agent = RecordingKnowledgeAgent()
    graph = build_graph(agent, memory.checkpointer)

    ask_in(graph, conversation_id, "Synthetic first question?")
    ask_in(graph, conversation_id, "Synthetic follow-up?")

    assert agent.histories[1] == ["Synthetic first question?", "Answer to: Synthetic first question?"]


def test_memory_survives_a_new_graph(memory, conversation_id):

    # A new graph is what an API restart builds
    ask_in(build_graph(RecordingKnowledgeAgent(), memory.checkpointer), conversation_id, "Synthetic first question?")

    agent = RecordingKnowledgeAgent()
    ask_in(build_graph(agent, memory.checkpointer), conversation_id, "Synthetic follow-up?")

    assert agent.histories[0] == ["Synthetic first question?", "Answer to: Synthetic first question?"]


def test_stored_rows_hold_no_plaintext(memory, conversation_id):

    ask_in(build_graph(RecordingKnowledgeAgent(), memory.checkpointer), conversation_id, "Synthetic secret question?")

    with memory.pool.connection() as connection:
        rows = connection.execute(
            "SELECT type, blob FROM checkpoint_blobs WHERE thread_id = %s AND blob IS NOT NULL",
            (conversation_id,),
        ).fetchall()

    assert rows
    for row in rows:
        assert row["type"].endswith("+aesgcm")
        assert b"Synthetic secret" not in bytes(row["blob"])


def test_compact_keeps_one_checkpoint_and_forget_removes_it(memory, conversation_id):

    graph = build_graph(RecordingKnowledgeAgent(), memory.checkpointer)
    ask_in(graph, conversation_id, "Synthetic first question?")
    ask_in(graph, conversation_id, "Synthetic second question?")

    memory.compact(conversation_id)
    assert count_checkpoints(memory, conversation_id) == 1

    memory.forget(conversation_id)
    assert count_checkpoints(memory, conversation_id) == 0


# Helper functions

def count_checkpoints(memory, conversation_id):

    with memory.pool.connection() as connection:
        row = connection.execute(
            "SELECT count(*) AS total FROM checkpoints WHERE thread_id = %s",
            (conversation_id,),
        ).fetchone()

    return row["total"]
