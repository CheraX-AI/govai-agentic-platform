"""Tests for app/embeddings/embedding_service.py against the real NVIDIA API.

Settings come from rag/.env; the tests are skipped when NVIDIA_API_KEY is
missing. Only short synthetic sentences are sent. Failures are produced for
real: a key NVIDIA rejects (403), and a chunk longer than NVIDIA accepts (400).
"""

import numpy as np
import pytest
from langchain_nvidia_ai_endpoints import NVIDIAEmbeddings

from app.embeddings import embedding_service
from app.embeddings.config import load_settings
from app.embeddings.embedding_service import create_embedder, embed_chunks, embed_document, embedding_text
from app.embeddings.exceptions import EmbeddingAuthError
from app.embeddings.validators import validate_chunks
from tests.helpers import (
    TOO_LONG_TEXT,
    chunk_ids,
    make_chunk_records,
    make_embedder,
    make_env,
    make_invalid_key_embedder,
    real_settings,
    topic_records,
)


# Helper functions

def load_test_chunks(records):

    # Check the records and turn them into Chunk objects, as the pipeline does
    chunks = validate_chunks(records, "test.pdf")

    return chunks


def cosine_similarity(first, second):

    # Convert the vectors to numpy arrays
    first = np.array(first)
    second = np.array(second)

    # Dot product divided by the product of the lengths
    similarity = np.dot(first, second) / (np.linalg.norm(first) * np.linalg.norm(second))

    return similarity


# NVIDIA client

def test_create_embedder_uses_the_env_settings():

    # Creating the client does not call the API
    embedder = create_embedder(load_settings(make_env()))

    assert isinstance(embedder.embedding_model, NVIDIAEmbeddings)
    assert embedder.embedding_model.model == "nvidia/test-embed-model"
    assert embedder.embedding_model.truncate == "NONE"
    assert embedder.embedding_model.max_batch_size == 20


def test_create_embedder_passes_truncate_from_env():

    # The truncate mode comes from .env, not from the code
    embedder = create_embedder(load_settings(make_env(EMBEDDING_TRUNCATE="END")))

    assert embedder.embedding_model.truncate == "END"


def test_real_settings_come_from_rag_env():

    # The real-API tests use the model configured in rag/.env
    settings = real_settings()

    assert settings.model
    assert settings.api_key.get_secret_value().startswith("nvapi-")


# Embedding in batches

def test_chunks_are_embedded_in_batches():

    # Five chunks with batch size 2 are sent to NVIDIA in three requests
    chunks = load_test_chunks(make_chunk_records(5))

    result = embed_chunks(chunks, make_embedder(batch_size=2))

    assert result.failed == []
    assert result.auth_failed is False

    # Each embedding keeps its chunk ID, text, and metadata, in order
    assert chunk_ids(result.embedded) == chunk_ids(chunks)
    for embedded, chunk in zip(result.embedded, chunks):
        assert embedded.text == chunk.text
        assert embedded.metadata == chunk.metadata

    # Every vector has the same size and is not all zeros
    dimension = len(result.embedded[0].embedding)
    assert dimension > 0
    for embedded in result.embedded:
        assert len(embedded.embedding) == dimension
        assert np.linalg.norm(embedded.embedding) > 0


def test_question_is_closest_to_the_relevant_chunk():

    # A TIN question should be nearer the TIN chunk than the museum chunk
    embedder = make_embedder()
    chunks = load_test_chunks(topic_records())
    result = embed_chunks(chunks, embedder)
    question = embedder.embedding_model.embed_query("How do I get a TIN number?")

    tin_similarity = cosine_similarity(question, result.embedded[0].embedding)
    museum_similarity = cosine_similarity(question, result.embedded[2].embedding)

    assert tin_similarity > museum_similarity


def test_no_chunks_means_no_calls():

    # With a key NVIDIA rejects, any call would fail; an empty list makes none
    result = embed_chunks([], make_invalid_key_embedder())

    assert result.embedded == []
    assert result.failed == []


