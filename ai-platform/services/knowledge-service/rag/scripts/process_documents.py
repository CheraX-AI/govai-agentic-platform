"""Extract, chunk, embed, and store in ChromaDB the PDFs stored in SeaweedFS for one department and service.

Upload the PDFs first with scripts/upload_documents.py. Then run (from the rag folder):

    python scripts/process_documents.py --department-id IRD --service-id TIN_REGISTRATION \
        --document-type service_guideline

Only documents whose chunks were all embedded are stored in ChromaDB, so a
partly embedded document never replaces a complete older version.

Exit codes: 0 everything succeeded, 1 some PDFs, chunks, or vector storage failed,
2 invalid input, settings, storage, or vector database.
"""

import argparse
import logging
import sys
import tempfile
from pathlib import Path

# Running "python scripts/process_documents.py" puts only scripts/ on the path, so add rag/
RAG_ROOT = Path(__file__).resolve().parent.parent
if str(RAG_ROOT) not in sys.path:
    sys.path.insert(0, str(RAG_ROOT))

from pydantic import ValidationError

from app.embeddings.embedding_service import (
    EmbeddingConfigError,
    InvalidEmbeddingError,
    create_embedder,
    format_validation_error,
    load_embeddings,
    load_settings,
)
from app.processing.pdf_extractor import DocumentSource, DocumentType
from app.processing.pipeline import FailedDocument, process_documents
from app.storage.config import load_storage_settings
from app.storage.exceptions import StorageError
from app.storage.object_storage import create_document_store
from app.vector_store.config import load_vector_store_settings
from app.vector_store.exceptions import VectorStoreError
from app.vector_store.vector_store import create_vector_store

EXIT_OK = 0
EXIT_FAILURES = 1
EXIT_INVALID_INPUT = 2

