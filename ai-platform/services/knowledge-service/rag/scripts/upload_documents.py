"""Upload PDF files to SeaweedFS object storage.

Usage (from the rag folder), with one or more PDF files from any folder:

    python scripts/upload_documents.py --department-id IRD --service-id TIN_REGISTRATION \
        --document-type service_guideline --file "C:/Users/me/Downloads/TIN_guide.pdf"

A PDF that is already stored is skipped, so running the script again is safe.
Process the stored PDFs afterwards with scripts/process_documents.py.

Exit codes: 0 everything succeeded, 1 some PDFs failed, 2 invalid input, settings, or storage.
"""

import argparse
import logging
import sys
from pathlib import Path

# Running "python scripts/upload_documents.py" puts only scripts/ on the path, so add rag/
RAG_ROOT = Path(__file__).resolve().parent.parent
if str(RAG_ROOT) not in sys.path:
    sys.path.insert(0, str(RAG_ROOT))

from pydantic import ValidationError

from app.embeddings.exceptions import format_validation_error
from app.processing.pdf_extractor import DocumentSource, DocumentType, PDFExtractionError
from app.storage.config import load_storage_settings
from app.storage.exceptions import StorageError
from app.storage.object_storage import create_document_store

EXIT_OK = 0
EXIT_FAILURES = 1
EXIT_INVALID_INPUT = 2

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

    try:
        # Connect to SeaweedFS and check the bucket before uploading anything
        store = create_document_store(load_storage_settings(env_file=args.env_file))
        store.check_bucket()

    except StorageError as error:
        print(f"Object storage unavailable: {error}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    # Upload each PDF and print what happened to it
    print()
    failed_count = 0
    for file_name in args.file:
        uploaded = upload_one(store, Path(file_name), source)
        if not uploaded:
            failed_count += 1

    # Report failures through the exit code
    print(f"\nPDFs: {len(args.file)}, failed: {failed_count}")
    if failed_count:
        return EXIT_FAILURES

    return EXIT_OK


# Helper functions

def parse_arguments(argv):

    parser = argparse.ArgumentParser(description="Upload PDF files to SeaweedFS object storage.")
    parser.add_argument("--department-id", required=True, help="Department that owns the documents")
    parser.add_argument("--service-id", default=None, help="Service ID, required for service guidelines")
    parser.add_argument(
        "--document-type",
        required=True,
        choices=[document_type.value for document_type in DocumentType],
    )
    parser.add_argument("--file", required=True, nargs="+", help="One or more PDF files to upload")
    parser.add_argument("--env-file", default=DEFAULT_ENV_FILE, help="File with the storage settings")

    return parser.parse_args(argv)


def upload_one(store, pdf_path, source):
    """Upload one PDF and print the result. Returns False when it failed."""

    try:
        # Upload the PDF, or skip it when it is already stored
        stored = store.upload_document(pdf_path, source)

    except PDFExtractionError as error:
        print(f"  FAILED   {pdf_path.name}: {error}")
        return False

    except StorageError as error:
        print(f"  FAILED   {pdf_path.name}: {type(error).__name__} - {error}")
        return False

    # Show where the PDF is stored
    if stored.uploaded:
        print(f"  UPLOADED {pdf_path.name} -> {stored.object_key}")
    else:
        print(f"  SKIPPED  {pdf_path.name} (already stored as {stored.object_key})")

    return True


if __name__ == "__main__":
    sys.exit(main())
