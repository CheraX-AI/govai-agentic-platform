"""Tests for app/graph/workflow.py with a fake Knowledge Agent."""

from langchain_core.messages import AIMessage, HumanMessage

from app.agents.knowledge_agent.validation import EMPTY_QUESTION_REPLY, SHORT_QUESTION_REPLY, InvalidQuestionError
from app.agents.knowledge_agent.schemas import KnowledgeAnswer, SourceReference
from app.graph.workflow import (
    ASSISTANT_UNAVAILABLE_REPLY,
    FAILURE_INVALID_QUESTION,
    FAILURE_UNAVAILABLE,
    KNOWLEDGE_AGENT_NODE,
    build_graph,
)
from app.llm.exceptions import LLMTimeoutError
from app.tools.knowledge_rag_client import KnowledgeServiceUnavailableError


# Fakes

class FakeKnowledgeAgent:
    """Answers like KnowledgeAgent.answer, or raises the given error, and records each call."""

    def __init__(self, error=None):

        self.error = error
        self.calls = []

    def answer(self, question, request_id=None):

        self.calls.append({"question": question, "request_id": request_id})
        if self.error is not None:
            raise self.error

        answer = KnowledgeAnswer(
            answer=f"Answer to: {question} [1]",
            sources=[SourceReference(number=1, document_title="Synthetic guide", page_number=1)],
            scope="govai_documents",
            prompt_version="v1",
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
    assert messages[-1].content == "Answer to: Synthetic question? [1]"


def test_full_answer_is_kept_in_the_state():

    result = ask(build_graph(FakeKnowledgeAgent()), "Synthetic question?")

    answer = result["knowledge_answer"]
    assert answer["sources"][0]["number"] == 1
    assert answer["scope"] == "govai_documents"
    assert answer["prompt_version"] == "v1"


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

    # e.g. "hi": the citizen is told how to ask, not that the assistant is down
    agent = FakeKnowledgeAgent(error=InvalidQuestionError(SHORT_QUESTION_REPLY))

    result = ask(build_graph(agent), "hi")

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

    result = ask(build_graph(agent), "hi")

    assert result["failure"] == FAILURE_INVALID_QUESTION
