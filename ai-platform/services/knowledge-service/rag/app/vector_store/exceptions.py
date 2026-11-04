"""Vector database errors, with short plain messages."""


# Errors

class VectorStoreError(Exception):
    """Base class for vector database errors."""


class VectorStoreConfigError(VectorStoreError):
    """The ChromaDB settings are missing or invalid."""


class VectorStoreConnectionError(VectorStoreError):
    """ChromaDB cannot be reached, e.g. the container is not running."""


class EmbeddingMismatchError(VectorStoreError):
    """The vectors come from another model or have another size than those already stored."""


class ChunkNotFoundError(VectorStoreError):
    """No record is stored under the requested chunk ID."""
