"""Tests for document text chunking."""

import pytest
from langchain_core.documents import Document

from app.processing.chunker import (
    ChunkingError,
    chunk_documents,
    to_chunk_records,
)


def test_short_document():

    # A short document should produce one chunk
    documents = [
        Document(
            page_content="TIN registration information.",
            metadata={
                "document_id": "doc001",
                "page_number": 1
            }
        )
    ]

    chunks = chunk_documents(documents)

    assert len(chunks) == 1
    assert chunks[0].page_content == "TIN registration information."


def test_long_document():

    # A long document should produce multiple chunks
    text = "TIN registration is required for taxpayers. " * 50

    documents = [
        Document(
            page_content=text,
            metadata={
                "document_id": "doc001",
                "page_number": 1
            }
        )
    ]

    chunks = chunk_documents(
        documents,
        chunk_size=100,
        chunk_overlap=20
    )

    assert len(chunks) > 1

    for chunk in chunks:
        assert len(chunk.page_content) <= 100


def test_metadata_preservation():

    # Check that document ownership metadata is preserved
    documents = [
        Document(
            page_content="TIN registration guidelines.",
            metadata={
                "document_id": "doc001",
                "department_id": "IRD",
                "service_id": "TIN_REGISTRATION",
                "document_type": "service_guideline",
                "page_number": 2
            }
        )
    ]

    chunks = chunk_documents(documents)

    assert len(chunks) == 1
    assert chunks[0].metadata["document_id"] == "doc001"
    assert chunks[0].metadata["department_id"] == "IRD"
    assert chunks[0].metadata["service_id"] == "TIN_REGISTRATION"
    assert chunks[0].metadata["document_type"] == "service_guideline"
    assert chunks[0].metadata["page_number"] == 2


def test_multiple_documents():

    # Check that multiple PDF pages are processed separately
    documents = [
        Document(
            page_content="First page content.",
            metadata={
                "document_id": "doc001",
                "page_number": 1
            }
        ),
        Document(
            page_content="Second page content.",
            metadata={
                "document_id": "doc001",
                "page_number": 2
            }
        )
    ]

    chunks = chunk_documents(
        documents,
        chunk_size=100,
        chunk_overlap=20
    )

    assert len(chunks) == 2
    assert chunks[0].metadata["page_number"] == 1
    assert chunks[1].metadata["page_number"] == 2


def test_empty_documents():

    # An empty document list should return no chunks
    chunks = chunk_documents([])

    assert chunks == []


def test_invalid_chunk_size():

    # Chunk size must be greater than zero
    with pytest.raises(ChunkingError):
        chunk_documents([], chunk_size=0)


def test_invalid_chunk_overlap():

    # Overlap cannot be equal to or greater than chunk size
    with pytest.raises(ChunkingError):
        chunk_documents(
            [],
            chunk_size=100,
            chunk_overlap=100
        )


def test_negative_chunk_overlap():

    # Overlap cannot be negative
    with pytest.raises(ChunkingError):
        chunk_documents(
            [],
            chunk_size=100,
            chunk_overlap=-10
        )


def test_missing_document_id():

    # Every page must have a document ID
    documents = [
        Document(
            page_content="TIN registration information.",
            metadata={
                "page_number": 1
            }
        )
    ]

    with pytest.raises(ChunkingError):
        chunk_documents(documents)


def test_chunk_metadata():

    # Check automatically generated chunk metadata
    documents = [
        Document(
            page_content="TIN registration information.",
            metadata={
                "document_id": "doc001",
                "page_number": 1
            }
        )
    ]

    chunks = chunk_documents(documents)

    assert chunks[0].metadata["chunk_index"] == 0
    assert chunks[0].metadata["chunk_id"] == "doc001:0"
    assert chunks[0].metadata["char_count"] == len(chunks[0].page_content)


def test_chunk_ids_are_unique():

    # Each chunk should have a unique ID
    text = "TIN registration is required for taxpayers. " * 50

    documents = [
        Document(
            page_content=text,
            metadata={
                "document_id": "doc001",
                "page_number": 1
            }
        )
    ]

    chunks = chunk_documents(
        documents,
        chunk_size=100,
        chunk_overlap=20
    )

    chunk_ids = [chunk.metadata["chunk_id"] for chunk in chunks]

    assert len(chunk_ids) == len(set(chunk_ids))


def test_chunk_records():

    # Chunks become chunk_id, text, and metadata records for the embedder
    documents = [
        Document(
            page_content="TIN registration information.",
            metadata={
                "document_id": "doc001",
                "page_number": 1
            }
        )
    ]

    chunks = chunk_documents(documents)

    records = to_chunk_records(chunks)

    assert len(records) == 1
    assert records[0]["text"] == "TIN registration information."
    assert records[0]["chunk_id"] == "doc001:0"
    assert records[0]["metadata"]["document_id"] == "doc001"


def test_multiple_document_ids():

    # Check that different documents are processed independently
    documents = [
        Document(
            page_content="TIN registration guidelines.",
            metadata={
                "document_id": "doc001",
                "page_number": 1
            }
        ),
        Document(
            page_content="Income tax guidelines.",
            metadata={
                "document_id": "doc002",
                "page_number": 1
            }
        )
    ]

    chunks = chunk_documents(documents)

    assert len(chunks) == 2

    assert chunks[0].metadata["chunk_id"] == "doc001:0"
    assert chunks[1].metadata["chunk_id"] == "doc002:0"
