"""Tests for app/api/main.py with a fake Retriever; no service is called."""

import pytest
from fastapi.testclient import TestClient

from app.api.main import create_app
from app.retrieval.exceptions import (
    InvalidQueryError,
    QueryEmbeddingError,
    RetrievalConfigError,
    RetrievalError,
    RetrievalUnavailableError,
)
from app.retrieval.models import RetrievalResponse, RetrievedChunk


# Fakes

class FakeRetriever:
    """Answers like Retriever.retrieve and records each call."""

    def __init__(self, error=None):

        self.error = error
        self.calls = []

    def retrieve(self, question, top_k=5, department_id=None, service_id=None, max_distance=None):

        self.calls.append(
            {
                "question": question,
                "top_k": top_k,
                "department_id": department_id,
                "service_id": service_id,
                "max_distance": max_distance,
            }
        )
        if self.error is not None:
            raise self.error

        chunk = RetrievedChunk(
            rank=1,
            chunk_id="doc:0",
            document_id="doc",
            text="Synthetic chunk text.",
            document_title="Synthetic guide",
            source_file="synthetic.pdf",
            page_number=2,
            distance=0.2,
            similarity=0.8,
        )
        response = RetrievalResponse(
            question=question,
            top_k=top_k,
            department_id=department_id,
            service_id=service_id,
            embedding_model="test-model",
            results=[chunk],
        )

        return response


def make_client(retriever):

    # Entering the client runs the startup, which stores the fake retriever
    client = TestClient(create_app(retriever=retriever))

    return client


# Requests

def test_health():

    with make_client(FakeRetriever()) as client:
        response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_retrieve_returns_the_results():

    retriever = FakeRetriever()

    with make_client(retriever) as client:
        response = client.post("/api/v1/retrieve", json={"question": "Synthetic question?", "top_k": 3})

    assert response.status_code == 200
    body = response.json()
    assert body["results"][0]["chunk_id"] == "doc:0"
    assert body["results"][0]["page_number"] == 2


def test_retrieve_passes_every_option_to_the_retriever():

    retriever = FakeRetriever()
    options = {
        "question": "Synthetic question?",
        "top_k": 3,
        "department_id": "TEST",
        "service_id": "TEST_SERVICE",
        "max_distance": 0.5,
    }

    with make_client(retriever) as client:
        client.post("/api/v1/retrieve", json=options)

    assert retriever.calls == [options]


def test_missing_options_use_the_retriever_defaults():

    retriever = FakeRetriever()

    with make_client(retriever) as client:
        client.post("/api/v1/retrieve", json={"question": "Synthetic question?"})

    assert retriever.calls[0]["top_k"] == 5
    assert retriever.calls[0]["department_id"] is None
    assert retriever.calls[0]["service_id"] is None


def test_missing_question_is_rejected():

    with make_client(FakeRetriever()) as client:
        response = client.post("/api/v1/retrieve", json={"top_k": 3})

    assert response.status_code == 422


# Errors

@pytest.mark.parametrize(
    "error, status_code",
    [
        (InvalidQueryError("question is empty"), 422),
        (QueryEmbeddingError("NVIDIA rejected the API key"), 503),
        (RetrievalUnavailableError("ChromaDB is not reachable"), 503),
        (RetrievalConfigError("Invalid ChromaDB settings"), 500),
        (RetrievalError("Something else"), 500),
    ],
)
def test_retrieval_errors_become_status_codes(error, status_code):

    with make_client(FakeRetriever(error=error)) as client:
        response = client.post("/api/v1/retrieve", json={"question": "Synthetic question?"})

    assert response.status_code == status_code
    assert response.json() == {"detail": str(error)}
