"""Tests for app/agents/knowledge_agent/agent.py with a fake RAG client and a fake chat model."""

import pytest
from langchain_core.language_models.fake_chat_models import FakeListChatModel

from app.agents.knowledge_agent.agent import KnowledgeAgent, cited_references
from app.agents.knowledge_agent.prompts import (
    AVAILABLE_DOCUMENTS_SCOPE,
    INSUFFICIENT_INFORMATION_REPLY,
    PROMPT_VERSION,
)
from app.agents.knowledge_agent.schemas import KnowledgeSource, ServiceScope
from app.agents.knowledge_agent.validation import InvalidQuestionError
from app.llm.provider import LLMProvider
from app.tools.knowledge_rag_client import KnowledgeServiceUnavailableError
from helpers import SYNTHETIC_QUESTION, SYNTHETIC_SOURCE, make_settings


# Fakes

class FakeRagClient:
    """Answers like KnowledgeRagClient.retrieve and records each call."""

    def __init__(self, sources=(), error=None):

        self.sources = list(sources)
        self.error = error
        self.calls = []

    def retrieve(self, question, department_id=None, service_id=None, request_id=None):

        self.calls.append(
            {"question": question, "department_id": department_id, "service_id": service_id, "request_id": request_id}
        )
        if self.error is not None:
            raise self.error

        return self.sources


class RecordingChatModel(FakeListChatModel):
    """A fake chat model that counts its calls."""

    calls: int = 0

    def invoke(self, messages, config=None, **kwargs):

        self.calls += 1

        return super().invoke(messages, config=config, **kwargs)


def make_source(number):

    # Synthetic sources with different titles and pages
    source = KnowledgeSource(
        **{
            **SYNTHETIC_SOURCE,
            "chunk_id": f"doc:{number}",
            "document_title": f"Synthetic guide {number}",
            "page_number": number,
        }
    )

    return source


def make_agent(sources=(), replies=("The answer [1].",), rag_error=None, scope=AVAILABLE_DOCUMENTS_SCOPE):
    """An agent with a fake RAG client and a fake chat model; returns the agent, client, and model."""

    rag_client = FakeRagClient(sources=sources, error=rag_error)
    chat_model = RecordingChatModel(responses=list(replies))
    provider = LLMProvider(chat_model=chat_model, settings=make_settings())
    agent = KnowledgeAgent(provider=provider, rag_client=rag_client, service_scope=scope)

    return agent, rag_client, chat_model


# Answering

def test_answer_uses_the_retrieved_documents():

    agent, _, chat_model = make_agent(sources=[make_source(1)], replies=["Hand in form TF-1 [1]."])

    answer = agent.answer(SYNTHETIC_QUESTION)

    assert answer.answer == "Hand in form TF-1 [1]."
    assert answer.scope == AVAILABLE_DOCUMENTS_SCOPE.scope_id
    assert answer.prompt_version == PROMPT_VERSION
    assert chat_model.calls == 1


def test_only_cited_sources_are_returned():

    sources = [make_source(1), make_source(2), make_source(3)]
    agent, _, _ = make_agent(sources=sources, replies=["First [3], then [1]."])

    answer = agent.answer(SYNTHETIC_QUESTION)

    assert [source.number for source in answer.sources] == [1, 3]
    assert answer.sources[1].document_title == "Synthetic guide 3"
    assert answer.sources[1].page_number == 3


def test_no_documents_gives_the_fixed_reply_without_calling_the_model():

    # The model must never answer from its own knowledge
    agent, _, chat_model = make_agent(sources=[])

    answer = agent.answer(SYNTHETIC_QUESTION)

    assert answer.answer == INSUFFICIENT_INFORMATION_REPLY
    assert answer.sources == []
    assert chat_model.calls == 0


def test_default_scope_searches_every_document():

    agent, rag_client, _ = make_agent(sources=[make_source(1)])

    agent.answer(SYNTHETIC_QUESTION, request_id="abc123")

    assert rag_client.calls == [
        {"question": SYNTHETIC_QUESTION, "department_id": None, "service_id": None, "request_id": "abc123"}
    ]


def test_a_narrower_scope_filters_the_search():

    scope = ServiceScope(scope_id="test_scope", description="Synthetic services", department_id="TEST", service_id="TEST_SERVICE")
    agent, rag_client, _ = make_agent(sources=[make_source(1)], scope=scope)

    answer = agent.answer(SYNTHETIC_QUESTION)

    assert rag_client.calls[0]["department_id"] == "TEST"
    assert rag_client.calls[0]["service_id"] == "TEST_SERVICE"
    assert answer.scope == "test_scope"


def test_question_is_cleaned_before_retrieval():

    agent, rag_client, _ = make_agent(sources=[])

    agent.answer("  What   do I\n hand in?  ")

    assert rag_client.calls[0]["question"] == "What do I hand in?"


@pytest.mark.parametrize("question", ["   ", "hi", "x" * 1001])
def test_unsearchable_question_is_rejected_before_retrieval(question):

    # The knowledge service would reject these, so it is never called
    agent, rag_client, chat_model = make_agent()

    with pytest.raises(InvalidQuestionError):
        agent.answer(question)

    assert rag_client.calls == []
    assert chat_model.calls == 0


def test_retrieval_errors_reach_the_caller():

    # No answer is made without the documents
    agent, _, chat_model = make_agent(rag_error=KnowledgeServiceUnavailableError("Knowledge service is not reachable"))

    with pytest.raises(KnowledgeServiceUnavailableError):
        agent.answer(SYNTHETIC_QUESTION)

    assert chat_model.calls == 0


# Citations

def test_citations_outside_the_sources_are_ignored():

    references = cited_references("See [0], [2], and [9].", [make_source(1), make_source(2)])

    assert [reference.number for reference in references] == [2]


def test_repeated_citations_are_listed_once():

    references = cited_references("Yes [1]. Also [1].", [make_source(1)])

    assert len(references) == 1


def test_answer_without_citations_has_no_sources():

    references = cited_references(INSUFFICIENT_INFORMATION_REPLY, [make_source(1)])

    assert references == []
