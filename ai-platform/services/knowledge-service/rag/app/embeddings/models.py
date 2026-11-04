"""Data models for the embedder, chunks, embeddings, results, and vector-store output."""

from pathlib import Path

from langchain_core.embeddings import Embeddings
from pydantic import BaseModel, ConfigDict, Field

from app.embeddings.config import EmbeddingSettings


# Models

class Embedder(BaseModel):
    """An embedding model together with the settings used to run it."""

    model_config = ConfigDict(arbitrary_types_allowed=True)

    embedding_model: Embeddings
    settings: EmbeddingSettings


class Chunk(BaseModel):
    chunk_id: str = Field(min_length=1)
    text: str = Field(min_length=1)
    metadata: dict


class EmbeddedChunk(BaseModel):
    chunk_id: str
    text: str
    embedding: list[float]
    metadata: dict


class FailedBatch(BaseModel):
    chunk_ids: list[str]
    error_type: str
    message: str


class EmbeddingResult(BaseModel):
    embedded: list[EmbeddedChunk] = []
    failed: list[FailedBatch] = []
    auth_failed: bool = False


class EmbeddingFile(BaseModel):
    """Embeddings of one document, as saved to <document_id>.embeddings.json."""

    document_id: str
    model: str
    dimension: int = Field(ge=1)
    records: list[EmbeddedChunk]


class EmbeddingFileResult(BaseModel):
    document_id: str
    embedded_count: int
    reused_count: int
    failed: list[FailedBatch] = []
    embeddings_path: Path | None = None

    @property
    def failed_chunk_ids(self):

        # Collect the chunk IDs of every failed batch
        chunk_ids = []
        for batch in self.failed:
            chunk_ids.extend(batch.chunk_ids)

        return chunk_ids


class VectorStoreBatch(BaseModel):
    """Parallel lists in the shape ChromaDB's collection.add() expects."""

    ids: list[str]
    embeddings: list[list[float]]
    documents: list[str]
    metadatas: list[dict]
