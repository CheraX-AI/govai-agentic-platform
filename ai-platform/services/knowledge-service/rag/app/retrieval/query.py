"""Clean and check a search question and its options before any service is called."""

import re
import unicodedata

from pydantic import BaseModel, Field, ValidationError, field_validator

from app.embeddings.exceptions import format_validation_error
from app.retrieval.exceptions import InvalidQueryError
from app.vector_store.vector_store import DEFAULT_TOP_K, MAX_TOP_K

MIN_QUESTION_LENGTH = 3
MAX_QUESTION_LENGTH = 1000

# Department and service IDs use the same characters as the storage keys
SAFE_ID_PATTERN = r"^[A-Za-z0-9_-]+$"

# Invisible control characters; zero-width joiners are kept because Sinhala and Tamil need them
CONTROL_CHARACTERS = re.compile(r"[\x00-\x1f\x7f]")

# Cosine distance runs from 0 (same direction) to 2 (opposite direction)
MAX_COSINE_DISTANCE = 2


# Models

class SearchQuery(BaseModel):
    """A cleaned question with its search options."""

    question: str
    top_k: int = Field(default=DEFAULT_TOP_K, ge=1, le=MAX_TOP_K)
    department_id: str | None = Field(default=None, pattern=SAFE_ID_PATTERN)
    service_id: str | None = Field(default=None, pattern=SAFE_ID_PATTERN)
    # Matches farther away than this are dropped; None keeps every match
    max_distance: float | None = Field(default=None, gt=0, le=MAX_COSINE_DISTANCE)

    @field_validator("question", mode="before")
    @classmethod
    def check_question(cls, question):

        # Only text can be embedded
        if not isinstance(question, str):
            raise ValueError("question must be text")

        # Clean first, so a question of only spaces counts as empty
        question = clean_question(question)

        # Check the length of the cleaned question
        if not question:
            raise ValueError("question is empty")
        if len(question) < MIN_QUESTION_LENGTH:
            raise ValueError(f"question must have at least {MIN_QUESTION_LENGTH} characters")
        if len(question) > MAX_QUESTION_LENGTH:
            raise ValueError(f"question must have at most {MAX_QUESTION_LENGTH} characters")

        return question


# Public functions

def build_query(question, top_k=DEFAULT_TOP_K, department_id=None, service_id=None, max_distance=None):
    """Create a SearchQuery, raising InvalidQueryError with a short message when an input is invalid."""

    try:
        # Clean and check every input
        query = SearchQuery(
            question=question,
            top_k=top_k,
            department_id=department_id,
            service_id=service_id,
            max_distance=max_distance,
        )

    except ValidationError as error:
        # The message names the field and the reason, never the question text
        raise InvalidQueryError(format_validation_error(error))

    return query


def clean_question(question):

    # Use the same Unicode form as the document text (NFC), so the same words embed the same way
    question = unicodedata.normalize("NFC", question)

    # Remove invisible control characters such as NUL
    question = CONTROL_CHARACTERS.sub(" ", question)

    # Join runs of spaces, tabs, and line breaks into single spaces, and trim the ends
    question = " ".join(question.split())

    return question
