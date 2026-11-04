"""Integration tests for the Retriever against the real ChromaDB container.

Each test gets a new, empty test collection (see conftest.py). The stored
vectors are small synthetic ones and the query vector comes from a fake
embedder, so no NVIDIA call is made: a chunk's vector points along one axis,
and the query points along the axis of the chunk it should find.
"""

import pytest

from app.retrieval.exceptions import RetrievalConfigError, RetrievalUnavailableError
from app.retrieval.retriever import Retriever, create_retriever
from app.vector_store.config import REQUIRED_VARIABLES as CHROMA_VARIABLES
from app.vector_store.exceptions import CollectionNotFoundError
from app.vector_store.vector_store import create_vector_store
from tests.helpers import TEST_MODEL, chroma_env_values, make_env, make_fake_embedder, write_env_file
from tests.integration.test_chromadb import axis_vector, make_document

MISSING_COLLECTION = "govai_knowledge_missing_test"


# Helper functions

def make_status_document(document_id, status="published", **options):
    """A synthetic document whose chunks carry a publication status, as the pipeline now stores them."""

    document_embeddings = make_document(document_id, **options)
    for record in document_embeddings.records:
        record.metadata["publication_status"] = status

    return document_embeddings


def make_retriever(vector_store, query_axis):

    # The fake embedder returns the unit vector along query_axis for every question
    embedder = make_fake_embedder(vector=axis_vector(query_axis))
    retriever = Retriever(embedder=embedder, vector_store=vector_store)

    return retriever


# Published-only rule

def test_draft_documents_are_never_returned(vector_store):

    # The draft's chunk is the exact match, but only the published document may be found
    vector_store.store_document(make_status_document("published-doc", first_axis=0, source_file="tin_guide.pdf"))
    vector_store.store_document(make_status_document("draft-doc", "draft", first_axis=5, source_file="tin_draft.pdf"))

    results = make_retriever(vector_store, query_axis=5).retrieve("How do I get a TIN?", top_k=10).results

    assert results
    for result in results:
        assert result.document_id == "published-doc"


def test_chunks_without_a_status_are_not_returned(vector_store):

    # Records stored before GA-76 have no status; they stay hidden until processed again
    vector_store.store_document(make_document("old-doc"))

    response = make_retriever(vector_store, query_axis=0).retrieve("How do I get a TIN?")

    assert response.results == []


# Search

def test_closest_chunk_comes_first_with_its_source(vector_store):

    vector_store.store_document(make_status_document("doc-a", chunk_count=4))

    results = make_retriever(vector_store, query_axis=2).retrieve("How do I get a TIN?", top_k=2).results

    assert results[0].chunk_id == "doc-a:2"
    assert results[0].rank == 1
    assert results[0].distance == pytest.approx(0.0, abs=1e-5)
    assert results[0].similarity == pytest.approx(1.0, abs=1e-5)
    assert results[0].source_file == "tin_guide.pdf"
    assert results[0].page_number == 1
    assert results[0].object_key == "documents/IRD/TIN_REGISTRATION/doc-a.pdf"
    assert results[1].distance > results[0].distance


def test_top_k_limits_the_number_of_results(vector_store):

    vector_store.store_document(make_status_document("doc-a", chunk_count=6))

    results = make_retriever(vector_store, query_axis=0).retrieve("How do I get a TIN?", top_k=3).results

    assert len(results) == 3


def test_search_is_limited_to_the_service(vector_store):

    # The closest chunk overall belongs to VAT, but the search is limited to TIN
    vector_store.store_document(make_status_document("tin-doc", service_id="TIN_REGISTRATION", first_axis=0))
    vector_store.store_document(make_status_document("vat-doc", service_id="VAT_REGISTRATION", first_axis=5))

    retriever = make_retriever(vector_store, query_axis=5)
    results = retriever.retrieve("How do I register?", top_k=10, department_id="IRD", service_id="TIN_REGISTRATION").results

    assert results
    for result in results:
        assert result.service_id == "TIN_REGISTRATION"


