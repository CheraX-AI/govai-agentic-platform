"""The agent graph: START -> knowledge_agent -> END."""

import logging

from langchain_core.messages import AIMessage, HumanMessage
from langgraph.graph import END, START, MessagesState, StateGraph

from app.agents.knowledge_agent.agent import create_knowledge_agent
from app.agents.knowledge_agent.validation import EMPTY_QUESTION_REPLY, InvalidQuestionError
from app.llm.exceptions import LLMError
from app.tools.knowledge_rag_client import KnowledgeServiceError

logger = logging.getLogger(__name__)

KNOWLEDGE_AGENT_NODE = "knowledge_agent"

# Shown instead of internal errors; the details are only logged
ASSISTANT_UNAVAILABLE_REPLY = "The GovAI assistant cannot answer right now. Please try again later."

# Why a turn has no answer, so callers such as the HTTP API can pick a status code
FAILURE_INVALID_QUESTION = "invalid_question"
FAILURE_UNAVAILABLE = "unavailable"


# Models

class AgentState(MessagesState):
    """Conversation state shared by every node.

    messages holds the chat; knowledge_answer holds the last answer with its
    cited sources, scope, and prompt version (None when no answer was made).
    failure says why there is no answer: FAILURE_INVALID_QUESTION,
    FAILURE_UNAVAILABLE, or None when the turn succeeded.
    """

    knowledge_answer: dict | None
    failure: str | None


# Node

class KnowledgeAgentNode:
    """Graph node that passes the latest question to the Knowledge Agent."""

    def __init__(self, knowledge_agent):

        self.knowledge_agent = knowledge_agent

    def __call__(self, state, config):

        # The callers put a request ID in the run metadata
        request_id = (config.get("metadata") or {}).get("request_id")

        try:
            # Answer the latest question from the knowledge service documents
            question = latest_question(state["messages"])
            answer = self.knowledge_agent.answer(question, request_id=request_id)

        except InvalidQuestionError as error:
            # Empty, too short, or too long: tell the citizen how to ask instead
            update = reply_without_answer(str(error), FAILURE_INVALID_QUESTION)
            return update

        except (KnowledgeServiceError, LLMError) as error:
            # Never show internal errors to the citizen; log the type for support
            logger.warning(
                "knowledge agent failed",
                extra={"request_id": request_id, "error_type": type(error).__name__},
            )
            update = reply_without_answer(ASSISTANT_UNAVAILABLE_REPLY, FAILURE_UNAVAILABLE)
            return update

        # Add the reply to the chat and keep the full answer for the caller
        update = {
            "messages": [AIMessage(content=answer.answer)],
            "knowledge_answer": answer.model_dump(),
            "failure": None,
        }

        return update


# Public functions

def build_graph(knowledge_agent):
    """Build the graph around a Knowledge Agent (real, or a fake in tests)."""

    # Create the graph with the shared conversation state
    builder = StateGraph(AgentState)

    # Add the agent nodes
    builder.add_node(KNOWLEDGE_AGENT_NODE, KnowledgeAgentNode(knowledge_agent))

    # Connect the nodes: start -> knowledge agent -> end
    builder.add_edge(START, KNOWLEDGE_AGENT_NODE)
    builder.add_edge(KNOWLEDGE_AGENT_NODE, END)

    graph = builder.compile()

    return graph


def create_graph():
    """Build the graph with the real Knowledge Agent from .env."""

    knowledge_agent = create_knowledge_agent()
    graph = build_graph(knowledge_agent)

    return graph


# Helper functions

def latest_question(messages):

    # Walk back to the newest message from the citizen
    for message in reversed(messages):
        if isinstance(message, HumanMessage):
            question = str(message.text)
            return question

    raise InvalidQuestionError(EMPTY_QUESTION_REPLY)


def reply_without_answer(reply, failure):

    # A chat reply with no knowledge answer behind it, and the reason why
    update = {
        "messages": [AIMessage(content=reply)],
        "knowledge_answer": None,
        "failure": failure,
    }

    return update
