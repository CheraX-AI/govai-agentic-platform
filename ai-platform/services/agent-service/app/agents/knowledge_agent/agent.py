"""The Knowledge Agent: retrieves documents, asks the LLM, and returns an answer that does not name its sources."""

import logging
import re
from pathlib import Path

from app.agents.knowledge_agent.prompts import (
    AVAILABLE_DOCUMENTS_SCOPE,
    INSUFFICIENT_INFORMATION_REPLY,
    PROMPT_VERSION,
    build_rag_messages,
    build_trace_config,
)
from app.agents.knowledge_agent.rewrite import rewrite_question
from app.agents.knowledge_agent.schemas import KnowledgeAnswer
from app.agents.knowledge_agent.validation import clean_question
from app.llm.exceptions import LLMError
from app.llm.provider import invoke_llm, load_llm_provider
from app.tools.knowledge_rag_client import create_rag_client

logger = logging.getLogger(__name__)

ENV_FILE = Path(__file__).resolve().parents[3] / ".env"

# A document number the model may still write, e.g. [2] or [1, 3], with the space before it
CITATION_PATTERN = re.compile(r"\s*\[\d+(?:\s*,\s*\d+)*\]")


# Agent

class KnowledgeAgent:
    """Answers questions from the documents the knowledge service returns."""

    def __init__(self, provider, rag_client, service_scope=AVAILABLE_DOCUMENTS_SCOPE):

        self.provider = provider
        self.rag_client = rag_client
        self.service_scope = service_scope

    def answer(self, question, request_id=None, history=()):
        """Answer one question; history is the earlier conversation, oldest first.

        Raises InvalidQuestionError for an empty, too short, or too long question
        (before any service is called), a KnowledgeServiceError when the
        documents cannot be retrieved, and an LLMError when the model fails, so an
        answer is never made without the documents.
        """

        question = clean_question(question)

        # Turn a follow-up such as "what about the fee?" into a full question before searching
        question = rewrite_question(self.provider, question, history, request_id=request_id)

        # Find the matching chunks, limited to the scope's department and service if it has them
        sources = self.rag_client.retrieve(
            question,
            department_id=self.service_scope.department_id,
            service_id=self.service_scope.service_id,
            request_id=request_id,
        )

        # Nothing was found: say so, without asking the model
        if not sources:
            answer = self.build_answer(INSUFFICIENT_INFORMATION_REPLY)
            self.log_answer(request_id, retrieved=0)
            return answer

        # Build the prompt and ask the model, labelled for LangSmith
        messages = build_rag_messages(question, sources, self.service_scope)
        answer_text = invoke_llm(self.provider, messages, config=build_trace_config(self.service_scope))

        # Citizens are not shown where the answer came from, so drop any document numbers
        answer_text = remove_citations(answer_text)
        if not answer_text:
            raise LLMError("The model returned only document numbers")

        answer = self.build_answer(answer_text)
        self.log_answer(request_id, retrieved=len(sources))

        return answer

    # Helper methods

    def build_answer(self, answer_text):

        # The answer with the scope and prompt version that produced it
        answer = KnowledgeAnswer(
            answer=answer_text,
            scope=self.service_scope.scope_id,
            prompt_version=PROMPT_VERSION,
        )

        return answer

    def log_answer(self, request_id, retrieved):

        # Log the outcome without the question or answer text
        logger.info(
            "knowledge answer generated",
            extra={
                "request_id": request_id,
                "scope": self.service_scope.scope_id,
                "retrieved": retrieved,
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


def remove_citations(answer_text):
    """Remove document numbers such as [1] or [1, 3] from the answer text."""

    # Drop each marker with the space before it, so no gap is left before punctuation
    cleaned = CITATION_PATTERN.sub("", answer_text).strip()

    return cleaned
