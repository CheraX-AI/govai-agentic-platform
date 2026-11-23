"""Errors shared by the memory stores; their messages never hold content, keys or addresses."""


# Errors

class MemoryUnavailableError(Exception):
    """A memory store cannot be reached, is not set up, or refused a write."""


class MemoryDataError(Exception):
    """Stored memory cannot be decrypted, e.g. an unknown key or changed data."""
