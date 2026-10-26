"""Run PDF extraction and chunking, saving the results of each step."""

import logging
from pathlib import Path

from pydantic import BaseModel

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


class FailedDocument(BaseModel):
    source_file: str
    error_type: str
    message: str


class PipelineReport(BaseModel):
    processed: list[ProcessedDocument] = []
    failed: list[FailedDocument] = []


# Public functions

def process_document(pdf_path, source, output_dir, chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP):
    """Extract, chunk, and save one PDF.

    Raises a PDFExtractionError subclass when the PDF cannot be processed.
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

    return ProcessedDocument(
        document_id=document.document_id,
        source_file=document.source_file,
        total_pages=document.total_pages,
        chunk_count=len(chunks),
        needs_ocr=document.needs_ocr,
        extracted_path=extracted_path,
        chunks_path=chunks_path,
    )


def process_documents(items, output_dir, chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP):
    """Process many PDFs, given as (pdf_path, source) pairs.

    A PDF that cannot be processed is recorded as failed and the rest continue.
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
            )
        except PDFExtractionError as error:
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
        extra={"processed": len(report.processed), "failed": len(report.failed)},
    )

    return report
