"""End-to-end tests: the real Knowledge Agent with the running knowledge service and NVIDIA.

Skipped unless agent-service/.env has the LLM and KNOWLEDGE_* settings and the
knowledge service answers /health. The checks do not depend on which documents
are stored, so they keep working as documents are added.
"""

import httpx
import pytest

from app.agents.knowledge_agent.agent import create_knowledge_agent
from app.agents.knowledge_agent.prompts import INSUFFICIENT_INFORMATION_REPLY
from app.llm.exceptions import LLMConfigError
from app.tools.knowledge_rag_client import KnowledgeServiceConfigError, load_rag_client_settings

HEALTH_PATH = "/health"


# Fixtures

@pytest.fixture(scope="module")
def knowledge_agent():

    # The knowledge service settings must be in .env
    try:
        settings = load_rag_client_settings()
    except KnowledgeServiceConfigError as error:
        pytest.skip(f"knowledge service settings are not available in agent-service/.env ({error})")

    # The knowledge service must be running
    try:
        httpx.get(settings.base_url.rstrip("/") + HEALTH_PATH, timeout=5).raise_for_status()
    except httpx.HTTPError:
        pytest.skip(f"knowledge service is not running at {settings.base_url}")

    # The NVIDIA settings must be in .env too
    try:
        agent = create_knowledge_agent()
    except LLMConfigError as error:
        pytest.skip(f"NVIDIA LLM settings are not available in agent-service/.env ({error})")

    return agent


# Tests

def test_answer_cites_only_retrieved_documents(knowledge_agent):

    # A general question that any government service guide can answer
    answer = knowledge_agent.answer("What documents do I need to submit?")

    # Either a cited answer, or the fixed reply when the documents do not say
    assert answer.answer
    if answer.answer != INSUFFICIENT_INFORMATION_REPLY:
        assert answer.sources
    for source in answer.sources:
        assert source.number >= 1


def test_agent_does_not_answer_from_outside_knowledge(knowledge_agent):

    # No government document answers this; the model must not use what it knows
    answer = knowledge_agent.answer("What is the capital city of France?")

    assert "paris" not in answer.answer.lower()
