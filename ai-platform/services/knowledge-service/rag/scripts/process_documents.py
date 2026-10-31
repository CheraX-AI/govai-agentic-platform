"""Extract, chunk, and embed every PDF in an input folder.

Usage (from the rag folder):

    python scripts/process_documents.py --department-id IRD --service-id TIN_REGISTRATION \
        --document-type service_guideline

Exit codes: 0 everything succeeded, 1 some PDFs or chunks failed, 2 invalid input or settings.
"""

import argparse
import logging
import sys
from pathlib import Path

# Running "python scripts/process_documents.py" puts only scripts/ on the path, so add rag/
RAG_ROOT = Path(__file__).resolve().parent.parent
if str(RAG_ROOT) not in sys.path:
    sys.path.insert(0, str(RAG_ROOT))

from pydantic import ValidationError

from app.embeddings.embedding_service import (
    EmbeddingConfigError,
    create_embedder,
    format_validation_error,
    load_settings,
)
from app.processing.pdf_extractor import DocumentSource, DocumentType
from app.processing.pipeline import process_documents

EXIT_OK = 0
EXIT_FAILURES = 1
EXIT_INVALID_INPUT = 2

DEFAULT_INPUT_DIR = RAG_ROOT / "data" / "input"
DEFAULT_OUTPUT_DIR = RAG_ROOT / "data" / "output"
DEFAULT_ENV_FILE = RAG_ROOT / ".env"


# Public functions

def main(argv=None):

    # Read the command-line options
    args = parse_arguments(argv)
    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(name)s: %(message)s")

    # Check the ownership metadata applied to every PDF in this run
    try:
        source = DocumentSource(
            department_id=args.department_id,
            service_id=args.service_id,
            document_type=args.document_type,
        )
    except ValidationError as error:
        print(f"Invalid document source: {format_validation_error(error)}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    # Find the PDFs to process
    input_dir = Path(args.input_dir)
    if not input_dir.is_dir():
        print(f"Input folder not found: {input_dir}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    pdf_paths = find_pdfs(input_dir)
    if not pdf_paths:
        print(f"No PDF files found in {input_dir}")
        return EXIT_OK

    # Check the embedding settings before any work, so a missing key fails fast
    embedder = None
    if not args.skip_embeddings:
        try:
            embedder = create_embedder(load_settings(env_file=args.env_file))
        except EmbeddingConfigError as error:
            print(f"Invalid embedding settings: {error}", file=sys.stderr)
            return EXIT_INVALID_INPUT

    # Run extraction, chunking, and embedding
    items = []
    for pdf_path in pdf_paths:
        items.append((pdf_path, source))
    report = process_documents(items, args.output_dir, embedder=embedder)

    # Print a summary and report failures through the exit code
    print_summary(report, embedder)
    if report.failed or report.failed_chunk_count:
        return EXIT_FAILURES

    return EXIT_OK


# Helper functions

def parse_arguments(argv):

    parser = argparse.ArgumentParser(description="Extract, chunk, and embed PDF documents.")
    parser.add_argument("--department-id", required=True, help="Department that owns the documents")
    parser.add_argument("--service-id", default=None, help="Service ID, required for service guidelines")
    parser.add_argument(
        "--document-type",
        required=True,
        choices=[document_type.value for document_type in DocumentType],
    )
    parser.add_argument("--input-dir", default=DEFAULT_INPUT_DIR, help="Folder with the PDFs")
    parser.add_argument("--output-dir", default=DEFAULT_OUTPUT_DIR, help="Folder for generated files")
    parser.add_argument("--env-file", default=DEFAULT_ENV_FILE, help="File with the embedding settings")
    parser.add_argument("--skip-embeddings", action="store_true", help="Only extract and chunk")

    return parser.parse_args(argv)


def find_pdfs(input_dir):

    # Match the extension case-insensitively, e.g. guide.PDF
    pdf_paths = []
    for path in sorted(input_dir.iterdir()):
        if path.is_file() and path.suffix.lower() == ".pdf":
            pdf_paths.append(path)

    return pdf_paths


def print_summary(report, embedder):

    # Totals for the run
    print()
    print(f"Processed PDFs:  {len(report.processed)}")
    print(f"Failed PDFs:     {len(report.failed)}")

    if embedder is not None:
        embedded = 0
        reused = 0
        for document in report.processed:
            embedded += document.embedded_chunk_count
            reused += document.reused_chunk_count
        print(f"Model:           nvidia / {embedder.settings.model}")
        print(f"Chunks embedded: {embedded} (reused {reused})")
        print(f"Chunks failed:   {report.failed_chunk_count}")

    # Details of each failure
    for failure in report.failed:
        print(f"  FAILED {failure.source_file}: {failure.error_type} - {failure.message}")

    for document in report.processed:
        if document.needs_ocr:
            print(f"  NEEDS OCR {document.source_file}: no extractable text")
        if document.failed_chunk_ids:
            print(f"  PARTIAL {document.source_file}: {len(document.failed_chunk_ids)} chunks not embedded")

    if report.embedding_stopped:
        print("  Embedding stopped: NVIDIA rejected the API key. Fix it and run again;")
        print("  chunks that were already embedded are reused.")


if __name__ == "__main__":
    sys.exit(main())
