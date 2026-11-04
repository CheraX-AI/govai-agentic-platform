"""Build the vector-store input from a document's embeddings."""

from app.embeddings.models import VectorStoreBatch


# Public functions

def to_vector_store_batch(document_embeddings):
    """Convert a document's embeddings into the lists a vector database add() call takes."""

    # Create empty lists for the vector database
    batch = VectorStoreBatch(ids=[], embeddings=[], documents=[], metadatas=[])

    # Add each record, keeping the four lists in the same order
    for record in document_embeddings.records:

        # Record the embedding model so stored vectors can be traced to it
        metadata = dict(record.metadata)
        metadata["embedding_model"] = document_embeddings.model

        batch.ids.append(record.chunk_id)
        batch.embeddings.append(record.embedding)
        batch.documents.append(record.text)
        batch.metadatas.append(metadata)

    return batch
