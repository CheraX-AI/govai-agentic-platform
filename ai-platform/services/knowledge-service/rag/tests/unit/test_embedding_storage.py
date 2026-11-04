"""Tests for app/embeddings/storage.py: the vector-store input built from a document's embeddings."""

from langchain_core.documents import Document
from langchain_core.vectorstores import InMemoryVectorStore

from app.embeddings.embedding_service import embed_document
from app.embeddings.storage import to_vector_store_batch
from app.embeddings.validators import METADATA_VALUE_TYPES, validate_chunks
from tests.helpers import TEST_MODEL, make_document_embeddings, make_embedder, topic_records


# Vector database output

def test_vector_store_batch_has_chromadb_shape():

    # Build the parallel lists for collection.add()
    batch = to_vector_store_batch(make_document_embeddings(3))

    assert batch.ids == ["doc001:0", "doc001:1", "doc001:2"]
    assert len(batch.embeddings) == len(batch.documents) == len(batch.metadatas) == 3
    assert batch.documents[0] == "TIN registration step 0 for taxpayers."

    # Metadata keeps the ownership fields and names the model, using only storable types
    for metadata in batch.metadatas:
        assert metadata["department_id"] == "IRD"
        assert metadata["embedding_model"] == TEST_MODEL
        for value in metadata.values():
            assert isinstance(value, METADATA_VALUE_TYPES)


def test_vector_store_batch_does_not_change_the_records():

    # The model name is added to a copy, not to the record's own metadata
    document_embeddings = make_document_embeddings(1)

    to_vector_store_batch(document_embeddings)

    assert "embedding_model" not in document_embeddings.records[0].metadata


def test_embeddings_work_in_a_langchain_vector_store():

    # Embed three chunks on different topics with the real NVIDIA model
    embedder = make_embedder()
    chunks = validate_chunks(topic_records(), "topics")
    result = embed_document(chunks, embedder)
    batch = to_vector_store_batch(result.embeddings)

    # Load the texts and metadata into a LangChain vector store using the same model
    store = InMemoryVectorStore(embedding=embedder.embedding_model)
    documents = []
    for text, metadata in zip(batch.documents, batch.metadatas):
        documents.append(Document(page_content=text, metadata=metadata))
    store.add_documents(documents, ids=batch.ids)

    # Searching with a stored vector finds its own chunk and metadata
    found = store.similarity_search_by_vector(batch.embeddings[2], k=1)[0]

    assert found.id == "doc001:2"
    assert found.metadata["service_id"] == "TIN_REGISTRATION"
    assert found.metadata["embedding_model"] == embedder.settings.model

    # A citizen question finds the TIN chunk
    answer = store.similarity_search("How do I get a TIN number?", k=1)[0]

    assert answer.id == "doc001:0"
