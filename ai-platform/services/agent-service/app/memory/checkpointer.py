"""Opens the PostgreSQL checkpointer that keeps each conversation's memory, encrypted."""

from langgraph.checkpoint.postgres import PostgresSaver
from psycopg import Error as DatabaseError
from psycopg.rows import dict_row
from psycopg_pool import ConnectionPool

from app.memory.encryption import create_serializer

# PostgresSaver runs one query at a time under its own lock, so a few connections are enough
POOL_MIN_SIZE = 1
POOL_MAX_SIZE = 4
POOL_TIMEOUT_SECONDS = 5

# Short limits so a slow database fails a request instead of hanging it
CONNECTION_OPTIONS = {
    "autocommit": True,
    "prepare_threshold": 0,
    "row_factory": dict_row,
    "connect_timeout": 5,
    "application_name": "govai-agent",
    "options": "-c statement_timeout=5000 -c lock_timeout=3000",
}

# The tables scripts/setup_memory_tables.py creates
MEMORY_TABLES = ("checkpoints", "checkpoint_blobs", "checkpoint_writes")


# Errors

class MemoryUnavailableError(Exception):
    """The memory database cannot be reached or is not set up."""


# Memory

class ConversationMemory:
    """The checkpointer for the memory graph, plus forget and compact for one conversation."""

    def __init__(self, checkpointer, pool=None):

        self.checkpointer = checkpointer
        self.pool = pool

    def forget(self, conversation_id):
        """Delete everything remembered for the conversation."""

        self.checkpointer.delete_thread(conversation_id)

    def compact(self, conversation_id):
        """Keep only the newest checkpoint; older ones are never read again."""

        self.checkpointer.prune([conversation_id], strategy="keep_latest")

    def close(self):

        # Close the connections when the API stops
        if self.pool is not None:
            self.pool.close()


# Public functions

def open_memory(settings):
    """Connect to the memory database and check its tables. Raises MemoryUnavailableError."""

    # The pool opens its connections now, so a wrong URL stops startup
    pool = ConnectionPool(
        conninfo=settings.database_url.get_secret_value(),
        min_size=POOL_MIN_SIZE,
        max_size=POOL_MAX_SIZE,
        timeout=POOL_TIMEOUT_SECONDS,
        kwargs=CONNECTION_OPTIONS,
        open=False,
    )

    try:
        # Connect and check the tables before the API accepts requests
        pool.open(wait=True, timeout=POOL_TIMEOUT_SECONDS)
        check_tables(pool)

    except DatabaseError:
        # The error text can hold the host or login, so it is dropped
        pool.close()
        raise MemoryUnavailableError(
            "Memory database is unreachable or not set up (see README, Conversation memory)"
        ) from None

    checkpointer = PostgresSaver(pool, serde=create_serializer(settings.data_key.get_secret_value()))
    memory = ConversationMemory(checkpointer, pool)

    return memory


# Helper functions

def check_tables(pool):

    # Each table must exist and be readable by the runtime login
    with pool.connection() as connection:
        for table in MEMORY_TABLES:
            connection.execute(f"SELECT 1 FROM {table} LIMIT 1")
