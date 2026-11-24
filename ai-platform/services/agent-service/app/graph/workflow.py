"""The agent graph: START -> small_talk or knowledge_agent -> [remember] -> END."""

import logging

from langchain_core.messages import AIMessage, HumanMessage, RemoveMessage
from langgraph.graph import END, START, MessagesState, StateGraph

from app.agents.knowledge_agent.agent import create_knowledge_agent
from app.agents.knowledge_agent.validation import EMPTY_QUESTION_REPLY, InvalidQuestionError
from app.agents.small_talk import small_talk_reply
from app.graph.constants import (
    ASSISTANT_UNAVAILABLE_REPLY,
    FAILURE_INVALID_QUESTION,
    FAILURE_UNAVAILABLE,
    KNOWLEDGE_AGENT_NODE,
    REMEMBER_NODE,
    SMALL_TALK_NODE,
)
from app.llm.exceptions import LLMError
from app.memory.config import DEFAULT_MAX_TURNS
from app.memory.window import earlier_messages, messages_to_forget
from app.tools.knowledge_rag_client import KnowledgeServiceError

logger = logging.getLogger(__name__)


# Models

class AgentState(MessagesState):
    """Conversation state shared by every node; messages holds the chat."""

    # The message sent back to the citizen; a message, not a str, so memory stores it encrypted
    reply: AIMessage | None

    # The last answer with its scope and prompt version; None when no answer was made
    knowledge_answer: dict | None

    # Why there is no answer: FAILURE_INVALID_QUESTION, FAILURE_UNAVAILABLE, or None on success
    failure: str | None


# Nodes

class KnowledgeAgentNode:
    """Graph node that passes the latest question, and the conversation before it, to the Knowledge Agent."""

    def __init__(self, knowledge_agent):

        self.knowledge_agent = knowledge_agent

    def __call__(self, state, config):

        # The callers put a request ID in the run metadata
        request_id = (config.get("metadata") or {}).get("request_id")

        try:
            # Answer the latest question; earlier messages explain follow-ups
            question = latest_question(state["messages"])
            history = earlier_messages(state["messages"])
            answer = self.knowledge_agent.answer(question, request_id=request_id, history=history)

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
        reply = AIMessage(content=answer.answer)
        update = {
            "messages": [reply],
            "reply": reply,
            "knowledge_answer": answer.model_dump(),
            "failure": None,
        }

        return update


class SmallTalkNode:
    """Graph node that answers a greeting, thanks or acknowledgment with a fixed reply, without the documents."""

    def __call__(self, state):

        # route_message sent only small talk here, so there is always a reply
        reply = AIMessage(content=small_talk_reply(latest_question(state["messages"])))
        update = {
            "messages": [reply],
            "reply": reply,
            "knowledge_answer": None,
            "failure": None,
        }

        return update


class RememberNode:
    """Graph node that trims the remembered chat before it is saved."""

    def __init__(self, max_turns):

        self.max_turns = max_turns

    def __call__(self, state):

        # Forget failed and repeated turns, and the oldest beyond the limit
        forget = messages_to_forget(state["messages"], self.max_turns, answered=state.get("failure") is None)

        removals = []
        for message in forget:
            removals.append(RemoveMessage(id=message.id))

        update = {"messages": removals}

        return update


# Public functions

def build_graph(knowledge_agent, checkpointer=None, max_turns=DEFAULT_MAX_TURNS):
    """Build the graph around a Knowledge Agent (real, or a fake in tests).

    With a checkpointer the graph remembers each conversation (thread_id), and a
    remember node keeps that memory to the last max_turns answered turns.
    """

    # Create the graph with the shared conversation state
    builder = StateGraph(AgentState)

    # Add the answer nodes: start -> small talk or knowledge agent
    builder.add_node(SMALL_TALK_NODE, SmallTalkNode())
    builder.add_node(KNOWLEDGE_AGENT_NODE, KnowledgeAgentNode(knowledge_agent))
    builder.add_conditional_edges(START, route_message, [SMALL_TALK_NODE, KNOWLEDGE_AGENT_NODE])

    # Only a remembering graph trims its chat; without memory nothing is kept anyway
    last_node = END
    if checkpointer is not None:
        builder.add_node(REMEMBER_NODE, RememberNode(max_turns))
        builder.add_edge(REMEMBER_NODE, END)
        last_node = REMEMBER_NODE

    builder.add_edge(SMALL_TALK_NODE, last_node)
    builder.add_edge(KNOWLEDGE_AGENT_NODE, last_node)

    graph = builder.compile(checkpointer=checkpointer)

    return graph


def create_graph():
    """Build the graph with the real Knowledge Agent from .env, without memory."""

    knowledge_agent = create_knowledge_agent()
    graph = build_graph(knowledge_agent)

    return graph


# Helper functions

def route_message(state):

    # Greetings, thanks and acknowledgments need no documents; everything else goes to the Knowledge Agent
    try:
        question = latest_question(state["messages"])
    except InvalidQuestionError:
        return KNOWLEDGE_AGENT_NODE

    node = SMALL_TALK_NODE if small_talk_reply(question) is not None else KNOWLEDGE_AGENT_NODE

    return node


def latest_question(messages):

    # Walk back to the newest message from the citizen
    for message in reversed(messages):
        if isinstance(message, HumanMessage):
            question = str(message.text)
            return question

    raise InvalidQuestionError(EMPTY_QUESTION_REPLY)


def reply_without_answer(reply, failure):

    # A chat reply with no knowledge answer behind it, and the reason why
    message = AIMessage(content=reply)
    update = {
        "messages": [message],
        "reply": message,
        "knowledge_answer": None,
        "failure": failure,
    }

    return update
