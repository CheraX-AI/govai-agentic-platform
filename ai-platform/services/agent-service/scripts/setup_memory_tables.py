"""Create the conversation and checkpoint tables and let the runtime login use them.

Usage (from the agent-service folder, after scripts/create_memory_database.sql):

    $env:AGENT_MEMORY_OWNER_DATABASE_URL = "postgres://govai_agent_owner:<password>@127.0.0.1:5432/govai_agent?sslmode=disable"
    python scripts/setup_memory_tables.py

The owner URL is read only from the environment, never from .env, so the owner
password is not stored next to the runtime settings. Safe to run again: LangGraph's
checkpoint migrations and scripts/memory_tables.sql only add what is missing.

Exit codes: 0 the tables are ready, 1 setup failed, 2 the owner URL is not set.
"""

import os
import sys
from pathlib import Path

import psycopg
from langgraph.checkpoint.postgres import PostgresSaver
from psycopg import Error as DatabaseError
from psycopg.rows import dict_row

EXIT_OK = 0
EXIT_FAILED = 1
EXIT_NOT_SET = 2

OWNER_URL_VARIABLE = "AGENT_MEMORY_OWNER_DATABASE_URL"
TABLES_FILE = Path(__file__).resolve().parent / "memory_tables.sql"


def main():

    # The owner login, from the environment only
    owner_url = os.environ.get(OWNER_URL_VARIABLE, "").strip()
    if not owner_url:
        print(f"Set {OWNER_URL_VARIABLE} to the govai_agent_owner URL first.")
        return EXIT_NOT_SET

    with open(TABLES_FILE, "r", encoding="utf-8") as file:
        statements = file.read()

    try:
        # LangGraph's checkpoint tables first; its migrations build indexes outside a transaction
        with psycopg.connect(owner_url, connect_timeout=5, autocommit=True, prepare_threshold=0, row_factory=dict_row) as connection:
            PostgresSaver(connection).setup()

        # One transaction: either every conversation table, trigger and grant is in place, or none changes
        with psycopg.connect(owner_url, connect_timeout=5) as connection:
            connection.execute(statements)

    except DatabaseError as error:
        # The error text can hold the host or login, so only its type is shown
        print(f"Setup failed ({type(error).__name__}). Check the owner URL and that the database exists.")
        return EXIT_FAILED

    print(
        "Memory tables are ready: conversations, conversation_turns, checkpoints, checkpoint_blobs, "
        "checkpoint_writes (runtime login: govai_agent_app)."
    )

    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())
