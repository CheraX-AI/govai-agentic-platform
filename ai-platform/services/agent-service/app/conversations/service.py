"""Citizens' conversations: start and finish turns, keep short-term memory in step, serve and delete history."""

import logging

from app.conversations.models import START_BUSY, ConversationBusyError, ConversationNotFoundError
from app.conversations.rules import decide_start, title_of
from app.memory.errors import MemoryDataError, MemoryUnavailableError
from app.memory.window import turns_to_messages

logger = logging.getLogger(__name__)

# Checkpoint metadata: the turn a run answered, and the newest answered turn its memory started from
TURN_ID_KEY = "turn_id"
AFTER_TURN_ID_KEY = "after_turn_id"


# Service

class ConversationService:
    """The store holds every turn and is the truth; the checkpointer holds each conversation's recent state.

    Both are injected (ConversationStore and a LangGraph checkpointer), so this
    module imports no database code. A checkpoint is used only when it holds exactly
    the answered turns the store has; anything else is deleted and rebuilt from the store.
    """

    def __init__(self, store, checkpointer, max_turns, in_flight_seconds, replay_seconds):

        self.store = store
        self.checkpointer = checkpointer
        self.max_turns = max_turns
        self.in_flight_seconds = in_flight_seconds
        self.replay_seconds = replay_seconds

    def start(self, subject, conversation_id, question):
        """Save the question as pending (creating the conversation when conversation_id is None).

        Raises ConversationNotFoundError, ConversationBusyError or MemoryUnavailableError.
        """

        def decide(last):
            return decide_start(last, question, self.replay_seconds)

        started = self.store.start_turn(subject, conversation_id, question, title_of(question), self.in_flight_seconds, decide)
        if started.kind == START_BUSY:
            raise ConversationBusyError()

        return started

    def earlier_messages(self, started):
        """The messages to send before the question: none when the checkpoint is current, else the stored turns.

        Raises when the checkpointer cannot be read or a stale checkpoint cannot be deleted.
        """

        # A new conversation has no checkpoint yet
        if started.created or self.is_current(started):
            return []

        # Nothing answered yet: nothing to restore
        if started.last_answered_turn_id is None:
            return []

        messages = self.restore(started.conversation_id)

        return messages

    def run_metadata(self, started):
        """Run metadata that tags the run's checkpoint with its turn and the answered turn before it."""

        # The run's memory holds the answered turns up to last_answered_turn_id, from the checkpoint or a restore
        metadata = {TURN_ID_KEY: started.turn_id}
        if started.last_answered_turn_id is not None:
            metadata[AFTER_TURN_ID_KEY] = started.last_answered_turn_id

        return metadata

    def restore(self, conversation_id):
        """The last answered turns from PostgreSQL, as chat messages."""

        turns = self.store.recent_turns(conversation_id, self.max_turns)
        messages = turns_to_messages(turns)

        return messages

    def complete(self, started, answer):
        """Save the answer. On failure the run's checkpoint is deleted, so memory is never ahead of PostgreSQL."""

        try:
            self.store.complete_turn(started.conversation_id, started.turn_id, answer)

        except Exception:
            # The checkpoint already holds this answer; is_current() also catches it if this delete fails
            self.forget(started.conversation_id)
            self.fail(started)
            raise

        self.compact(started.conversation_id)

    def fail(self, started):
        """Mark the question failed so a retry reopens it; a failure here is only logged."""

        try:
            self.store.fail_turn(started.conversation_id, started.turn_id)
        except MemoryUnavailableError:
            logger.warning("conversation turn not marked failed")

        self.compact(started.conversation_id)

    def conversations(self, subject, offset, limit):

        page = self.store.conversations(subject, offset, limit)

        return page

    def turns(self, subject, conversation_id, before, limit):
        """A page of answered turns. Raises ConversationNotFoundError for another citizen's conversation."""

        page = self.store.turns(subject, conversation_id, before, limit)

        return page

    def delete(self, subject, conversation_id):
        """Delete one conversation and its memory. Raises ConversationNotFoundError, or the store's error."""

        # Check the owner first, so nobody can clear another citizen's memory
        if not self.store.owns(subject, conversation_id):
            raise ConversationNotFoundError()

        # Memory first: if it fails nothing is deleted and the caller retries
        self.checkpointer.delete_thread(conversation_id)
        if not self.store.delete(subject, conversation_id):
            raise ConversationNotFoundError()

        # A run that saved in between would leave state behind, so clear again
        self.forget(conversation_id)

    def delete_all(self, subject):
        """Delete every conversation of the citizen and its memory. Raises the store's error."""

        conversation_ids = self.store.conversation_ids(subject)
        self.checkpointer.prune(conversation_ids, strategy="delete")

        self.store.delete_all(subject)

        # A run that saved in between would leave state behind, so clear again
        try:
            self.checkpointer.prune(conversation_ids, strategy="delete")
        except Exception as error:
            log_failure("short-term memory not deleted", error)

    def close(self):

        self.store.close()

    # Short-term memory

    def is_current(self, started):

        conversation_id = started.conversation_id

        try:
            saved = self.checkpointer.get_tuple(thread_config(conversation_id))

        except MemoryDataError:
            # e.g. sealed with a key that was removed; PostgreSQL rebuilds it
            logger.warning("short-term memory unreadable, rebuilt from conversation turns")
            self.checkpointer.delete_thread(conversation_id)
            return False

        if saved is None:
            return False

        # Current only when its run answered the newest answered turn and started from the one before it
        remembered = saved.metadata.get(TURN_ID_KEY)
        after = saved.metadata.get(AFTER_TURN_ID_KEY)
        if remembered is not None and remembered == started.last_answered_turn_id and after == started.second_last_answered_turn_id:
            return True

        # A failed or overlapping run, or one whose answer was not saved: delete it, so the restore is not added on top
        self.checkpointer.delete_thread(conversation_id)

        return False

    def compact(self, conversation_id):

        # Best effort: older checkpoints are never read again, and the next run compacts again
        try:
            self.checkpointer.prune([conversation_id], strategy="keep_latest")
        except Exception as error:
            log_failure("short-term memory not compacted", error)

    def forget(self, conversation_id):

        # Best effort: a checkpoint left behind is caught by is_current() on the next question
        try:
            self.checkpointer.delete_thread(conversation_id)
        except Exception as error:
            log_failure("short-term memory not deleted", error)


# Helper functions

def thread_config(conversation_id):

    # The conversation ID is the LangGraph thread ID
    config = {"configurable": {"thread_id": conversation_id}}

    return config


def log_failure(message, error):

    # Only the error type: its text could hold a database address
    logger.warning(message, extra={"error_type": type(error).__name__})
