"""Integration tests for VectorStore against the real ChromaDB container.

Each test gets a new, empty test collection (see conftest.py). The vectors are
small synthetic ones, so no NVIDIA call is made: a chunk's vector points along
one axis, which makes the nearest chunk for a query predictable.
"""

import pytest

from app.embeddings.models import EmbeddedChunk, EmbeddingFile
from app.vector_store.exceptions import (
    ChunkNotFoundError,
    EmbeddingMismatchError,
    VectorStoreConnectionError,
)
from app.vector_store.vector_store import create_vector_store, metadata_filter

DIMENSION = 8
TEST_MODEL = "test-model"


# Helper functions

def axis_vector(axis, dimension=DIMENSION):

    # A unit vector along one axis; different axes are as far apart as possible
    vector = [0.0] * dimension
    vector[axis % dimension] = 1.0

    return vector


def make_document(
    document_id,
    chunk_count=3,
    source_file="tin_guide.pdf",
    department_id="IRD",
    service_id="TIN_REGISTRATION",
    model=TEST_MODEL,
    dimension=DIMENSION,
    first_axis=0,
):
    """An embeddings file shaped like the pipeline's output, with one axis vector per chunk."""

    # Metadata shared by every chunk, as the extractor and chunker write it
    shared = {
        "document_id": document_id,
        "department_id": department_id,
        "source_file": source_file,
        "object_key": f"documents/{department_id}/{service_id or '_department'}/{document_id}.pdf",
        "total_pages": 1,
        "page_number": 1,
    }
    if service_id is None:
        shared["document_type"] = "department_guideline"
    else:
        shared["document_type"] = "service_guideline"
        shared["service_id"] = service_id

    # One record per chunk
    records = []
    for index in range(chunk_count):
        chunk_id = f"{document_id}:{index}"
        records.append(
            EmbeddedChunk(
                chunk_id=chunk_id,
                text=f"{source_file} chunk {index}",
                embedding=axis_vector(first_axis + index, dimension),
                metadata={**shared, "chunk_index": index, "chunk_id": chunk_id},
            )
        )

    embedding_file = EmbeddingFile(document_id=document_id, model=model, dimension=dimension, records=records)

    return embedding_file


# Connection

def test_collection_is_created_and_starts_empty(vector_store):

    assert vector_store.count() == 0


def test_opening_the_collection_again_keeps_its_records(vector_store, chroma_settings):

    # get_or_create: a second connection sees the same collection
    vector_store.store_document(make_document("doc-a"))

    reopened = create_vector_store(chroma_settings)

    assert reopened.count() == 3


def test_stopped_database_is_a_connection_error(chroma_settings):

    # Needs no running ChromaDB: nothing listens on port 1
    unreachable = chroma_settings.model_copy(update={"host": "127.0.0.1", "port": 1})

    with pytest.raises(VectorStoreConnectionError):
        create_vector_store(unreachable)


# Storing

def test_stored_chunks_keep_text_and_metadata(vector_store):

    result = vector_store.store_document(make_document("doc-a"))
    chunks = vector_store.get_document_chunks("doc-a")

    assert result.stored_count == 3
    assert [chunk.chunk_id for chunk in chunks] == ["doc-a:0", "doc-a:1", "doc-a:2"]
    assert chunks[0].text == "tin_guide.pdf chunk 0"
    assert chunks[0].metadata["service_id"] == "TIN_REGISTRATION"
    assert chunks[0].metadata["object_key"] == "documents/IRD/TIN_REGISTRATION/doc-a.pdf"
    assert chunks[0].metadata["embedding_model"] == TEST_MODEL


def test_storing_the_same_document_twice_creates_no_duplicates(vector_store):

    vector_store.store_document(make_document("doc-a"))
    second = vector_store.store_document(make_document("doc-a"))

    assert vector_store.count() == 3
    assert second.removed_stale_count == 0
    assert second.removed_old_versions == []


def test_fewer_chunks_remove_the_stale_ones(vector_store):

    # After a chunk-size change the document has 2 chunks instead of 3
    vector_store.store_document(make_document("doc-a", chunk_count=3))
    result = vector_store.store_document(make_document("doc-a", chunk_count=2))

    assert result.removed_stale_count == 1
    assert [chunk.chunk_id for chunk in vector_store.get_document_chunks("doc-a")] == ["doc-a:0", "doc-a:1"]


def test_many_chunks_are_stored_in_several_requests(vector_store, monkeypatch):

    # Pretend ChromaDB accepts only 100 records per request, so 250 chunks need 3 requests
    monkeypatch.setattr(vector_store.client, "get_max_batch_size", lambda: 100)

    vector_store.store_document(make_document("doc-a", chunk_count=250))

    assert vector_store.count() == 250


# Document versions

