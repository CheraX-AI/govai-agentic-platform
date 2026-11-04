"""Embed a document's chunks with the NVIDIA API: batching and retries.

Vectors from different models cannot be mixed, so every DocumentEmbeddings
records the model that produced it.

This module is also the package's public API: names from the other embedding
modules are re-exported here, and app.embeddings re-exports this module.
"""

import logging
import time

from app.embeddings.config import EmbeddingSettings, load_settings
from app.embeddings.exceptions import (
    EmbeddingAuthError,
    EmbeddingConfigError,
    EmbeddingError,
    InvalidChunkError,
    InvalidEmbeddingError,
    format_validation_error,
    get_status_code,
    is_auth_error,
    is_retryable,
    short_message,
)
from app.embeddings.models import (
    Chunk,
    DocumentEmbeddingResult,
    DocumentEmbeddings,
    EmbeddedChunk,
    Embedder,
    EmbeddingResult,
    FailedBatch,
    VectorStoreBatch,
)
from app.embeddings.storage import to_vector_store_batch
from app.embeddings.validators import METADATA_VALUE_TYPES, validate_chunks, validate_vectors

logger = logging.getLogger(__name__)

# Public API; existing imports from embedding_service and app.embeddings use these names
__all__ = [
    "METADATA_VALUE_TYPES",
    "Chunk",
    "DocumentEmbeddingResult",
    "DocumentEmbeddings",
    "EmbeddedChunk",
    "Embedder",
    "EmbeddingAuthError",
    "EmbeddingConfigError",
    "EmbeddingError",
    "EmbeddingResult",
    "EmbeddingSettings",
    "FailedBatch",
    "InvalidChunkError",
    "InvalidEmbeddingError",
    "VectorStoreBatch",
    "create_embedder",
    "embed_chunks",
    "embed_document",
    "embed_query",
    "format_validation_error",
    "load_settings",
    "to_vector_store_batch",
    "validate_chunks",
]


# Public functions

def create_embedder(settings):
    """Create the NVIDIA embedding client from the settings."""

    # Imported here so the client library loads only when embeddings are used
    from langchain_nvidia_ai_endpoints import NVIDIAEmbeddings

    # Build the NVIDIA client; creating it does not call the API
    embedding_model = NVIDIAEmbeddings(
        model=settings.model,
        api_key=settings.api_key.get_secret_value(),
        max_batch_size=settings.batch_size,
        # NONE fails on over-long chunks instead of silently cutting their text
        truncate=settings.truncate,
    )

    # Keep the client together with its settings
    embedder = Embedder(embedding_model=embedding_model, settings=settings)

    return embedder


def embed_chunks(chunks, embedder):
    """Embed chunks in batches.

    A failed batch is recorded and the remaining batches continue, so one bad
    batch never discards the chunks that were embedded successfully. After an
    authentication failure the remaining chunks are marked failed without
    calling NVIDIA again.
    """

    settings = embedder.settings
    result = EmbeddingResult()
    expected_dimension = None

    for start in range(0, len(chunks), settings.batch_size):

        # Take the next batch of chunks
        batch = chunks[start:start + settings.batch_size]
        chunk_ids = []
        texts = []
        for chunk in batch:
            chunk_ids.append(chunk.chunk_id)
            texts.append(embedding_text(chunk))

        try:
            # Send the batch to NVIDIA
            vectors = embed_with_retry(embedder, texts)

            # Check the vectors; every batch must have the same dimension
            validate_vectors(vectors, len(batch), expected_dimension)

        except EmbeddingAuthError as error:
            # The key is wrong or expired: mark every remaining chunk failed and stop
            remaining_ids = []
            for chunk in chunks[start:]:
                remaining_ids.append(chunk.chunk_id)
            result.failed.append(failed_batch(remaining_ids, error))
            result.auth_failed = True
            logger.error("embedding stopped: authentication failed", extra={"chunks": len(remaining_ids)})
            break

        except Exception as error:
            # Any other error only fails this batch
            result.failed.append(failed_batch(chunk_ids, error))
            logger.warning(
                "embedding batch failed",
                extra={"chunks": len(chunk_ids), "error_type": type(error).__name__},
            )
            continue

        # The first successful batch sets the dimension for the rest
        expected_dimension = len(vectors[0])

        # Keep each vector with its chunk ID, text, and metadata
        for chunk, vector in zip(batch, vectors):
            result.embedded.append(
                EmbeddedChunk(
                    chunk_id=chunk.chunk_id,
                    text=chunk.text,
                    embedding=vector,
                    metadata=chunk.metadata,
                )
            )

    # Log a summary without chunk text
    logger.info(
        "chunks embedded",
        extra={"embedded": len(result.embedded), "failed": len(chunks) - len(result.embedded)},
    )

    return result


