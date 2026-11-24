"""Tests for app/conversations/service.py, with the fake store and the in-memory checkpointer."""

import pytest
from langchain_core.messages import AIMessage, HumanMessage
from langgraph.checkpoint.base import empty_checkpoint

from app.conversations.models import (
    COMPLETED,
    FAILED,
    PENDING,
    START_NEW,
    START_REOPEN,
    ConversationBusyError,
    ConversationNotFoundError,
)
from app.conversations.service import ConversationService
from app.memory.errors import MemoryUnavailableError
from fake_checkpointer import FakeCheckpointer, make_serializer
from fake_conversations import FakeConversationStore

CITIZEN = "citizen-a"
OTHER_CITIZEN = "citizen-b"
MAX_TURNS = 2
IN_FLIGHT = 90
REPLAY = 600


# Fixtures

@pytest.fixture
def service():

    created = ConversationService(FakeConversationStore(), FakeCheckpointer(), MAX_TURNS, IN_FLIGHT, REPLAY)

    return created


def answered_turn(service, question, conversation_id=None, subject=CITIZEN):

    # Ask and answer one question
    started = service.start(subject, conversation_id, question)
    service.complete(started, f"Answer to: {question}")

    return started


def remember(service, conversation_id, started=None):

    # Some short-term state, tagged as the graph run of started would save it (untagged without one)
    saved = empty_checkpoint()
    saved["channel_values"] = {"messages": [HumanMessage(content="Synthetic question?")]}
    metadata = service.run_metadata(started) if started is not None else {}
    config = {"configurable": {"thread_id": conversation_id, "checkpoint_ns": ""}, "metadata": metadata}
    service.checkpointer.put(config, saved, {}, {})


def is_remembered(service, conversation_id):

    found = service.checkpointer.get_tuple({"configurable": {"thread_id": conversation_id}}) is not None

    return found


# Starting turns

def test_first_message_creates_a_conversation_for_the_citizen(service):

    started = service.start(CITIZEN, None, "Synthetic question?")

    assert started.created
    assert started.kind == START_NEW
    assert service.store.owns(CITIZEN, started.conversation_id)
    assert service.store.statuses(started.conversation_id) == [PENDING]


def test_another_citizens_conversation_is_not_found(service):

    started = answered_turn(service, "Synthetic question?")

    with pytest.raises(ConversationNotFoundError):
        service.start(OTHER_CITIZEN, started.conversation_id, "Synthetic question?")


def test_same_question_while_pending_is_busy(service):

    started = service.start(CITIZEN, None, "Synthetic question?")

    with pytest.raises(ConversationBusyError):
        service.start(CITIZEN, started.conversation_id, "Synthetic question?")


def test_failed_question_is_reopened_not_added_twice(service):

    started = service.start(CITIZEN, None, "Synthetic question?")
    service.fail(started)

    retried = service.start(CITIZEN, started.conversation_id, "Synthetic question?")

    assert retried.kind == START_REOPEN
    assert retried.turn_id == started.turn_id
    assert service.store.statuses(started.conversation_id) == [PENDING]


def test_quick_repeat_replays_the_stored_answer_and_a_later_one_is_new(service):

    started = answered_turn(service, "Synthetic question?")

    replayed = service.start(CITIZEN, started.conversation_id, "Synthetic question?")
    service.store.advance(REPLAY + 1)
    later = service.start(CITIZEN, started.conversation_id, "Synthetic question?")

    assert replayed.replayed_answer == "Answer to: Synthetic question?"
    assert later.kind == START_NEW
    assert service.store.statuses(started.conversation_id) == [COMPLETED, PENDING]


def test_abandoned_pending_question_can_be_asked_again(service):

    started = service.start(CITIZEN, None, "Synthetic question?")
    service.store.advance(IN_FLIGHT + 1)

    retried = service.start(CITIZEN, started.conversation_id, "Synthetic question?")

    assert retried.kind == START_REOPEN
    assert retried.turn_id == started.turn_id


# Restoring short-term memory

def test_restore_gives_the_last_answered_turns_oldest_first(service):

    started = answered_turn(service, "One?")
    for question in ("Two?", "Three?"):
        answered_turn(service, question, started.conversation_id)
    failed = service.start(CITIZEN, started.conversation_id, "Four?")
    service.fail(failed)

    messages = service.restore(started.conversation_id)

    assert [type(message) for message in messages] == [HumanMessage, AIMessage, HumanMessage, AIMessage]
    assert [message.content for message in messages] == ["Two?", "Answer to: Two?", "Three?", "Answer to: Three?"]


