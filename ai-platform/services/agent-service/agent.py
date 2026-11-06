"""GovAI agent graph skeleton, exposed to LangGraph as `graph`.

The Coordinator Agent is a placeholder. It does not call a model or any tool
yet; routing to the Knowledge, Service Guide, Application, Tracking/Appointment
agents and the Staff Copilot will be added as each one is implemented.
"""

from langchain_core.messages import AIMessage
from langgraph.graph import END, START, MessagesState, StateGraph

PLACEHOLDER_REPLY = "The GovAI assistant is not available yet. Please try again later."


# Models

class AgentState(MessagesState):
    """Conversation state shared by every node. Holds the chat messages."""


# Nodes

def coordinator(state):
    """Placeholder Coordinator Agent that replies with a fixed message."""

    # TODO: route the request to the right agent once the agents exist
    reply = AIMessage(content=PLACEHOLDER_REPLY)

    return {"messages": [reply]}


# Public functions

def build_graph():

    # Create the graph with the shared conversation state
    builder = StateGraph(AgentState)

    # Add the agent nodes
    builder.add_node("coordinator", coordinator)

    # Connect the nodes: start -> coordinator -> end
    builder.add_edge(START, "coordinator")
    builder.add_edge("coordinator", END)

    return builder.compile()


# Compiled graph loaded by langgraph.json
graph = build_graph()
