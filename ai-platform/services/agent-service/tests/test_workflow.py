"""Tests for app/graph/workflow.py with a fake Knowledge Agent; memory runs on the in-memory checkpointer."""

from langchain_core.messages import AIMessage, HumanMessage

from app.agents.knowledge_agent.validation import EMPTY_QUESTION_REPLY, SHORT_QUESTION_REPLY, InvalidQuestionError
from app.agents.knowledge_agent.schemas import KnowledgeAnswer
from app.graph.constants import (
    ASSISTANT_UNAVAILABLE_REPLY,
    FAILURE_INVALID_QUESTION,
    FAILURE_UNAVAILABLE,
    KNOWLEDGE_AGENT_NODE,
    REMEMBER_NODE,
)
from app.graph.workflow import build_graph
from app.llm.exceptions import LLMTimeoutError
from app.tools.knowledge_rag_client import KnowledgeServiceUnavailableError
from fake_checkpointer import FakeCheckpointer


# Fakes

class FakeKnowledgeAgent:
    """Answers like KnowledgeAgent.answer, or raises the given errors in turn, and records each call."""

    def __init__(self, error=None, errors=()):

        self.errors = list(errors) if errors else [error]
        self.calls = []

    def answer(self, question, request_id=None, history=()):

        self.calls.append({"question": question, "request_id": request_id, "history": list(history)})

        # The next error, or the last one again; None means answer
        error = self.errors.pop(0) if len(self.errors) > 1 else self.errors[0]
        if error is not None:
            raise error

        answer = KnowledgeAnswer(
            answer=f"Answer to: {question}",
            scope="govai_documents",
            prompt_version="v2",
        )

        return answer


def ask(graph, question, config=None):

    # Send one citizen message through the graph
    result = graph.invoke({"messages": [HumanMessage(content=question)]}, config=config)

    return result


# Graph

def test_graph_has_the_knowledge_agent_node():

    graph = build_graph(FakeKnowledgeAgent())

    assert KNOWLEDGE_AGENT_NODE in graph.get_graph().nodes


def test_reply_is_added_to_the_chat():

    result = ask(build_graph(FakeKnowledgeAgent()), "Synthetic question?")

    messages = result["messages"]
    assert len(messages) == 2
    assert isinstance(messages[-1], AIMessage)
    assert messages[-1].content == "Answer to: Synthetic question?"


def test_full_answer_is_kept_in_the_state():

    result = ask(build_graph(FakeKnowledgeAgent()), "Synthetic question?")

    answer = result["knowledge_answer"]
    assert answer["answer"] == "Answer to: Synthetic question?"
    assert answer["scope"] == "govai_documents"
    assert answer["prompt_version"] == "v2"


def test_latest_question_is_answered():

    agent = FakeKnowledgeAgent()
    graph = build_graph(agent)

    graph.invoke(
        {
            "messages": [
                HumanMessage(content="First question?"),
                AIMessage(content="First answer."),
                HumanMessage(content="Second question?"),
            ]
        }
    )

    assert agent.calls[0]["question"] == "Second question?"


def test_request_id_comes_from_the_run_metadata():

    agent = FakeKnowledgeAgent()

    ask(build_graph(agent), "Synthetic question?", config={"metadata": {"request_id": "abc123"}})

    assert agent.calls[0]["request_id"] == "abc123"


# Errors

def test_unavailable_knowledge_service_gives_a_safe_reply():

    agent = FakeKnowledgeAgent(error=KnowledgeServiceUnavailableError("ChromaDB is not reachable"))

    result = ask(build_graph(agent), "Synthetic question?")

    assert result["messages"][-1].content == ASSISTANT_UNAVAILABLE_REPLY
    assert "ChromaDB" not in result["messages"][-1].content
    assert result["knowledge_answer"] is None


def test_llm_failure_gives_a_safe_reply():

    agent = FakeKnowledgeAgent(error=LLMTimeoutError("NVIDIA did not respond in time"))

    result = ask(build_graph(agent), "Synthetic question?")

    assert result["messages"][-1].content == ASSISTANT_UNAVAILABLE_REPLY


def test_unsearchable_question_gets_its_reply_not_the_unavailable_message():

    # e.g. "x?": the citizen is told how to ask, not that the assistant is down
    agent = FakeKnowledgeAgent(error=InvalidQuestionError(SHORT_QUESTION_REPLY))

    result = ask(build_graph(agent), "x?")

    assert result["messages"][-1].content == SHORT_QUESTION_REPLY
    assert result["knowledge_answer"] is None


def test_conversation_without_a_question_asks_for_one():

    # Only an assistant message: there is nothing to answer
    graph = build_graph(FakeKnowledgeAgent())

    result = graph.invoke({"messages": [AIMessage(content="Hello")]})

    assert result["messages"][-1].content == EMPTY_QUESTION_REPLY


# Failure reason

def test_answer_has_no_failure():

    result = ask(build_graph(FakeKnowledgeAgent()), "Synthetic question?")

    assert result["failure"] is None


def test_unavailable_service_is_marked_as_unavailable():

    agent = FakeKnowledgeAgent(error=KnowledgeServiceUnavailableError("down"))

    result = ask(build_graph(agent), "Synthetic question?")

    assert result["failure"] == FAILURE_UNAVAILABLE


