"""Run PDF extraction, chunking, and optional embedding for each document, in memory.

Each step passes its result straight to the next; nothing is written to disk, so
no government document text stays on the machine. The embedded documents are
returned in the report, ready for the vector store.
"""

import logging
from pathlib import Path

from pydantic import BaseModel

from app.embeddings.embedding_service import (
    DocumentEmbeddings,
    EmbeddingAuthError,
    InvalidChunkError,
    embed_document,
    validate_chunks,
)
from app.processing.chunker import CHUNK_OVERLAP, CHUNK_SIZE, chunk_documents, to_chunk_records
from app.processing.pdf_extractor import PDFExtractionError, extract_pdf, to_langchain_documents

logger = logging.getLogger(__name__)


# Models

class ProcessedDocument(BaseModel):
    """One processed PDF. embeddings is None when it was not embedded (dry run, no text, or every chunk failed)."""

    document_id: str
    source_file: str
    total_pages: int
    chunk_count: int
    needs_ocr: bool
    embedded_chunk_count: int = 0
    failed_chunk_ids: list[str] = []
    embeddings: DocumentEmbeddings | None = None


class FailedDocument(BaseModel):
    source_file: str
    error_type: str
    message: str


class PipelineReport(BaseModel):
    processed: list[ProcessedDocument] = []
    failed: list[FailedDocument] = []
    embedding_stopped: bool = False

    @property
    def failed_chunk_count(self):

        # Count the chunks that could not be embedded across all documents
        count = 0
        for document in self.processed:
            count += len(document.failed_chunk_ids)

        return count


# Public functions

def process_document(pdf_path, source, chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP, embedder=None):
    """Extract and chunk one PDF, then embed its chunks when an embedder is given.

    Raises a PDFExtractionError subclass when the PDF cannot be processed, and
    EmbeddingAuthError when the provider rejects the key.
    """

    # Extract the text of every page
    pdf_path = Path(pdf_path)
    document = extract_pdf(pdf_path, source)

    # Split the pages that have text into chunks; a scanned PDF has none until OCR is added
    pages = to_langchain_documents(document)
    chunks = chunk_documents(pages, chunk_size=chunk_size, chunk_overlap=chunk_overlap)

    result = ProcessedDocument(
        document_id=document.document_id,
        source_file=document.source_file,
        total_pages=document.total_pages,
        chunk_count=len(chunks),
        needs_ocr=document.needs_ocr,
    )

    # Without an embedder (dry run) or without chunks there is nothing more to do
    if embedder is None or not chunks:
        return result

    # Check the chunks, then embed them; failed chunks are recorded, not lost
    chunk_models = validate_chunks(to_chunk_records(chunks), document.source_file)
    embedding = embed_document(chunk_models, embedder)

    result.embeddings = embedding.embeddings
    result.embedded_chunk_count = embedding.embedded_count
    result.failed_chunk_ids = failed_chunk_ids(embedding.failed)

    return result


def process_documents(items, chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP, embedder=None):
    """Process many PDFs, given as (pdf_path, source) pairs.

    A PDF that cannot be processed is recorded as failed and the rest continue.
    If the embedding provider rejects the key, the remaining PDFs are still
    extracted and chunked, but no longer embedded.
    """

    report = PipelineReport()

    for pdf_path, source in items:
        pdf_path = Path(pdf_path)

        # Process the PDF; one bad file should not stop the batch
        try:
            result = process_document(
                pdf_path,
                source,
                chunk_size=chunk_size,
                chunk_overlap=chunk_overlap,
                embedder=embedder,
            )

        # Retrying with the same key cannot succeed, so stop embedding the rest
        except EmbeddingAuthError as error:
            logger.error("embedding stopped: authentication failed", extra={"source_file": pdf_path.name})
            report.failed.append(failed_document(pdf_path, error))
            report.embedding_stopped = True
            embedder = None
            continue

        except (PDFExtractionError, InvalidChunkError) as error:
            logger.warning(
                "pdf skipped",
                extra={"source_file": pdf_path.name, "error_type": type(error).__name__},
            )
            report.failed.append(failed_document(pdf_path, error))
            continue

        report.processed.append(result)

    # Log a summary of the batch
    logger.info(
        "pipeline finished",
        extra={
            "processed": len(report.processed),
            "failed": len(report.failed),
            "failed_chunks": report.failed_chunk_count,
        },
    )

    return report


# Helper functions

def failed_chunk_ids(failed_batches):

    # Collect the chunk IDs of every failed batch
    chunk_ids = []
    for batch in failed_batches:
        chunk_ids.extend(batch.chunk_ids)

    return chunk_ids


def failed_document(pdf_path, error):

    # Record which PDF failed and why
    failure = FailedDocument(
        source_file=pdf_path.name,
        error_type=type(error).__name__,
        message=str(error),
    )

    return failure
