"""Test-wide setup: tests never send traces to LangSmith.

Set when this file is imported, before any test module loads LangChain, because
LangSmith reads its environment variables once and caches them. A developer's
own LANGSMITH_TRACING=true therefore never sends test runs to LangSmith.
"""

import os

# Turn off LangSmith tracing (both the current and the older variable name)
os.environ["LANGSMITH_TRACING"] = "false"
os.environ["LANGCHAIN_TRACING_V2"] = "false"
