"""Find the stored document chunks that best answer a question.

Design:

- Facade: Retriever.retrieve() is the one call a caller needs. It hides the
  NVIDIA embedder, ChromaDB, and the filters, so the search script today and
  the Agent Orchestrator later use retrieval the same way.
- Factory: create_retriever() reads .env and builds the embedder and the
  vector store.
- Dependency injection: Retriever takes the embedder and the vector store, so
  tests can pass fakes or a test collection.

Steps for each question:

1. Clean and check the question and options (SearchQuery).
2. Check the collection: an empty one has no results, and its records must
   come from the same embedding model as the query.
3. Embed the question with the same NVIDIA model as the documents.
4. Search ChromaDB, always limited to published documents, and to the given
   department and service.
5. Drop matches beyond max_distance and number the rest, best first.
"""

import logging

from app.embeddings.config import ENV_FILE
from app.embeddings.embedding_service import (
    EmbeddingAuthError,
    EmbeddingConfigError,
    InvalidEmbeddingError,
    create_embedder,
    embed_query,
    load_settings,
)
from app.embeddings.exceptions import short_message
from app.processing.pdf_extractor import PublicationStatus
from app.retrieval.exceptions import (
    QueryEmbeddingError,
    RetrievalConfigError,
    RetrievalUnavailableError,
)
from app.retrieval.models import RetrievalResponse, RetrievedChunk
from app.retrieval.query import build_query
from app.vector_store.config import load_vector_store_settings
from app.vector_store.exceptions import VectorStoreConfigError, VectorStoreConnectionError, VectorStoreError
from app.vector_store.vector_store import DEFAULT_TOP_K, create_vector_store, metadata_filter

logger = logging.getLogger(__name__)


# Facade

class Retriever:
    """Semantic search over the published government documents in one ChromaDB collection."""

    def __init__(self, embedder, vector_store):

        self.embedder = embedder
        self.vector_store = vector_store

    def retrieve(self, question, top_k=DEFAULT_TOP_K, department_id=None, service_id=None, max_distance=None):
        """Return the chunks closest to the question, best first.

        No match is a normal result with an empty list. Every failure raises a
        RetrievalError subclass.
        """

        # Clean and check the inputs before any service is called
        query = build_query(question, top_k, department_id, service_id, max_distance)

        # An empty collection has nothing to find, so NVIDIA is not called
        stored = self.check_collection()
        if stored is None:
            response = self.build_response(query, [])
            return response

        # Turn the question into a vector with the documents' model
        query_embedding = self.embed_question(query.question)
        if len(query_embedding) != stored.dimension:
            raise RetrievalConfigError(
                f"Query vectors have {len(query_embedding)} dimensions, stored vectors {stored.dimension}"
            )

        # Search only published documents; callers cannot turn this filter off
        where = metadata_filter(
            department_id=query.department_id,
            service_id=query.service_id,
            publication_status=PublicationStatus.PUBLISHED.value,
        )
        matches = run_vector_store(self.vector_store.search, query_embedding, top_k=query.top_k, where=where)

        # Keep the close enough matches and build the response
        results = to_retrieved_chunks(matches, query.max_distance)
        response = self.build_response(query, results)

        # Log the outcome without the question, which may hold personal details
        logger.info(
            "semantic search finished",
            extra={
                "question_length": len(query.question),
                "top_k": query.top_k,
                "department_id": query.department_id,
                "service_id": query.service_id,
                "matches": len(matches),
                "results": len(results),
            },
        )

        return response

    # Helper methods

    def check_collection(self):
        """Return the stored model and vector size, or None for an empty collection."""

        # Read one stored record
        stored = run_vector_store(self.vector_store.embedding_info)
        if stored is None:
            return None

        # Vectors from different models cannot be compared
        query_model = self.embedder.settings.model
        if stored.model != query_model:
            raise RetrievalConfigError(
                f"Documents were embedded with {stored.model}, but queries use {query_model}; "
                "set EMBEDDING_MODEL to the documents' model or re-process the documents"
            )

        return stored

    def embed_question(self, question):

        try:
            # Embed the question as a query
            query_embedding = embed_query(self.embedder, question)

        except EmbeddingAuthError:
            raise QueryEmbeddingError("NVIDIA rejected the API key")

        except InvalidEmbeddingError as error:
            raise QueryEmbeddingError(f"NVIDIA returned an unusable vector: {error}")

        except Exception as error:
            # The NVIDIA client raises plain exceptions; keep only a short message
            raise QueryEmbeddingError(f"Question could not be embedded: {short_message(error)}")

        return query_embedding

    def build_response(self, query, results):

        # The cleaned question and the options used, with the results
        response = RetrievalResponse(
            question=query.question,
            top_k=query.top_k,
            department_id=query.department_id,
            service_id=query.service_id,
            max_distance=query.max_distance,
            embedding_model=self.embedder.settings.model,
            results=results,
        )

        return response


# Factory

def create_retriever(env_file=ENV_FILE):
    """Build a Retriever from the NVIDIA and ChromaDB settings in .env."""

    try:
        # Create the same NVIDIA embedder that embedded the documents
        embedder = create_embedder(load_settings(env_file=env_file))

        # Open the existing collection; search never creates an empty one
        vector_store = create_vector_store(load_vector_store_settings(env_file=env_file), create_if_missing=False)

    except EmbeddingConfigError as error:
        raise RetrievalConfigError(f"Invalid embedding settings: {error}")

    except VectorStoreConfigError as error:
        raise RetrievalConfigError(f"Invalid ChromaDB settings: {error}")

    except VectorStoreError as error:
        # ChromaDB is not reachable, or the collection does not exist
        raise RetrievalUnavailableError(str(error))

    retriever = Retriever(embedder=embedder, vector_store=vector_store)

    return retriever


# Helper functions

def run_vector_store(operation, *arguments, **keyword_arguments):
    """Call a VectorStore method and turn its errors into retrieval errors."""

    try:
        # Send the request to ChromaDB
        result = operation(*arguments, **keyword_arguments)

    except VectorStoreConnectionError:
        raise RetrievalUnavailableError("ChromaDB is not reachable")

    except VectorStoreError as error:
        raise RetrievalUnavailableError(f"Search failed: {error}")

    return result


def to_retrieved_chunks(matches, max_distance=None):
    """Turn ChromaDB matches into numbered results, dropping those beyond max_distance."""

    results = []
    for match in matches:

        # Too far away to be relevant
        if max_distance is not None and match.distance > max_distance:
            continue

        # Copy the source details from the chunk metadata
        metadata = match.metadata
        results.append(
            RetrievedChunk(
                rank=len(results) + 1,
                chunk_id=match.chunk_id,
                document_id=metadata.get("document_id"),
                text=match.text,
                document_title=metadata.get("document_title"),
                source_file=metadata.get("source_file"),
                page_number=metadata.get("page_number"),
                department_id=metadata.get("department_id"),
                service_id=metadata.get("service_id"),
                document_type=metadata.get("document_type"),
                object_key=metadata.get("object_key"),
                distance=match.distance,
                similarity=1 - match.distance,
            )
        )

    return results
