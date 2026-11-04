"""Search the published government documents in ChromaDB with a question.

Usage (from the rag folder):

    python scripts/search_documents.py "How do I register for a TIN?" \
        --department-id IRD --service-id TIN_REGISTRATION --top-k 3

The question is embedded with the same NVIDIA model as the documents, then the
closest chunks of published documents are printed with their similarity and
source. --json prints the full result, as the Agent Orchestrator receives it.

Exit codes: 0 search ran (with or without results), 2 invalid input or
settings, or NVIDIA or ChromaDB unavailable.
"""

import argparse
import sys
from pathlib import Path

# Running "python scripts/search_documents.py" puts only scripts/ on the path, so add rag/
RAG_ROOT = Path(__file__).resolve().parent.parent
if str(RAG_ROOT) not in sys.path:
    sys.path.insert(0, str(RAG_ROOT))

from app.retrieval.exceptions import InvalidQueryError, RetrievalError
from app.retrieval.query import build_query
from app.retrieval.retriever import create_retriever
from app.vector_store.vector_store import DEFAULT_TOP_K

EXIT_OK = 0
EXIT_INVALID_INPUT = 2

DEFAULT_ENV_FILE = RAG_ROOT / ".env"
PREVIEW_LENGTH = 300


# Public functions

def main(argv=None):

    # Read the command-line options
    args = parse_arguments(argv)

    # Check the question and options first, so a mistake fails before any connection
    try:
        build_query(args.question, args.top_k, args.department_id, args.service_id, args.max_distance)
    except InvalidQueryError as error:
        print(f"Invalid search: {error}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    try:
        # Connect to NVIDIA and ChromaDB, then search
        retriever = create_retriever(env_file=args.env_file)
        response = retriever.retrieve(
            args.question,
            top_k=args.top_k,
            department_id=args.department_id,
            service_id=args.service_id,
            max_distance=args.max_distance,
        )

    except RetrievalError as error:
        print(f"Search failed: {error}", file=sys.stderr)
        return EXIT_INVALID_INPUT

    # Print the matches, best first
    if args.json:
        print(response.model_dump_json(indent=2))
    else:
        print_results(response)

    return EXIT_OK


# Helper functions

def parse_arguments(argv):

    parser = argparse.ArgumentParser(description="Search the published document chunks with a question.")
    parser.add_argument("question", help="The question to search for")
    parser.add_argument("--top-k", type=int, default=DEFAULT_TOP_K, help="Number of results (1-50)")
    parser.add_argument("--department-id", default=None, help="Only search this department's documents")
    parser.add_argument("--service-id", default=None, help="Only search this service's documents")
    parser.add_argument(
        "--max-distance",
        type=float,
        default=None,
        help="Drop matches farther away than this cosine distance (0-2)",
    )
    parser.add_argument("--json", action="store_true", help="Print the full result as JSON")
    parser.add_argument("--env-file", default=DEFAULT_ENV_FILE, help="File with the embedding and ChromaDB settings")

    return parser.parse_args(argv)


def print_results(response):

    if not response.results:
        print("No matching chunks found")
        return

    # One block per match: rank, similarity, source, then the start of the text
    for result in response.results:
        print()
        print(f"{result.rank}. similarity {result.similarity:.4f} (distance {result.distance:.4f})")
        print(f"   title:  {result.document_title or '(no title)'}")
        print(f"   source: {result.source_file}, page {result.page_number}")
        print(f"   chunk:  {result.chunk_id}")
        print(f"   stored: {result.object_key or '(no object key)'}")
        preview = " ".join(result.text.split())[:PREVIEW_LENGTH]
        print(f"   {preview}")


if __name__ == "__main__":
    sys.exit(main())
