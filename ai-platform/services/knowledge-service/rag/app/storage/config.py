"""SeaweedFS object storage settings, read from the rag/.env file.

The values must match the OBJECT_STORAGE_* values in the repository root .env,
which docker-compose uses to create the SeaweedFS user. Real environment
variables override the file, which is useful in CI.
"""

import os
from pathlib import Path

from dotenv import dotenv_values
from pydantic import BaseModel, Field, SecretStr, ValidationError

from app.embeddings.exceptions import format_validation_error
from app.storage.exceptions import StorageConfigError

ENV_FILE = Path(__file__).resolve().parents[2] / ".env"

# S3 bucket names: 3-63 lowercase letters, digits, and hyphens
BUCKET_NAME_PATTERN = r"^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$"

# The endpoint is a URL such as http://localhost:8333
ENDPOINT_PATTERN = r"^https?://\S+$"

# Every setting must be present in .env (or the environment)
REQUIRED_VARIABLES = [
    "OBJECT_STORAGE_ENDPOINT",
    "OBJECT_STORAGE_BUCKET",
    "OBJECT_STORAGE_REGION",
    "OBJECT_STORAGE_ACCESS_KEY",
    "OBJECT_STORAGE_SECRET_KEY",
]


# Models

class StorageSettings(BaseModel):
    """Object storage settings. Both keys are SecretStr so they are never printed or logged."""

    endpoint: str = Field(pattern=ENDPOINT_PATTERN)
    bucket: str = Field(pattern=BUCKET_NAME_PATTERN)
    region: str = Field(min_length=1)
    access_key: SecretStr
    secret_key: SecretStr


# Public functions

def load_storage_settings(environ=None, env_file=ENV_FILE):
    """Build StorageSettings from the .env file.

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
            raise StorageConfigError(f"{name} is required in .env")
        values[name] = value

    try:
        # Check the values and build the settings
        settings = StorageSettings(
            endpoint=values["OBJECT_STORAGE_ENDPOINT"],
            bucket=values["OBJECT_STORAGE_BUCKET"],
            region=values["OBJECT_STORAGE_REGION"],
            access_key=values["OBJECT_STORAGE_ACCESS_KEY"],
            secret_key=values["OBJECT_STORAGE_SECRET_KEY"],
        )

    except ValidationError as error:
        # "from None" drops the Pydantic error, whose input values include the keys
        raise StorageConfigError(format_validation_error(error)) from None

    return settings
