"""Create the conversation memory tables and let the runtime login use them.

Usage (from the agent-service folder, after scripts/create_memory_database.sql):

    $env:AGENT_MEMORY_OWNER_DATABASE_URL = "postgres://govai_agent_owner:<password>@127.0.0.1:5432/govai_agent?sslmode=disable"
    python scripts/setup_memory_tables.py

The owner URL is read only from the environment, never from .env, so the owner
password is not stored next to the runtime settings. Safe to run again: LangGraph
applies only the table changes that are missing.

Exit codes: 0 the tables are ready, 1 setup failed, 2 the owner URL is not set.
"""

import os
import sys
from pathlib import Path

# Running "python scripts/setup_memory_tables.py" puts only scripts/ on the path, so add agent-service/
SERVICE_ROOT = Path(__file__).resolve().parent.parent
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from langgraph.checkpoint.postgres import PostgresSaver
from psycopg import Error as DatabaseError
from psycopg import sql

from app.memory.checkpointer import MEMORY_TABLES

EXIT_OK = 0
EXIT_FAILED = 1
EXIT_NOT_SET = 2

OWNER_URL_VARIABLE = "AGENT_MEMORY_OWNER_DATABASE_URL"
RUNTIME_LOGIN = "govai_agent_app"

# Runtime rights: read, save, compact and forget memory; never create or change tables
RUNTIME_GRANT = "GRANT SELECT, INSERT, UPDATE, DELETE ON {table} TO {login}"


def main():

    # The owner login, from the environment only
    owner_url = os.environ.get(OWNER_URL_VARIABLE, "").strip()
    if not owner_url:
        print(f"Set {OWNER_URL_VARIABLE} to the govai_agent_owner URL first.")
        return EXIT_NOT_SET

    try:
        # Create or update the LangGraph checkpoint tables, then grant the runtime login
        with PostgresSaver.from_conn_string(owner_url) as saver:
            saver.setup()
            for table in MEMORY_TABLES:
                saver.conn.execute(
                    sql.SQL(RUNTIME_GRANT).format(table=sql.Identifier(table), login=sql.Identifier(RUNTIME_LOGIN))
                )

    except DatabaseError as error:
        # The error text can hold the host or login, so only its type is shown
        print(f"Setup failed ({type(error).__name__}). Check the owner URL and that the database exists.")
        return EXIT_FAILED

    print(f"Memory tables are ready: {', '.join(MEMORY_TABLES)} (runtime login: {RUNTIME_LOGIN}).")

    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())
