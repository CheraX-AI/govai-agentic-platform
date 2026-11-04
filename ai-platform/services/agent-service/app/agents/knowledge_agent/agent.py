"""The Knowledge Agent: answers a citizen question from the knowledge-service documents.

Steps for each question:

1. Ask the knowledge service (RAG) for the chunks that match the question.
2. No chunks: reply that the information is not available, without calling
   the LLM, so the model can never answer from its own knowledge.
3. Put the question and the numbered chunks into the RAG prompt and send it
   to the NVIDIA chat model.
4. Return the answer with only the documents it cited.

Design:

- Dependency injection: the agent takes the LLM provider and the RAG client,
  so tests pass fakes and the graph shares one of each.
- Factory: create_knowledge_agent() builds both from .env.
"""

import logging
import re
from pathlib import Path

from app.agents.knowledge_agent.prompts import (
    AVAILABLE_DOCUMENTS_SCOPE,
    INSUFFICIENT_INFORMATION_REPLY,
    PROMPT_VERSION,
    build_rag_messages,
    build_trace_config,
    clean_question,
)
from app.agents.knowledge_agent.schemas import KnowledgeAnswer, SourceReference
from app.llm.provider import invoke_llm, load_llm_provider
from app.tools.knowledge_rag_client import create_rag_client

logger = logging.getLogger(__name__)

ENV_FILE = Path(__file__).resolve().parents[3] / ".env"

# A citation in the answer text, e.g. [2]
CITATION_PATTERN = re.compile(r"\[(\d+)\]")


# Agent

class KnowledgeAgent:
    """Answers questions from the documents the knowledge service returns."""

    def __init__(self, provider, rag_client, service_scope=AVAILABLE_DOCUMENTS_SCOPE):

        self.provider = provider
        self.rag_client = rag_client
        self.service_scope = service_scope

    def answer(self, question, request_id=None):
        """Answer one question.

        Raises InvalidQuestionError for an empty, too short, or too long question
        (before any service is called), a KnowledgeServiceError when the
        documents cannot be retrieved, and an LLMError when the model fails, so an
        answer is never made without the documents.
        """

        # Check the question before any service is called
        question = clean_question(question)

        # Find the matching chunks, limited to the scope's department and service if it has them
        sources = self.rag_client.retrieve(
            question,
            department_id=self.service_scope.department_id,
            service_id=self.service_scope.service_id,
            request_id=request_id,
        )

        # Nothing was found: say so, without asking the model
        if not sources:
            answer = self.build_answer(INSUFFICIENT_INFORMATION_REPLY, [])
            self.log_answer(request_id, retrieved=0, cited=0)
            return answer

        # Build the prompt and ask the model, labelled for LangSmith
        messages = build_rag_messages(question, sources, self.service_scope)
        answer_text = invoke_llm(self.provider, messages, config=build_trace_config(self.service_scope))

        # Keep only the documents the answer cites
        answer = self.build_answer(answer_text, cited_references(answer_text, sources))
        self.log_answer(request_id, retrieved=len(sources), cited=len(answer.sources))

        return answer

    # Helper methods

    def build_answer(self, answer_text, sources):

        # The answer with the scope and prompt version that produced it
        answer = KnowledgeAnswer(
            answer=answer_text,
            sources=sources,
            scope=self.service_scope.scope_id,
            prompt_version=PROMPT_VERSION,
        )

        return answer

    def log_answer(self, request_id, retrieved, cited):

        # Log the outcome without the question or answer text
        logger.info(
            "knowledge answer generated",
            extra={
                "request_id": request_id,
                "scope": self.service_scope.scope_id,
                "retrieved": retrieved,
                "cited": cited,
            },
        )


# Public functions

def create_knowledge_agent(env_file=ENV_FILE):
    """Build the Knowledge Agent with the RAG client and the NVIDIA LLM from .env."""

    # The RAG client first: it needs no connection to be created
    rag_client = create_rag_client(env_file=env_file)

    # The NVIDIA chat model
    provider = load_llm_provider(env_file=env_file)

    agent = KnowledgeAgent(provider=provider, rag_client=rag_client)

    return agent


def cited_references(answer_text, sources):
    """The sources cited as [n] in the answer, in citation-number order."""

    # Collect the cited numbers that match a retrieved source
    cited_numbers = set()
    for match in CITATION_PATTERN.finditer(answer_text):
        number = int(match.group(1))
        if 1 <= number <= len(sources):
            cited_numbers.add(number)

    # Describe each cited source by its title, file, and page
    references = []
    for number in sorted(cited_numbers):
        source = sources[number - 1]
        references.append(
            SourceReference(
                number=number,
                document_title=source.document_title,
                source_file=source.source_file,
                page_number=source.page_number,
            )
        )

    return references
