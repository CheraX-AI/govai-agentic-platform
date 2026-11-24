"""An in-memory ConversationStore for tests; tests/test_memory_live.py checks the real one behaves the same."""

import uuid
from datetime import datetime, timedelta, timezone

from app.conversations.models import (
    COMPLETED,
    FAILED,
    PENDING,
    START_NEW,
    START_REOPEN,
    START_REPLAY,
    ConversationNotFoundError,
    ConversationSummary,
    LastTurn,
    StartedTurn,
    Turn,
)
from app.memory.errors import MemoryUnavailableError


# Fake store

class FakeConversationStore:
    """Plain-text rows in dictionaries; set errors[method] to make that method raise."""

    def __init__(self):

        self.now = datetime(2026, 11, 22, 9, 0, tzinfo=timezone.utc)
        self.conversations_by_id = {}
        self.turn_rows = []
        self.errors = {}
        self.closed = False

    def start_turn(self, subject, conversation_id, question, title, in_flight_seconds, decide):

        self.maybe_fail("start_turn")

        if conversation_id is None:
            conversation_id = str(uuid.uuid4())
            self.conversations_by_id[conversation_id] = {
                "owner": subject, "title": title, "created_at": self.now, "last_turn_at": self.now,
            }
            created, last = True, None
        else:
            self.check_owned(subject, conversation_id)
            self.fail_stale(conversation_id, in_flight_seconds)
            created, last = False, self.last_turn(conversation_id)

        kind = decide(last)
        if kind == START_NEW:
            turn_id = str(uuid.uuid4())
            self.turn_rows.append({
                "seq": len(self.turn_rows) + 1, "turn_id": turn_id, "conversation_id": conversation_id,
                "status": PENDING, "question": question, "answer": None,
                "created_at": self.now, "updated_at": self.now, "answered_at": None,
            })
            started = StartedTurn(conversation_id=conversation_id, kind=kind, turn_id=turn_id, created=created)

        elif kind == START_REOPEN:
            row = self.row(last.turn_id)
            row["status"], row["updated_at"] = PENDING, self.now
            started = StartedTurn(conversation_id=conversation_id, kind=kind, turn_id=last.turn_id)

        elif kind == START_REPLAY:
            started = StartedTurn(conversation_id=conversation_id, kind=kind, turn_id=last.turn_id, replayed_answer=last.answer)

        else:
            started = StartedTurn(conversation_id=conversation_id, kind=kind)

        answered_ids = (self.answered_ids(conversation_id) + [None, None])[:2]
        started = started.model_copy(update={
            "last_answered_turn_id": answered_ids[0],
            "second_last_answered_turn_id": answered_ids[1],
        })

        return started

    def complete_turn(self, conversation_id, turn_id, answer):

        self.maybe_fail("complete_turn")
        row = self.row(turn_id)
        if row is None or row["conversation_id"] != conversation_id or row["status"] != PENDING:
            raise MemoryUnavailableError("The turn is no longer pending")

        row.update(status=COMPLETED, answer=answer, answered_at=self.now, updated_at=self.now)
        self.conversations_by_id[conversation_id]["last_turn_at"] = self.now

    def fail_turn(self, conversation_id, turn_id):

        self.maybe_fail("fail_turn")
        row = self.row(turn_id)
        if row is not None and row["status"] == PENDING:
            row.update(status=FAILED, updated_at=self.now)

    def recent_turns(self, conversation_id, limit):

        self.maybe_fail("recent_turns")
        completed = self.completed(conversation_id)
        turns = completed[-limit:]

        return turns

    def turns(self, subject, conversation_id, before, limit):

        self.maybe_fail("turns")
        self.check_owned(subject, conversation_id)

        older = []
        for turn in self.completed(conversation_id):
            if before is None or turn.seq < before:
                older.append(turn)

        page = older[-limit:]
        next_before = page[0].seq if len(older) > limit else None

        return page, next_before

    def conversations(self, subject, offset, limit):

        self.maybe_fail("conversations")
        owned = []
        for conversation_id, row in self.conversations_by_id.items():
            if row["owner"] == subject and self.completed(conversation_id):
                owned.append(ConversationSummary(
                    id=conversation_id, title=row["title"], created_at=row["created_at"], last_turn_at=row["last_turn_at"],
                ))

        owned.sort(key=lambda summary: (summary.last_turn_at, summary.id), reverse=True)
        page = owned[offset:offset + limit]
        next_offset = offset + limit if len(owned) > offset + limit else None

        return page, next_offset

    def owns(self, subject, conversation_id):

        self.maybe_fail("owns")
        row = self.conversations_by_id.get(conversation_id)

        return row is not None and row["owner"] == subject

    def conversation_ids(self, subject):

        ids = []
        for conversation_id, row in self.conversations_by_id.items():
            if row["owner"] == subject:
                ids.append(conversation_id)

        return ids

    def delete(self, subject, conversation_id):

        self.maybe_fail("delete")
        if not self.owns(subject, conversation_id):
            return False

        del self.conversations_by_id[conversation_id]
        self.turn_rows = [row for row in self.turn_rows if row["conversation_id"] != conversation_id]

        return True

    def delete_all(self, subject):

        self.maybe_fail("delete_all")
        for conversation_id in self.conversation_ids(subject):
            self.delete(subject, conversation_id)

    def close(self):

        self.closed = True

    # Test controls

    def advance(self, seconds):

        self.now = self.now + timedelta(seconds=seconds)

    def statuses(self, conversation_id):

        found = []
        for row in self.turn_rows:
            if row["conversation_id"] == conversation_id:
                found.append(row["status"])

        return found

    # Helpers

    def maybe_fail(self, method):

        error = self.errors.get(method)
        if error is not None:
            raise error

    def check_owned(self, subject, conversation_id):

        row = self.conversations_by_id.get(conversation_id)
        if row is None or row["owner"] != subject:
            raise ConversationNotFoundError()

    def fail_stale(self, conversation_id, in_flight_seconds):

        for row in self.turn_rows:
            stale = self.now - row["updated_at"] > timedelta(seconds=in_flight_seconds)
            if row["conversation_id"] == conversation_id and row["status"] == PENDING and stale:
                row.update(status=FAILED, updated_at=self.now)

    def last_turn(self, conversation_id):

        rows = [row for row in self.turn_rows if row["conversation_id"] == conversation_id]
        if not rows:
            return None

        row = rows[-1]
        age = (self.now - row["answered_at"]).total_seconds() if row["answered_at"] else None
        last = LastTurn(turn_id=row["turn_id"], status=row["status"], question=row["question"], answer=row["answer"], answer_age_seconds=age)

        return last

    def answered_ids(self, conversation_id):

        # Answered turns, newest first
        turn_ids = []
        for row in reversed(self.turn_rows):
            if row["conversation_id"] == conversation_id and row["status"] == COMPLETED:
                turn_ids.append(row["turn_id"])

        return turn_ids

    def row(self, turn_id):

        for row in self.turn_rows:
            if row["turn_id"] == turn_id:
                return row

        return None

    def completed(self, conversation_id):

        turns = []
        for row in self.turn_rows:
            if row["conversation_id"] == conversation_id and row["status"] == COMPLETED:
                turns.append(Turn(
                    seq=row["seq"], question=row["question"], answer=row["answer"],
                    created_at=row["created_at"], answered_at=row["answered_at"],
                ))

        return turns
