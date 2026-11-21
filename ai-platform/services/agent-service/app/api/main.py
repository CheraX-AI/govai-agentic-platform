"""Internal chat API that core-service calls: POST /api/v1/chat and GET /health."""

import logging
import re
import uuid
from contextlib import asynccontextmanager

from fastapi import Depends, FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from langchain_core.messages import HumanMessage

from app.agents.knowledge_agent.agent import create_knowledge_agent
from app.api.auth import require_service_token
from app.api.config import load_api_settings
from app.api.schemas import ChatRequest, ChatResponse, to_chat_response
from app.graph.constants import FAILURE_UNAVAILABLE
from app.graph.workflow import build_graph
from app.memory.checkpointer import open_memory
from app.memory.config import load_memory_settings
from app.memory.encryption import MemoryDataError

logger = logging.getLogger(__name__)

CHAT_PATH = "/api/v1/chat"
HEALTH_PATH = "/health"

REQUEST_ID_HEADER = "X-Request-ID"
REQUEST_ID_PATTERN = re.compile(r"^[A-Za-z0-9-]{1,64}$")

# Error bodies: short, fixed, and never holding the question
INVALID_REQUEST_DETAIL = (
    'The body must be {"message": "<question>"} with 1-4000 characters, '
    'and an optional "conversation_id" (a lowercase UUID).'
)
UNAVAILABLE_DETAIL = "The GovAI assistant is unavailable."


# Public functions

def create_app(graph=None, settings=None, memory_graph=None, memory=None):
    """Create the API. Tests pass fakes; otherwise everything is built from .env.

    graph answers without memory. memory_graph (with memory, its ConversationMemory)
    answers requests that carry a conversation_id; both are None when memory is off.
    """

    @asynccontextmanager
    async def lifespan(app):

        # Settings first: a missing token must stop startup before the LLM client is made
        app.state.settings = settings if settings is not None else load_api_settings()

        # Tests pass their graphs; otherwise build them, with memory when it is configured
        if graph is not None:
            app.state.graph, app.state.memory_graph, app.state.memory = graph, memory_graph, memory
        else:
            app.state.graph, app.state.memory_graph, app.state.memory = build_graphs()

        yield

        # Close the memory database connections
        if app.state.memory is not None:
            app.state.memory.close()

    # No interactive docs: the API is internal and its contract is in schemas.py
    app = FastAPI(
        title="GovAI Agent API (internal)",
        lifespan=lifespan,
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
    )
    app.add_exception_handler(RequestValidationError, handle_invalid_request)

    @app.get(HEALTH_PATH)
    def health():

        return {"status": "ok"}

    @app.post(
        CHAT_PATH,
        response_model=ChatResponse,
        response_model_exclude_none=True,
        dependencies=[Depends(require_service_token)],
    )
    def chat(body: ChatRequest, request: Request):

        # Use core-service's request ID, so one question can be followed through every service
        request_id = read_request_id(request)

        # Run metadata for logs and traces
        metadata = {"request_id": request_id}
        if body.conversation_id:
            metadata["conversation_id"] = body.conversation_id

        try:
            # One graph run per question; the memory graph adds the earlier turns itself
            result = run_graph(request.app.state, body, metadata)

        except Exception as error:
            # The graph handles agent errors itself, so this is a bug or a broken dependency
            logger.error("chat graph failed", extra={"request_id": request_id, "error_type": type(error).__name__})
            return unavailable(request_id)

        # The RAG or the LLM failed: a status code, so core-service does not show it as an answer
        if result.get("failure") == FAILURE_UNAVAILABLE:
            logger.warning("chat answer unavailable", extra={"request_id": request_id})
            return unavailable(request_id)

        try:
            # An answer, or advice on how to ask (an empty or too long question)
            # Return the same conversation ID core-service sent; the agent never makes one
            response = to_chat_response(result.get("reply"), body.conversation_id)

        except ValueError as error:
            # e.g. an empty answer; Pydantic's message holds the answer, so log only the type
            logger.error("chat answer invalid", extra={"request_id": request_id, "error_type": type(error).__name__})
            return unavailable(request_id)

        logger.info(
            "chat request finished",
            extra={"request_id": request_id, "failure": result.get("failure")},
        )

        return JSONResponse(
            content=response.model_dump(exclude_none=True),
            headers={REQUEST_ID_HEADER: request_id},
        )

    return app


# Helper functions

def build_graphs():

    # One Knowledge Agent shared by both graphs
    knowledge_agent = create_knowledge_agent()
    graph = build_graph(knowledge_agent)

    # Memory is off when AGENT_MEMORY_DATABASE_URL is blank
    memory_settings = load_memory_settings()
    if memory_settings is None:
        logger.info("conversation memory is off")
        return graph, None, None

    # An unreachable or unprepared memory database stops startup
    memory = open_memory(memory_settings)
    memory_graph = build_graph(knowledge_agent, memory.checkpointer, memory_settings.max_turns)
    logger.info("conversation memory is on", extra={"max_turns": memory_settings.max_turns})

    return graph, memory_graph, memory


def run_graph(state, body, metadata):

    # The question alone, without a conversation ID or without memory
    question = {"messages": [HumanMessage(content=body.message)]}
    if not body.conversation_id or state.memory_graph is None:
        result = state.graph.invoke(question, config={"metadata": metadata})
        return result

    # One memory thread per conversation; saved once, when the run ends
    config = {"metadata": metadata, "configurable": {"thread_id": body.conversation_id}}
    try:
        result = state.memory_graph.invoke(question, config=config, durability="exit")

    except MemoryDataError:
        # Memory saved with another key cannot be read: forget it and answer once more without it
        logger.warning("conversation memory unreadable, reset", extra={"request_id": metadata["request_id"]})
        state.memory.forget(body.conversation_id)
        result = state.memory_graph.invoke(question, config=config, durability="exit")

    compact_memory(state.memory, body.conversation_id, metadata["request_id"])

    return result


def compact_memory(memory, conversation_id, request_id):

    try:
        # Delete the older checkpoints; the answer is already saved, so a failure only wastes space
        memory.compact(conversation_id)

    except Exception as error:
        logger.warning("conversation memory not compacted", extra={"request_id": request_id, "error_type": type(error).__name__})


def handle_invalid_request(request, error):

    # FastAPI's default 422 body repeats the input, which is the citizen's question
    logger.warning("chat request rejected", extra={"request_id": read_request_id(request), "errors": len(error.errors())})

    response = JSONResponse(status_code=422, content={"detail": INVALID_REQUEST_DETAIL})

    return response


def unavailable(request_id):

    # 503: core-service shows the citizen "Chat temporarily unavailable"
    response = JSONResponse(
        status_code=503,
        content={"detail": UNAVAILABLE_DETAIL},
        headers={REQUEST_ID_HEADER: request_id},
    )

    return response


def read_request_id(request):

    # Accept only a safe ID from the caller; otherwise make a new one
    request_id = request.headers.get(REQUEST_ID_HEADER, "")
    if not REQUEST_ID_PATTERN.match(request_id):
        request_id = uuid.uuid4().hex

    return request_id


# The app uvicorn runs; the graph and settings are built when it starts, not on import
app = create_app()
