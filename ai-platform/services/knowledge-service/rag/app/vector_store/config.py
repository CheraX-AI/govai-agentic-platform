"""ChromaDB settings, read from the rag/.env file.

CHROMA_PORT is the port published on your machine (the CHROMA_PORT in the
repository root .env, 8001 by default). Inside Docker, use host "chromadb"
and port 8000. Real environment variables override the file.
"""

import os
from pathlib import Path

from dotenv import dotenv_values
from pydantic import BaseModel, Field, ValidationError

from app.embeddings.exceptions import format_validation_error
from app.vector_store.exceptions import VectorStoreConfigError

ENV_FILE = Path(__file__).resolve().parents[2] / ".env"

# ChromaDB collection names: 3-512 letters, digits, ".", "_" and "-", starting and ending with a letter or digit
COLLECTION_NAME_PATTERN = r"^[A-Za-z0-9][A-Za-z0-9._-]{1,510}[A-Za-z0-9]$"

# Every setting must be present in .env (or the environment)
REQUIRED_VARIABLES = [
    "CHROMA_HOST",
    "CHROMA_PORT",
    "CHROMA_COLLECTION",
]


# Models

class VectorStoreSettings(BaseModel):

    host: str = Field(min_length=1)
    port: int = Field(ge=1, le=65535)
    collection: str = Field(pattern=COLLECTION_NAME_PATTERN)


# Public functions

def load_vector_store_settings(environ=None, env_file=ENV_FILE):
    """Build VectorStoreSettings from the .env file.

    When environ is given (as in tests), only that mapping is used and no
    file is read.
    """

    # Read the .env file; real environment variables take priority over it
    if environ is None:
        environ = {**dotenv_values(env_file), **os.environ}

    # Every setting must be present and not blank
    values = {}
    for name in REQUIRED_VARIABLES:
        value = (environ.get(name) or "").strip()
        if not value:
            raise VectorStoreConfigError(f"{name} is required in .env")
        values[name] = value

    try:
        # Check the values and build the settings
        settings = VectorStoreSettings(
            host=values["CHROMA_HOST"],
            port=values["CHROMA_PORT"],
            collection=values["CHROMA_COLLECTION"],
        )

    except ValidationError as error:
        raise VectorStoreConfigError(format_validation_error(error)) from None

    return settings
