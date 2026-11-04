"""Tests for app/retrieval/retriever.py with a fake NVIDIA client and a fake vector store.

No service is called: the fakes record what the Retriever sends and return
fixed answers, so the filters, result building, and error handling can be
checked exactly.
"""

import json

import pytest

from app.retrieval.exceptions import (
    InvalidQueryError,
    QueryEmbeddingError,
    RetrievalConfigError,
    RetrievalError,
    RetrievalUnavailableError,
)
from app.retrieval.retriever import Retriever, create_retriever, to_retrieved_chunks
from app.vector_store.exceptions import VectorStoreConnectionError, VectorStoreError
from app.vector_store.vector_store import EmbeddingInfo, SearchResult
from tests.helpers import DIMENSION, TEST_MODEL, make_env, make_fake_embedder, write_env_file

PUBLISHED_ONLY = {"publication_status": "published"}


# Fakes

class FakeVectorStore:
    """Answers like VectorStore and records each search."""

    def __init__(self, matches=(), info=None, error=None):

        self.matches = list(matches)
        self.info = info if info is not None else EmbeddingInfo(model=TEST_MODEL, dimension=DIMENSION)
        self.error = error
        self.searches = []

    def embedding_info(self):

        if self.error is not None:
            raise self.error

        return self.info

    def search(self, query_embedding, top_k=5, where=None):

        self.searches.append({"query_embedding": query_embedding, "top_k": top_k, "where": where})
        matches = self.matches[:top_k]

        return matches


class EmptyVectorStore(FakeVectorStore):

    def embedding_info(self):

        # An empty collection has no stored model
        return None


# Helper functions

def make_match(index, distance, **metadata):

    # A search result shaped like a stored TIN guide chunk
    chunk_metadata = {
        "document_id": "doc-a",
        "document_title": "Obtaining a Taxpayer Identification Number (TIN)",
        "source_file": "tin_guide.pdf",
        "page_number": index + 1,
        "department_id": "IRD",
        "service_id": "TIN_REGISTRATION",
        "document_type": "service_guideline",
        "publication_status": "published",
        "object_key": "documents/IRD/TIN_REGISTRATION/doc-a.pdf",
        "chunk_index": index,
        "embedding_model": TEST_MODEL,
    }
    chunk_metadata.update(metadata)

    match = SearchResult(
        chunk_id=f"doc-a:{index}",
        text=f"TIN registration step {index}.",
        metadata=chunk_metadata,
        distance=distance,
    )

    return match


def make_retriever(matches=(), embedder=None, vector_store=None):

    # A Retriever with fakes; nothing leaves the test
    if embedder is None:
        embedder = make_fake_embedder()
    if vector_store is None:
        vector_store = FakeVectorStore(matches)

    retriever = Retriever(embedder=embedder, vector_store=vector_store)

    return retriever


# Published-only filter

def test_search_without_ids_is_still_limited_to_published_documents():

    retriever = make_retriever()

    retriever.retrieve("How do I get a TIN?")

    assert retriever.vector_store.searches[0]["where"] == PUBLISHED_ONLY


def test_department_and_service_are_added_to_the_published_filter():

    retriever = make_retriever()

    retriever.retrieve("How do I get a TIN?", department_id="IRD", service_id="TIN_REGISTRATION")

    where = retriever.vector_store.searches[0]["where"]
    assert where == {
        "$and": [
            PUBLISHED_ONLY,
            {"department_id": "IRD"},
            {"service_id": "TIN_REGISTRATION"},
        ]
    }


# Query embedding and search options

def test_cleaned_question_is_embedded_and_its_vector_searched():

    embedder = make_fake_embedder()
    retriever = make_retriever(embedder=embedder)

    retriever.retrieve("  How do I\n get a TIN?  ")

    assert embedder.embedding_model.query_calls == ["How do I get a TIN?"]
    assert retriever.vector_store.searches[0]["query_embedding"] == embedder.embedding_model.vector


def test_top_k_is_passed_to_the_search():

    retriever = make_retriever()

    retriever.retrieve("How do I get a TIN?", top_k=7)

    assert retriever.vector_store.searches[0]["top_k"] == 7


# Results

