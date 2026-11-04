"""Search the stored document chunks in ChromaDB with a question, to check retrieval.

Usage (from the rag folder):

    python scripts/search_documents.py "How do I register for a TIN?" --department-id IRD --top-k 3

The question is embedded with the same NVIDIA model as the documents, then the
closest chunks are printed with their distance and source.

Exit codes: 0 search ran (with or without results), 2 invalid input or settings, or ChromaDB unavailable.
"""

import argparse
import sys
from pathlib import Path

# Running "python scripts/search_documents.py" puts only scripts/ on the path, so add rag/
RAG_ROOT = Path(__file__).resolve().parent.parent
if str(RAG_ROOT) not in sys.path:
    sys.path.insert(0, str(RAG_ROOT))

from app.embeddings.embedding_service import EmbeddingConfigError, create_embedder, load_settings
from app.embeddings.exceptions import short_message
from app.vector_store.config import load_vector_store_settings
from app.vector_store.exceptions import VectorStoreError
from app.vector_store.vector_store import DEFAULT_TOP_K, create_vector_store, metadata_filter

EXIT_OK = 0
EXIT_INVALID_INPUT = 2

DEFAULT_ENV_FILE = RAG_ROOT / ".env"
PREVIEW_LENGTH = 300


# Public functions

def main(argv=None):

    # Read the command-line options
    args = parse_arguments(argv)

    question = args.question.strip()
    if not question:
        print("The question is empty", file=sys.stderr)
        return EXIT_INVALID_INPUT

    try:
        # Connect to NVIDIA and ChromaDB
        embedder = create_embedder(load_settings(env_file=args.env_file))
        vector_store = create_vector_store(load_vector_store_settings(env_file=args.env_file))

    except EmbeddingConfigError as error:
        print(f"Invalid embedding settings: {error}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    except VectorStoreError as error:
        print(f"Vector database unavailable: {error}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    try:
        # Embed the question as a query, with the same model as the documents
        query_embedding = embedder.embedding_model.embed_query(question)

    except Exception as error:
        # The NVIDIA client raises plain exceptions; show only a short message
        print(f"Question could not be embedded: {short_message(error)}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    try:
        # Find the closest chunks, limited to the given department and service
        where = metadata_filter(department_id=args.department_id, service_id=args.service_id)
        results = vector_store.search(query_embedding, top_k=args.top_k, where=where)

    except VectorStoreError as error:
        print(f"Search failed: {error}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    # Print the matches, closest first
    print_results(results)

    return EXIT_OK


# Helper functions

def parse_arguments(argv):

    parser = argparse.ArgumentParser(description="Search the stored document chunks with a question.")
    parser.add_argument("question", help="The question to search for")
    parser.add_argument("--top-k", type=int, default=DEFAULT_TOP_K, help="Number of results (1-50)")
    parser.add_argument("--department-id", default=None, help="Only search this department's documents")
    parser.add_argument("--service-id", default=None, help="Only search this service's documents")
    parser.add_argument("--env-file", default=DEFAULT_ENV_FILE, help="File with the embedding and ChromaDB settings")

    return parser.parse_args(argv)


def print_results(results):

    if not results:
        print("No matching chunks found")
        return

    # One block per match: rank, distance, source, then the start of the text
    for rank, result in enumerate(results, start=1):
        metadata = result.metadata
        print()
        print(f"{rank}. distance {result.distance:.4f}  {metadata.get('source_file')}  page {metadata.get('page_number')}")
        print(f"   chunk:  {result.chunk_id}")
        print(f"   stored: {metadata.get('object_key', '(no object key)')}")
        preview = " ".join(result.text.split())[:PREVIEW_LENGTH]
        print(f"   {preview}")


if __name__ == "__main__":
    sys.exit(main())
