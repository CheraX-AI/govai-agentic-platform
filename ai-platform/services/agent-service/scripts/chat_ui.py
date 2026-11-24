"""Chainlit chat with the GovAI agent graph.

Run from the agent-service folder, after starting the knowledge service:

    chainlit run scripts/chat_ui.py --port 8502

In Docker, the agent-chat service runs it (see ai-platform/docker-compose.ai.yml).

Each message goes through the same LangGraph graph as LangGraph Studio: the
Knowledge Agent retrieves the documents from the knowledge service (RAG) and
the Azure OpenAI model answers from them. With LANGSMITH_TRACING=true every question
is one LangSmith trace named "govai_chat".

The chat has no login yet; it is for development and testing, on 127.0.0.1 only.
Use only synthetic questions while LangSmith inputs and outputs are not hidden.
"""

import asyncio
import sys
import uuid
from functools import lru_cache
from pathlib import Path

# Chainlit runs this file directly, so add agent-service/ to the import path
SERVICE_ROOT = Path(__file__).resolve().parent.parent
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

import chainlit as cl
from dotenv import load_dotenv
from langchain_core.messages import HumanMessage

from app.graph.workflow import create_graph
from app.llm.exceptions import LLMError
from app.tools.knowledge_rag_client import KnowledgeServiceError

# Put the .env values, including the LANGSMITH_* settings, into the environment
load_dotenv(SERVICE_ROOT / ".env")

RUN_NAME = "govai_chat"
WELCOME_MESSAGE = "Welcome to GovAI! Ask a question about the government services in the official documents."


# Chat handlers

@cl.on_chat_start
async def start():

    await cl.Message(content=WELCOME_MESSAGE).send()


@cl.on_message
async def answer(message):

    try:
        # Build the graph once; a missing setting is reported here, by name
        graph = get_graph()

    except (KnowledgeServiceError, LLMError) as error:
        await cl.Message(content=f"The GovAI assistant is not configured: {error}").send()
        return

    # One request ID per question, so it can be followed through both services and LangSmith
    config = {"run_name": RUN_NAME, "metadata": {"request_id": uuid.uuid4().hex}}
    state = {"messages": [HumanMessage(content=message.content)]}

    # The graph call is blocking, so run it off the Chainlit event loop
    result = await asyncio.to_thread(graph.invoke, state, config)

    # Show the reply; it does not say which documents it came from
    reply = result["messages"][-1].content
    await cl.Message(content=reply).send()


# Helper functions

@lru_cache(maxsize=1)
def get_graph():

    # The same graph as LangGraph Studio, built from .env
    graph = create_graph()

    return graph