# Keeping short-term memory in step with the turns

def test_new_conversation_needs_no_earlier_messages(service):

    started = service.start(CITIZEN, None, "Synthetic question?")
    service.checkpointer.errors["get_tuple"] = AssertionError("a new conversation has no checkpoint to read")

    assert service.earlier_messages(started) == []


def test_current_checkpoint_is_used_without_reading_the_turns(service):

    first = answered_turn(service, "One?")
    remember(service, first.conversation_id, first)
    service.store.errors["recent_turns"] = AssertionError("the turns must not be read")

    started = service.start(CITIZEN, first.conversation_id, "Two?")

    assert service.earlier_messages(started) == []
    assert is_remembered(service, first.conversation_id)


def test_missing_checkpoint_is_restored_from_the_turns(service):

    first = answered_turn(service, "One?")
    started = service.start(CITIZEN, first.conversation_id, "Two?")

    messages = service.earlier_messages(started)

    assert [message.content for message in messages] == ["One?", "Answer to: One?"]


def test_checkpoint_of_another_turn_is_deleted_and_restored(service):

    first = answered_turn(service, "One?")
    failed = service.start(CITIZEN, first.conversation_id, "Two?")
    remember(service, first.conversation_id, failed)
    service.fail(failed)
    started = service.start(CITIZEN, first.conversation_id, "Three?")

    messages = service.earlier_messages(started)

    assert [message.content for message in messages] == ["One?", "Answer to: One?"]
    assert not is_remembered(service, first.conversation_id)


def test_checkpoint_of_an_overlapping_run_is_rebuilt(service):

    first = answered_turn(service, "One?")
    conversation_id = first.conversation_id

    # Two different questions at once: both runs start from the same memory
    second = service.start(CITIZEN, conversation_id, "Two?")
    third = service.start(CITIZEN, conversation_id, "Three?")
    remember(service, conversation_id, second)
    service.complete(second, "Answer to: Two?")
    remember(service, conversation_id, third)
    service.complete(third, "Answer to: Three?")

    started = service.start(CITIZEN, conversation_id, "Four?")
    messages = service.earlier_messages(started)

    # The newest checkpoint misses "Two?", so every answered turn comes from the store
    assert [message.content for message in messages] == ["Two?", "Answer to: Two?", "Three?", "Answer to: Three?"]


def test_checkpoint_after_a_sequential_run_is_current(service):

    first = answered_turn(service, "One?")
    second = service.start(CITIZEN, first.conversation_id, "Two?")
    remember(service, first.conversation_id, second)
    service.complete(second, "Answer to: Two?")

    started = service.start(CITIZEN, first.conversation_id, "Three?")

    assert service.earlier_messages(started) == []


def test_untagged_checkpoint_is_never_current(service):

    started = service.start(CITIZEN, None, "One?")
    service.fail(started)
    remember(service, started.conversation_id)
    retried = service.start(CITIZEN, started.conversation_id, "One?")

    assert service.earlier_messages(retried) == []
    assert not is_remembered(service, started.conversation_id)


def test_unreadable_checkpoint_is_deleted_and_restored(service):

    first = answered_turn(service, "One?")
    service.checkpointer.serde = make_serializer(key_id=2, key=bytes(32))
    remember(service, first.conversation_id, first)
    service.checkpointer.serde = make_serializer()
    started = service.start(CITIZEN, first.conversation_id, "Two?")

    messages = service.earlier_messages(started)

    assert [message.content for message in messages] == ["One?", "Answer to: One?"]
    assert not is_remembered(service, first.conversation_id)


def test_checkpoint_read_failure_is_raised(service):

    first = answered_turn(service, "One?")
    started = service.start(CITIZEN, first.conversation_id, "Two?")
    service.checkpointer.errors["get_tuple"] = RuntimeError("down")

    with pytest.raises(RuntimeError):
        service.earlier_messages(started)


def test_run_metadata_tags_the_checkpoint_with_its_turn(service):

    started = service.start(CITIZEN, None, "One?")

    assert service.run_metadata(started) == {"turn_id": started.turn_id}


# Finishing turns

