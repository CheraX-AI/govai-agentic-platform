"""Tests for app/embeddings/storage.py: chunk files, embeddings files, and vector-store output."""

import json

import pytest
from langchain_core.documents import Document
from langchain_core.vectorstores import InMemoryVectorStore

from app.embeddings.embedding_service import embed_chunk_file
from app.embeddings.exceptions import ChunkFileError, InvalidEmbeddingError
from app.embeddings.storage import (
    embeddings_path_for,
    load_chunks,
    load_embeddings,
    load_reusable_embeddings,
    save_embeddings,
    to_vector_store_batch,
)
from app.embeddings.validators import METADATA_VALUE_TYPES
from app.processing.chunker import chunk_documents, save_chunks
from tests.helpers import (
    TEST_MODEL,
    chunk_ids,
    make_chunk_records,
    make_embedder,
    make_embedding_file,
    make_settings,
    topic_records,
    write_chunk_file,
)


# Loading chunks

def test_load_chunks_reads_chunker_output(tmp_path):

    # Chunk a page with the real chunker and save it
    page = Document(
        page_content="TIN registration is required for every taxpayer. " * 40,
        metadata={
            "document_id": "doc001",
            "source_file": "tin_guide.pdf",
            "department_id": "IRD",
            "service_id": "TIN_REGISTRATION",
            "document_type": "service_guideline",
            "total_pages": 1,
            "page_number": 1,
        },
    )
    saved = chunk_documents([page], chunk_size=300, chunk_overlap=50)
    chunks_path = save_chunks(saved, tmp_path)[0]

    # Load the saved file
    chunks = load_chunks(chunks_path)

    # Every chunk keeps its ID, text, and metadata
    assert len(chunks) == len(saved)
    for loaded, original in zip(chunks, saved):
        assert loaded.chunk_id == original.metadata["chunk_id"]
        assert loaded.text == original.page_content
        assert loaded.metadata == original.metadata


def test_load_chunks_accepts_a_string_path(tmp_path):

    chunks_path = write_chunk_file(tmp_path, make_chunk_records(2))

    chunks = load_chunks(str(chunks_path))

    assert chunk_ids(chunks) == ["doc001:0", "doc001:1"]


def test_missing_chunk_file_is_rejected(tmp_path):

    with pytest.raises(ChunkFileError, match="Chunk file not found"):
        load_chunks(tmp_path / "missing.chunks.json")


@pytest.mark.parametrize("content", ["not json {", ""])
def test_unreadable_chunk_file_is_rejected(tmp_path, content):

    # A file that is not valid JSON cannot be loaded
    chunks_path = tmp_path / "doc001.chunks.json"
    chunks_path.write_text(content, encoding="utf-8")

    with pytest.raises(ChunkFileError, match="Unable to read chunk JSON file"):
        load_chunks(chunks_path)


def test_non_utf8_chunk_file_is_rejected(tmp_path):

    chunks_path = tmp_path / "doc001.chunks.json"
    chunks_path.write_bytes(b"\xff\xfe\x00 not utf-8")

    with pytest.raises(ChunkFileError, match="Unable to read chunk JSON file"):
        load_chunks(chunks_path)


def test_load_chunks_runs_the_validators(tmp_path):

    # Valid JSON with bad content is rejected by the chunk validators
    records = make_chunk_records(2)
    records[1]["chunk_id"] = records[0]["chunk_id"]
    chunks_path = write_chunk_file(tmp_path, records)

    with pytest.raises(ChunkFileError, match="Duplicate chunk ID"):
        load_chunks(chunks_path)


# Saving and loading embeddings

def test_embeddings_path_for():

    embeddings_path = embeddings_path_for("data/output/embeddings", "doc001")

    assert embeddings_path.as_posix() == "data/output/embeddings/doc001.embeddings.json"


def test_saved_embeddings_load_back_unchanged(tmp_path):

    # Save an embeddings file into a folder that does not exist yet
    embedding_file = make_embedding_file(3)
    output_dir = tmp_path / "new" / "embeddings"

    output_path = save_embeddings(embedding_file, output_dir)

    # The file is named by document ID and loads back the same
    assert output_path == output_dir / "doc001.embeddings.json"
    assert load_embeddings(output_path) == embedding_file

    # No temporary file is left behind
    assert not list(output_dir.glob("*.tmp"))


def test_saving_again_replaces_the_file(tmp_path):

    save_embeddings(make_embedding_file(3), tmp_path)
    output_path = save_embeddings(make_embedding_file(1), tmp_path)

    assert len(load_embeddings(output_path).records) == 1


def test_save_rejects_a_wrong_dimension_without_writing(tmp_path):

    # A record whose vector does not match the file dimension
    embedding_file = make_embedding_file(2)
    embedding_file.records[1].embedding = [0.1, 0.2]

    with pytest.raises(InvalidEmbeddingError, match="wrong dimension"):
        save_embeddings(embedding_file, tmp_path)

    assert not (tmp_path / "doc001.embeddings.json").exists()


