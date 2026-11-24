"""PostgreSQL storage of conversations and every turn, with questions and answers sealed by the key ring."""

import logging
import uuid

from psycopg import Error as DatabaseError
from psycopg_pool import ConnectionPool

from app.conversations.models import (
    COMPLETED,
    START_NEW,
    START_REOPEN,
    START_REPLAY,
    ConversationNotFoundError,
    ConversationSummary,
    LastTurn,
    StartedTurn,
    Turn,
)
from app.memory.config import CONNECTION_OPTIONS
from app.memory.encryption import field_aad
from app.memory.errors import MemoryDataError, MemoryUnavailableError

logger = logging.getLogger(__name__)

# Chat requests hold a connection only for short transactions, never during the model call
POOL_MIN_SIZE = 1
POOL_MAX_SIZE = 4
POOL_TIMEOUT_SECONDS = 5

# The tables scripts/memory_tables.sql creates
TABLES = ("conversations", "conversation_turns")

UNAVAILABLE_MESSAGE = "Conversation database is unreachable, not set up, or refused the change"

TURN_COLUMNS = "seq, turn_id::text AS turn_id, question_ciphertext, answer_ciphertext, created_at, answered_at"


# Store

class ConversationStore:
    """Conversations and their turns. Every database error becomes MemoryUnavailableError."""

    def __init__(self, pool, ring):

        self.pool = pool
        self.ring = ring

    def start_turn(self, subject, conversation_id, question, title, in_flight_seconds, decide):
        """Create or check the conversation, then save the question as decide(last turn) says.

        One transaction, with the conversation row locked, so two requests of one
        conversation never start turns at the same time. Raises ConversationNotFoundError.
        """

        try:
            with self.pool.connection() as connection, connection.transaction():
                if conversation_id is None:
                    # A new conversation, owned by the citizen who asked
                    conversation_id = str(uuid.uuid4())
                    sealed_title = self.ring.seal_text(title, field_aad(conversation_id, "title")) if title else None
                    connection.execute(
                        "INSERT INTO conversations (id, owner_subject, title_ciphertext) VALUES (%s, %s, %s)",
                        (conversation_id, subject, sealed_title),
                    )
                    created, last, answered_ids = True, None, [None, None]
                else:
                    self.lock_owned(connection, subject, conversation_id)
                    self.fail_stale(connection, conversation_id, in_flight_seconds)
                    last = self.read_last_turn(connection, conversation_id)
                    created, answered_ids = False, self.read_last_answered_ids(connection, conversation_id)

                started = self.apply(connection, conversation_id, question, created, last, decide(last))
                started = started.model_copy(update={
                    "last_answered_turn_id": answered_ids[0],
                    "second_last_answered_turn_id": answered_ids[1],
                })

        except DatabaseError:
            raise MemoryUnavailableError(UNAVAILABLE_MESSAGE) from None

        return started

    def complete_turn(self, conversation_id, turn_id, answer):
        """Save the answer. Raises MemoryUnavailableError when the turn is no longer pending."""

        sealed = self.ring.seal_text(answer, field_aad(conversation_id, turn_id, "answer"))

        try:
            with self.pool.connection() as connection, connection.transaction():
                updated = connection.execute(
                    "UPDATE conversation_turns SET status = 'completed', answer_ciphertext = %s, answered_at = clock_timestamp() "
                    "WHERE turn_id = %s AND conversation_id = %s AND status = 'pending'",
                    (sealed, turn_id, conversation_id),
                ).rowcount
                if updated != 1:
                    raise MemoryUnavailableError("The turn is no longer pending")

                connection.execute("UPDATE conversations SET last_turn_at = clock_timestamp() WHERE id = %s", (conversation_id,))

        except DatabaseError:
            raise MemoryUnavailableError(UNAVAILABLE_MESSAGE) from None

    def fail_turn(self, conversation_id, turn_id):
        """Mark the question failed, so a retry reopens it."""

        self.run(
            "UPDATE conversation_turns SET status = 'failed' WHERE turn_id = %s AND conversation_id = %s AND status = 'pending'",
            (turn_id, conversation_id),
        )

    def recent_turns(self, conversation_id, limit):
        """The newest answered turns, oldest first. The caller has already checked ownership."""

        rows = self.fetch(
            f"SELECT {TURN_COLUMNS} FROM conversation_turns "
            "WHERE conversation_id = %s AND status = 'completed' ORDER BY seq DESC LIMIT %s",
            (conversation_id, limit),
        )
        turns = self.open_turns(conversation_id, reversed(rows))

        return turns

    def turns(self, subject, conversation_id, before, limit):
        """A page of answered turns, oldest first, and the seq to pass as before for the next page."""

        try:
            with self.pool.connection() as connection:
                self.check_owned(connection, subject, conversation_id)
                rows = connection.execute(
                    f"SELECT {TURN_COLUMNS} FROM conversation_turns "
                    "WHERE conversation_id = %s AND status = 'completed' AND (%s::bigint IS NULL OR seq < %s) "
                    "ORDER BY seq DESC LIMIT %s",
                    (conversation_id, before, before, limit + 1),
                ).fetchall()

        except DatabaseError:
            raise MemoryUnavailableError(UNAVAILABLE_MESSAGE) from None

        # One row more than the page shows whether there is an older page
        page = rows[:limit]
        next_before = page[-1]["seq"] if len(rows) > limit else None
        turns = self.open_turns(conversation_id, reversed(page))

        return turns, next_before

    def conversations(self, subject, offset, limit):
        """A page of the citizen's conversations, newest first, and the offset of the next page."""

        # Only conversations with an answer: one whose first question failed or is still pending is not shown
        rows = self.fetch(
            "SELECT id::text AS id, title_ciphertext, created_at, last_turn_at FROM conversations c "
            "WHERE owner_subject = %s AND EXISTS ("
            "SELECT 1 FROM conversation_turns t WHERE t.conversation_id = c.id AND t.status = 'completed') "
            "ORDER BY last_turn_at DESC, id DESC LIMIT %s OFFSET %s",
            (subject, limit + 1, offset),
        )

        summaries = []
        for row in rows[:limit]:
            summaries.append(ConversationSummary(
                id=row["id"],
                title=self.open_or_none(row["title_ciphertext"], field_aad(row["id"], "title")),
                created_at=row["created_at"],
                last_turn_at=row["last_turn_at"],
            ))

        next_offset = offset + limit if len(rows) > limit else None

        return summaries, next_offset

    def owns(self, subject, conversation_id):

        rows = self.fetch("SELECT 1 FROM conversations WHERE id = %s AND owner_subject = %s", (conversation_id, subject))

        return bool(rows)

    def conversation_ids(self, subject):

        rows = self.fetch("SELECT id::text AS id FROM conversations WHERE owner_subject = %s", (subject,))

        ids = []
        for row in rows:
            ids.append(row["id"])

        return ids

    def delete(self, subject, conversation_id):
        """Delete one conversation and its turns; False when the citizen has no such conversation."""

        rows = self.fetch(
            "DELETE FROM conversations WHERE id = %s AND owner_subject = %s RETURNING id",
            (conversation_id, subject),
        )

        return bool(rows)

    def delete_all(self, subject):

        self.run("DELETE FROM conversations WHERE owner_subject = %s", (subject,))

    def close(self):

        self.pool.close()

    # Turn steps

    def lock_owned(self, connection, subject, conversation_id):

        # Lock the conversation row for this transaction; another citizen's conversation is "not found"
        row = connection.execute(
            "SELECT id FROM conversations WHERE id = %s AND owner_subject = %s FOR UPDATE",
            (conversation_id, subject),
        ).fetchone()
        if row is None:
            raise ConversationNotFoundError()

    def check_owned(self, connection, subject, conversation_id):

        row = connection.execute(
            "SELECT 1 FROM conversations WHERE id = %s AND owner_subject = %s",
            (conversation_id, subject),
        ).fetchone()
        if row is None:
            raise ConversationNotFoundError()

    def fail_stale(self, connection, conversation_id, in_flight_seconds):

        # A question pending longer than any request can take was abandoned
        connection.execute(
            "UPDATE conversation_turns SET status = 'failed' "
            "WHERE conversation_id = %s AND status = 'pending' AND updated_at < clock_timestamp() - make_interval(secs => %s)",
            (conversation_id, in_flight_seconds),
        )

    def read_last_turn(self, connection, conversation_id):

        row = connection.execute(
            "SELECT turn_id::text AS turn_id, status, question_ciphertext, answer_ciphertext, "
            "extract(epoch FROM clock_timestamp() - answered_at)::float AS answer_age_seconds "
            "FROM conversation_turns WHERE conversation_id = %s ORDER BY seq DESC LIMIT 1",
            (conversation_id,),
        ).fetchone()
        if row is None:
            return None

        # An unreadable question counts as a different one, so a new turn is started
        turn_id = row["turn_id"]
        answer = None
        if row["status"] == COMPLETED:
            answer = self.open_or_none(row["answer_ciphertext"], field_aad(conversation_id, turn_id, "answer"))

        last = LastTurn(
            turn_id=turn_id,
            status=row["status"],
            question=self.open_or_none(row["question_ciphertext"], field_aad(conversation_id, turn_id, "question")),
            answer=answer,
            answer_age_seconds=row["answer_age_seconds"],
        )

        return last

    def read_last_answered_ids(self, connection, conversation_id):

        # The newest two answered turns, newest first (None when missing)
        rows = connection.execute(
            "SELECT turn_id::text AS turn_id FROM conversation_turns "
            "WHERE conversation_id = %s AND status = 'completed' ORDER BY seq DESC LIMIT 2",
            (conversation_id,),
        ).fetchall()

        turn_ids = [None, None]
        for index, row in enumerate(rows):
            turn_ids[index] = row["turn_id"]

        return turn_ids

    def apply(self, connection, conversation_id, question, created, last, kind):

        # A new pending turn, the failed turn reopened, a stored answer, or busy (nothing written)
        if kind == START_NEW:
            turn_id = str(uuid.uuid4())
            connection.execute(
                "INSERT INTO conversation_turns (turn_id, conversation_id, question_ciphertext) VALUES (%s, %s, %s)",
                (turn_id, conversation_id, self.ring.seal_text(question, field_aad(conversation_id, turn_id, "question"))),
            )
            started = StartedTurn(conversation_id=conversation_id, kind=kind, turn_id=turn_id, created=created)

        elif kind == START_REOPEN:
            connection.execute("UPDATE conversation_turns SET status = 'pending' WHERE turn_id = %s", (last.turn_id,))
            started = StartedTurn(conversation_id=conversation_id, kind=kind, turn_id=last.turn_id)

        elif kind == START_REPLAY:
            started = StartedTurn(conversation_id=conversation_id, kind=kind, turn_id=last.turn_id, replayed_answer=last.answer)

        else:
            started = StartedTurn(conversation_id=conversation_id, kind=kind)

        return started

    # Query helpers

    def run(self, query, params):

        try:
            with self.pool.connection() as connection:
                connection.execute(query, params)

        except DatabaseError:
            raise MemoryUnavailableError(UNAVAILABLE_MESSAGE) from None

    def fetch(self, query, params):

        try:
            with self.pool.connection() as connection:
                rows = connection.execute(query, params).fetchall()

        except DatabaseError:
            raise MemoryUnavailableError(UNAVAILABLE_MESSAGE) from None

        return rows

    def open_turns(self, conversation_id, rows):

        # Turns that cannot be decrypted (an unknown key or changed data) are left out and counted
        turns = []
        unreadable = 0
        for row in rows:
            try:
                turns.append(Turn(
                    seq=row["seq"],
                    question=self.ring.unseal_text(row["question_ciphertext"], field_aad(conversation_id, row["turn_id"], "question")),
                    answer=self.ring.unseal_text(row["answer_ciphertext"], field_aad(conversation_id, row["turn_id"], "answer")),
                    created_at=row["created_at"],
                    answered_at=row["answered_at"],
                ))
            except MemoryDataError:
                unreadable += 1

        if unreadable:
            logger.warning("conversation turns unreadable, left out", extra={"count": unreadable})

        return turns

    def open_or_none(self, sealed, aad):

        if sealed is None:
            return None

        try:
            text = self.ring.unseal_text(sealed, aad)
        except MemoryDataError:
            return None

        return text


# Public functions

def open_conversation_store(settings, ring):
    """Connect to the conversation database and check its tables. Raises MemoryUnavailableError."""

    # The pool opens its connections now, so a wrong URL stops startup
    pool = ConnectionPool(
        conninfo=settings.database_url.get_secret_value(),
        min_size=POOL_MIN_SIZE,
        max_size=POOL_MAX_SIZE,
        timeout=POOL_TIMEOUT_SECONDS,
        kwargs=CONNECTION_OPTIONS,
        check=ConnectionPool.check_connection,
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
            "Conversation database is unreachable or not set up (see README, Conversations and memory)"
        ) from None

    store = ConversationStore(pool, ring)

    return store


# Helper functions

def check_tables(pool):

    # Each table must exist and be readable by the runtime login
    with pool.connection() as connection:
        for table in TABLES:
            connection.execute(f"SELECT 1 FROM {table} LIMIT 1")
