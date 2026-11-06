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
from app.embeddings.embedding_service import create_embedder, embed_chunk_file, embed_chunks, embedding_text
from app.embeddings.exceptions import EmbeddingAuthError
from app.embeddings.storage import load_chunks, load_embeddings, save_embeddings
from tests.helpers import (
    TOO_LONG_TEXT,
    chunk_ids,
    make_chunk_records,
    make_embedder,
    make_embedding_file,
    make_env,
    make_invalid_key_embedder,
    real_settings,
    topic_records,
    write_chunk_file,
)


# Helper functions

def load_test_chunks(tmp_path, records):

    # Write a chunk file and load it back as Chunk objects
    chunks_path = write_chunk_file(tmp_path, records)
    chunks = load_chunks(chunks_path)

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

def test_chunks_are_embedded_in_batches(tmp_path):

    # Five chunks with batch size 2 are sent to NVIDIA in three requests
    chunks = load_test_chunks(tmp_path, make_chunk_records(5))

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


def test_question_is_closest_to_the_relevant_chunk(tmp_path):

    # A TIN question should be nearer the TIN chunk than the museum chunk
    embedder = make_embedder()
    chunks = load_test_chunks(tmp_path, topic_records())
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


def test_rejected_batch_keeps_the_other_batches(tmp_path):

    # The middle chunk is longer than NVIDIA accepts; batch size 1 isolates it
    records = make_chunk_records(3)
    records[1]["text"] = TOO_LONG_TEXT
    chunks = load_test_chunks(tmp_path, records)

    result = embed_chunks(chunks, make_embedder(batch_size=1))

    # NVIDIA rejects only that chunk, with a 400 error
    assert chunk_ids(result.embedded) == ["doc001:0", "doc001:2"]
    assert len(result.failed) == 1
    assert result.failed[0].chunk_ids == ["doc001:1"]
    assert result.failed[0].message.startswith("[400]")
    assert result.auth_failed is False


# Authentication

def test_rejected_key_stops_every_batch(tmp_path):

    # NVIDIA rejects the key on the first batch, so no further batch is sent
    chunks = load_test_chunks(tmp_path, make_chunk_records(4))

    result = embed_chunks(chunks, make_invalid_key_embedder(batch_size=2))

    assert result.auth_failed is True
    assert result.embedded == []
    assert result.failed[0].chunk_ids == ["doc001:0", "doc001:1", "doc001:2", "doc001:3"]
    assert result.failed[0].error_type == "EmbeddingAuthError"
    assert result.failed[0].message.startswith("[403]")


def test_rejected_key_is_not_retried(tmp_path, monkeypatch):

    # Record any retry wait instead of sleeping; a rejected key must cause none
    delays = []
    monkeypatch.setattr(embedding_service.time, "sleep", delays.append)
    chunks = load_test_chunks(tmp_path, make_chunk_records(2))

    embed_chunks(chunks, make_invalid_key_embedder(max_retries=3))

    assert delays == []


# Embedding a chunk file

def test_embed_chunk_file_saves_embeddings(tmp_path):

    # Embed a chunk file and save the result
    chunks_path = write_chunk_file(tmp_path, make_chunk_records(3))
    output_dir = tmp_path / "embeddings"
    embedder = make_embedder()

    result = embed_chunk_file(chunks_path, embedder, output_dir)

    assert result.document_id == "doc001"
    assert result.embedded_count == 3
    assert result.reused_count == 0
    assert result.failed_chunk_ids == []
    assert result.embeddings_path == output_dir / "doc001.embeddings.json"

    # The file records the real model and keeps every chunk in order
    saved = load_embeddings(result.embeddings_path)

    assert saved.document_id == "doc001"
    assert saved.model == embedder.settings.model
    assert saved.dimension == len(saved.records[0].embedding)
    assert chunk_ids(saved.records) == ["doc001:0", "doc001:1", "doc001:2"]
    assert saved.records[0].metadata["service_id"] == "TIN_REGISTRATION"


