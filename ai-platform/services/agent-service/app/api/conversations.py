"""Conversation history routes core-service calls: list, read and delete a citizen's conversations."""

import logging

from fastapi import APIRouter, Depends, Path, Query, Request, Response

from app.api.auth import require_citizen
from app.api.responses import json_response, not_found, read_request_id, unavailable
from app.api.schemas import (
    CONVERSATION_ID_PATTERN,
    DEFAULT_PAGE_SIZE,
    MAX_OFFSET,
    MAX_PAGE_SIZE,
    ConversationItem,
    ConversationsPage,
    TurnItem,
    TurnsPage,
)
from app.conversations.models import ConversationNotFoundError

logger = logging.getLogger(__name__)

CONVERSATIONS_PATH = "/api/v1/conversations"

router = APIRouter(prefix=CONVERSATIONS_PATH)


# Routes

@router.get("")
def list_conversations(
    request: Request,
    subject: str = Depends(require_citizen),
    offset: int = Query(default=0, ge=0, le=MAX_OFFSET),
    limit: int = Query(default=DEFAULT_PAGE_SIZE, ge=1, le=MAX_PAGE_SIZE),
):

    request_id = read_request_id(request)
    conversations = request.app.state.conversations
    if conversations is None:
        return unavailable(request_id)

    try:
        summaries, next_offset = conversations.conversations(subject, offset, limit)
    except Exception as error:
        log_failure("conversation list failed", request_id, error)
        return unavailable(request_id)

    items = []
    for summary in summaries:
        items.append(ConversationItem(**summary.model_dump()))

    page = ConversationsPage(conversations=items, next_offset=next_offset)
    response = json_response(page.model_dump(mode="json"), request_id)

    return response


@router.get("/{conversation_id}/turns")
def list_turns(
    request: Request,
    conversation_id: str = Path(pattern=CONVERSATION_ID_PATTERN),
    subject: str = Depends(require_citizen),
    before: int | None = Query(default=None, ge=1),
    limit: int = Query(default=DEFAULT_PAGE_SIZE, ge=1, le=MAX_PAGE_SIZE),
):

    request_id = read_request_id(request)
    conversations = request.app.state.conversations
    if conversations is None:
        return unavailable(request_id)

    try:
        turns, next_before = conversations.turns(subject, conversation_id, before, limit)

    except ConversationNotFoundError:
        return not_found(request_id)

    except Exception as error:
        log_failure("conversation turns failed", request_id, error)
        return unavailable(request_id)

    items = []
    for turn in turns:
        items.append(TurnItem(**turn.model_dump()))

    page = TurnsPage(turns=items, next_before=next_before)
    response = json_response(page.model_dump(mode="json"), request_id)

    return response


@router.delete("/{conversation_id}")
def delete_conversation(
    request: Request,
    conversation_id: str = Path(pattern=CONVERSATION_ID_PATTERN),
    subject: str = Depends(require_citizen),
):

    request_id = read_request_id(request)
    conversations = request.app.state.conversations
    if conversations is None:
        return unavailable(request_id)

    try:
        conversations.delete(subject, conversation_id)

    except ConversationNotFoundError:
        return not_found(request_id)

    except Exception as error:
        # Nothing or only part was deleted; core-service retries the whole delete
        log_failure("conversation delete failed", request_id, error)
        return unavailable(request_id)

    response = Response(status_code=204)

    return response


@router.delete("")
def delete_all_conversations(request: Request, subject: str = Depends(require_citizen)):

    request_id = read_request_id(request)
    conversations = request.app.state.conversations
    if conversations is None:
        return unavailable(request_id)

    try:
        conversations.delete_all(subject)
    except Exception as error:
        log_failure("conversation delete all failed", request_id, error)
        return unavailable(request_id)

    response = Response(status_code=204)

    return response


# Helper functions

def log_failure(message, request_id, error):

    # Only the error type: its text could hold a database address
    logger.warning(message, extra={"request_id": request_id, "error_type": type(error).__name__})
