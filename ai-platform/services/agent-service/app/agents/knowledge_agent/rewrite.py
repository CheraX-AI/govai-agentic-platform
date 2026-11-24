"""Turns a follow-up message into a standalone question, using the earlier conversation."""

import logging

from app.agents.knowledge_agent.prompts import build_rewrite_messages, build_rewrite_trace_config
from app.agents.knowledge_agent.validation import InvalidQuestionError, clean_question
from app.llm.provider import invoke_llm

logger = logging.getLogger(__name__)


# Public functions

def rewrite_question(provider, question, history, request_id=None):
    """Return a standalone version of question, or question itself when there is no history.

    An LLMError is raised, not hidden: the answer would need the same model, and a
    second slow call would overrun core-service's time budget.
    """

    # A first message needs no rewriting, so it costs no extra model call
    if not history:
        return question

    # Ask the model for the standalone question (never empty); keep only its first line
    messages = build_rewrite_messages(question, history)
    reply = invoke_llm(provider, messages, config=build_rewrite_trace_config())
    first_line = reply.splitlines()[0]

    try:
        # The rewrite must still be a searchable question
        rewritten = clean_question(first_line)

    except InvalidQuestionError:
        # An unusable rewrite: search with the citizen's own words instead
        logger.warning("⚠️ We couldn't improve the question, so we'll use the original one.", extra={"request_id": request_id})
        rewritten = question

    return rewritten