# Generated files stay local until the embeddings are stored in ChromaDB (GA-75)
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
            title=args.title,
        )
    except ValidationError as error:
        print(f"Invalid document source: {format_validation_error(error)}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    try:
        # Connect to SeaweedFS and check the bucket before any work
        store = create_document_store(load_storage_settings(env_file=args.env_file))
        store.check_bucket()

        # Find the newest stored version of each PDF for this department and service
        object_keys = store.list_latest_documents(source)

    except StorageError as error:
        print(f"Object storage unavailable: {error}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    if not object_keys:
        print("No stored PDF files found for this department and service")
        return EXIT_OK

    # Check the embedding settings before downloading, so a missing key fails fast
    try:
        embedder = load_embedder(args)
    except EmbeddingConfigError as error:
        print(f"Invalid embedding settings: {error}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    # Connect to ChromaDB before downloading, so a stopped database fails fast
    try:
        vector_store = load_vector_store(args, embedder)
    except VectorStoreError as error:
        print(f"Vector database unavailable: {error}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    # The temporary folder is deleted afterwards, so no government PDFs stay on disk
    download_failures = []
    with tempfile.TemporaryDirectory() as download_dir:
        items = []
        for object_key in object_keys:

            # Download the PDF; one failed download should not stop the batch
            try:
                pdf_path = store.download_document(object_key, download_dir)
            except StorageError as error:
                download_failures.append(
                    FailedDocument(source_file=object_key, error_type=type(error).__name__, message=str(error))
                )
                continue

            # Record where the original is stored, so every chunk points back to it
            document_source = source.model_copy(update={"object_key": object_key})
            items.append((pdf_path, document_source))

        # Run extraction, chunking, and embedding
        report = process_documents(items, args.output_dir, embedder=embedder)

    report.failed.extend(download_failures)

    # Store the fully embedded documents in ChromaDB
    store_results = []
    if vector_store is not None:
        store_results = store_documents(report, vector_store)

    # Print a summary, then report failures through the exit code
    print_summary(report, embedder, vector_store, store_results)
    if report.failed or report.failed_chunk_count:
        return EXIT_FAILURES

    return EXIT_OK


# Helper functions

def parse_arguments(argv):

    parser = argparse.ArgumentParser(description="Extract, chunk, and embed the PDFs stored in SeaweedFS.")
    parser.add_argument("--department-id", required=True, help="Department that owns the documents")
    parser.add_argument("--service-id", default=None, help="Service ID, required for service guidelines")
    parser.add_argument("--title", default=None, help="Document title, added to every chunk's embedding for context")
    parser.add_argument(
        "--document-type",
        required=True,
        choices=[document_type.value for document_type in DocumentType],
    )
    parser.add_argument("--output-dir", default=DEFAULT_OUTPUT_DIR, help="Folder for generated files")
    parser.add_argument("--env-file", default=DEFAULT_ENV_FILE, help="File with the embedding, storage, and ChromaDB settings")
    parser.add_argument("--skip-embeddings", action="store_true", help="Only extract and chunk")
    parser.add_argument(
        "--skip-vector-store",
        action="store_true",
        help="Embed, but do not store the embeddings in ChromaDB",
    )

    return parser.parse_args(argv)


def load_embedder(args):
    """Create the NVIDIA embedder, or return None when embeddings are skipped."""

    if args.skip_embeddings:
        return None

    # Raises EmbeddingConfigError when the settings are missing or invalid
    embedder = create_embedder(load_settings(env_file=args.env_file))

    return embedder


def load_vector_store(args, embedder):
    """Connect to ChromaDB, or return None when there will be no embeddings to store."""

    if embedder is None or args.skip_vector_store:
        return None

    # Raises a VectorStoreError when the settings are invalid or ChromaDB is not reachable
    vector_store = create_vector_store(load_vector_store_settings(env_file=args.env_file))

    return vector_store


def store_documents(report, vector_store):
    """Store each fully embedded document in ChromaDB. Failures are added to the report."""

    store_results = []
    for document in report.processed:

        # Nothing was embedded, e.g. a scanned PDF or a rejected key
        if document.embeddings_path is None:
            continue

        # A partly embedded document must not replace a complete older version
        if document.failed_chunk_ids:
            continue

        try:
            # Read the saved embeddings and store them
            embedding_file = load_embeddings(document.embeddings_path)
            store_results.append(vector_store.store_document(embedding_file))

        except (InvalidEmbeddingError, VectorStoreError) as error:
            report.failed.append(
                FailedDocument(
                    source_file=document.source_file,
                    error_type=type(error).__name__,
                    message=str(error),
                )
            )

    return store_results


def print_summary(report, embedder, vector_store=None, store_results=()):

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

    if vector_store is not None:
        stored_chunks = 0
        old_versions = 0
        for result in store_results:
            stored_chunks += result.stored_count
            old_versions += len(result.removed_old_versions)
        print(f"ChromaDB:        {len(store_results)} documents, {stored_chunks} chunks stored")
        print(f"Old versions:    {old_versions} removed")

        # The count is only informative, so a lost connection here does not fail the run
        try:
            print(f"Collection size: {vector_store.count()} chunks")
        except VectorStoreError:
            print("Collection size: unknown (ChromaDB not reachable)")

    # Details of each failure
    for failure in report.failed:
        print(f"  FAILED {failure.source_file}: {failure.error_type} - {failure.message}")

    for document in report.processed:
        if document.needs_ocr:
            print(f"  NEEDS OCR {document.source_file}: no extractable text")
        if document.failed_chunk_ids:
            print(f"  PARTIAL {document.source_file}: {len(document.failed_chunk_ids)} chunks not embedded")
            if vector_store is not None:
                print(f"  NOT STORED {document.source_file}: run again to embed the rest, then it is stored")

    if report.embedding_stopped:
        print("  Embedding stopped: NVIDIA rejected the API key. Fix it and run again;")
        print("  chunks that were already embedded are reused.")


if __name__ == "__main__":
    sys.exit(main())
