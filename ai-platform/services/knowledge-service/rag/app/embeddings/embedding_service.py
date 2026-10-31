"""Embed saved document chunks with the NVIDIA API: batching, retries, and re-run reuse.

Vectors from different models cannot be mixed, so every embeddings file records
the model that produced it.

This module is also the package's public API: names from the other embedding
modules are re-exported here, and app.embeddings re-exports this module.
"""

import logging
import time
from pathlib import Path

from app.embeddings.config import EmbeddingSettings, load_settings
from app.embeddings.exceptions import (
    ChunkFileError,
    EmbeddingAuthError,
    EmbeddingConfigError,
    EmbeddingError,
    InvalidEmbeddingError,
    format_validation_error,
    get_status_code,
    is_auth_error,
    is_retryable,
    short_message,
)
from app.embeddings.models import (
    Chunk,
    EmbeddedChunk,
    Embedder,
    EmbeddingFile,
    EmbeddingFileResult,
    EmbeddingResult,
    FailedBatch,
    VectorStoreBatch,
)
from app.embeddings.storage import (
    EMBEDDINGS_FOLDER,
    embeddings_path_for,
    load_chunks,
    load_embeddings,
    load_reusable_embeddings,
    save_embeddings,
    to_vector_store_batch,
)
from app.embeddings.validators import METADATA_VALUE_TYPES, validate_vectors

logger = logging.getLogger(__name__)

# Public API; existing imports from embedding_service and app.embeddings use these names
__all__ = [
    "EMBEDDINGS_FOLDER",
    "METADATA_VALUE_TYPES",
    "Chunk",
    "ChunkFileError",
    "EmbeddedChunk",
    "Embedder",
    "EmbeddingAuthError",
    "EmbeddingConfigError",
    "EmbeddingError",
    "EmbeddingFile",
    "EmbeddingFileResult",
    "EmbeddingResult",
    "EmbeddingSettings",
    "FailedBatch",
    "InvalidEmbeddingError",
    "VectorStoreBatch",
    "create_embedder",
    "embed_chunk_file",
    "embed_chunks",
    "format_validation_error",
    "load_chunks",
    "load_embeddings",
    "load_settings",
    "save_embeddings",
    "to_vector_store_batch",
]


# Public functions

def create_embedder(settings: EmbeddingSettings) -> Embedder:
    """Create the NVIDIA embedding client from the settings."""

    # Imported here so the client library loads only when embeddings are used
    from langchain_nvidia_ai_endpoints import NVIDIAEmbeddings

    embedding_model = NVIDIAEmbeddings(
        model=settings.model,
        api_key=settings.api_key.get_secret_value(),
        max_batch_size=settings.batch_size,
        # NONE fails on over-long chunks instead of silently cutting their text
        truncate=settings.truncate,
    )

    return Embedder(embedding_model=embedding_model, settings=settings)


def embed_chunks(chunks: list[Chunk], embedder: Embedder) -> EmbeddingResult:
    """Embed chunks in batches.

    A failed batch is recorded and the remaining batches continue, so one bad
    batch never discards the chunks that were embedded successfully. After an
    authentication failure the remaining chunks are marked failed without
    calling the provider again.
    """

    settings = embedder.settings
    result = EmbeddingResult()
    expected_dimension = None

    for start in range(0, len(chunks), settings.batch_size):
        batch = chunks[start:start + settings.batch_size]
        chunk_ids = [chunk.chunk_id for chunk in batch]

        # Embed and check the batch; every vector must have the same dimension
        try:
            vectors = embed_with_retry(embedder, [chunk.text for chunk in batch])
            validate_vectors(vectors, len(batch), expected_dimension)

        # The key is wrong or expired: stop calling the provider
        except EmbeddingAuthError as error:
            remaining_ids = [chunk.chunk_id for chunk in chunks[start:]]
            result.failed.append(failed_batch(remaining_ids, error))
            result.auth_failed = True
            logger.error("embedding stopped: authentication failed", extra={"chunks": len(remaining_ids)})
            break

        # Any other error only fails this batch
        except Exception as error:
            result.failed.append(failed_batch(chunk_ids, error))
            logger.warning(
                "embedding batch failed",
                extra={"chunks": len(chunk_ids), "error_type": type(error).__name__},
            )
            continue

        expected_dimension = len(vectors[0])

        # Keep each vector with its chunk ID, text and metadata
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


