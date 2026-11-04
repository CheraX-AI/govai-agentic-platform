"""Retrieval errors, with short plain messages.

Callers such as the Agent Orchestrator can catch RetrievalError for every
failure, or a subclass to react to one kind.
"""


# Errors

class RetrievalError(Exception):
    """Base class for retrieval errors."""


class RetrievalConfigError(RetrievalError):
    """The settings are missing or invalid, or the query model differs from the stored documents' model."""


class InvalidQueryError(RetrievalError):
    """The question or a search option is invalid, e.g. an empty question."""


class QueryEmbeddingError(RetrievalError):
    """The question could not be turned into a vector, e.g. NVIDIA is down or rejected the key."""


class RetrievalUnavailableError(RetrievalError):
    """ChromaDB cannot be reached, the collection is missing, or the search request failed."""
