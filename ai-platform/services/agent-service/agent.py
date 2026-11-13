"""LangGraph entry point: builds the agent graph for langgraph.json (LangGraph Studio)."""

from app.graph.workflow import create_graph

# Compiled graph loaded by langgraph.json
graph = create_graph()