def embed_chunk_file(
    chunks_path: str | Path,
    embedder: Embedder,
    output_dir: str | Path,
) -> EmbeddingFileResult:
    """Embed one chunk file and save the successful embeddings.

    Chunks already embedded by the same model with the same text are reused,
    so a re-run only embeds new or previously failed chunks. Raises
    EmbeddingAuthError after saving, so the caller can stop the batch.
    """

    settings = embedder.settings

    # Load the chunks saved by the chunker
    chunks = load_chunks(chunks_path)
    document_id = chunks[0].metadata["document_id"]

    # Split the chunks into those that can be reused and those to embed
    existing = load_reusable_embeddings(embeddings_path_for(output_dir, document_id), settings)
    reused, to_embed = split_reusable(chunks, existing)

    # Embed the remaining chunks
    result = embed_chunks(to_embed, embedder)

    # Save reused and new embeddings together, in chunk order
    records = order_records(chunks, reused + result.embedded)

    embeddings_path = None
    if records:
        embeddings_path = save_embeddings(
            EmbeddingFile(
                document_id=document_id,
                model=settings.model,
                dimension=len(records[0].embedding),
                records=records,
            ),
            output_dir,
        )

    file_result = EmbeddingFileResult(
        document_id=document_id,
        embedded_count=len(result.embedded),
        reused_count=len(reused),
        failed=result.failed,
        embeddings_path=embeddings_path,
    )

    # Stop the batch only after the successful embeddings are safely saved
    if result.auth_failed:
        raise EmbeddingAuthError(result.failed[-1].message)

    return file_result


# Helper functions

def embed_with_retry(embedder: Embedder, texts: list[str]) -> list[list[float]]:
    """Call the provider, retrying only errors that a later attempt can fix."""

    settings = embedder.settings

    for attempt in range(settings.max_retries + 1):

        # The NVIDIA client raises a plain Exception, so every error is classified here
        try:
            return embedder.embedding_model.embed_documents(texts)
        except Exception as error:
            status_code = get_status_code(error)

            # A rejected key fails every batch the same way
            if is_auth_error(status_code):
                raise EmbeddingAuthError(short_message(error)) from None

            # Give up on errors a retry cannot fix, or when retries are used up
            if not is_retryable(status_code, error) or attempt == settings.max_retries:
                raise

            # Wait longer after each failed attempt
            delay = settings.retry_delay_seconds * (2 ** attempt)
            logger.warning(
                "embedding request failed; retrying",
                extra={"attempt": attempt + 1, "status_code": status_code, "delay_seconds": delay},
            )
            time.sleep(delay)

    # Not reached: the last attempt either returns or raises
    raise AssertionError("retry loop ended without a result")


def split_reusable(
    chunks: list[Chunk],
    existing: dict[str, EmbeddedChunk],
) -> tuple[list[EmbeddedChunk], list[Chunk]]:
    """Reuse earlier vectors when the chunk text is unchanged.

    The metadata is taken from the current chunk in case it was corrected.
    """

    reused = []
    to_embed = []

    for chunk in chunks:
        record = existing.get(chunk.chunk_id)

        # Same text: keep the vector, refresh the metadata
        if record is not None and record.text == chunk.text:
            reused.append(
                EmbeddedChunk(
                    chunk_id=chunk.chunk_id,
                    text=chunk.text,
                    embedding=record.embedding,
                    metadata=chunk.metadata,
                )
            )

        # New or changed text: embed it
        else:
            to_embed.append(chunk)

    return reused, to_embed


def order_records(chunks: list[Chunk], records: list[EmbeddedChunk]) -> list[EmbeddedChunk]:

    # Index the embedded records by chunk ID
    records_by_id = {}
    for record in records:
        records_by_id[record.chunk_id] = record

    # Return them in the chunk file's order, skipping chunks that failed
    ordered = []
    for chunk in chunks:
        if chunk.chunk_id in records_by_id:
            ordered.append(records_by_id[chunk.chunk_id])

    return ordered


def failed_batch(chunk_ids: list[str], error: Exception) -> FailedBatch:

    return FailedBatch(
        chunk_ids=chunk_ids,
        error_type=type(error).__name__,
        message=short_message(error),
    )
