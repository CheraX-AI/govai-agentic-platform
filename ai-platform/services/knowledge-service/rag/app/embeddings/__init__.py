"""Embedding generation for the GovAI knowledge service.

Modules:

- config.py             NVIDIA settings read from .env
- exceptions.py         errors and NVIDIA API error classification
- models.py             Pydantic models
- validators.py         chunk, metadata, and vector checks
- storage.py            vector-store output (to_vector_store_batch)
- embedding_service.py  NVIDIA client, batching, retries, and embed_document()
"""

from app.embeddings.embedding_service import *  # noqa: F401,F403
from app.embeddings.embedding_service import __all__  # noqa: F401
