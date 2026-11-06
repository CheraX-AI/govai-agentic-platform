"""Split extracted PDF pages into overlapping chunks for embedding."""

import json
import logging
from pathlib import Path

from langchain_text_splitters import RecursiveCharacterTextSplitter

logger = logging.getLogger(__name__)

CHUNK_SIZE = 1000
CHUNK_OVERLAP = 200

# Split on paragraphs first, then lines, sentences, words, and finally characters
SEPARATORS = ["\n\n", "\n", ". ", " ", ""]


# Errors

class ChunkingError(Exception):
    """The chunk settings or input documents are invalid."""


# Public functions

def chunk_documents(documents, chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP):
    """Split page Documents into chunks that keep the page metadata.

    Each page is split on its own, so a chunk never spans two pages and its
    page_number is always correct.
    """

    # Check the chunk settings
    validate_chunk_settings(chunk_size, chunk_overlap)

    # Nothing to split
    if not documents:
        return []

    # Create the LangChain text splitter
    splitter = RecursiveCharacterTextSplitter(
        chunk_size=chunk_size,
        chunk_overlap=chunk_overlap,
        separators=SEPARATORS,
        add_start_index=True,
    )

    # Count chunks separately for each document
    chunk_counts = {}
    chunks = []

    for page in documents:

        # Every page must come from the PDF extractor with a document_id
        document_id = page.metadata.get("document_id")
        if not document_id:
            raise ChunkingError("Page is missing document_id metadata")

        # Split the page and number each chunk within its document
        for chunk in splitter.split_documents([page]):

            # Skip chunks that contain only whitespace
            text = chunk.page_content.strip()
            if not text:
                continue

            chunk_index = chunk_counts.get(document_id, 0)
            chunk_counts[document_id] = chunk_index + 1

            # A stable ID lets the same chunk be replaced instead of duplicated
            chunk.page_content = text
            chunk.metadata["chunk_index"] = chunk_index
            chunk.metadata["chunk_id"] = f"{document_id}:{chunk_index}"
            chunk.metadata["char_count"] = len(text)
            chunks.append(chunk)

    # Log a summary without document content
    logger.info(
        "documents chunked",
        extra={
            "documents": len(chunk_counts),
            "pages": len(documents),
            "chunks": len(chunks),
        },
    )

    return chunks


def save_chunks(chunks, output_dir):
    """Save chunks as one UTF-8 JSON file per document, named by document_id."""

    # Create output directory
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    # Group chunks by the document they came from
    chunks_by_document = {}
    for chunk in chunks:
        document_id = chunk.metadata["document_id"]
        chunks_by_document.setdefault(document_id, []).append(
            {
                "chunk_id": chunk.metadata["chunk_id"],
                "text": chunk.page_content,
                "metadata": chunk.metadata,
            }
        )

    # Save each document's chunks as JSON
    output_paths = []
    for document_id, document_chunks in chunks_by_document.items():
        output_path = output_dir / f"{document_id}.chunks.json"
        output_path.write_text(
            json.dumps(document_chunks, ensure_ascii=False, indent=2),
            encoding="utf-8"
        )
        output_paths.append(output_path)

    return output_paths


# Helper functions

def validate_chunk_settings(chunk_size, chunk_overlap):

    # Chunks must have a positive size
    if chunk_size <= 0:
        raise ChunkingError("chunk_size must be greater than 0")

    # Overlap must be smaller than the chunk so the splitter always moves forward
    if chunk_overlap < 0 or chunk_overlap >= chunk_size:
        raise ChunkingError("chunk_overlap must be at least 0 and smaller than chunk_size")
