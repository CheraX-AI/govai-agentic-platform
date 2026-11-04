"""Data models for the embedder, chunks, embeddings, results, and vector-store output."""

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


class DocumentEmbeddings(BaseModel):
    """The embedded chunks of one document, in chunk order, ready for the vector store."""

    document_id: str
    model: str
    dimension: int = Field(ge=1)
    records: list[EmbeddedChunk]


class DocumentEmbeddingResult(BaseModel):
    """What happened when one document was embedded.

    embeddings is None when no chunk could be embedded.
    """

    document_id: str
    embeddings: DocumentEmbeddings | None = None
    failed: list[FailedBatch] = []

    @property
    def embedded_count(self):

        # Chunks that have a vector
        count = len(self.embeddings.records) if self.embeddings is not None else 0

        return count

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
