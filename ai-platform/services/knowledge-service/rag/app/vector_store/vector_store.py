"""Store, read, and search document embeddings in ChromaDB.

Design:

- Adapter: VectorStore wraps one ChromaDB collection. The rest of the RAG
  service calls its methods and never uses chromadb directly, so the vector
  database can be replaced by changing only this module.
- Factory: create_vector_store() connects and opens (or creates) the collection.
- Dependency injection: scripts create one VectorStore and pass it on, and
  tests build one for a separate test collection.

Each record is one chunk: its ID is the chunk_id ("<document_id>:<index>"), its
document is the chunk text, and its metadata is the chunk metadata plus the
embedding model. The object_key in the metadata points to the original PDF in
SeaweedFS.

Storing a document follows three rules:

1. Records are upserted by chunk_id, so running ingestion again updates
   records instead of creating duplicates.
2. Chunks of the same document that were not produced this time (for example
   after a chunk-size change) are removed.
3. Older versions are replaced: records with the same department, service (or
   department guideline), and original file name but another document_id are
   removed. The new records are written first, so a failure never leaves the
   document missing.
"""

import logging

import chromadb
import httpx
from chromadb.config import Settings
from chromadb.errors import ChromaError, NotFoundError
from pydantic import BaseModel

from app.embeddings.storage import to_vector_store_batch
from app.vector_store.exceptions import (
    ChunkNotFoundError,
    CollectionNotFoundError,
    EmbeddingMismatchError,
    VectorStoreConnectionError,
    VectorStoreError,
)

logger = logging.getLogger(__name__)

# Cosine distance suits normalized text embeddings: 0 means the same direction
DISTANCE_SPACE = "cosine"

DEFAULT_TOP_K = 5
MAX_TOP_K = 50


# Models

class StoreResult(BaseModel):
    """What happened when one document was stored."""

    document_id: str
    stored_count: int
    removed_stale_count: int = 0
    removed_old_versions: list[str] = []


class StoredChunk(BaseModel):
    chunk_id: str
    text: str
    metadata: dict


class EmbeddingInfo(BaseModel):
    """The embedding model and vector size of the records in a collection."""

    model: str | None
    dimension: int


class SearchResult(BaseModel):
    """One search match. A smaller distance means a closer match (0 to 2 for cosine)."""

    chunk_id: str
    text: str
    metadata: dict
    distance: float


# Adapter

