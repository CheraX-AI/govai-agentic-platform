"""Run PDF extraction, chunking, and optional embedding, saving the results of each step."""

import logging
from pathlib import Path

from pydantic import BaseModel

from app.embeddings.embedding_service import (
    EMBEDDINGS_FOLDER,
    ChunkFileError,
    EmbeddingAuthError,
    embed_chunk_file,
)
from app.processing.chunker import CHUNK_OVERLAP, CHUNK_SIZE, chunk_documents, save_chunks
from app.processing.pdf_extractor import (
    PDFExtractionError,
    extract_pdf,
    save_extracted_document,
    to_langchain_documents,
)

logger = logging.getLogger(__name__)

EXTRACTED_FOLDER = "extracted"
CHUNKS_FOLDER = "chunks"


# Models

class ProcessedDocument(BaseModel):
    document_id: str
    source_file: str
    total_pages: int
    chunk_count: int
    needs_ocr: bool
    extracted_path: Path
    chunks_path: Path | None = None
    embedded_chunk_count: int = 0
    reused_chunk_count: int = 0
    failed_chunk_ids: list[str] = []
    embeddings_path: Path | None = None


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

def process_document(
    pdf_path,
    source,
    output_dir,
    chunk_size=CHUNK_SIZE,
    chunk_overlap=CHUNK_OVERLAP,
    embedder=None,
):
    """Extract, chunk, and save one PDF, then embed its chunks when an embedder is given.

    Raises a PDFExtractionError subclass when the PDF cannot be processed, and
    EmbeddingAuthError (after saving what succeeded) when the provider rejects the key.
    """

    # Extract the text of every page
    pdf_path = Path(pdf_path)
    output_dir = Path(output_dir)
    document = extract_pdf(pdf_path, source)

    # Save the extracted pages with their metadata
    extracted_path = save_extracted_document(document, output_dir / EXTRACTED_FOLDER)

    # Split the pages that have text into chunks
    pages = to_langchain_documents(document)
    chunks = chunk_documents(pages, chunk_size=chunk_size, chunk_overlap=chunk_overlap)

    # Save the chunks; a scanned PDF has none until OCR is added
    chunks_path = None
    if chunks:
        chunks_path = save_chunks(chunks, output_dir / CHUNKS_FOLDER)[0]

    result = ProcessedDocument(
        document_id=document.document_id,
        source_file=document.source_file,
        total_pages=document.total_pages,
        chunk_count=len(chunks),
        needs_ocr=document.needs_ocr,
        extracted_path=extracted_path,
        chunks_path=chunks_path,
    )

    # Embed the saved chunks; failed chunks are recorded, not lost
    if embedder is not None and chunks_path is not None:
        embedding = embed_chunk_file(chunks_path, embedder, output_dir / EMBEDDINGS_FOLDER)
        result.embedded_chunk_count = embedding.embedded_count
        result.reused_chunk_count = embedding.reused_count
        result.failed_chunk_ids = embedding.failed_chunk_ids
        result.embeddings_path = embedding.embeddings_path

    return result


def process_documents(
    items,
    output_dir,
    chunk_size=CHUNK_SIZE,
    chunk_overlap=CHUNK_OVERLAP,
    embedder=None,
):
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
                output_dir,
                chunk_size=chunk_size,
                chunk_overlap=chunk_overlap,
                embedder=embedder,
            )

        # Retrying with the same key cannot succeed, so stop embedding the rest
        except EmbeddingAuthError as error:
            logger.error("embedding stopped: authentication failed", extra={"source_file": pdf_path.name})
            report.failed.append(
                FailedDocument(
                    source_file=pdf_path.name,
                    error_type=type(error).__name__,
                    message=str(error),
                )
            )
            report.embedding_stopped = True
            embedder = None
            continue

        except (PDFExtractionError, ChunkFileError) as error:
            logger.warning(
                "pdf skipped",
                extra={"source_file": pdf_path.name, "error_type": type(error).__name__},
            )
            report.failed.append(
                FailedDocument(
                    source_file=pdf_path.name,
                    error_type=type(error).__name__,
                    message=str(error),
                )
            )
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
