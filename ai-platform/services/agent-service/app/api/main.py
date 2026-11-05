"""Internal chat API: core-service sends a citizen question, the agent graph answers it.

Only core-service calls it (the "FastAPI AI API" box in the GovAI block
diagram). Citizens reach it through core-service, which has already checked their
Keycloak login and role. Run it with:

    uvicorn app.api.main:app --host 127.0.0.1 --port 8030

Design:

- Factory: create_app() builds the FastAPI app. The graph and the settings
  are built once at startup from .env, or passed in by tests (dependency injection).
- The API calls the graph, not the Knowledge Agent, so a Coordinator Agent can
  be added inside the graph later without changing this contract.
- The API adds no answer logic; it turns HTTP into one graph run and the graph's
  result into the contract in app/api/schemas.py.
- Logs and error bodies never hold the question or the answer.

FastAPI reads request bodies from parameter type hints, so the endpoint
functions keep them; no other function here has type hints.
"""

import logging
import re
import uuid
from contextlib import asynccontextmanager

from fastapi import Depends, FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from langchain_core.messages import HumanMessage

from app.api.auth import require_service_token
from app.api.config import load_api_settings
from app.api.schemas import ChatRequest, ChatResponse, to_chat_response
from app.graph.workflow import FAILURE_UNAVAILABLE, create_graph

logger = logging.getLogger(__name__)

CHAT_PATH = "/api/v1/chat"
HEALTH_PATH = "/health"

REQUEST_ID_HEADER = "X-Request-ID"
REQUEST_ID_PATTERN = re.compile(r"^[A-Za-z0-9-]{1,64}$")

# Error bodies: short, fixed, and never holding the question
INVALID_REQUEST_DETAIL = 'The body must be {"message": "<question>"} with 1-4000 characters.'
UNAVAILABLE_DETAIL = "The GovAI assistant is unavailable."


# Public functions

def create_app(graph=None, settings=None):
    """Create the API. Tests pass a fake graph and settings; otherwise both are built from .env."""

    @asynccontextmanager
    async def lifespan(app):

        # Settings first: a missing token must stop startup before the LLM client is made
        app.state.settings = settings if settings is not None else load_api_settings()
        app.state.graph = graph if graph is not None else create_graph()
        yield

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

        try:
            # One graph run per question; earlier messages are not sent by core-service
            result = request.app.state.graph.invoke(
                {"messages": [HumanMessage(content=body.message)]},
                config={"metadata": {"request_id": request_id}},
            )

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
            response = to_chat_response(result["messages"][-1].content, result.get("knowledge_answer"))

        except ValueError as error:
            # e.g. an empty answer; Pydantic's message holds the answer, so log only the type
            logger.error("chat answer invalid", extra={"request_id": request_id, "error_type": type(error).__name__})
            return unavailable(request_id)

        logger.info(
            "chat request finished",
            extra={"request_id": request_id, "failure": result.get("failure"), "sources": len(response.sources)},
        )

        return JSONResponse(
            content=response.model_dump(exclude_none=True),
            headers={REQUEST_ID_HEADER: request_id},
        )

    return app


# Helper functions

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
