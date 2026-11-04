"""Tests against the real NVIDIA chat API, with the settings in agent-service/.env.

Skipped when NVIDIA_API_KEY or the LLM_* settings are missing. Only the
synthetic test document and question are sent, never real government
documents or citizen data. Tracing is off in tests (see conftest.py).
"""

import pytest
from pydantic import SecretStr

from app.agents.knowledge_agent.prompts import build_rag_messages
from app.llm.exceptions import LLMAuthError
from app.llm.provider import create_llm_provider, invoke_llm
from helpers import SYNTHETIC_QUESTION, SYNTHETIC_SOURCE, TEST_NVIDIA_KEY, real_settings


def test_model_answers_from_the_document():

    # One synthetic question with one synthetic document
    provider = create_llm_provider(real_settings())
    messages = build_rag_messages(SYNTHETIC_QUESTION, [SYNTHETIC_SOURCE])

    answer = invoke_llm(provider, messages)

    # The answer uses the document and cites it
    assert "TF-1" in answer
    assert "[1]" in answer


def test_model_says_when_there_are_no_documents():

    # Without documents the model must not answer from its own knowledge
    provider = create_llm_provider(real_settings())
    messages = build_rag_messages(SYNTHETIC_QUESTION, [])

    answer = invoke_llm(provider, messages)

    assert "could not find this information" in answer.lower()


def test_rejected_key_is_an_auth_error():

    # The real settings with a key NVIDIA rejects
    settings = real_settings().model_copy(update={"api_key": SecretStr(TEST_NVIDIA_KEY)})
    messages = build_rag_messages(SYNTHETIC_QUESTION, [SYNTHETIC_SOURCE])

    # Unknown models fail while the client is created, known ones on the first call
    with pytest.raises(LLMAuthError):
        provider = create_llm_provider(settings)
        invoke_llm(provider, messages)
