"""Internal agent API that core-service calls: chat, conversation history, and GET /health."""

import logging
from contextlib import asynccontextmanager

from fastapi import Depends, FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from langchain_core.messages import HumanMessage

from app.agents.knowledge_agent.agent import create_knowledge_agent
from app.api.auth import require_citizen
from app.api.config import load_api_settings
from app.api.conversations import router as conversations_router
from app.api.responses import busy, json_response, not_found, read_request_id, unavailable
from app.api.schemas import ChatRequest, ChatResponse, to_chat_response
from app.conversations.models import ConversationBusyError, ConversationNotFoundError
from app.conversations.service import ConversationService
from app.conversations.store import open_conversation_store
from app.graph.constants import FAILURE_UNAVAILABLE
from app.graph.workflow import build_graph
from app.memory.checkpointer import close_checkpointer, open_checkpointer
from app.memory.config import load_memory_settings
from app.memory.encryption import build_key_ring, create_serializer

logger = logging.getLogger(__name__)

CHAT_PATH = "/api/v1/chat"
HEALTH_PATH = "/health"

# Error bodies: short, fixed, and never holding the question
INVALID_REQUEST_DETAIL = (
    'The body must be {"message": "<question>"} with 1-4000 characters, '
    'and an optional "conversation_id" (a lowercase UUID).'
)
INVALID_QUERY_DETAIL = "The request is invalid."


# Public functions

def create_app(graph=None, settings=None, memory_graph=None, conversations=None):
    """Create the API. Tests pass fakes; otherwise everything is built from .env.

    graph answers without memory. memory_graph has the PostgreSQL checkpointer, and
    conversations is the ConversationService; both are None when memory is off (then
    nothing is stored and no conversation ID is returned).
    """

    @asynccontextmanager
    async def lifespan(app):

        # Settings first: a missing token must stop startup before the LLM client is made
        app.state.settings = settings if settings is not None else load_api_settings()

        # Tests pass their graphs; otherwise build them, with memory when it is configured
        checkpointer = None
        if graph is not None:
            app.state.graph, app.state.memory_graph, app.state.conversations = graph, memory_graph, conversations
        else:
            app.state.graph, app.state.memory_graph, app.state.conversations, checkpointer = build_graphs()

        yield

        # Close both database pools
        if app.state.conversations is not None:
            app.state.conversations.close()
        if checkpointer is not None:
            close_checkpointer(checkpointer)

    # No interactive docs: the API is internal and its contract is in schemas.py
    app = FastAPI(
        title="GovAI Agent API (internal)",
        lifespan=lifespan,
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
    )
    app.add_exception_handler(RequestValidationError, handle_invalid_request)
    app.include_router(conversations_router)

    @app.get(HEALTH_PATH)
    def health():

        return {"status": "ok"}

    @app.post(CHAT_PATH, response_model=ChatResponse, response_model_exclude_none=True)
    def chat(body: ChatRequest, request: Request, subject: str = Depends(require_citizen)):

        # Use core-service's request ID, so one question can be followed through every service
        request_id = read_request_id(request)
        metadata = {"request_id": request_id}

        # Memory off: answer the question alone; nothing is stored and no ID is returned
        if request.app.state.conversations is None:
            response = answer_without_memory(request.app.state, body, metadata)
        else:
            response = answer_in_conversation(request.app.state, body, subject, metadata)

        return response

    return app


# Helper functions

def build_graphs():

    # One Knowledge Agent shared by both graphs
    knowledge_agent = create_knowledge_agent()
    graph = build_graph(knowledge_agent)

    # Memory is off when AGENT_MEMORY_DATABASE_URL is blank
    settings = load_memory_settings()
    if settings is None:
        logger.info("conversation memory is off")
        return graph, None, None, None

    # An unreachable or unprepared memory database stops startup
    ring = build_key_ring(settings)
    store = open_conversation_store(settings, ring)
    try:
        checkpointer = open_checkpointer(settings, create_serializer(ring))
    except Exception:
        store.close()
        raise

    memory_graph = build_graph(knowledge_agent, checkpointer, settings.max_turns)
    conversations = ConversationService(
        store,
        checkpointer,
        settings.max_turns,
        settings.in_flight_seconds,
        settings.replay_seconds,
    )
    logger.info("conversation memory is on", extra={"max_turns": settings.max_turns})

    return graph, memory_graph, conversations, checkpointer