def test_rejected_batch_keeps_the_other_batches():

    # The middle chunk is longer than NVIDIA accepts; batch size 1 isolates it
    records = make_chunk_records(3)
    records[1]["text"] = TOO_LONG_TEXT
    chunks = load_test_chunks(records)

    result = embed_chunks(chunks, make_embedder(batch_size=1))

    # NVIDIA rejects only that chunk, with a 400 error
    assert chunk_ids(result.embedded) == ["doc001:0", "doc001:2"]
    assert len(result.failed) == 1
    assert result.failed[0].chunk_ids == ["doc001:1"]
    assert result.failed[0].message.startswith("[400]")
    assert result.auth_failed is False


# Authentication

def test_rejected_key_stops_every_batch():

    # NVIDIA rejects the key on the first batch, so no further batch is sent
    chunks = load_test_chunks(make_chunk_records(4))

    result = embed_chunks(chunks, make_invalid_key_embedder(batch_size=2))

    assert result.auth_failed is True
    assert result.embedded == []
    assert result.failed[0].chunk_ids == ["doc001:0", "doc001:1", "doc001:2", "doc001:3"]
    assert result.failed[0].error_type == "EmbeddingAuthError"
    assert result.failed[0].message.startswith("[403]")


def test_rejected_key_is_not_retried(monkeypatch):

    # Record any retry wait instead of sleeping; a rejected key must cause none
    delays = []
    monkeypatch.setattr(embedding_service.time, "sleep", delays.append)
    chunks = load_test_chunks(make_chunk_records(2))

    embed_chunks(chunks, make_invalid_key_embedder(max_retries=3))

    assert delays == []


# Embedding a document

def test_embed_document_keeps_every_chunk_in_order():

    # Embed one document's chunks with the real NVIDIA model
    chunks = load_test_chunks(make_chunk_records(3))
    embedder = make_embedder()

    result = embed_document(chunks, embedder)

    assert result.document_id == "doc001"
    assert result.embedded_count == 3
    assert result.failed == []

    # The embeddings record the real model and keep every chunk in order
    embeddings = result.embeddings

    assert embeddings.document_id == "doc001"
    assert embeddings.model == embedder.settings.model
    assert embeddings.dimension == len(embeddings.records[0].embedding)
    assert chunk_ids(embeddings.records) == ["doc001:0", "doc001:1", "doc001:2"]
    assert embeddings.records[0].metadata["service_id"] == "TIN_REGISTRATION"


def test_rejected_chunks_are_listed_and_left_out():

    # Every chunk is longer than NVIDIA accepts
    records = make_chunk_records(2)
    for record in records:
        record["text"] = TOO_LONG_TEXT

    result = embed_document(load_test_chunks(records), make_embedder(batch_size=1))

    assert result.embeddings is None
    assert result.embedded_count == 0
    failed = []
    for batch in result.failed:
        failed.extend(batch.chunk_ids)
    assert failed == ["doc001:0", "doc001:1"]


def test_rejected_key_raises_for_the_document():

    # NVIDIA rejects the key: the caller stops embedding the other documents
    with pytest.raises(EmbeddingAuthError, match="403"):
        embed_document(load_test_chunks(make_chunk_records(2)), make_invalid_key_embedder())


# Document title context

def test_embedding_text_puts_the_title_first():

    # With a title, NVIDIA receives the title and then the chunk text
    records = make_chunk_records(1)
    records[0]["metadata"]["document_title"] = "Obtaining a TIN"
    chunk = load_test_chunks(records)[0]

    assert embedding_text(chunk) == "Obtaining a TIN\n\nTIN registration step 0 for taxpayers."


def test_embedding_text_without_a_title_is_the_chunk_text():

    chunk = load_test_chunks(make_chunk_records(1))[0]

    assert embedding_text(chunk) == "TIN registration step 0 for taxpayers."


def test_title_is_used_for_the_vector_but_not_stored_in_the_text():

    # Embed a chunk with a title using the real NVIDIA model
    records = make_chunk_records(1)
    records[0]["metadata"]["document_title"] = "Obtaining a TIN"

    result = embed_document(load_test_chunks(records), make_embedder())

    # The stored text is the chunk text; the title stays in the metadata
    record = result.embeddings.records[0]
    assert record.text == "TIN registration step 0 for taxpayers."
    assert record.metadata["document_title"] == "Obtaining a TIN"
