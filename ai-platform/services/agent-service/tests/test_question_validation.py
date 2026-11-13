"""Tests for app/agents/knowledge_agent/validation.py: question cleaning and limits."""

import pytest

from app.agents.knowledge_agent.validation import (
    EMPTY_QUESTION_REPLY,
    LONG_QUESTION_REPLY,
    MAX_QUESTION_LENGTH,
    SHORT_QUESTION_REPLY,
    InvalidQuestionError,
    clean_question,
)


@pytest.mark.parametrize(
    "question, reply",
    [
        ("   ", EMPTY_QUESTION_REPLY),
        ("hi", SHORT_QUESTION_REPLY),
        ("x" * (MAX_QUESTION_LENGTH + 1), LONG_QUESTION_REPLY),
    ],
)
def test_unsearchable_questions_are_rejected_with_a_reply(question, reply):

    # The error message is the reply shown to the citizen
    with pytest.raises(InvalidQuestionError) as error:
        clean_question(question)

    assert str(error.value) == reply


def test_question_limits_are_inclusive():

    assert clean_question("abc") == "abc"
    assert clean_question("x" * MAX_QUESTION_LENGTH) == "x" * MAX_QUESTION_LENGTH


def test_whitespace_is_joined():

    assert clean_question("  What   do I\n hand in?  ") == "What do I hand in?"
