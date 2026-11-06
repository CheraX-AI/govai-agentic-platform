"""Validation of chunk data, chunk metadata, and embedding vectors."""

import numpy as np
from pydantic import ValidationError

from app.embeddings.exceptions import ChunkFileError, InvalidEmbeddingError, format_validation_error
from app.embeddings.models import Chunk

# ChromaDB accepts only these metadata value types; None, lists and dicts are rejected
METADATA_VALUE_TYPES = (str, int, float, bool)


# Public functions

def validate_chunks(data, file_name):
    """Check the content of one chunk file and return its chunks."""

    # The chunker saves a list of chunks
    if not isinstance(data, list):
        raise ChunkFileError(f"Chunk file {file_name} must contain a list of chunks")

    # An empty file has nothing to embed
    if not data:
        raise ChunkFileError(f"Chunk file {file_name} has no chunks")

    chunks = []
    seen_ids = set()
    document_ids = set()

    for position, item in enumerate(data):

        try:
            # Check the fields of the chunk
            chunk = Chunk.model_validate(item)

        except ValidationError as error:
            raise ChunkFileError(f"Chunk {position} in {file_name} is invalid: {format_validation_error(error)}")

        # A chunk with only whitespace has nothing to embed
        if not chunk.text.strip():
            raise ChunkFileError(f"Chunk {chunk.chunk_id} has empty text")

        # A duplicate ID would overwrite another chunk in the vector database
        if chunk.chunk_id in seen_ids:
            raise ChunkFileError(f"Duplicate chunk ID {chunk.chunk_id}")
        seen_ids.add(chunk.chunk_id)

        # The chunk must belong to the document named in its ID
        document_id = chunk.metadata.get("document_id")
        if not document_id or not chunk.chunk_id.startswith(f"{document_id}:"):
            raise ChunkFileError(f"Chunk {chunk.chunk_id} does not match its document ID")
        document_ids.add(document_id)

        # Metadata must be storable in ChromaDB
        validate_metadata(chunk)

        chunks.append(chunk)

    # One chunk file holds exactly one document
    if len(document_ids) > 1:
        raise ChunkFileError(f"Chunk file {file_name} mixes chunks from several documents")

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


def validate_embedding_file(embedding_file, file_name):

    # Every vector must have the dimension recorded for the file
    for record in embedding_file.records:
        if len(record.embedding) != embedding_file.dimension:
            raise InvalidEmbeddingError(f"Embedding {record.chunk_id} in {file_name} has the wrong dimension")


# Helper functions

def validate_metadata(chunk):

    # Each value must be a type ChromaDB can store
    for key, value in chunk.metadata.items():
        if not isinstance(value, METADATA_VALUE_TYPES):
            raise ChunkFileError(
                f"Chunk {chunk.chunk_id} metadata '{key}' has unsupported type {type(value).__name__}"
            )


def is_number(value):

    # True and False are ints in Python, but they are not vector values
    if isinstance(value, bool):
        return False

    # Only ints and floats are allowed
    is_numeric = isinstance(value, (int, float))

    return is_numeric
