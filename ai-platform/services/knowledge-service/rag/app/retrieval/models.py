"""Search results returned by the Retriever.

They are Pydantic models, so a future API endpoint or the Agent Orchestrator
can return or read them as JSON without extra conversion.
"""

from pydantic import BaseModel


# Models

class RetrievedChunk(BaseModel):
    """One matching chunk with its source.

    distance is the cosine distance (0 is the same meaning, 2 the opposite);
    similarity is 1 - distance, so a higher similarity is a better match.
    """

    rank: int
    chunk_id: str
    document_id: str | None = None
    text: str
    document_title: str | None = None
    source_file: str | None = None
    page_number: int | None = None
    department_id: str | None = None
    service_id: str | None = None
    document_type: str | None = None
    object_key: str | None = None
    distance: float
    similarity: float


class RetrievalResponse(BaseModel):
    """The cleaned question, the search options used, and the matches, best first."""

    question: str
    top_k: int
    department_id: str | None = None
    service_id: str | None = None
    max_distance: float | None = None
    embedding_model: str
    results: list[RetrievedChunk]