def test_invalid_question_is_marked_as_invalid():

    agent = FakeKnowledgeAgent(error=InvalidQuestionError(SHORT_QUESTION_REPLY))

    result = ask(build_graph(agent), "x?")

    assert result["failure"] == FAILURE_INVALID_QUESTION


# Memory

def memory_saver():

    # An in-memory checkpointer with the real encrypting serializer
    saver = FakeCheckpointer()

    return saver


def ask_in(graph, conversation_id, question):

    # Send one citizen message in a remembered conversation, saved when the run ends
    config = {"configurable": {"thread_id": conversation_id}}
    result = graph.invoke({"messages": [HumanMessage(content=question)]}, config=config, durability="exit")

    return result


def remembered(graph, conversation_id):

    # The message texts saved for the conversation
    state = graph.get_state({"configurable": {"thread_id": conversation_id}})
    texts = []
    for message in state.values.get("messages", []):
        texts.append(message.content)

    return texts


def test_greeting_is_answered_without_the_knowledge_agent():

    agent = FakeKnowledgeAgent()

    result = ask(build_graph(agent), "Hello!")

    assert result["reply"].content.startswith("Hello! 👋 Welcome to GovAI.")
    assert result["failure"] is None
    assert agent.calls == []


def test_greeting_inside_a_question_goes_to_the_knowledge_agent():

    agent = FakeKnowledgeAgent()

    result = ask(build_graph(agent), "Hi, how do I renew my passport?")

    assert result["reply"].content == "Answer to: Hi, how do I renew my passport?"
    assert len(agent.calls) == 1


def test_small_talk_is_remembered_like_an_answered_turn():

    agent = FakeKnowledgeAgent()
    graph = build_graph(agent, memory_saver())

    ask_in(graph, "c1", "Good morning")
    ask_in(graph, "c1", "How do I renew my passport?")

    history = agent.calls[0]["history"]
    assert [message.content for message in history][0] == "Good morning"
    assert remembered(graph, "c1")[-1] == "Answer to: How do I renew my passport?"


def test_remember_node_is_added_only_with_memory():

    assert REMEMBER_NODE not in build_graph(FakeKnowledgeAgent()).get_graph().nodes
    assert REMEMBER_NODE in build_graph(FakeKnowledgeAgent(), memory_saver()).get_graph().nodes


def test_reply_is_in_the_state():

    result = ask(build_graph(FakeKnowledgeAgent()), "Synthetic question?")

    assert result["reply"].content == "Answer to: Synthetic question?"


def test_second_question_sees_the_first_turn():

    agent = FakeKnowledgeAgent()
    graph = build_graph(agent, memory_saver())

    ask_in(graph, "c1", "First question?")
    ask_in(graph, "c1", "What about it?")

    history = agent.calls[1]["history"]
    assert [message.content for message in history] == ["First question?", "Answer to: First question?"]
    assert agent.calls[1]["question"] == "What about it?"


def test_conversations_do_not_share_memory():

    agent = FakeKnowledgeAgent()
    graph = build_graph(agent, memory_saver())

    ask_in(graph, "c1", "First question?")
    ask_in(graph, "c2", "Other question?")

    assert agent.calls[1]["history"] == []


def test_failed_turn_is_not_remembered():

    agent = FakeKnowledgeAgent(errors=[KnowledgeServiceUnavailableError("down"), None])
    graph = build_graph(agent, memory_saver())

    failed = ask_in(graph, "c1", "First question?")
    ask_in(graph, "c1", "First question?")

    # The citizen still gets the safe reply, but memory holds only the answered retry
    assert failed["reply"].content == ASSISTANT_UNAVAILABLE_REPLY
    assert agent.calls[1]["history"] == []
    assert remembered(graph, "c1") == ["First question?", "Answer to: First question?"]


def test_invalid_question_is_not_remembered():

    agent = FakeKnowledgeAgent(errors=[InvalidQuestionError(SHORT_QUESTION_REPLY), None])
    graph = build_graph(agent, memory_saver())

    ask_in(graph, "c1", "x?")

    assert remembered(graph, "c1") == []


def test_repeated_question_is_remembered_once():

    graph = build_graph(FakeKnowledgeAgent(), memory_saver())

    ask_in(graph, "c1", "Same question?")
    ask_in(graph, "c1", "Same question?")

    assert remembered(graph, "c1") == ["Same question?", "Answer to: Same question?"]


def test_memory_keeps_only_the_last_turns():

    graph = build_graph(FakeKnowledgeAgent(), memory_saver(), max_turns=2)

    for number in range(4):
        ask_in(graph, "c1", f"Question {number}?")

    assert remembered(graph, "c1") == [
        "Question 2?",
        "Answer to: Question 2?",
        "Question 3?",
        "Answer to: Question 3?",
    ]


def test_no_chat_text_is_stored_unencrypted():

    saver = memory_saver()
    graph = build_graph(FakeKnowledgeAgent(), saver)
    ask_in(graph, "c1", "Synthetic secret question?")

    stored = saver.stored_bytes()
    assert stored
    for value in stored:
        assert b"Synthetic secret" not in value


def test_one_checkpoint_is_saved_per_run():

    saver = memory_saver()
    graph = build_graph(FakeKnowledgeAgent(), saver)

    ask_in(graph, "c1", "First question?")
    ask_in(graph, "c1", "Second question?")

    assert saver.checkpoint_count("c1") == 2
