"""GA-76: check semantic search with sample TIN registration questions.

Runs against the real govai_knowledge collection with the real NVIDIA API,
using the settings in rag/.env. Process the IRD TIN guide first:

    python scripts/process_documents.py --department-id IRD --service-id TIN_REGISTRATION \
        --document-type service_guideline --title "Obtaining a Taxpayer Identification Number (TIN)"

Skipped when the settings are missing, ChromaDB is not running, or no
published TIN document is stored. Only reads; nothing is changed.

These checks confirm that every question finds published TIN chunks with
their source. How accurate the ranking is gets measured separately (GA-79).
"""

import pytest

from app.embeddings.embedding_service import EmbeddingConfigError, create_embedder, load_settings
from app.retrieval.retriever import Retriever
from app.vector_store.config import load_vector_store_settings
from app.vector_store.exceptions import VectorStoreError
from app.vector_store.vector_store import create_vector_store

TOP_K = 3

TIN_QUESTIONS = [
    "How do I get a Taxpayer Identification Number?",
    "What documents do I need to register for a TIN?",
    "Who needs to register for a TIN?",
    "How can a company register as a taxpayer?",
    "Can I register for a TIN online?",
    "Where do I submit the TIN registration application?",
    "What details are needed to register a partnership for tax?",
    "How do I register as an individual taxpayer?",
]


# Fixtures

@pytest.fixture(scope="module")
def retriever():

    # The real NVIDIA embedder from rag/.env
    try:
        embedder = create_embedder(load_settings())
    except EmbeddingConfigError as error:
        pytest.skip(f"real NVIDIA settings are not available in rag/.env ({error})")

    # The real collection, opened without creating it
    try:
        vector_store = create_vector_store(load_vector_store_settings(), create_if_missing=False)
    except VectorStoreError as error:
        pytest.skip(f"the govai_knowledge collection is not available ({error})")

    retriever = Retriever(embedder=embedder, vector_store=vector_store)

    # Skip when no published TIN document has been processed yet
    check = retriever.retrieve(TIN_QUESTIONS[0], top_k=1, department_id="IRD", service_id="TIN_REGISTRATION")
    if not check.results:
        pytest.skip("no published TIN document is stored; run scripts/process_documents.py first")

    return retriever


# Tests

@pytest.mark.parametrize("question", TIN_QUESTIONS)
def test_tin_question_finds_published_tin_chunks(retriever, question):

    response = retriever.retrieve(question, top_k=TOP_K, department_id="IRD", service_id="TIN_REGISTRATION")

    # Matches are found, at most top_k, from the IRD TIN service only
    assert 1 <= len(response.results) <= TOP_K
    for result in response.results:
        assert result.department_id == "IRD"
        assert result.service_id == "TIN_REGISTRATION"

        # Each match carries its text, IDs, and source
        assert result.text
        assert result.chunk_id.startswith(result.document_id)
        assert result.source_file
        assert result.page_number >= 1
        assert result.object_key
        assert 0 <= result.distance <= 2


@pytest.mark.parametrize("question", TIN_QUESTIONS[:2])
def test_results_are_ordered_best_first(retriever, question):

    results = retriever.retrieve(question, top_k=TOP_K, department_id="IRD", service_id="TIN_REGISTRATION").results

    distances = []
    for result in results:
        distances.append(result.distance)
    assert distances == sorted(distances)
    assert [result.rank for result in results] == list(range(1, len(results) + 1))


def test_unrelated_department_finds_nothing(retriever):

    # No documents are stored for this department, so nothing may leak in from IRD
    response = retriever.retrieve(TIN_QUESTIONS[0], department_id="NO_SUCH_DEPARTMENT")

    assert response.results == []