def test_answer_that_cannot_be_saved_deletes_the_checkpoint_and_fails_the_turn(service):

    started = service.start(CITIZEN, None, "Synthetic question?")
    remember(service, started.conversation_id, started)
    service.store.errors["complete_turn"] = MemoryUnavailableError("down")

    with pytest.raises(MemoryUnavailableError):
        service.complete(started, "Synthetic answer.")

    assert not is_remembered(service, started.conversation_id)
    assert service.store.statuses(started.conversation_id) == [FAILED]


def test_complete_keeps_only_the_latest_checkpoint(service):

    started = service.start(CITIZEN, None, "Synthetic question?")
    remember(service, started.conversation_id)
    remember(service, started.conversation_id, started)

    service.complete(started, "Synthetic answer.")

    assert service.checkpointer.checkpoint_count(started.conversation_id) == 1


def test_compaction_failure_does_not_fail_the_answer(service):

    started = service.start(CITIZEN, None, "Synthetic question?")
    service.checkpointer.errors["prune"] = RuntimeError("down")

    service.complete(started, "Synthetic answer.")

    assert service.store.statuses(started.conversation_id) == [COMPLETED]


def test_fail_never_raises(service):

    started = service.start(CITIZEN, None, "Synthetic question?")
    service.store.errors["fail_turn"] = MemoryUnavailableError("down")

    service.fail(started)


# History

def test_turns_page_back_through_the_history(service):

    started = answered_turn(service, "Q1?")
    for number in range(2, 6):
        answered_turn(service, f"Q{number}?", started.conversation_id)

    newest, before = service.turns(CITIZEN, started.conversation_id, None, 2)
    older, oldest_before = service.turns(CITIZEN, started.conversation_id, before, 2)

    assert [turn.question for turn in newest] == ["Q4?", "Q5?"]
    assert [turn.question for turn in older] == ["Q2?", "Q3?"]
    assert oldest_before is not None


def test_history_of_another_citizen_is_not_found(service):

    started = answered_turn(service, "Synthetic question?")

    with pytest.raises(ConversationNotFoundError):
        service.turns(OTHER_CITIZEN, started.conversation_id, None, 20)

    assert service.conversations(OTHER_CITIZEN, 0, 20) == ([], None)


def test_conversations_list_only_the_citizens_own(service):

    mine = answered_turn(service, "Mine?")
    answered_turn(service, "Theirs?", subject=OTHER_CITIZEN)

    summaries, next_offset = service.conversations(CITIZEN, 0, 20)

    assert [summary.id for summary in summaries] == [mine.conversation_id]
    assert summaries[0].title == "Mine?"
    assert next_offset is None


# Deleting

def test_delete_removes_the_turns_and_the_checkpoint(service):

    started = answered_turn(service, "Synthetic question?")
    remember(service, started.conversation_id, started)

    service.delete(CITIZEN, started.conversation_id)

    assert not service.store.owns(CITIZEN, started.conversation_id)
    assert not is_remembered(service, started.conversation_id)


def test_another_citizen_cannot_delete_or_clear_memory(service):

    started = answered_turn(service, "Synthetic question?")
    remember(service, started.conversation_id, started)

    with pytest.raises(ConversationNotFoundError):
        service.delete(OTHER_CITIZEN, started.conversation_id)

    assert service.store.owns(CITIZEN, started.conversation_id)
    assert is_remembered(service, started.conversation_id)


def test_delete_when_memory_cannot_be_deleted_deletes_nothing(service):

    started = answered_turn(service, "Synthetic question?")
    service.checkpointer.errors["delete_thread"] = RuntimeError("down")

    with pytest.raises(RuntimeError):
        service.delete(CITIZEN, started.conversation_id)

    assert service.store.owns(CITIZEN, started.conversation_id)


def test_delete_all_removes_only_the_citizens_conversations(service):

    first = answered_turn(service, "One?")
    second = answered_turn(service, "Two?")
    theirs = answered_turn(service, "Theirs?", subject=OTHER_CITIZEN)
    remember(service, first.conversation_id, first)
    remember(service, theirs.conversation_id, theirs)

    service.delete_all(CITIZEN)

    assert service.conversations(CITIZEN, 0, 20) == ([], None)
    assert not is_remembered(service, first.conversation_id)
    assert not service.store.owns(CITIZEN, second.conversation_id)
    assert service.store.owns(OTHER_CITIZEN, theirs.conversation_id)
    assert is_remembered(service, theirs.conversation_id)


def test_close_closes_the_store(service):

    service.close()

    assert service.store.closed
