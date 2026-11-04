"""Validation of chunk data, chunk metadata, and embedding vectors."""

import numpy as np
from pydantic import ValidationError

from app.embeddings.exceptions import InvalidChunkError, InvalidEmbeddingError, format_validation_error
from app.embeddings.models import Chunk

# ChromaDB accepts only these metadata value types; None, lists and dicts are rejected
METADATA_VALUE_TYPES = (str, int, float, bool)


# Public functions

def validate_chunks(data, document_name):
    """Check the chunks of one document and return them as Chunk models."""

    # The chunks come as a list of dicts
    if not isinstance(data, list):
        raise InvalidChunkError(f"Chunks of {document_name} must be a list")

    # No chunks means nothing to embed
    if not data:
        raise InvalidChunkError(f"{document_name} has no chunks")

    chunks = []
    seen_ids = set()
    document_ids = set()

    for position, item in enumerate(data):

        try:
            # Check the fields of the chunk
            chunk = Chunk.model_validate(item)

        except ValidationError as error:
            raise InvalidChunkError(f"Chunk {position} of {document_name} is invalid: {format_validation_error(error)}")

        # A chunk with only whitespace has nothing to embed
        if not chunk.text.strip():
            raise InvalidChunkError(f"Chunk {chunk.chunk_id} has empty text")

        # A duplicate ID would overwrite another chunk in the vector database
        if chunk.chunk_id in seen_ids:
            raise InvalidChunkError(f"Duplicate chunk ID {chunk.chunk_id}")
        seen_ids.add(chunk.chunk_id)

        # The chunk must belong to the document named in its ID
        document_id = chunk.metadata.get("document_id")
        if not document_id or not chunk.chunk_id.startswith(f"{document_id}:"):
            raise InvalidChunkError(f"Chunk {chunk.chunk_id} does not match its document ID")
        document_ids.add(document_id)

        # Metadata must be storable in ChromaDB
        validate_metadata(chunk)

        chunks.append(chunk)

    # The chunks must all come from the same document
    if len(document_ids) > 1:
        raise InvalidChunkError(f"Chunks of {document_name} come from several documents")

    return chunks


def validate_vectors(vectors, expected_count, expected_dimension):
    """Check the vectors returned for one batch before they are stored."""

    # One vector per chunk
    if not isinstance(vectors, list) or len(vectors) != expected_count:
        raise InvalidEmbeddingError(f"Expected {expected_count} vectors from the embedding model")

    # The first vector sets the dimension when no earlier batch has
    if expected_dimension is None and vectors and isinstance(vectors[0], list):
        expected_dimension = len(vectors[0])

    for vector in vectors:

        # Every vector is a non-empty list
        if not isinstance(vector, list) or not vector:
            raise InvalidEmbeddingError("Embedding is not a list of numbers")

        # Every vector has the same dimension
        if len(vector) != expected_dimension:
            raise InvalidEmbeddingError(f"Embedding dimension {len(vector)} does not match {expected_dimension}")

        # Only real numbers; numpy would silently turn True or "0.1" into numbers
        for value in vector:
            if not is_number(value):
                raise InvalidEmbeddingError("Embedding contains a value that is not a number")

        # NaN or infinity would break similarity search
        values = np.array(vector, dtype=float)
        if not np.all(np.isfinite(values)):
            raise InvalidEmbeddingError("Embedding contains a value that is not a finite number")


# Helper functions

def validate_metadata(chunk):

    # Each value must be a type ChromaDB can store
    for key, value in chunk.metadata.items():
        if not isinstance(value, METADATA_VALUE_TYPES):
            raise InvalidChunkError(
                f"Chunk {chunk.chunk_id} metadata '{key}' has unsupported type {type(value).__name__}"
            )


def is_number(value):

    # True and False are ints in Python, but they are not vector values
    if isinstance(value, bool):
        return False

    # Only ints and floats are allowed
    is_numeric = isinstance(value, (int, float))

    return is_numeric
