"""Data models used by the Knowledge Agent."""

from pydantic import BaseModel, Field


# Models

class KnowledgeSource(BaseModel):
    """One retrieved document chunk the agent may use as evidence for an answer."""

    chunk_id: str = Field(min_length=1)
    text: str = Field(min_length=1)
    document_title: str | None = None
    source_file: str | None = None
    page_number: int | None = None
    department_id: str | None = None
    service_id: str | None = None
    similarity: float | None = None


class ServiceScope(BaseModel):
    """The government services the agent answers questions about.

    scope_id is a short label for logs and LangSmith metadata; description is
    the text placed in the prompt. department_id and service_id narrow the
    document search; None searches every published document.
    """

    scope_id: str = Field(min_length=1, pattern=r"^[a-z0-9_]+$")
    description: str = Field(min_length=1)
    department_id: str | None = None
    service_id: str | None = None


class KnowledgeAnswer(BaseModel):
    """The agent's answer and the scope and prompt that produced it; it never names its sources."""

    answer: str
    scope: str
    prompt_version: str
