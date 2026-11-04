"""LangGraph entry point, loaded by langgraph.json (`langgraph dev` and LangGraph Studio).

The graph is defined in app/graph/workflow.py and built here from .env. The
knowledge service is only called when a question is asked, not at startup.
"""

from app.graph.workflow import create_graph

# Compiled graph loaded by langgraph.json
graph = create_graph()
