"""Read chunk files, read and write embeddings files, and build vector-store input."""

import json
import logging
from pathlib import Path

from pydantic import ValidationError

from app.embeddings.exceptions import ChunkFileError, InvalidEmbeddingError
from app.embeddings.models import EmbeddingFile, VectorStoreBatch
from app.embeddings.validators import validate_chunks, validate_embedding_file

logger = logging.getLogger(__name__)

EMBEDDINGS_FOLDER = "embeddings"


# Public functions

def load_chunks(chunks_path):
    """Read and validate chunks from a JSON file."""

    chunks_path = Path(chunks_path)

    try:
        # Open and read the JSON file
        with open(chunks_path, "r", encoding="utf-8") as file:
            data = json.load(file)

    except FileNotFoundError:
        raise ChunkFileError("Chunk file not found")

    except (OSError, UnicodeDecodeError, json.JSONDecodeError):
        raise ChunkFileError("Unable to read chunk JSON file")

    # Validate the chunk data
    chunks = validate_chunks(data, chunks_path.name)

    return chunks


def embeddings_path_for(output_dir, document_id):

    # Build the embeddings file path for a document
    embeddings_path = Path(output_dir) / f"{document_id}.embeddings.json"

    return embeddings_path


def save_embeddings(embedding_file, output_dir):
    """Save embeddings as a <document_id>.embeddings.json file."""

    # Create the output file path
    output_path = embeddings_path_for(output_dir, embedding_file.document_id)

    # Check that every vector has the file's dimension
    validate_embedding_file(embedding_file, output_path.name)

    # Create output directory
    output_path.parent.mkdir(parents=True, exist_ok=True)

    # Write to a temporary file first so a crash never leaves a half-written file
    temporary_path = output_path.with_suffix(".tmp")
    with open(temporary_path, "w", encoding="utf-8") as file:
        file.write(embedding_file.model_dump_json())

    # Replace the old file with the new one
    temporary_path.replace(output_path)

    return output_path


def load_embeddings(embeddings_path):
    """Read and validate embeddings from a JSON file."""

    embeddings_path = Path(embeddings_path)

    try:
        # Open and read the JSON file
        with open(embeddings_path, "r", encoding="utf-8") as file:
            data = json.load(file)

        # Check the structure of the embeddings file
        embedding_file = EmbeddingFile.model_validate(data)

    except FileNotFoundError:
        raise InvalidEmbeddingError("Embeddings file not found")

    except (OSError, UnicodeDecodeError, json.JSONDecodeError):
        raise InvalidEmbeddingError("Unable to read embeddings JSON file")

    except ValidationError:
        raise InvalidEmbeddingError("Embeddings file has an invalid structure")

    # Validate the vector dimensions
    validate_embedding_file(embedding_file, embeddings_path.name)

    return embedding_file


def load_reusable_embeddings(output_path, settings):
    """Return earlier embeddings by chunk ID, or nothing when they cannot be reused."""

    # Nothing to reuse on the first run
    if not output_path.exists():
        return {}

    try:
        # Read the embeddings saved by an earlier run
        embedding_file = load_embeddings(output_path)

    except InvalidEmbeddingError:
        # A damaged file is ignored; its chunks are embedded again
        logger.warning("existing embeddings file is invalid; re-embedding", extra={"file": output_path.name})
        return {}

    # Vectors from another model cannot be mixed with new ones
    if embedding_file.model != settings.model:
        logger.info("embedding model changed; re-embedding", extra={"file": output_path.name})
        return {}

    # Index the saved embeddings by chunk ID
    records = {}
    for record in embedding_file.records:
        records[record.chunk_id] = record

    return records


def to_vector_store_batch(embedding_file):
    """Convert saved embeddings into the lists a vector database add() call takes."""

    # Create empty lists for the vector database
    batch = VectorStoreBatch(ids=[], embeddings=[], documents=[], metadatas=[])

    # Add each record, keeping the four lists in the same order
    for record in embedding_file.records:

        # Record the embedding model so stored vectors can be traced to it
        metadata = dict(record.metadata)
        metadata["embedding_model"] = embedding_file.model

        batch.ids.append(record.chunk_id)
        batch.embeddings.append(record.embedding)
        batch.documents.append(record.text)
        batch.metadatas.append(metadata)

    return batch