class VectorStore:
    """Store and search the chunks of government documents in one ChromaDB collection."""

    def __init__(self, client, collection):

        self.client = client
        self.collection = collection

    def store_document(self, embedding_file):
        """Store one document's embeddings, following the three rules in the module docstring."""

        document_id = embedding_file.document_id

        # An empty file has nothing to store
        if not embedding_file.records:
            result = StoreResult(document_id=document_id, stored_count=0)
            return result

        # Vectors from another model cannot be compared with the stored ones
        self.check_embedding_model(embedding_file.model, embedding_file.dimension)

        # Rule 1: write the records, updating any with the same chunk_id
        batch = to_vector_store_batch(embedding_file)
        self.upsert(batch)

        # Rule 2: remove this document's chunks that were not produced this time
        removed_stale_count = self.remove_stale_chunks(document_id, batch.ids)

        # Rule 3: remove older versions of the same document
        removed_old_versions = self.remove_old_versions(document_id, batch.metadatas[0])

        result = StoreResult(
            document_id=document_id,
            stored_count=len(batch.ids),
            removed_stale_count=removed_stale_count,
            removed_old_versions=removed_old_versions,
        )

        logger.info(
            "document stored in vector database",
            extra={
                "document_id": document_id,
                "stored": result.stored_count,
                "removed_stale": removed_stale_count,
                "removed_versions": len(removed_old_versions),
            },
        )

        return result

    def delete_document(self, document_id):
        """Remove every chunk of a document, e.g. when its PDF is withdrawn. Returns the number removed."""

        # Find the document's records
        chunk_ids = self.chunk_ids_where({"document_id": document_id})

        # Remove them
        if chunk_ids:
            run_chroma(self.collection.delete, ids=chunk_ids)

        removed_count = len(chunk_ids)

        return removed_count

    def get_document_chunks(self, document_id):
        """Return a document's chunks in reading order."""

        # Read the records of the document
        response = run_chroma(
            self.collection.get,
            where={"document_id": document_id},
            include=["documents", "metadatas"],
        )

        chunks = to_stored_chunks(response)

        # Chunk order, so the text reads as in the PDF
        chunks.sort(key=chunk_order)

        return chunks

    def get_chunk(self, chunk_id):

        # Read one record by its ID
        response = run_chroma(self.collection.get, ids=[chunk_id], include=["documents", "metadatas"])

        chunks = to_stored_chunks(response)
        if not chunks:
            raise ChunkNotFoundError(f"Chunk not found: {chunk_id}")

        chunk = chunks[0]

        return chunk

    def search(self, query_embedding, top_k=DEFAULT_TOP_K, where=None):
        """Find the chunks closest to a query embedding, closest first.

        where limits the search by metadata; build it with metadata_filter().
        """

        # Keep the number of results within a sensible range
        if not 1 <= top_k <= MAX_TOP_K:
            raise VectorStoreError(f"top_k must be between 1 and {MAX_TOP_K}")

        # Ask ChromaDB for the nearest records
        response = run_chroma(
            self.collection.query,
            query_embeddings=[query_embedding],
            n_results=top_k,
            where=where,
            include=["documents", "metadatas", "distances"],
        )

        # The response holds one list per query; there is one query
        results = []
        for chunk_id, text, metadata, distance in zip(
            response["ids"][0],
            response["documents"][0],
            response["metadatas"][0],
            response["distances"][0],
        ):
            results.append(
                SearchResult(chunk_id=chunk_id, text=text, metadata=dict(metadata), distance=float(distance))
            )

        return results

    def count(self):

        # Number of chunk records in the collection
        record_count = run_chroma(self.collection.count)

        return record_count

    def embedding_info(self):
        """The model and vector size of the stored records, or None when the collection is empty."""

        # Every record has the same model and size (store_document checks it), so one is enough
        sample = run_chroma(self.collection.get, limit=1, include=["metadatas", "embeddings"])
        if not sample["ids"]:
            return None

        info = EmbeddingInfo(
            model=sample["metadatas"][0].get("embedding_model"),
            dimension=len(sample["embeddings"][0]),
        )

        return info

    # Helper methods

    def check_embedding_model(self, model, dimension):
        """Fail when the collection already holds vectors from another model or of another size."""

        # An empty collection accepts any model
        stored = self.embedding_info()
        if stored is None:
            return

        if stored.model != model or stored.dimension != dimension:
            raise EmbeddingMismatchError(
                f"Collection holds vectors from {stored.model} ({stored.dimension} dimensions), "
                f"not {model} ({dimension}); re-embed every document or use a new collection"
            )

    def upsert(self, batch):

        # ChromaDB limits how many records one request may hold
        batch_size = run_chroma(self.client.get_max_batch_size)

        for start in range(0, len(batch.ids), batch_size):
            end = start + batch_size
            run_chroma(
                self.collection.upsert,
                ids=batch.ids[start:end],
                embeddings=batch.embeddings[start:end],
                documents=batch.documents[start:end],
                metadatas=batch.metadatas[start:end],
            )

    def remove_stale_chunks(self, document_id, current_chunk_ids):

        # Chunk IDs stored for this document
        stored_ids = self.chunk_ids_where({"document_id": document_id})

        # Those not produced this time are out of date
        current = set(current_chunk_ids)
        stale_ids = []
        for chunk_id in stored_ids:
            if chunk_id not in current:
                stale_ids.append(chunk_id)

        if stale_ids:
            run_chroma(self.collection.delete, ids=stale_ids)

        removed_count = len(stale_ids)

        return removed_count

    def remove_old_versions(self, document_id, metadata):
        """Remove other versions of the document and return their document IDs."""

        # Records of the same document under another content hash
        response = run_chroma(
            self.collection.get,
            where=old_version_filter(document_id, metadata),
            include=["metadatas"],
        )

        # Collect the old document IDs
        old_document_ids = set()
        for old_metadata in response["metadatas"]:
            old_document_ids.add(old_metadata["document_id"])

        # Remove every chunk of the old versions
        if old_document_ids:
            run_chroma(self.collection.delete, ids=response["ids"])
            logger.info(
                "older document versions removed",
                extra={"document_id": document_id, "old_versions": len(old_document_ids)},
            )

        removed_versions = sorted(old_document_ids)

        return removed_versions

    def chunk_ids_where(self, where):

        # Read only the IDs of the matching records
        response = run_chroma(self.collection.get, where=where, include=[])
        chunk_ids = list(response["ids"])

        return chunk_ids


