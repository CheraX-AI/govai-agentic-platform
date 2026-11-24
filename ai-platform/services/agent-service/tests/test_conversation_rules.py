"""Tests for app/conversations/rules.py: the turn rules moved from core-service."""

import pytest

from app.conversations.models import (
    COMPLETED,
    FAILED,
    PENDING,
    START_BUSY,
    START_NEW,
    START_REOPEN,
    START_REPLAY,
    LastTurn,
)
from app.conversations.rules import TITLE_LENGTH, decide_start, same_question, title_of

REPLAY_SECONDS = 600


def last(status, question="Synthetic question?", answer=None, age=None):

    turn = LastTurn(turn_id="t1", status=status, question=question, answer=answer, answer_age_seconds=age)

    return turn


# Deciding

def test_first_question_is_new():

    assert decide_start(None, "Synthetic question?", REPLAY_SECONDS) == START_NEW


def test_different_question_is_new_whatever_the_last_status():

    for status in (PENDING, FAILED, COMPLETED):
        assert decide_start(last(status, answer="A", age=1), "Another question?", REPLAY_SECONDS) == START_NEW


def test_same_question_still_pending_is_busy():

    assert decide_start(last(PENDING), "Synthetic question?", REPLAY_SECONDS) == START_BUSY


def test_same_question_after_a_failure_reopens_the_turn():

    assert decide_start(last(FAILED), "Synthetic  question?", REPLAY_SECONDS) == START_REOPEN


@pytest.mark.parametrize("age, expected", [(0, START_REPLAY), (REPLAY_SECONDS, START_REPLAY), (REPLAY_SECONDS + 1, START_NEW)])
def test_quick_repeat_of_an_answer_replays_and_a_later_one_is_new(age, expected):

    assert decide_start(last(COMPLETED, answer="Stored answer.", age=age), "Synthetic question?", REPLAY_SECONDS) == expected


def test_replay_window_of_zero_never_replays_an_older_answer():

    assert decide_start(last(COMPLETED, answer="Stored answer.", age=1), "Synthetic question?", 0) == START_NEW


def test_unreadable_last_question_counts_as_different():

    assert decide_start(last(PENDING, question=None), "Synthetic question?", REPLAY_SECONDS) == START_NEW


# Text

def test_same_question_ignores_spacing_only():

    assert same_question("What  is\nthe fee?", " What is the fee? ")
    assert not same_question("What is the fee?", "what is the fee?")


def test_title_is_the_first_question_shortened():

    assert title_of("  Passport   renewal fee?  ") == "Passport renewal fee?"
    assert len(title_of("x" * 500)) == TITLE_LENGTH
    assert title_of(" \n ") is None