def embed_query(embedder, question):
    """Embed one search question with the same model and settings as the documents.

    NVIDIA embeds a question as a "query" and a document chunk as a "passage".
    Both come from the same model, so they can be compared, and a question
    then lands closer to the passages that answer it.

    Raises EmbeddingAuthError when NVIDIA rejects the key, InvalidEmbeddingError
    for an unusable vector, and the NVIDIA client's own error for other failures.
    """

    # Send the question to NVIDIA, retrying errors a later attempt can fix
    vector = call_with_retry(embedder.embedding_model.embed_query, question, embedder.settings)

    # Check the vector like a batch of one: a non-empty list of finite numbers
    validate_vectors([vector], 1, None)

    return vector


def embed_document(chunks, embedder):
    """Embed one document's chunks and return its DocumentEmbeddingResult.

    A failed batch is recorded and the other batches are kept. Nothing is saved
    to disk: the embeddings go straight to the vector store. Raises
    EmbeddingAuthError when NVIDIA rejects the key, so the caller can stop.
    """

    settings = embedder.settings
    document_id = chunks[0].metadata["document_id"]

    # Embed the chunks in batches, in chunk order
    result = embed_chunks(chunks, embedder)

    # A rejected key fails every document the same way, so stop here
    if result.auth_failed:
        raise EmbeddingAuthError(result.failed[-1].message)

    # Keep the embedded chunks together with the model that made them
    embeddings = None
    if result.embedded:
        embeddings = DocumentEmbeddings(
            document_id=document_id,
            model=settings.model,
            dimension=len(result.embedded[0].embedding),
            records=result.embedded,
        )

    document_result = DocumentEmbeddingResult(
        document_id=document_id,
        embeddings=embeddings,
        failed=result.failed,
    )

    return document_result


# Helper functions

def embed_with_retry(embedder, texts):

    # Embed the document texts as passages
    vectors = call_with_retry(embedder.embedding_model.embed_documents, texts, embedder.settings)

    return vectors


def call_with_retry(embed, inputs, settings):
    """Call NVIDIA, retrying only errors that a later attempt can fix."""

    for attempt in range(settings.max_retries + 1):

        try:
            # Send the inputs to NVIDIA
            vectors = embed(inputs)

        except Exception as error:
            # The NVIDIA client raises a plain Exception, so the status is read from its message
            status_code = get_status_code(error)

            # A rejected key fails every batch the same way
            if is_auth_error(status_code):
                raise EmbeddingAuthError(short_message(error))

            # Give up on errors a retry cannot fix, or when retries are used up
            if not is_retryable(status_code, error) or attempt == settings.max_retries:
                raise

            # Wait longer after each failed attempt, then try again
            delay = settings.retry_delay_seconds * (2 ** attempt)
            logger.warning(
                "embedding request failed; retrying",
                extra={"attempt": attempt + 1, "status_code": status_code, "delay_seconds": delay},
            )
            time.sleep(delay)
            continue

        return vectors

    # Not reached: the last attempt either returns or raises
    raise AssertionError("retry loop ended without a result")


def embedding_text(chunk):
    """The text sent to NVIDIA for a chunk: its document title, then the chunk text.

    A chunk deep in a document often does not repeat what the document is about,
    e.g. a page listing company forms never says "TIN". The title gives every
    vector that context. Only the vector uses it; the stored text stays unchanged.
    """

    # Without a title, the chunk text is embedded as it is
    title = chunk.metadata.get("document_title")
    if not title:
        return chunk.text

    # Put the title on its own line before the chunk text
    text = f"{title}\n\n{chunk.text}"

    return text


def failed_batch(chunk_ids, error):

    # Record which chunks failed and why, with a short error message
    batch = FailedBatch(
        chunk_ids=chunk_ids,
        error_type=type(error).__name__,
        message=short_message(error),
    )

    return batch