# Factory

def create_vector_store(settings, create_if_missing=True):
    """Connect to ChromaDB and open the collection.

    Ingestion creates the collection the first time. Search passes
    create_if_missing=False, so a missing collection is reported instead of
    being replaced by a new, empty one.
    """

    try:
        # Telemetry is off so nothing about the documents leaves this machine
        client = chromadb.HttpClient(
            host=settings.host,
            port=settings.port,
            settings=Settings(anonymized_telemetry=False),
        )

        # The service always supplies its own NVIDIA vectors, so no embedding function is set
        if create_if_missing:
            collection = client.get_or_create_collection(
                name=settings.collection,
                configuration={"hnsw": {"space": DISTANCE_SPACE}},
                embedding_function=None,
            )
        else:
            collection = client.get_collection(name=settings.collection, embedding_function=None)

    except NotFoundError:
        raise CollectionNotFoundError(f"Collection {settings.collection} does not exist; process documents first")

    except ValueError:
        # The client raises ValueError when no server answers
        raise VectorStoreConnectionError("ChromaDB is not reachable")

    except httpx.HTTPError:
        raise VectorStoreConnectionError("ChromaDB is not reachable")

    except ChromaError as error:
        raise VectorStoreError(f"ChromaDB request failed: {type(error).__name__}")

    vector_store = VectorStore(client=client, collection=collection)

    return vector_store


# Filters

def metadata_filter(department_id=None, service_id=None, publication_status=None):
    """Build a search filter from optional values, or None for no filter."""

    # Add a condition for each value that is given
    conditions = []
    if publication_status is not None:
        conditions.append({"publication_status": publication_status})
    if department_id is not None:
        conditions.append({"department_id": department_id})
    if service_id is not None:
        conditions.append({"service_id": service_id})

    # ChromaDB needs "$and" for two or more conditions
    if not conditions:
        return None

    if len(conditions) == 1:
        where = conditions[0]
        return where

    where = {"$and": conditions}

    return where


def old_version_filter(document_id, metadata):
    """Match records of the same department, service, and file name, but another document_id."""

    conditions = [
        {"department_id": metadata["department_id"]},
        {"document_type": metadata["document_type"]},
        {"source_file": metadata["source_file"]},
        {"document_id": {"$ne": document_id}},
    ]

    # Department guidelines have no service ID; the document type keeps them apart from services
    if "service_id" in metadata:
        conditions.append({"service_id": metadata["service_id"]})

    where = {"$and": conditions}

    return where


# Helper functions

def run_chroma(operation, **arguments):
    """Call a ChromaDB method and turn its errors into project errors."""

    try:
        # Send the request
        result = operation(**arguments)

    except httpx.HTTPError:
        raise VectorStoreConnectionError("ChromaDB is not reachable")

    except ChromaError as error:
        raise VectorStoreError(f"ChromaDB request failed: {type(error).__name__}")

    return result


def to_stored_chunks(response):

    # Pair each ID with its text and metadata
    chunks = []
    for chunk_id, text, metadata in zip(response["ids"], response["documents"], response["metadatas"]):
        chunks.append(StoredChunk(chunk_id=chunk_id, text=text, metadata=dict(metadata)))

    return chunks


def chunk_order(chunk):

    # Order by chunk_index; records without one go last
    order = chunk.metadata.get("chunk_index", float("inf"))

    return order
