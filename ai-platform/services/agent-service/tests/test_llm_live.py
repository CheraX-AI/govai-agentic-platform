"""Tests against the real Azure OpenAI deployment, sending only synthetic text; skipped without the .env settings."""

import pytest
from pydantic import SecretStr

from app.agents.knowledge_agent.prompts import OUT_OF_SCOPE_REPLY, build_rag_messages
from app.llm.exceptions import LLMAuthError
from app.llm.provider import create_llm_provider, invoke_llm
from helpers import SYNTHETIC_QUESTION, SYNTHETIC_SOURCE, TEST_API_KEY, real_settings


def test_model_answers_from_the_document():

    # One synthetic question with one synthetic document
    provider = create_llm_provider(real_settings())
    messages = build_rag_messages(SYNTHETIC_QUESTION, [SYNTHETIC_SOURCE])

    answer = invoke_llm(provider, messages)

    # The answer uses the document without naming it
    assert "TF-1" in answer
    assert "[1]" not in answer


def test_model_says_when_there_are_no_documents():

    # Without documents the model must not answer from its own knowledge
    provider = create_llm_provider(real_settings())
    messages = build_rag_messages(SYNTHETIC_QUESTION, [])

    answer = invoke_llm(provider, messages)

    assert "could not find this information" in answer.lower()


def test_unrelated_question_is_redirected_to_government_services():

    # Retrieval always returns some chunks, so the model must notice the question is off topic
    provider = create_llm_provider(real_settings())
    messages = build_rag_messages("Which cricket bat should I buy for a beginner?", [SYNTHETIC_SOURCE])

    answer = invoke_llm(provider, messages)

    assert OUT_OF_SCOPE_REPLY.split(" 😊")[0] in answer


def test_rejected_key_is_an_auth_error():

    # The real settings with a key Azure rejects
    settings = real_settings().model_copy(update={"api_key": SecretStr(TEST_API_KEY)})
    messages = build_rag_messages(SYNTHETIC_QUESTION, [SYNTHETIC_SOURCE])
    provider = create_llm_provider(settings)

    with pytest.raises(LLMAuthError):
        invoke_llm(provider, messages)
