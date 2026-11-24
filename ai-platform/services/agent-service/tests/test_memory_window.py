"""Tests for app/memory/window.py: which messages memory keeps."""

from langchain_core.messages import AIMessage, HumanMessage

from app.memory.window import earlier_messages, messages_to_forget, turns_to_messages


# Helpers

def turn(question, answer=None):

    # A citizen message and, when given, the assistant's reply
    messages = [HumanMessage(content=question, id=f"q-{question}-{answer}")]
    if answer is not None:
        messages.append(AIMessage(content=answer, id=f"a-{question}-{answer}"))

    return messages


def chat(*turns):

    # Join turns into one message list
    messages = []
    for messages_of_turn in turns:
        messages.extend(messages_of_turn)

    return messages


def ids(messages):

    # Message IDs, to compare what is forgotten
    found = []
    for message in messages:
        found.append(message.id)

    return found


# Forgetting

def test_answered_turns_within_the_limit_are_kept():

    messages = chat(turn("One?", "1"), turn("Two?", "2"))

    assert messages_to_forget(messages, max_turns=6, answered=True) == []


def test_oldest_turns_beyond_the_limit_are_forgotten():

    messages = chat(turn("One?", "1"), turn("Two?", "2"), turn("Three?", "3"))

    forget = messages_to_forget(messages, max_turns=2, answered=True)

    assert ids(forget) == ids(turn("One?", "1"))


def test_failed_newest_turn_is_forgotten():

    messages = chat(turn("One?", "1"), turn("Two?", "unavailable"))

    forget = messages_to_forget(messages, max_turns=6, answered=False)

    assert ids(forget) == ids(turn("Two?", "unavailable"))


def test_unanswered_older_turn_is_forgotten():

    # A run that crashed left a question without a reply
    messages = chat(turn("Lost?"), turn("Two?", "2"))

    forget = messages_to_forget(messages, max_turns=6, answered=True)

    assert ids(forget) == ids(turn("Lost?"))


def test_repeated_question_is_kept_once():

    # core-service retried a question the agent had already answered
    messages = chat(turn("One?", "1"), turn("Same  question?", "a"), turn("Same question?", "b"))

    forget = messages_to_forget(messages, max_turns=6, answered=True)

    assert ids(forget) == ids(turn("Same  question?", "a"))


def test_question_asked_again_later_is_not_a_repeat():

    messages = chat(turn("Same?", "a"), turn("Other?", "b"), turn("Same?", "c"))

    assert messages_to_forget(messages, max_turns=6, answered=True) == []


def test_assistant_message_before_any_question_is_forgotten():

    messages = [AIMessage(content="Hello", id="hello")] + turn("One?", "1")

    forget = messages_to_forget(messages, max_turns=6, answered=True)

    assert ids(forget) == ["hello"]


def test_empty_chat_forgets_nothing():

    assert messages_to_forget([], max_turns=6, answered=False) == []


# Earlier messages

def test_earlier_messages_are_those_before_the_newest_question():

    messages = chat(turn("One?", "1"), turn("Two?"))

    assert ids(earlier_messages(messages)) == ids(turn("One?", "1"))


def test_first_question_has_no_earlier_messages():

    assert earlier_messages(turn("One?")) == []
    assert earlier_messages([]) == []


# Restored turns

class StoredTurn:

    def __init__(self, question, answer):

        self.question = question
        self.answer = answer


def test_stored_turns_become_question_answer_pairs_oldest_first():

    messages = turns_to_messages([StoredTurn("One?", "1"), StoredTurn("Two?", "2")])

    assert [type(message) for message in messages] == [HumanMessage, AIMessage, HumanMessage, AIMessage]
    assert [message.content for message in messages] == ["One?", "1", "Two?", "2"]


def test_no_stored_turns_give_no_messages():

    assert turns_to_messages([]) == []
