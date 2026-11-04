"""Tests for the GovAI agent graph skeleton."""

from langchain_core.messages import AIMessage, HumanMessage

from agent import PLACEHOLDER_REPLY, graph


def test_graph_has_coordinator_node():

    # The compiled graph must contain the coordinator node
    assert "coordinator" in graph.get_graph().nodes


def test_graph_replies_with_placeholder():

    # Send one user message through the graph
    result = graph.invoke({"messages": [HumanMessage(content="How do I renew my passport?")]})

    # The user message is kept and the coordinator adds one reply
    messages = result["messages"]
    assert len(messages) == 2
    assert isinstance(messages[-1], AIMessage)
    assert messages[-1].content == PLACEHOLDER_REPLY
