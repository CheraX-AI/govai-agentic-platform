"""Request and response models: the contract with core-service (keep in sync with backend/pkg/chatcontract)."""

from pydantic import BaseModel, ConfigDict, Field, StrictStr

# core-service sends at most 4000 characters; longer questions are refused here
MAX_MESSAGE_LENGTH = 4000

# A canonical lowercase UUID (versions 1-8, RFC variant), the same rule as chatcontract.ValidConversationID
CONVERSATION_ID_PATTERN = r"^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"


# Models

class ChatRequest(BaseModel):
    """One citizen question, and the conversation it belongs to.

    core-service creates the conversation ID and checks that it belongs to the
    citizen before sending it, so it is never made or checked for ownership
    here. No identity, token, or history is sent.
    """

    model_config = ConfigDict(extra="forbid")

    message: StrictStr = Field(min_length=1, max_length=MAX_MESSAGE_LENGTH)
    conversation_id: StrictStr | None = Field(default=None, pattern=CONVERSATION_ID_PATTERN)


class ChatResponse(BaseModel):
    """The agent's reply, with the same conversation ID it was sent; it does not name its documents."""

    answer: str = Field(min_length=1)
    conversation_id: str | None = None


# Public functions

def to_chat_response(reply, conversation_id=None):
    """Build the response from the graph's reply text and the request's conversation ID."""

    response = ChatResponse(answer=reply, conversation_id=conversation_id)

    return response