def test_results_carry_the_text_ids_and_source():

    retriever = make_retriever([make_match(0, 0.2)])

    result = retriever.retrieve("How do I get a TIN?").results[0]

    assert result.rank == 1
    assert result.chunk_id == "doc-a:0"
    assert result.document_id == "doc-a"
    assert result.text == "TIN registration step 0."
    assert result.document_title == "Obtaining a Taxpayer Identification Number (TIN)"
    assert result.source_file == "tin_guide.pdf"
    assert result.page_number == 1
    assert result.department_id == "IRD"
    assert result.service_id == "TIN_REGISTRATION"
    assert result.document_type == "service_guideline"
    assert result.object_key == "documents/IRD/TIN_REGISTRATION/doc-a.pdf"


def test_distance_and_similarity_are_returned():

    retriever = make_retriever([make_match(0, 0.25)])

    result = retriever.retrieve("How do I get a TIN?").results[0]

    assert result.distance == pytest.approx(0.25)
    assert result.similarity == pytest.approx(0.75)


def test_results_keep_the_search_order_and_are_numbered():

    retriever = make_retriever([make_match(0, 0.1), make_match(1, 0.3), make_match(2, 0.5)])

    results = retriever.retrieve("How do I get a TIN?").results

    assert [result.chunk_id for result in results] == ["doc-a:0", "doc-a:1", "doc-a:2"]
    assert [result.rank for result in results] == [1, 2, 3]


def test_max_distance_drops_weak_matches_and_renumbers():

    retriever = make_retriever([make_match(0, 0.1), make_match(1, 0.6), make_match(2, 0.3)])

    results = retriever.retrieve("How do I get a TIN?", max_distance=0.4).results

    assert [result.chunk_id for result in results] == ["doc-a:0", "doc-a:2"]
    assert [result.rank for result in results] == [1, 2]


def test_department_guideline_result_has_no_service():

    match = make_match(0, 0.2, document_type="department_guideline")
    del match.metadata["service_id"]

    result = to_retrieved_chunks([match])[0]

    assert result.service_id is None
    assert result.document_type == "department_guideline"


def test_response_records_the_question_and_options():

    retriever = make_retriever([make_match(0, 0.2)])

    response = retriever.retrieve(
        " How do I get a TIN? ",
        top_k=3,
        department_id="IRD",
        service_id="TIN_REGISTRATION",
        max_distance=0.8,
    )

    assert response.question == "How do I get a TIN?"
    assert response.top_k == 3
    assert response.department_id == "IRD"
    assert response.service_id == "TIN_REGISTRATION"
    assert response.max_distance == 0.8
    assert response.embedding_model == TEST_MODEL


def test_response_can_be_sent_as_json():

    # The Agent Orchestrator will receive the response as JSON
    retriever = make_retriever([make_match(0, 0.2)])

    data = json.loads(retriever.retrieve("How do I get a TIN?").model_dump_json())

    assert data["results"][0]["chunk_id"] == "doc-a:0"
    assert data["results"][0]["page_number"] == 1


# Empty results

def test_no_match_is_an_empty_result_not_an_error():

    retriever = make_retriever(matches=[])

    response = retriever.retrieve("How do I get a TIN?")

    assert response.results == []


def test_empty_collection_returns_no_results_without_calling_nvidia():

    embedder = make_fake_embedder()
    retriever = make_retriever(embedder=embedder, vector_store=EmptyVectorStore())

    response = retriever.retrieve("How do I get a TIN?")

    assert response.results == []
    assert embedder.embedding_model.query_calls == []
    assert retriever.vector_store.searches == []


# Invalid input

def test_empty_question_is_rejected_before_any_call():

    embedder = make_fake_embedder()
    retriever = make_retriever(embedder=embedder)

    with pytest.raises(InvalidQueryError, match="empty"):
        retriever.retrieve("   ")

    assert embedder.embedding_model.query_calls == []
    assert retriever.vector_store.searches == []


def test_invalid_top_k_is_rejected_before_any_call():

    retriever = make_retriever()

    with pytest.raises(InvalidQueryError, match="top_k"):
        retriever.retrieve("How do I get a TIN?", top_k=0)

    assert retriever.vector_store.searches == []


# Model checks

