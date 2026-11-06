"""Tests for app/embeddings/models.py."""

import pytest
from pydantic import ValidationError

from app.embeddings.models import Chunk, Embedder, EmbeddingFile, EmbeddingFileResult, FailedBatch
from tests.helpers import DIMENSION, TEST_MODEL, make_settings


# Chunks

@pytest.mark.parametrize("field", ["chunk_id", "text"])
def test_chunk_rejects_empty_fields(field):

    # A chunk needs an ID and text
    values = {"chunk_id": "doc001:0", "text": "TIN step.", "metadata": {}}
    values[field] = ""

    with pytest.raises(ValidationError):
        Chunk(**values)


# Embeddings files

def test_embedding_file_rejects_zero_dimension():

    with pytest.raises(ValidationError):
        EmbeddingFile(document_id="doc001", model=TEST_MODEL, dimension=0, records=[])


def test_embedding_file_ignores_the_old_provider_field():

    # Files saved before the NVIDIA-only change have a "provider" field
    embedding_file = EmbeddingFile.model_validate(
        {"document_id": "doc001", "provider": "nvidia", "model": TEST_MODEL, "dimension": DIMENSION, "records": []}
    )

    assert embedding_file.model == TEST_MODEL


# Results

def test_failed_chunk_ids_combines_every_failed_batch():

    # The IDs of all failed batches are listed in order
    result = EmbeddingFileResult(
        document_id="doc001",
        embedded_count=1,
        reused_count=0,
        failed=[
            FailedBatch(chunk_ids=["doc001:1", "doc001:2"], error_type="Exception", message="[400] Bad Request"),
            FailedBatch(chunk_ids=["doc001:5"], error_type="Exception", message="[400] Bad Request"),
        ],
    )

    assert result.failed_chunk_ids == ["doc001:1", "doc001:2", "doc001:5"]


def test_failed_chunk_ids_is_empty_without_failures():

    result = EmbeddingFileResult(document_id="doc001", embedded_count=3, reused_count=0)

    assert result.failed_chunk_ids == []


# Embedder

def test_embedder_needs_a_langchain_embeddings_model():

    # Anything that is not a LangChain Embeddings object is rejected
    with pytest.raises(ValidationError):
        Embedder(embedding_model="not a model", settings=make_settings())