def test_search_is_limited_to_the_department(vector_store):

    vector_store.store_document(make_status_document("ird-doc", department_id="IRD"))
    vector_store.store_document(make_status_document("dmt-doc", department_id="DMT", source_file="dmt_guide.pdf"))

    results = make_retriever(vector_store, query_axis=0).retrieve("How do I register?", top_k=10, department_id="DMT").results

    assert results
    for result in results:
        assert result.department_id == "DMT"


def test_max_distance_drops_far_chunks(vector_store):

    # Chunks along other axes are at distance 1, the matching one at 0
    vector_store.store_document(make_status_document("doc-a", chunk_count=4))

    results = make_retriever(vector_store, query_axis=1).retrieve("How do I get a TIN?", top_k=4, max_distance=0.5).results

    assert [result.chunk_id for result in results] == ["doc-a:1"]


def test_other_service_has_no_results(vector_store):

    vector_store.store_document(make_status_document("tin-doc"))

    response = make_retriever(vector_store, query_axis=0).retrieve("How do I register?", service_id="VAT_REGISTRATION")

    assert response.results == []


def test_empty_collection_has_no_results(vector_store):

    response = make_retriever(vector_store, query_axis=0).retrieve("How do I get a TIN?")

    assert response.results == []


# Model checks

def test_documents_from_another_model_are_rejected(vector_store):

    vector_store.store_document(make_status_document("doc-a", model="nvidia/other-model"))

    with pytest.raises(RetrievalConfigError, match="nvidia/other-model"):
        make_retriever(vector_store, query_axis=0).retrieve("How do I get a TIN?")


def test_stored_model_and_size_are_read(vector_store):

    vector_store.store_document(make_status_document("doc-a"))

    info = vector_store.embedding_info()

    assert info.model == TEST_MODEL
    assert info.dimension == 8


# Missing collection and stopped database

def test_missing_collection_is_reported_not_created(chroma_settings, vector_store):

    # vector_store is requested only to skip this test when ChromaDB is not running
    settings = chroma_settings.model_copy(update={"collection": MISSING_COLLECTION})

    with pytest.raises(CollectionNotFoundError, match=MISSING_COLLECTION):
        create_vector_store(settings, create_if_missing=False)

    # Opening it for search did not create it
    collection_names = []
    for collection in vector_store.client.list_collections():
        collection_names.append(collection.name)
    assert MISSING_COLLECTION not in collection_names


def test_existing_collection_is_opened_for_search(chroma_settings, vector_store):

    vector_store.store_document(make_status_document("doc-a"))

    opened = create_vector_store(chroma_settings, create_if_missing=False)

    assert opened.count() == 3


def test_retriever_reports_a_missing_collection(chroma_settings, vector_store, tmp_path, monkeypatch):

    # Valid fake NVIDIA settings, and a collection that does not exist
    for name in list(make_env()) + CHROMA_VARIABLES:
        monkeypatch.delenv(name, raising=False)
    values = make_env()
    values.update(chroma_env_values(chroma_settings, CHROMA_COLLECTION=MISSING_COLLECTION))
    env_file = write_env_file(tmp_path / ".env", values)

    with pytest.raises(RetrievalUnavailableError, match="does not exist"):
        create_retriever(env_file=env_file)


def test_stopped_database_is_reported(chroma_settings, tmp_path, monkeypatch):

    # Needs no running ChromaDB: nothing listens on port 1
    for name in list(make_env()) + CHROMA_VARIABLES:
        monkeypatch.delenv(name, raising=False)
    values = make_env()
    values.update(chroma_env_values(chroma_settings, CHROMA_HOST="127.0.0.1", CHROMA_PORT="1"))
    env_file = write_env_file(tmp_path / ".env", values)

    with pytest.raises(RetrievalUnavailableError, match="not reachable"):
        create_retriever(env_file=env_file)
