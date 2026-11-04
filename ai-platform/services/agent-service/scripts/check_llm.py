"""Send one synthetic passport question to the configured NVIDIA chat model.

Usage (from the agent-service folder):

    python scripts/check_llm.py

Checks that the LLM settings in .env work: it builds the RAG prompt with one
synthetic test document, sends it to NVIDIA, and prints the answer. When
LANGSMITH_TRACING=true in .env, the call is also sent to LangSmith as one
trace named "knowledge_agent_answer".

Only synthetic text is sent, never real government documents or citizen data.

Exit codes: 0 the model answered, 1 the model call failed, 2 invalid settings.
"""

import argparse
import os
import sys
from pathlib import Path

# Running "python scripts/check_llm.py" puts only scripts/ on the path, so add agent-service/
SERVICE_ROOT = Path(__file__).resolve().parent.parent
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from dotenv import load_dotenv
from langchain_core.tracers.langchain import wait_for_all_tracers

from app.agents.knowledge_agent.prompts import PROMPT_VERSION, build_rag_messages, build_trace_config
from app.llm.exceptions import LLMConfigError, LLMError
from app.llm.provider import invoke_llm, load_llm_provider

EXIT_OK = 0
EXIT_LLM_FAILED = 1
EXIT_INVALID_SETTINGS = 2

DEFAULT_ENV_FILE = SERVICE_ROOT / ".env"

# A clearly fictional document, so no real passport rule is ever stated
SYNTHETIC_SOURCE = {
    "chunk_id": "synthetic-doc:0",
    "text": (
        "SYNTHETIC TEST DOCUMENT - not real government information. "
        "At the test passport counter, applicants hand in the completed test form TF-1."
    ),
    "document_title": "Synthetic passport test guide",
    "source_file": "synthetic_passport_test_guide.pdf",
    "page_number": 1,
}

SYNTHETIC_QUESTION = "what are the type of allowed for get tin number?"


# Public functions

def main(argv=None):

    # Read the command-line options
    args = parse_arguments(argv)

    # Put the .env values, including the LANGSMITH_* settings, into the environment;
    # LangSmith reads only the environment, and real environment variables are kept
    load_dotenv(args.env_file)

    try:
        # Read the LLM settings and create the NVIDIA client
        provider = load_llm_provider(env_file=args.env_file)

    except LLMConfigError as error:
        print(f"Invalid LLM settings: {error}", file=sys.stderr)
        return EXIT_INVALID_SETTINGS

    except LLMError as error:
        print(f"LLM client could not be created: {error}", file=sys.stderr)
        return EXIT_INVALID_SETTINGS

    # Build the prompt from the synthetic question and document
    messages = build_rag_messages(SYNTHETIC_QUESTION, [SYNTHETIC_SOURCE])

    try:
        # Ask the model, labelled as a Knowledge Agent call in LangSmith
        answer = invoke_llm(provider, messages, config=build_trace_config())

    except LLMError as error:
        print(f"LLM call failed ({type(error).__name__}): {error}", file=sys.stderr)
        return EXIT_LLM_FAILED

    finally:
        # Send any pending traces before the script exits
        wait_for_all_tracers()

    # Show the result
    print_result(provider, answer)

    return EXIT_OK


# Helper functions

def parse_arguments(argv):

    parser = argparse.ArgumentParser(description="Check the NVIDIA LLM settings with one synthetic question.")
    parser.add_argument("--env-file", type=Path, default=DEFAULT_ENV_FILE, help="Settings file, default agent-service/.env")

    args = parser.parse_args(argv)

    return args


def print_result(provider, answer):

    # The settings used, without the API key
    settings = provider.settings
    print(f"Model:          {settings.model}")
    print(f"Temperature:    {settings.temperature}")
    print(f"Max tokens:     {settings.max_tokens}")
    print(f"Prompt version: {PROMPT_VERSION}")

    # Where the trace went, if anywhere
    if os.environ.get("LANGSMITH_TRACING", "").strip().lower() == "true":
        project = os.environ.get("LANGSMITH_PROJECT") or "default"
        print(f"LangSmith:      traced to project '{project}'")
    else:
        print("LangSmith:      tracing is off")

    # The model's answer
    print()
    print(f"Question: {SYNTHETIC_QUESTION}")
    print()
    print(answer)


if __name__ == "__main__":
    sys.exit(main())
