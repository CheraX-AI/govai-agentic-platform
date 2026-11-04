"""Shared test helpers: the real NVIDIA embedder from rag/.env, and test data builders.

Tests that need NVIDIA call the real API with the settings in rag/.env. When the
key is missing they are skipped with a clear reason. They send only the short
synthetic sentences built here, never real government documents.
"""

import json

import pytest
from dotenv import dotenv_values
from langchain_core.embeddings import Embeddings
from pydantic import SecretStr

from app.embeddings import (
    EmbeddedChunk,
    Embedder,
    EmbeddingConfigError,
    EmbeddingFile,
    EmbeddingSettings,
    create_embedder,
    load_settings,
)
from app.embeddings.config import ENV_FILE, REQUIRED_VARIABLES

# Has the NVIDIA key format, so settings accept it, but it is not a real key: NVIDIA rejects it
TEST_NVIDIA_KEY = "nvapi-test-key-not-real-1234"

# Model name used in test data that never reaches NVIDIA
TEST_MODEL = "test-model"

# Vector size used in test data that never reaches NVIDIA
DIMENSION = 8

# NVIDIA rejects inputs longer than 65,536 characters with a 400 error
TOO_LONG_TEXT = "TIN registration requires a completed form. " * 2000

# Chunks on different topics, for checking that search finds the right one
TOPIC_TEXTS = [
    "A Taxpayer Identification Number (TIN) is required to register as a taxpayer.",
    "Applicants must submit a copy of their national identity card.",
    "The museum is open on weekends and closed on public holidays.",
]


# Real NVIDIA

def real_settings(**overrides):
    """The settings from rag/.env, with some values replaced. Skips the test when they are missing."""

    # Load the real settings; without them the test cannot reach NVIDIA
    try:
        settings = load_settings()
    except EmbeddingConfigError as error:
        pytest.skip(f"real NVIDIA settings are not available in rag/.env ({error})")

    # Replace the values the test needs, such as a small batch size
    settings = settings.model_copy(update=overrides)

    return settings


def make_embedder(**overrides):

    # The real NVIDIA client, built from the settings in rag/.env
    embedder = create_embedder(real_settings(**overrides))

    return embedder


def make_invalid_key_embedder(**overrides):

    # The real NVIDIA client with a key NVIDIA rejects; any API call fails with 403
    overrides["api_key"] = SecretStr(TEST_NVIDIA_KEY)
    embedder = make_embedder(**overrides)

    return embedder


# Fake NVIDIA, for tests that must not call the API

class FakeEmbeddings(Embeddings):
    """Returns a fixed query vector, or raises the given errors first, and records each call."""

    def __init__(self, vector=None, errors=()):

        self.vector = vector if vector is not None else [1.0] + [0.0] * (DIMENSION - 1)
        self.errors = list(errors)
        self.query_calls = []
        self.document_calls = []

    def embed_query(self, text):

        # Fail with the next queued error, then return the vector
        self.query_calls.append(text)
        if self.errors:
            raise self.errors.pop(0)

        return self.vector

    def embed_documents(self, texts):

        self.document_calls.append(texts)
        vectors = []
        for _ in texts:
            vectors.append(self.vector)

        return vectors


def make_fake_embedder(vector=None, errors=(), **overrides):

    # The fake client with valid settings; retries wait 0 seconds
    embedder = Embedder(embedding_model=FakeEmbeddings(vector, errors), settings=make_settings(**overrides))

    return embedder


def write_real_env_file(path, **overrides):

    # Copy the real settings from rag/.env into another .env file, with some values replaced
    real_settings()
    values = {}
    real_values = dotenv_values(ENV_FILE)
    for name in REQUIRED_VARIABLES:
        values[name] = real_values.get(name, "")
    values.update(overrides)

    env_file = write_env_file(path, values)

    return env_file


# Settings data

def make_settings(**overrides):

    # Valid settings for code that does not call NVIDIA
    values = {
        "api_key": TEST_NVIDIA_KEY,
        "model": TEST_MODEL,
        "batch_size": 2,
        "max_retries": 2,
        "retry_delay_seconds": 0,
        "truncate": "NONE",
    }
    values.update(overrides)

    settings = EmbeddingSettings(**values)

    return settings


def make_env(**overrides):
    """A complete, valid set of .env values, with some replaced by overrides."""

    values = {
        "NVIDIA_API_KEY": TEST_NVIDIA_KEY,
        "EMBEDDING_MODEL": "nvidia/test-embed-model",
        "EMBEDDING_BATCH_SIZE": "20",
        "EMBEDDING_MAX_RETRIES": "3",
        "EMBEDDING_RETRY_DELAY_SECONDS": "0.5",
        "EMBEDDING_TRUNCATE": "NONE",
    }
    values.update(overrides)

    return values


def storage_env_values(settings, **overrides):
    """The storage settings in .env format, with some values replaced by overrides."""

    values = {
        "OBJECT_STORAGE_ENDPOINT": settings.endpoint,
        "OBJECT_STORAGE_BUCKET": settings.bucket,
        "OBJECT_STORAGE_REGION": settings.region,
        "OBJECT_STORAGE_ACCESS_KEY": settings.access_key.get_secret_value(),
        "OBJECT_STORAGE_SECRET_KEY": settings.secret_key.get_secret_value(),
    }
    values.update(overrides)

    return values


def chroma_env_values(settings, **overrides):
    """The ChromaDB settings in .env format, with some values replaced by overrides."""

    values = {
        "CHROMA_HOST": settings.host,
        "CHROMA_PORT": str(settings.port),
        "CHROMA_COLLECTION": settings.collection,
    }
    values.update(overrides)

    return values


def write_env_file(path, values):

    # Write the values in .env format
    lines = []
    for name, value in values.items():
        lines.append(f"{name}={value}")
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")

    return path


# Chunk and embeddings data

def make_chunk_records(count, document_id="doc001"):

    # Records in the same shape the chunker saves
    records = []
    for index in range(count):
        records.append(
            {
                "chunk_id": f"{document_id}:{index}",
                "text": f"TIN registration step {index} for taxpayers.",
                "metadata": {
                    "document_id": document_id,
                    "department_id": "IRD",
                    "service_id": "TIN_REGISTRATION",
                    "document_type": "service_guideline",
                    "page_number": 1,
                    "chunk_index": index,
                    "chunk_id": f"{document_id}:{index}",
                },
            }
        )

    return records


def topic_records():

    # Three chunks whose texts are TOPIC_TEXTS
    records = make_chunk_records(len(TOPIC_TEXTS))
    for record, text in zip(records, TOPIC_TEXTS):
        record["text"] = text

    return records


def write_chunk_file(folder, records, document_id="doc001"):

    # Save records as a chunk file
    chunks_path = folder / f"{document_id}.chunks.json"
    chunks_path.write_text(json.dumps(records), encoding="utf-8")

    return chunks_path


def make_embedding_file(count, model=TEST_MODEL, dimension=DIMENSION):

    # Build an embeddings file whose records follow make_chunk_records
    records = []
    for record in make_chunk_records(count):
        records.append(
            EmbeddedChunk(
                chunk_id=record["chunk_id"],
                text=record["text"],
                embedding=[0.1] * dimension,
                metadata=record["metadata"],
            )
        )

    embedding_file = EmbeddingFile(document_id="doc001", model=model, dimension=dimension, records=records)

    return embedding_file


def chunk_ids(records):

    # Collect the chunk IDs in order
    ids = []
    for record in records:
        ids.append(record.chunk_id)

    return ids
