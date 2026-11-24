"""Turn rules (moved from core-service): one question at a time, retries reopen, quick repeats replay."""

from app.conversations.models import (
    COMPLETED,
    FAILED,
    PENDING,
    START_BUSY,
    START_NEW,
    START_REOPEN,
    START_REPLAY,
)

# Characters of the first question kept as the conversation's title
TITLE_LENGTH = 80


# Public functions

def decide_start(last, question, replay_seconds):
    """What to do with a new question, given the conversation's newest turn (or None)."""

    # A different question (or an unreadable last one) is always a new turn
    if last is None or last.question is None or not same_question(last.question, question):
        return START_NEW

    # The same question is still being answered
    if last.status == PENDING:
        return START_BUSY

    # A failed question is answered again in the same turn, never added twice
    if last.status == FAILED:
        return START_REOPEN

    # A quick repeat of an answered question missed the reply; a later one is a new question
    answered_recently = last.answer_age_seconds is not None and last.answer_age_seconds <= replay_seconds
    if last.status == COMPLETED and last.answer and answered_recently:
        return START_REPLAY

    return START_NEW


def same_question(first, second):
    """True when two questions are the same text, ignoring spacing."""

    same = normalise(first) == normalise(second)

    return same


def title_of(question):
    """A short title for a conversation, from its first question; None when it has no words."""

    title = normalise(question)[:TITLE_LENGTH] or None

    return title


# Helper functions

def normalise(text):

    # Collapse runs of spaces and line breaks
    normalised = " ".join(text.split())

    return normalised
