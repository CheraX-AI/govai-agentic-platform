"""Internal retrieval API: finds the published document chunks for a question.

Only agent-service calls it. Clients never do, so it has no CORS settings and
must listen on 127.0.0.1 only:

    uvicorn app.api.main:app --host 127.0.0.1 --port 8020

Design:

- Factory: create_app() builds the FastAPI app. The Retriever (the existing
  facade over NVIDIA embeddings and ChromaDB) is created once at startup from
  rag/.env, or passed in by tests (dependency injection).
- The API adds no search logic; it only turns HTTP into Retriever calls and
  RetrievalError subclasses into HTTP status codes.

FastAPI reads request bodies from parameter type hints, so the endpoint
functions keep them; no other function here has type hints.
"""

import logging
import re
import uuid
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from app.retrieval.exceptions import (
    InvalidQueryError,
    QueryEmbeddingError,
    RetrievalConfigError,
    RetrievalError,
    RetrievalUnavailableError,
)
from app.retrieval.models import RetrievalResponse
from app.retrieval.retriever import create_retriever
from app.vector_store.vector_store import DEFAULT_TOP_K

logger = logging.getLogger(__name__)

RETRIEVE_PATH = "/api/v1/retrieve"
HEALTH_PATH = "/health"

REQUEST_ID_HEADER = "X-Request-ID"
REQUEST_ID_PATTERN = re.compile(r"^[A-Za-z0-9-]{1,64}$")

# HTTP status for each retrieval error; any other RetrievalError is a 500
ERROR_STATUS_CODES = {
    InvalidQueryError: 422,
    QueryEmbeddingError: 503,
    RetrievalUnavailableError: 503,
    RetrievalConfigError: 500,
}


# Models

class RetrieveRequest(BaseModel):
    """A search request. The values are checked by the Retriever (SearchQuery), not here."""

    question: str
    top_k: int = DEFAULT_TOP_K
    department_id: str | None = None
    service_id: str | None = None
    max_distance: float | None = None


# Public functions

def create_app(retriever=None):
    """Create the API. Tests pass a fake retriever; otherwise one is built from rag/.env."""

    @asynccontextmanager
    async def lifespan(app):

        # Build the Retriever once, before the first request
        app.state.retriever = retriever if retriever is not None else create_retriever()
        yield

    app = FastAPI(title="GovAI Knowledge Service (internal)", lifespan=lifespan)
    app.add_exception_handler(RetrievalError, handle_retrieval_error)

    @app.get(HEALTH_PATH)
    def health():

        return {"status": "ok"}

    @app.post(RETRIEVE_PATH, response_model=RetrievalResponse)
    def retrieve(body: RetrieveRequest, request: Request):

        # Use the agent's request ID, so one question can be followed through both services
        request_id = read_request_id(request)

        # Search the published documents; a RetrievalError goes to handle_retrieval_error
        response = request.app.state.retriever.retrieve(
            body.question,
            top_k=body.top_k,
            department_id=body.department_id,
            service_id=body.service_id,
            max_distance=body.max_distance,
        )

        # Log the outcome without the question, which may hold personal details
        logger.info("retrieve request finished", extra={"request_id": request_id, "results": len(response.results)})

        return response

    return app


# Helper functions

def handle_retrieval_error(request, error):

    # Pick the status for the error type; the messages are short and never hold the question
    status_code = 500
    for error_class, code in ERROR_STATUS_CODES.items():
        if isinstance(error, error_class):
            status_code = code
            break

    # Log the failure without the question
    logger.warning(
        "retrieve request failed",
        extra={"request_id": read_request_id(request), "error_type": type(error).__name__},
    )

    response = JSONResponse(status_code=status_code, content={"detail": str(error)})

    return response


def read_request_id(request):

    # Accept only a safe ID from the caller; otherwise make a new one
    request_id = request.headers.get(REQUEST_ID_HEADER, "")
    if not REQUEST_ID_PATTERN.match(request_id):
        request_id = uuid.uuid4().hex

    return request_id


# The app uvicorn runs
app = create_app()
