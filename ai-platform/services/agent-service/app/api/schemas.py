"""Request and response models: the contract with core-service (keep in sync with backend/pkg/chatcontract)."""

from datetime import datetime

from pydantic import BaseModel, ConfigDict, Field, StrictStr

# core-service sends at most 4000 characters; longer questions are refused here
MAX_MESSAGE_LENGTH = 4000

# A canonical lowercase UUID (versions 1-8, RFC variant), the same rule as chatcontract.ValidConversationID
CONVERSATION_ID_PATTERN = r"^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"

# History pages
MAX_PAGE_SIZE = 50
DEFAULT_PAGE_SIZE = 20
MAX_OFFSET = 10000


# Models

class ChatRequest(BaseModel):
    """One citizen question, and the conversation it belongs to.

    Without conversation_id the agent creates a new conversation for the citizen
    (the X-GovAI-Citizen header). With one, the agent checks that it belongs to
    that citizen. No history is sent: the agent keeps it.
    """

    model_config = ConfigDict(extra="forbid")

    message: StrictStr = Field(min_length=1, max_length=MAX_MESSAGE_LENGTH)
    conversation_id: StrictStr | None = Field(default=None, pattern=CONVERSATION_ID_PATTERN)


class ChatResponse(BaseModel):
    """The agent's reply and the conversation it belongs to; it does not name its documents."""

    answer: str = Field(min_length=1)
    conversation_id: str | None = None


class ConversationItem(BaseModel):
    """One conversation in the citizen's list, newest activity first."""

    id: str
    title: str | None
    created_at: datetime
    last_turn_at: datetime


class ConversationsPage(BaseModel):

    conversations: list[ConversationItem]
    next_offset: int | None = None


class TurnItem(BaseModel):
    """One answered question; seq orders the turns of a conversation."""

    seq: int
    question: str
    answer: str
    created_at: datetime
    answered_at: datetime | None = None


class TurnsPage(BaseModel):
    """Turns oldest first; pass next_before as before to read the older page."""

    turns: list[TurnItem]
    next_before: int | None = None


# Public functions

def to_chat_response(reply, conversation_id=None):
    """Build the response from the graph's reply text and the conversation ID."""

    response = ChatResponse(answer=reply, conversation_id=conversation_id)

    return response
