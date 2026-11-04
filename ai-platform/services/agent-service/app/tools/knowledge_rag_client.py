"""Client for the knowledge-service retrieval API: the agent's only way to reach the RAG.

The knowledge service is a separate service with an internal API that only
agent-service calls. This client sends the question there and reads the
retrieved chunks back as KnowledgeSource objects.

Design:

- Adapter: KnowledgeRagClient.retrieve() hides HTTP, JSON, and status codes
  behind one method that returns KnowledgeSource objects or raises a
  KnowledgeServiceError subclass.
- Factory: create_rag_client() reads the settings from .env.
- Dependency injection: tests pass an httpx client with a fake transport.

Each call is traced in LangSmith as a "retriever" step inside the agent's trace.
"""

import os
from pathlib import Path

import httpx
from dotenv import dotenv_values
from langsmith import traceable
from pydantic import BaseModel, Field, ValidationError

from app.agents.knowledge_agent.schemas import KnowledgeSource
from app.llm.exceptions import format_validation_error

ENV_FILE = Path(__file__).resolve().parents[2] / ".env"

RETRIEVE_PATH = "/api/v1/retrieve"
REQUEST_ID_HEADER = "X-Request-ID"

# Limits the .env values are checked against (the knowledge service allows top_k up to 50)
MAX_TOP_K = 50
MAX_TIMEOUT_SECONDS = 120
MAX_ERROR_DETAIL_LENGTH = 200

# Every setting must be present in .env (or the environment)
REQUIRED_VARIABLES = [
    "KNOWLEDGE_SERVICE_URL",
    "KNOWLEDGE_TOP_K",
    "KNOWLEDGE_TIMEOUT_SECONDS",
]


# Errors

class KnowledgeServiceError(Exception):
    """Base class for knowledge service errors."""


class KnowledgeServiceConfigError(KnowledgeServiceError):
    """The knowledge service settings are missing or invalid."""


class KnowledgeServiceUnavailableError(KnowledgeServiceError):
    """The knowledge service is not reachable, timed out, or could not search."""


class KnowledgeServiceRequestError(KnowledgeServiceError):
    """The knowledge service rejected the request, e.g. a question that is too short."""


# Models

class RagClientSettings(BaseModel):
    """Knowledge service settings. Each field's alias is its .env name."""

    base_url: str = Field(alias="KNOWLEDGE_SERVICE_URL", pattern=r"^https?://\S+$")
    top_k: int = Field(alias="KNOWLEDGE_TOP_K", ge=1, le=MAX_TOP_K)
    timeout_seconds: float = Field(alias="KNOWLEDGE_TIMEOUT_SECONDS", gt=0, le=MAX_TIMEOUT_SECONDS, allow_inf_nan=False)


# Adapter

class KnowledgeRagClient:
    """Calls the knowledge-service retrieval API; one httpx client is reused for every request."""

    def __init__(self, settings, http_client=None):

        self.settings = settings
        self.http_client = http_client or httpx.Client(
            base_url=settings.base_url.rstrip("/"),
            timeout=settings.timeout_seconds,
        )

    @traceable(run_type="retriever", name="knowledge_rag_retrieve", process_inputs=lambda inputs: trace_inputs(inputs))
    def retrieve(self, question, department_id=None, service_id=None, request_id=None):
        """Return the chunks that best match the question, best first.

        No match is a normal result with an empty list. Every failure raises a
        KnowledgeServiceError subclass.
        """

        # The search request; no department or service searches every published document
        body = {
            "question": question,
            "top_k": self.settings.top_k,
            "department_id": department_id,
            "service_id": service_id,
        }

        # Pass the request ID on, so one question can be followed through both services
        headers = {}
        if request_id:
            headers[REQUEST_ID_HEADER] = request_id

        try:
            # Send the question to the knowledge service
            response = self.http_client.post(RETRIEVE_PATH, json=body, headers=headers)

        except httpx.TimeoutException:
            raise KnowledgeServiceUnavailableError("Knowledge service did not respond in time")

        except httpx.HTTPError:
            raise KnowledgeServiceUnavailableError("Knowledge service is not reachable")

        # Turn an error status into a knowledge service error
        check_response(response)

        # Read the results into the agent's own model
        sources = read_sources(response)

        return sources


# Public functions

def load_rag_client_settings(environ=None, env_file=ENV_FILE):
    """Build RagClientSettings from the .env file.

    When environ is given (as in tests), only that mapping is used and no
    file is read.
    """

    # Read the .env file; real environment variables take priority over it
    if environ is None:
        environ = {**dotenv_values(env_file), **os.environ}

    # Every setting must be present and not blank
    values = {}
    for name in REQUIRED_VARIABLES:
        value = (environ.get(name) or "").strip()
        if not value:
            raise KnowledgeServiceConfigError(f"{name} is required in .env")
        values[name] = value

    try:
        # Check the values and build the settings
        settings = RagClientSettings.model_validate(values)

    except ValidationError as error:
        raise KnowledgeServiceConfigError(format_validation_error(error)) from None

    return settings


def create_rag_client(env_file=ENV_FILE):
    """Read the knowledge service settings from .env and create the client."""

    # Read and check the settings
    settings = load_rag_client_settings(env_file=env_file)

    # Create the client; no request is sent yet
    client = KnowledgeRagClient(settings)

    return client


# Helper functions

def trace_inputs(inputs):

    # Leave the client object out of the LangSmith trace; keep the question and filters
    traced = {}
    for name, value in inputs.items():
        if name != "self":
            traced[name] = value

    return traced


def check_response(response):

    # Success needs no action
    if response.is_success:
        return

    # The knowledge service sends a short reason in "detail"
    detail = error_detail(response)

    # 4xx: the request itself was wrong, e.g. an invalid question
    if 400 <= response.status_code < 500:
        raise KnowledgeServiceRequestError(f"Knowledge service rejected the request: {detail}")

    # 5xx: the knowledge service, ChromaDB, or NVIDIA embeddings failed
    raise KnowledgeServiceUnavailableError(f"Knowledge service could not search: {detail}")


def read_sources(response):

    try:
        # The response holds the matches in "results", best first
        results = response.json()["results"]

        sources = []
        for result in results:
            sources.append(KnowledgeSource.model_validate(result))

    except (ValueError, KeyError, TypeError, ValidationError):
        raise KnowledgeServiceError("Knowledge service returned an invalid response")

    return sources


def error_detail(response):

    try:
        # FastAPI errors are {"detail": "..."}; validation errors hold a list
        detail = response.json().get("detail")

    except (ValueError, AttributeError):
        detail = None

    # Fall back to the HTTP status
    if not isinstance(detail, str) or not detail:
        detail = f"HTTP {response.status_code}"

    # Keep the message short
    detail = detail[:MAX_ERROR_DETAIL_LENGTH]

    return detail
