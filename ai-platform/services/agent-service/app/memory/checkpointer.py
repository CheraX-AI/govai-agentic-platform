"""Short-term memory: LangGraph's AsyncPostgresSaver on the memory database, every value encrypted by the serde."""

import asyncio
import threading

from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
from psycopg import Error as DatabaseError
from psycopg_pool import AsyncConnectionPool

from app.memory.config import CONNECTION_OPTIONS
from app.memory.errors import MemoryUnavailableError

# Each run reads once and saves once, so a few connections per worker are enough
POOL_MIN_SIZE = 1
POOL_MAX_SIZE = 4
POOL_TIMEOUT_SECONDS = 5

# Upper bound for opening or closing the pool from the calling thread
LOOP_TIMEOUT_SECONDS = 30

# The schema version scripts/setup_memory_tables.py creates (the saver's last migration)
SCHEMA_VERSION = len(AsyncPostgresSaver.MIGRATIONS) - 1

# Tables the runtime login must be able to read
TABLES = ("checkpoints", "checkpoint_blobs", "checkpoint_writes")

NOT_READY_MESSAGE = "Memory checkpoint tables are unreachable or not set up (see README, Conversations and memory)"


# Public functions

def open_checkpointer(settings, serde):
    """Connect the checkpointer and check its tables. Raises MemoryUnavailableError.

    The saver runs on its own event loop in a background thread, so it works under any
    server loop (psycopg cannot use Windows' default one). Its sync methods, which the
    graph's invoke uses, may be called from any other thread.
    """

    loop = start_loop()

    try:
        checkpointer = run_on(loop, connect(settings, serde))
    except Exception:
        stop_loop(loop)
        raise

    return checkpointer


def close_checkpointer(checkpointer):
    """Close the pool, then stop the saver's event loop."""

    try:
        run_on(checkpointer.loop, checkpointer.conn.close())
    finally:
        stop_loop(checkpointer.loop)


# Helper functions

async def connect(settings, serde):

    # Opened on the saver's loop, which the pool and the saver keep
    pool = AsyncConnectionPool(
        conninfo=settings.database_url.get_secret_value(),
        min_size=POOL_MIN_SIZE,
        max_size=POOL_MAX_SIZE,
        timeout=POOL_TIMEOUT_SECONDS,
        kwargs=CONNECTION_OPTIONS,
        check=AsyncConnectionPool.check_connection,
        open=False,
    )

    try:
        # Connect and check the tables before the API accepts requests
        await pool.open(wait=True, timeout=POOL_TIMEOUT_SECONDS)
        version = await read_schema_version(pool)

    except DatabaseError:
        # The error text can hold the host or login, so it is dropped
        await pool.close()
        raise MemoryUnavailableError(NOT_READY_MESSAGE) from None

    # The runtime login cannot create tables, so an older schema must be migrated by the owner
    if version != SCHEMA_VERSION:
        await pool.close()
        raise MemoryUnavailableError(NOT_READY_MESSAGE)

    checkpointer = AsyncPostgresSaver(pool, serde=serde)

    return checkpointer


async def read_schema_version(pool):

    async with pool.connection() as connection:
        cursor = await connection.execute("SELECT max(v) AS v FROM checkpoint_migrations")
        row = await cursor.fetchone()

        # Each table must exist and be readable by the runtime login
        for table in TABLES:
            await connection.execute(f"SELECT 1 FROM {table} LIMIT 1")

    version = row["v"]

    return version


def start_loop():

    # A selector loop: psycopg's async driver cannot use the Windows proactor loop
    loop = asyncio.SelectorEventLoop()
    thread = threading.Thread(target=run_loop, args=(loop,), name="memory-checkpointer", daemon=True)
    thread.start()

    return loop


def run_loop(loop):

    asyncio.set_event_loop(loop)
    loop.run_forever()
    loop.close()


def run_on(loop, coroutine):

    result = asyncio.run_coroutine_threadsafe(coroutine, loop).result(timeout=LOOP_TIMEOUT_SECONDS)

    return result


def stop_loop(loop):

    loop.call_soon_threadsafe(loop.stop)