def test_documents_from_another_model_are_rejected():

    vector_store = FakeVectorStore(info=EmbeddingInfo(model="nvidia/other-model", dimension=DIMENSION))
    retriever = make_retriever(vector_store=vector_store)

    with pytest.raises(RetrievalConfigError, match="nvidia/other-model"):
        retriever.retrieve("How do I get a TIN?")

    assert vector_store.searches == []


def test_query_vector_of_another_size_is_rejected():

    vector_store = FakeVectorStore(info=EmbeddingInfo(model=TEST_MODEL, dimension=DIMENSION * 2))
    retriever = make_retriever(vector_store=vector_store)

    with pytest.raises(RetrievalConfigError, match="dimensions"):
        retriever.retrieve("How do I get a TIN?")

    assert vector_store.searches == []


# NVIDIA errors

def test_rejected_key_is_a_query_embedding_error():

    embedder = make_fake_embedder(errors=[Exception("[401] Unauthorized")])
    retriever = make_retriever(embedder=embedder)

    with pytest.raises(QueryEmbeddingError, match="rejected the API key"):
        retriever.retrieve("How do I get a TIN?")


def test_nvidia_outage_is_a_query_embedding_error():

    errors = [Exception("[503] Service Unavailable")] * 3
    embedder = make_fake_embedder(errors=errors, max_retries=2)
    retriever = make_retriever(embedder=embedder)

    with pytest.raises(QueryEmbeddingError, match="503"):
        retriever.retrieve("How do I get a TIN?")

    assert retriever.vector_store.searches == []


def test_unusable_vector_is_a_query_embedding_error():

    embedder = make_fake_embedder(vector=[float("nan")] * DIMENSION)
    retriever = make_retriever(embedder=embedder)

    with pytest.raises(QueryEmbeddingError, match="unusable vector"):
        retriever.retrieve("How do I get a TIN?")


# ChromaDB errors

def test_stopped_database_is_unavailable():

    vector_store = FakeVectorStore(error=VectorStoreConnectionError("ChromaDB is not reachable"))
    retriever = make_retriever(vector_store=vector_store)

    with pytest.raises(RetrievalUnavailableError, match="not reachable"):
        retriever.retrieve("How do I get a TIN?")


def test_failed_chroma_request_is_unavailable():

    vector_store = FakeVectorStore(error=VectorStoreError("ChromaDB request failed: NotFoundError"))
    retriever = make_retriever(vector_store=vector_store)

    with pytest.raises(RetrievalUnavailableError, match="NotFoundError"):
        retriever.retrieve("How do I get a TIN?")


def test_every_failure_shares_one_base_class():

    # The orchestrator can catch RetrievalError for any retrieval failure
    for error_class in [InvalidQueryError, QueryEmbeddingError, RetrievalConfigError, RetrievalUnavailableError]:
        assert issubclass(error_class, RetrievalError)


# Factory

def test_missing_chromadb_settings_are_a_config_error(tmp_path, monkeypatch):

    # Valid NVIDIA settings, but no ChromaDB settings
    for name in ["CHROMA_HOST", "CHROMA_PORT", "CHROMA_COLLECTION"]:
        monkeypatch.delenv(name, raising=False)
    env_file = write_env_file(tmp_path / ".env", make_env())

    with pytest.raises(RetrievalConfigError, match="CHROMA_HOST"):
        create_retriever(env_file=env_file)


def test_missing_embedding_settings_are_a_config_error(tmp_path, monkeypatch):

    for name in make_env():
        monkeypatch.delenv(name, raising=False)
    env_file = write_env_file(tmp_path / ".env", {"CHROMA_HOST": "localhost"})

    with pytest.raises(RetrievalConfigError, match="embedding"):
        create_retriever(env_file=env_file)


def test_unreachable_database_is_unavailable(tmp_path, monkeypatch):

    # Nothing listens on port 1, so no running ChromaDB is needed
    for name in list(make_env()) + ["CHROMA_HOST", "CHROMA_PORT", "CHROMA_COLLECTION"]:
        monkeypatch.delenv(name, raising=False)
    values = make_env(CHROMA_HOST="127.0.0.1", CHROMA_PORT="1", CHROMA_COLLECTION="govai_knowledge")
    env_file = write_env_file(tmp_path / ".env", values)

    with pytest.raises(RetrievalUnavailableError, match="not reachable"):
        create_retriever(env_file=env_file)