def test_rerun_without_changes_makes_no_calls(tmp_path):

    # Embed once with the real key
    chunks_path = write_chunk_file(tmp_path, make_chunk_records(3))
    output_dir = tmp_path / "embeddings"
    embed_chunk_file(chunks_path, make_embedder(), output_dir)

    # Re-run with a key NVIDIA rejects: it succeeds only if nothing is sent
    result = embed_chunk_file(chunks_path, make_invalid_key_embedder(), output_dir)

    assert result.embedded_count == 0
    assert result.reused_count == 3


def test_rejected_chunk_is_completed_on_rerun(tmp_path):

    # The first run fails on one over-long chunk
    records = make_chunk_records(3)
    records[1]["text"] = TOO_LONG_TEXT
    chunks_path = write_chunk_file(tmp_path, records)
    output_dir = tmp_path / "embeddings"

    first = embed_chunk_file(chunks_path, make_embedder(batch_size=1), output_dir)

    assert first.embedded_count == 2
    assert first.failed_chunk_ids == ["doc001:1"]
    assert chunk_ids(load_embeddings(first.embeddings_path).records) == ["doc001:0", "doc001:2"]

    # After the chunk is fixed, the re-run embeds only that chunk and saves all three in order
    records[1]["text"] = "TIN registration step 1 for taxpayers."
    write_chunk_file(tmp_path, records)

    second = embed_chunk_file(chunks_path, make_embedder(batch_size=1), output_dir)

    assert second.embedded_count == 1
    assert second.reused_count == 2
    assert second.failed_chunk_ids == []
    assert chunk_ids(load_embeddings(second.embeddings_path).records) == ["doc001:0", "doc001:1", "doc001:2"]


def test_changed_chunk_text_is_embedded_again(tmp_path):

    # Embed, then change the text of one chunk
    records = make_chunk_records(3)
    chunks_path = write_chunk_file(tmp_path, records)
    output_dir = tmp_path / "embeddings"
    embed_chunk_file(chunks_path, make_embedder(), output_dir)

    records[1]["text"] = "Updated TIN registration step."
    write_chunk_file(tmp_path, records)

    # Only the changed chunk is sent again
    result = embed_chunk_file(chunks_path, make_embedder(), output_dir)

    assert result.embedded_count == 1
    assert result.reused_count == 2
    assert load_embeddings(result.embeddings_path).records[1].text == "Updated TIN registration step."


def test_changed_metadata_is_updated_without_embedding_again(tmp_path):

    # Embed, then correct the department of every chunk
    records = make_chunk_records(2)
    chunks_path = write_chunk_file(tmp_path, records)
    output_dir = tmp_path / "embeddings"
    embed_chunk_file(chunks_path, make_embedder(), output_dir)

    for record in records:
        record["metadata"]["department_id"] = "DMT"
    write_chunk_file(tmp_path, records)

    # A key NVIDIA rejects proves the vectors are reused, not sent again
    result = embed_chunk_file(chunks_path, make_invalid_key_embedder(), output_dir)

    assert result.reused_count == 2
    assert load_embeddings(result.embeddings_path).records[0].metadata["department_id"] == "DMT"


def test_removed_chunks_are_dropped_from_the_file(tmp_path):

    # Embed three chunks, then re-chunk the document into two
    chunks_path = write_chunk_file(tmp_path, make_chunk_records(3))
    output_dir = tmp_path / "embeddings"
    embed_chunk_file(chunks_path, make_embedder(), output_dir)

    write_chunk_file(tmp_path, make_chunk_records(2))
    result = embed_chunk_file(chunks_path, make_invalid_key_embedder(), output_dir)

    assert chunk_ids(load_embeddings(result.embeddings_path).records) == ["doc001:0", "doc001:1"]


def test_model_change_embeds_everything_again(tmp_path):

    # An earlier file made by another model
    chunks_path = write_chunk_file(tmp_path, make_chunk_records(2))
    output_dir = tmp_path / "embeddings"
    save_embeddings(make_embedding_file(2, model="old-model"), output_dir)

    # Vectors from another model are not mixed with new ones
    embedder = make_embedder()
    result = embed_chunk_file(chunks_path, embedder, output_dir)

    assert result.embedded_count == 2
    assert result.reused_count == 0
    assert load_embeddings(result.embeddings_path).model == embedder.settings.model


