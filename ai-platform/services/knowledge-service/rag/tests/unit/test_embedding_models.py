"""Tests for app/embeddings/models.py."""

import pytest
from pydantic import ValidationError

from app.embeddings.models import Chunk, DocumentEmbeddingResult, DocumentEmbeddings, Embedder, FailedBatch
from tests.helpers import DIMENSION, TEST_MODEL, make_document_embeddings, make_settings


# Chunks

@pytest.mark.parametrize("field", ["chunk_id", "text"])
def test_chunk_rejects_empty_fields(field):

    # A chunk needs an ID and text
    values = {"chunk_id": "doc001:0", "text": "TIN step.", "metadata": {}}
    values[field] = ""

    with pytest.raises(ValidationError):
        Chunk(**values)


# Document embeddings

def test_document_embeddings_reject_zero_dimension():

    with pytest.raises(ValidationError):
        DocumentEmbeddings(document_id="doc001", model=TEST_MODEL, dimension=0, records=[])


def test_document_embeddings_ignore_unknown_fields():

    # An old "provider" field is ignored
    document_embeddings = DocumentEmbeddings.model_validate(
        {"document_id": "doc001", "provider": "nvidia", "model": TEST_MODEL, "dimension": DIMENSION, "records": []}
    )

    assert document_embeddings.model == TEST_MODEL


# Results

def test_embedded_count_is_the_number_of_records():

    records = make_document_embeddings(3).records
    result = DocumentEmbeddingResult(
        document_id="doc001",
        embeddings=DocumentEmbeddings(document_id="doc001", model=TEST_MODEL, dimension=DIMENSION, records=records),
    )

    assert result.embedded_count == 3


def test_embedded_count_is_zero_without_embeddings():

    # Every chunk failed: there are no embeddings, only the failed batches
    result = DocumentEmbeddingResult(
        document_id="doc001",
        failed=[FailedBatch(chunk_ids=["doc001:0"], error_type="Exception", message="[400] Bad Request")],
    )

    assert result.embeddings is None
    assert result.embedded_count == 0


# Embedder

def test_embedder_needs_a_langchain_embeddings_model():

    # Anything that is not a LangChain Embeddings object is rejected
    with pytest.raises(ValidationError):
        Embedder(embedding_model="not a model", settings=make_settings())
