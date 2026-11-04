"""Tests for app/vector_store/vector_store.py that need no running ChromaDB:
filters, result conversion, input checks, and error mapping."""

import httpx
import pytest
from chromadb.errors import NotFoundError

from app.embeddings.models import EmbeddingFile
from app.vector_store.config import VectorStoreSettings
from app.vector_store.exceptions import EmbeddingMismatchError, VectorStoreConnectionError, VectorStoreError
from app.vector_store.vector_store import (
    MAX_TOP_K,
    VectorStore,
    chunk_order,
    create_vector_store,
    metadata_filter,
    old_version_filter,
    run_chroma,
    to_stored_chunks,
)

SERVICE_METADATA = {
    "document_id": "new-doc",
    "department_id": "IRD",
    "service_id": "TIN_REGISTRATION",
    "document_type": "service_guideline",
    "source_file": "tin_guide.pdf",
}

DEPARTMENT_METADATA = {
    "document_id": "new-doc",
    "department_id": "IRD",
    "document_type": "department_guideline",
    "source_file": "ird_guide.pdf",
}


# Search filters

def test_no_ids_means_no_filter():

    assert metadata_filter() is None


def test_one_id_is_a_single_condition():

    assert metadata_filter(department_id="IRD") == {"department_id": "IRD"}


def test_two_ids_are_joined_with_and():

    where = metadata_filter(department_id="IRD", service_id="TIN_REGISTRATION")

    assert where == {"$and": [{"department_id": "IRD"}, {"service_id": "TIN_REGISTRATION"}]}


# Old version filter

def test_old_version_filter_matches_same_service_and_file_with_another_id():

    where = old_version_filter("new-doc", SERVICE_METADATA)

    assert where == {
        "$and": [
            {"department_id": "IRD"},
            {"document_type": "service_guideline"},
            {"source_file": "tin_guide.pdf"},
            {"document_id": {"$ne": "new-doc"}},
            {"service_id": "TIN_REGISTRATION"},
        ]
    }


def test_department_guideline_filter_has_no_service_condition():

    # Department guidelines have no service_id; the document type keeps them apart
    where = old_version_filter("new-doc", DEPARTMENT_METADATA)

    assert {"document_type": "department_guideline"} in where["$and"]
    for condition in where["$and"]:
        assert "service_id" not in condition


# Result conversion

def test_get_response_becomes_stored_chunks():

    response = {
        "ids": ["doc:1", "doc:0"],
        "documents": ["second", "first"],
        "metadatas": [{"chunk_index": 1}, {"chunk_index": 0}],
    }

    chunks = to_stored_chunks(response)
    chunks.sort(key=chunk_order)

    assert [chunk.chunk_id for chunk in chunks] == ["doc:0", "doc:1"]
    assert chunks[0].text == "first"


def test_chunks_without_an_index_are_ordered_last():

    chunks = to_stored_chunks(
        {"ids": ["a", "b"], "documents": ["x", "y"], "metadatas": [{}, {"chunk_index": 3}]}
    )
    chunks.sort(key=chunk_order)

    assert [chunk.chunk_id for chunk in chunks] == ["b", "a"]


# Input checks that run before ChromaDB is called

def test_empty_embedding_file_stores_nothing():

    # No collection is needed: nothing is sent
    store = VectorStore(client=None, collection=None)
    embedding_file = EmbeddingFile(document_id="doc001", model="test-model", dimension=8, records=[])

    result = store.store_document(embedding_file)

    assert result.stored_count == 0
    assert result.removed_old_versions == []


@pytest.mark.parametrize("top_k", [0, -1, MAX_TOP_K + 1])
def test_top_k_outside_the_range_is_rejected(top_k):

    store = VectorStore(client=None, collection=None)

    with pytest.raises(VectorStoreError, match="top_k"):
        store.search([0.1] * 8, top_k=top_k)


# Error mapping

def test_network_errors_become_connection_errors():

    def fail():
        raise httpx.ConnectError("connection refused")

    with pytest.raises(VectorStoreConnectionError):
        run_chroma(fail)


def test_chroma_errors_become_vector_store_errors():

    def fail():
        raise NotFoundError("collection missing")

    with pytest.raises(VectorStoreError, match="NotFoundError"):
        run_chroma(fail)


def test_successful_calls_return_their_result():

    def add(a, b):
        return a + b

    assert run_chroma(add, a=2, b=3) == 5


# Publication status filter

def test_publication_status_alone_is_a_single_condition():

    assert metadata_filter(publication_status="published") == {"publication_status": "published"}


def test_publication_status_comes_first_with_the_ids():

    where = metadata_filter(department_id="IRD", service_id="TIN_REGISTRATION", publication_status="published")

    assert where == {
        "$and": [
            {"publication_status": "published"},
            {"department_id": "IRD"},
            {"service_id": "TIN_REGISTRATION"},
        ]
    }


# Stored embedding model

class FakeCollection:
    """Answers collection.get like ChromaDB, with the given records."""

    def __init__(self, ids=(), metadatas=(), embeddings=()):

        self.response = {"ids": list(ids), "metadatas": list(metadatas), "embeddings": list(embeddings)}

    def get(self, **arguments):

        return self.response


def test_empty_collection_has_no_embedding_info():

    store = VectorStore(client=None, collection=FakeCollection())

    assert store.embedding_info() is None


def test_embedding_info_reads_the_model_and_size_of_a_stored_record():

    collection = FakeCollection(ids=["doc:0"], metadatas=[{"embedding_model": "test-model"}], embeddings=[[0.1] * 8])
    store = VectorStore(client=None, collection=collection)

    info = store.embedding_info()

    assert info.model == "test-model"
    assert info.dimension == 8


def test_vectors_from_another_model_are_rejected_before_storing():

    collection = FakeCollection(ids=["doc:0"], metadatas=[{"embedding_model": "model-a"}], embeddings=[[0.1] * 8])
    store = VectorStore(client=None, collection=collection)

    with pytest.raises(EmbeddingMismatchError, match="model-a"):
        store.check_embedding_model("model-b", 8)


# Opening an existing collection

def test_opening_without_a_running_database_is_a_connection_error():

    # Nothing listens on port 1; search opens the collection without creating it
    settings = VectorStoreSettings(host="127.0.0.1", port=1, collection="govai_knowledge")

    with pytest.raises(VectorStoreConnectionError):
        create_vector_store(settings, create_if_missing=False)