def test_new_version_replaces_the_old_one(vector_store):

    # Same department, service, and file name, but new content (another document_id)
    vector_store.store_document(make_document("version-1", chunk_count=3))
    result = vector_store.store_document(make_document("version-2", chunk_count=2))

    assert result.removed_old_versions == ["version-1"]
    assert vector_store.get_document_chunks("version-1") == []
    assert len(vector_store.get_document_chunks("version-2")) == 2
    assert vector_store.count() == 2


def test_other_files_are_not_treated_as_versions(vector_store):

    vector_store.store_document(make_document("doc-a", source_file="tin_guide.pdf"))
    result = vector_store.store_document(make_document("doc-b", source_file="tin_faq.pdf"))

    assert result.removed_old_versions == []
    assert vector_store.count() == 6


def test_same_file_name_in_another_service_is_kept(vector_store):

    vector_store.store_document(make_document("doc-a", service_id="TIN_REGISTRATION"))
    result = vector_store.store_document(make_document("doc-b", service_id="VAT_REGISTRATION"))

    assert result.removed_old_versions == []
    assert vector_store.count() == 6


def test_department_guideline_does_not_replace_a_service_guideline(vector_store):

    vector_store.store_document(make_document("service-doc", service_id="TIN_REGISTRATION"))
    result = vector_store.store_document(make_document("department-doc", service_id=None))

    assert result.removed_old_versions == []
    assert vector_store.count() == 6


def test_new_department_guideline_version_replaces_the_old_one(vector_store):

    vector_store.store_document(make_document("dept-v1", service_id=None, source_file="ird_guide.pdf"))
    result = vector_store.store_document(make_document("dept-v2", service_id=None, source_file="ird_guide.pdf"))

    assert result.removed_old_versions == ["dept-v1"]


# Model checks

def test_vectors_from_another_model_are_rejected(vector_store):

    vector_store.store_document(make_document("doc-a", model="model-a"))

    with pytest.raises(EmbeddingMismatchError, match="model-a"):
        vector_store.store_document(make_document("doc-b", model="model-b", source_file="other.pdf"))

    assert vector_store.count() == 3


def test_vectors_of_another_size_are_rejected(vector_store):

    vector_store.store_document(make_document("doc-a"))

    with pytest.raises(EmbeddingMismatchError):
        vector_store.store_document(make_document("doc-b", dimension=16, source_file="other.pdf"))


# Reading and deleting

def test_chunk_is_read_by_its_id(vector_store):

    vector_store.store_document(make_document("doc-a"))

    chunk = vector_store.get_chunk("doc-a:1")

    assert chunk.text == "tin_guide.pdf chunk 1"
    assert chunk.metadata["chunk_index"] == 1


def test_missing_chunk_is_reported(vector_store):

    with pytest.raises(ChunkNotFoundError):
        vector_store.get_chunk("missing:0")


def test_unknown_document_has_no_chunks(vector_store):

    assert vector_store.get_document_chunks("missing") == []


def test_document_is_deleted_completely(vector_store):

    vector_store.store_document(make_document("doc-a"))
    vector_store.store_document(make_document("doc-b", source_file="other.pdf", first_axis=3))

    removed = vector_store.delete_document("doc-a")

    assert removed == 3
    assert vector_store.get_document_chunks("doc-a") == []
    assert vector_store.count() == 3


# Search

def test_search_returns_the_closest_chunk_first(vector_store):

    vector_store.store_document(make_document("doc-a", chunk_count=4))

    # The query points along axis 2, like chunk 2
    results = vector_store.search(axis_vector(2), top_k=2)

    assert results[0].chunk_id == "doc-a:2"
    assert results[0].distance == pytest.approx(0.0, abs=1e-5)
    assert results[0].distance < results[1].distance
    assert len(results) == 2


def test_search_results_carry_the_source(vector_store):

    vector_store.store_document(make_document("doc-a"))

    result = vector_store.search(axis_vector(0), top_k=1)[0]

    assert result.metadata["source_file"] == "tin_guide.pdf"
    assert result.metadata["object_key"] == "documents/IRD/TIN_REGISTRATION/doc-a.pdf"
    assert result.metadata["page_number"] == 1


def test_search_can_be_limited_to_one_service(vector_store):

    # The closest chunk overall belongs to VAT, but the search is limited to TIN
    vector_store.store_document(make_document("tin-doc", service_id="TIN_REGISTRATION", first_axis=0))
    vector_store.store_document(make_document("vat-doc", service_id="VAT_REGISTRATION", first_axis=5))

    where = metadata_filter(department_id="IRD", service_id="TIN_REGISTRATION")
    results = vector_store.search(axis_vector(5), top_k=3, where=where)

    for result in results:
        assert result.metadata["service_id"] == "TIN_REGISTRATION"


def test_search_in_an_empty_collection_finds_nothing(vector_store):

    assert vector_store.search(axis_vector(0), top_k=3) == []
