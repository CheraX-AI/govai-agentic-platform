"""Conversation and turn types, and the errors the conversation feature raises."""

from datetime import datetime

from pydantic import BaseModel, ConfigDict

# Turn status
PENDING = "pending"
COMPLETED = "completed"
FAILED = "failed"

# What starting a turn decided
START_NEW = "new"
START_REOPEN = "reopen"
START_REPLAY = "replay"
START_BUSY = "busy"


# Errors

class ConversationNotFoundError(Exception):
    """The conversation does not exist or belongs to another citizen (the same answer for both)."""


class ConversationBusyError(Exception):
    """The same question is still being answered."""


# Models

class LastTurn(BaseModel):
    """The newest turn of a conversation, as the start rules need it."""

    model_config = ConfigDict(frozen=True)

    turn_id: str
    status: str
    question: str | None
    answer: str | None = None
    answer_age_seconds: float | None = None


class StartedTurn(BaseModel):
    """The outcome of starting a turn: a pending turn to answer, a stored answer, or busy."""

    model_config = ConfigDict(frozen=True)

    conversation_id: str
    kind: str
    turn_id: str | None = None
    created: bool = False
    replayed_answer: str | None = None

    # The newest two answered turns when the question was started (None for none)
    last_answered_turn_id: str | None = None
    second_last_answered_turn_id: str | None = None


class Turn(BaseModel):
    """One answered question of a conversation."""

    model_config = ConfigDict(frozen=True)

    seq: int
    question: str
    answer: str
    created_at: datetime
    answered_at: datetime | None = None


class ConversationSummary(BaseModel):
    """One conversation in a citizen's list; title is None when it cannot be decrypted."""

    model_config = ConfigDict(frozen=True)

    id: str
    title: str | None
    created_at: datetime
    last_turn_at: datetime