def test_missing_embeddings_file_is_rejected(tmp_path):

    with pytest.raises(InvalidEmbeddingError, match="Embeddings file not found"):
        load_embeddings(tmp_path / "missing.embeddings.json")


def test_unreadable_embeddings_file_is_rejected(tmp_path):

    embeddings_path = tmp_path / "doc001.embeddings.json"
    embeddings_path.write_text("{ broken", encoding="utf-8")

    with pytest.raises(InvalidEmbeddingError, match="Unable to read embeddings JSON file"):
        load_embeddings(embeddings_path)


def test_embeddings_file_with_missing_fields_is_rejected(tmp_path):

    # Valid JSON that is not an embeddings file
    embeddings_path = tmp_path / "doc001.embeddings.json"
    embeddings_path.write_text(json.dumps({"document_id": "doc001"}), encoding="utf-8")

    with pytest.raises(InvalidEmbeddingError, match="Embeddings file has an invalid structure"):
        load_embeddings(embeddings_path)


def test_load_embeddings_rejects_wrong_dimension(tmp_path):

    # A file edited by hand with a vector of the wrong size
    output_path = save_embeddings(make_embedding_file(2), tmp_path)
    data = json.loads(output_path.read_text(encoding="utf-8"))
    data["records"][1]["embedding"] = [0.1, 0.2]
    output_path.write_text(json.dumps(data), encoding="utf-8")

    with pytest.raises(InvalidEmbeddingError, match="wrong dimension"):
        load_embeddings(output_path)


# Reusing embeddings

def test_nothing_to_reuse_on_the_first_run(tmp_path):

    records = load_reusable_embeddings(tmp_path / "doc001.embeddings.json", make_settings())

    assert records == {}


def test_embeddings_of_the_same_model_are_reused(tmp_path):

    output_path = save_embeddings(make_embedding_file(2), tmp_path)

    records = load_reusable_embeddings(output_path, make_settings())

    assert list(records) == ["doc001:0", "doc001:1"]
    assert records["doc001:1"].text == "TIN registration step 1 for taxpayers."


def test_embeddings_of_another_model_are_not_reused(tmp_path):

    output_path = save_embeddings(make_embedding_file(2, model="old-model"), tmp_path)

    records = load_reusable_embeddings(output_path, make_settings(model=TEST_MODEL))

    assert records == {}


def test_damaged_embeddings_file_is_not_reused(tmp_path):

    output_path = tmp_path / "doc001.embeddings.json"
    output_path.write_text("{ broken", encoding="utf-8")

    records = load_reusable_embeddings(output_path, make_settings())

    assert records == {}


# Vector database output

def test_vector_store_batch_has_chromadb_shape():

    # Build the parallel lists for collection.add()
    batch = to_vector_store_batch(make_embedding_file(3))

    assert batch.ids == ["doc001:0", "doc001:1", "doc001:2"]
    assert len(batch.embeddings) == len(batch.documents) == len(batch.metadatas) == 3
    assert batch.documents[0] == "TIN registration step 0 for taxpayers."

    # Metadata keeps the ownership fields and names the model, using only storable types
    for metadata in batch.metadatas:
        assert metadata["department_id"] == "IRD"
        assert metadata["embedding_model"] == TEST_MODEL
        for value in metadata.values():
            assert isinstance(value, METADATA_VALUE_TYPES)


def test_vector_store_batch_does_not_change_the_saved_records():

    # The model name is added to a copy, not to the record's own metadata
    embedding_file = make_embedding_file(1)

    to_vector_store_batch(embedding_file)

    assert "embedding_model" not in embedding_file.records[0].metadata


def test_embeddings_work_in_a_langchain_vector_store(tmp_path):

    # Embed three chunks on different topics with the real NVIDIA model
    embedder = make_embedder()
    chunks_path = write_chunk_file(tmp_path, topic_records())
    result = embed_chunk_file(chunks_path, embedder, tmp_path / "embeddings")
    batch = to_vector_store_batch(load_embeddings(result.embeddings_path))

    # Load the saved texts and metadata into a LangChain vector store using the same model
    store = InMemoryVectorStore(embedding=embedder.embedding_model)
    documents = []
    for text, metadata in zip(batch.documents, batch.metadatas):
        documents.append(Document(page_content=text, metadata=metadata))
    store.add_documents(documents, ids=batch.ids)

    # Searching with a saved vector finds its own chunk and metadata
    found = store.similarity_search_by_vector(batch.embeddings[2], k=1)[0]

    assert found.id == "doc001:2"
    assert found.metadata["service_id"] == "TIN_REGISTRATION"
    assert found.metadata["embedding_model"] == embedder.settings.model

    # A citizen question finds the TIN chunk
    answer = store.similarity_search("How do I get a TIN number?", k=1)[0]

    assert answer.id == "doc001:0"