def answer_without_memory(state, body, metadata):

    request_id = metadata["request_id"]

    try:
        result = state.graph.invoke({"messages": [HumanMessage(content=body.message)]}, config={"metadata": metadata})
    except Exception as error:
        log_graph_failure(request_id, error)
        return unavailable(request_id)

    reply = build_reply(result, None, request_id)
    if reply is None:
        return unavailable(request_id)

    log_finished(request_id, result)
    response = json_response(reply.model_dump(exclude_none=True), request_id)

    return response


def answer_in_conversation(state, body, subject, metadata):

    request_id = metadata["request_id"]
    conversations = state.conversations

    try:
        # Save the question as pending before the model is asked (a new conversation when no ID was sent)
        started = conversations.start(subject, body.conversation_id, body.message)

    except ConversationNotFoundError:
        return not_found(request_id)

    except ConversationBusyError:
        return busy(request_id)

    except Exception as error:
        logger.warning("conversation storage unavailable", extra={"request_id": request_id, "error_type": type(error).__name__})
        return unavailable(request_id)

    # A quick repeat of an answered question gets the stored answer; the model is not asked again
    if started.replayed_answer is not None:
        reply = ChatResponse(answer=started.replayed_answer, conversation_id=started.conversation_id)
        response = json_response(reply.model_dump(exclude_none=True), request_id)
        return response

    metadata["conversation_id"] = started.conversation_id

    try:
        result = run_in_conversation(state, started, body.message, metadata)
    except Exception as error:
        log_graph_failure(request_id, error)
        conversations.fail(started)
        return unavailable(request_id)

    reply = build_reply(result, started.conversation_id, request_id)
    if reply is None:
        conversations.fail(started)
        return unavailable(request_id)

    # Advice for an empty or too long question is shown but not kept, like a failed turn
    if result.get("failure") is not None:
        conversations.fail(started)
    else:
        try:
            # No answer is returned before it is saved
            conversations.complete(started, reply.answer)
        except Exception as error:
            logger.warning("conversation answer not saved", extra={"request_id": request_id, "error_type": type(error).__name__})
            return unavailable(request_id)

    log_finished(request_id, result)
    response = json_response(reply.model_dump(exclude_none=True), request_id)

    return response


def run_in_conversation(state, started, question, metadata):

    # The checkpoint holds the earlier turns; when it is missing or out of date PostgreSQL restores them
    conversations = state.conversations
    messages = conversations.earlier_messages(started) + [HumanMessage(content=question)]

    # One thread per conversation; the checkpoint is saved once, when the run ends, tagged with its turn
    config = {
        "metadata": {**metadata, **conversations.run_metadata(started)},
        "configurable": {"thread_id": started.conversation_id},
    }
    result = state.memory_graph.invoke({"messages": messages}, config=config, durability="exit")

    return result


def build_reply(result, conversation_id, request_id):

    # The RAG or the LLM failed: a status code, so core-service does not show it as an answer
    if result.get("failure") == FAILURE_UNAVAILABLE:
        logger.warning("chat answer unavailable", extra={"request_id": request_id})
        return None

    try:
        # An answer, or advice on how to ask (an empty or too long question)
        reply = to_chat_response(reply_text(result), conversation_id)

    except ValueError as error:
        # e.g. an empty answer; Pydantic's message holds the answer, so log only the type
        logger.error("chat answer invalid", extra={"request_id": request_id, "error_type": type(error).__name__})
        return None

    return reply


def reply_text(result):

    # The reply message's text, or None when the graph made no reply
    reply = result.get("reply")
    text = reply.content if reply is not None else None

    return text


def log_graph_failure(request_id, error):

    # The graph handles agent errors itself, so this is a bug or a broken dependency
    logger.error("chat graph failed", extra={"request_id": request_id, "error_type": type(error).__name__})


def log_finished(request_id, result):

    logger.info("chat request finished", extra={"request_id": request_id, "failure": result.get("failure")})


def handle_invalid_request(request, error):

    # FastAPI's default 422 body repeats the input, which is the citizen's question
    logger.warning("request rejected", extra={"request_id": read_request_id(request), "errors": len(error.errors())})

    detail = INVALID_REQUEST_DETAIL if request.url.path == CHAT_PATH else INVALID_QUERY_DETAIL
    response = JSONResponse(status_code=422, content={"detail": detail})

    return response


# The app uvicorn runs; the graph and settings are built when it starts, not on import
app = create_app()