def test_damaged_embeddings_file_is_replaced(tmp_path):

    # A corrupted earlier file is ignored and rewritten
    chunks_path = write_chunk_file(tmp_path, make_chunk_records(2))
    output_dir = tmp_path / "embeddings"
    output_dir.mkdir()
    (output_dir / "doc001.embeddings.json").write_text("{ broken", encoding="utf-8")

    result = embed_chunk_file(chunks_path, make_embedder(), output_dir)

    assert result.embedded_count == 2
    assert len(load_embeddings(result.embeddings_path).records) == 2


def test_rejected_key_keeps_reused_embeddings_before_raising(tmp_path):

    # Embed three chunks, then change one so it must be sent again
    records = make_chunk_records(3)
    chunks_path = write_chunk_file(tmp_path, records)
    output_dir = tmp_path / "embeddings"
    embed_chunk_file(chunks_path, make_embedder(), output_dir)

    records[2]["text"] = "Updated TIN registration step."
    write_chunk_file(tmp_path, records)

    # NVIDIA rejects the key when the changed chunk is sent
    with pytest.raises(EmbeddingAuthError, match="403"):
        embed_chunk_file(chunks_path, make_invalid_key_embedder(), output_dir)

    # The unchanged chunks were saved before the error was raised
    saved = load_embeddings(output_dir / "doc001.embeddings.json")
    assert chunk_ids(saved.records) == ["doc001:0", "doc001:1"]


def test_no_file_is_written_when_every_chunk_is_rejected(tmp_path):

    # Every chunk is longer than NVIDIA accepts
    records = make_chunk_records(2)
    for record in records:
        record["text"] = TOO_LONG_TEXT
    chunks_path = write_chunk_file(tmp_path, records)
    output_dir = tmp_path / "embeddings"

    result = embed_chunk_file(chunks_path, make_embedder(batch_size=1), output_dir)

    assert result.embeddings_path is None
    assert result.failed_chunk_ids == ["doc001:0", "doc001:1"]
    assert not (output_dir / "doc001.embeddings.json").exists()


# Document title context

def test_embedding_text_puts_the_title_first(tmp_path):

    # With a title, NVIDIA receives the title and then the chunk text
    records = make_chunk_records(1)
    records[0]["metadata"]["document_title"] = "Obtaining a TIN"
    chunk = load_test_chunks(tmp_path, records)[0]

    assert embedding_text(chunk) == "Obtaining a TIN\n\nTIN registration step 0 for taxpayers."


def test_embedding_text_without_a_title_is_the_chunk_text(tmp_path):

    chunk = load_test_chunks(tmp_path, make_chunk_records(1))[0]

    assert embedding_text(chunk) == "TIN registration step 0 for taxpayers."


def test_title_is_used_for_the_vector_but_not_saved_in_the_text(tmp_path):

    # Embed a chunk with a title using the real NVIDIA model
    records = make_chunk_records(1)
    records[0]["metadata"]["document_title"] = "Obtaining a TIN"
    chunks_path = write_chunk_file(tmp_path, records)

    result = embed_chunk_file(chunks_path, make_embedder(), tmp_path / "embeddings")

    # The saved text is the chunk text; the title stays in the metadata
    record = load_embeddings(result.embeddings_path).records[0]
    assert record.text == "TIN registration step 0 for taxpayers."
    assert record.metadata["document_title"] == "Obtaining a TIN"


def test_changed_title_is_embedded_again(tmp_path):

    # Embed with one title
    records = make_chunk_records(2)
    for record in records:
        record["metadata"]["document_title"] = "Old title"
    chunks_path = write_chunk_file(tmp_path, records)
    output_dir = tmp_path / "embeddings"
    embed_chunk_file(chunks_path, make_embedder(), output_dir)

    # Change the title: the vectors no longer match, so NVIDIA must be called again
    for record in records:
        record["metadata"]["document_title"] = "Obtaining a Taxpayer Identification Number (TIN)"
    write_chunk_file(tmp_path, records)

    with pytest.raises(EmbeddingAuthError):
        embed_chunk_file(chunks_path, make_invalid_key_embedder(), output_dir)
