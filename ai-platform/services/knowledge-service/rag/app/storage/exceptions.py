"""Object storage errors, with plain messages that never contain the credentials."""


# Errors

class StorageError(Exception):
    """Base class for object storage errors."""


class StorageConfigError(StorageError):
    """The object storage settings are missing or invalid."""


class StorageConnectionError(StorageError):
    """SeaweedFS cannot be reached, e.g. the container is not running."""


class StorageAuthError(StorageError):
    """SeaweedFS rejected the access key or secret key."""


class BucketNotFoundError(StorageError):
    """The configured bucket does not exist."""


class DocumentNotFoundError(StorageError):
    """No document is stored under the requested key."""


class InvalidObjectKeyError(StorageError):
    """An ID cannot be used in a storage key, or a key has an unexpected format."""
