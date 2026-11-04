"""Request and response bodies of the agent API: the contract with core-service.

core-service (backend/services/core-service/internal/chat/decode.go) accepts
exactly these fields and rejects any other, so a change here must be made there
too:

    request:  {"message": "..."}
    response: {"answer": "...", "sources": [{"number": 1, "title": "...", "page": 3}]}

number matches the [n] citation in the answer text. page is left out when it is
unknown. url is part of the contract but not sent yet: the documents have no
public address for citizens.
"""

from pydantic import BaseModel, ConfigDict, Field, StrictStr

# core-service sends at most 4000 characters; longer questions are refused here
MAX_MESSAGE_LENGTH = 4000

# The same limit as core-service, so a long document title is shortened, not rejected
MAX_TITLE_LENGTH = 512


# Models

class ChatRequest(BaseModel):
    """One citizen question. Only the question is sent: no identity, token, or history."""

    model_config = ConfigDict(extra="forbid")

    message: StrictStr = Field(min_length=1, max_length=MAX_MESSAGE_LENGTH)


class ChatSource(BaseModel):
    """A document cited in the answer."""

    number: int = Field(ge=1)
    title: str = Field(min_length=1, max_length=MAX_TITLE_LENGTH)
    page: int | None = Field(default=None, ge=1)


class ChatResponse(BaseModel):
    """The agent's reply, with the documents it cited (empty when none were used)."""

    answer: str = Field(min_length=1)
    sources: list[ChatSource]


# Public functions

def to_chat_response(reply, knowledge_answer):
    """Build the response from the graph's reply text and its knowledge answer (or None)."""

    sources = []
    for source in (knowledge_answer or {}).get("sources", []):
        sources.append(
            ChatSource(
                number=source["number"],
                title=source_title(source),
                page=source.get("page_number"),
            )
        )

    response = ChatResponse(answer=reply, sources=sources)

    return response


# Helper functions

def source_title(source):

    # The document's title, else its file name, else just its number
    title = (source.get("document_title") or source.get("source_file") or "").strip()
    if not title:
        title = f"Document {source['number']}"

    # core-service counts bytes, and Sinhala or Tamil letters take 3 bytes each
    title = title.encode("utf-8")[:MAX_TITLE_LENGTH].decode("utf-8", errors="ignore").strip()

    return title
