"""Re-seal stored questions, answers and titles that still use an old key, so that key can be removed.

Usage (from the agent-service folder), after AGENT_MEMORY_DATA_KEY / _KEY_ID in .env name the
new key and AGENT_MEMORY_OLD_DATA_KEYS still holds the old one:

    $env:AGENT_MEMORY_OWNER_DATABASE_URL = "postgres://govai_agent_owner:<password>@127.0.0.1:5432/govai_agent?sslmode=disable"
    python scripts/reencrypt_memory.py

The owner login is needed because the runtime login may not rewrite questions. Safe to
run again; when it reports 0 values left, remove the old key from .env. Checkpoints are
not re-sealed: those on an old key are deleted, and the next question rebuilds them from
the conversation turns.

Exit codes: 0 done, 1 failed, 2 the owner URL or the memory settings are missing.
"""

import os
import sys
from pathlib import Path

# Running "python scripts/reencrypt_memory.py" puts only scripts/ on the path, so add agent-service/
SERVICE_ROOT = Path(__file__).resolve().parent.parent
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

import psycopg
from psycopg import Error as DatabaseError
from psycopg.rows import dict_row

from app.memory.config import MemoryConfigError, load_memory_settings
from app.memory.encryption import CIPHER_PREFIX, build_key_ring, field_aad
from app.memory.errors import MemoryDataError

EXIT_OK = 0
EXIT_FAILED = 1
EXIT_NOT_SET = 2

OWNER_URL_VARIABLE = "AGENT_MEMORY_OWNER_DATABASE_URL"
BATCH_SIZE = 500


def main():

    # The owner login, from the environment only; the keys come from .env
    owner_url = os.environ.get(OWNER_URL_VARIABLE, "").strip()
    try:
        settings = load_memory_settings()
    except MemoryConfigError as error:
        print(f"Memory settings are invalid: {error}")
        return EXIT_NOT_SET

    if not owner_url or settings is None:
        print(f"Set {OWNER_URL_VARIABLE} and the AGENT_MEMORY_* settings first.")
        return EXIT_NOT_SET

    ring = build_key_ring(settings)

    try:
        with psycopg.connect(owner_url, connect_timeout=5, row_factory=dict_row, autocommit=True) as connection:
            turns = reseal_turns(connection, ring)
            titles = reseal_titles(connection, ring)
            checkpoints = forget_old_checkpoints(connection, ring.current_id)
            left = count_old(connection, ring.current_id)

    except DatabaseError as error:
        print(f"Re-encryption failed ({type(error).__name__}). Check the owner URL.")
        return EXIT_FAILED

    except MemoryDataError:
        print("A stored value cannot be decrypted with any key in the ring; add its old key to AGENT_MEMORY_OLD_DATA_KEYS.")
        return EXIT_FAILED

    print(
        f"Re-sealed {turns} turns and {titles} titles with key {ring.current_id}, deleted {checkpoints} "
        f"conversations' checkpoints on an old key; {left} values still use an old key."
    )

    return EXIT_OK


def reseal_turns(connection, ring):

    # Turns where the question or the answer still starts with another key ID
    total = 0
    while True:
        rows = connection.execute(
            "SELECT turn_id::text AS turn_id, conversation_id::text AS conversation_id, question_ciphertext, answer_ciphertext "
            "FROM conversation_turns WHERE get_byte(question_ciphertext, 0) <> %s "
            "OR (answer_ciphertext IS NOT NULL AND get_byte(answer_ciphertext, 0) <> %s) LIMIT %s",
            (ring.current_id, ring.current_id, BATCH_SIZE),
        ).fetchall()
        if not rows:
            return total

        for row in rows:
            question_aad = field_aad(row["conversation_id"], row["turn_id"], "question")
            answer_aad = field_aad(row["conversation_id"], row["turn_id"], "answer")
            question = ring.seal(ring.unseal(row["question_ciphertext"], question_aad), question_aad)
            answer = row["answer_ciphertext"]
            if answer is not None:
                answer = ring.seal(ring.unseal(answer, answer_aad), answer_aad)

            connection.execute(
                "UPDATE conversation_turns SET question_ciphertext = %s, answer_ciphertext = %s WHERE turn_id = %s",
                (question, answer, row["turn_id"]),
            )

        total += len(rows)


def reseal_titles(connection, ring):

    total = 0
    while True:
        rows = connection.execute(
            "SELECT id::text AS id, title_ciphertext FROM conversations "
            "WHERE title_ciphertext IS NOT NULL AND get_byte(title_ciphertext, 0) <> %s LIMIT %s",
            (ring.current_id, BATCH_SIZE),
        ).fetchall()
        if not rows:
            return total

        for row in rows:
            aad = field_aad(row["id"], "title")
            title = ring.seal(ring.unseal(row["title_ciphertext"], aad), aad)
            connection.execute("UPDATE conversations SET title_ciphertext = %s WHERE id = %s", (title, row["id"]))

        total += len(rows)


def forget_old_checkpoints(connection, current_id):

    # Checkpoint values record their key in the type, e.g. "msgpack+aesgcm-k1"; "empty" holds no value
    current_type = f"%+{CIPHER_PREFIX}{current_id}"

    with connection.transaction():
        rows = connection.execute(
            "SELECT thread_id FROM checkpoint_blobs WHERE type <> 'empty' AND type NOT LIKE %s "
            "UNION SELECT thread_id FROM checkpoint_writes WHERE type NOT LIKE %s",
            (current_type, current_type),
        ).fetchall()

        thread_ids = []
        for row in rows:
            thread_ids.append(row["thread_id"])

        for table in ("checkpoint_writes", "checkpoint_blobs", "checkpoints"):
            connection.execute(f"DELETE FROM {table} WHERE thread_id = ANY(%s)", (thread_ids,))

    return len(thread_ids)


def count_old(connection, current_id):

    row = connection.execute(
        "SELECT (SELECT count(*) FROM conversation_turns WHERE get_byte(question_ciphertext, 0) <> %s "
        "OR (answer_ciphertext IS NOT NULL AND get_byte(answer_ciphertext, 0) <> %s)) "
        "+ (SELECT count(*) FROM conversations WHERE title_ciphertext IS NOT NULL AND get_byte(title_ciphertext, 0) <> %s) AS left",
        (current_id, current_id, current_id),
    ).fetchone()

    return row["left"]


if __name__ == "__main__":
    sys.exit(main())
