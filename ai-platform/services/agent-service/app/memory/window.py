"""Chooses which messages memory keeps: the last answered turns, with no failures or repeats."""

from langchain_core.messages import AIMessage, HumanMessage


# Public functions

def messages_to_forget(messages, max_turns, answered):
    """Return the messages to remove so memory holds at most max_turns answered turns.

    A turn is a citizen message and the replies after it. The newest turn is
    forgotten when it was not answered, so a retry starts clean; an older turn
    with no reply (a run that crashed) is always forgotten; when the newest
    question repeats the one before it, the older copy is forgotten.
    """

    turns = split_turns(messages)
    forget = []

    # The newest turn failed or was not a question: keep nothing of it
    if not answered and turns:
        forget.extend(turns.pop())

    # Older turns without a reply belong to runs that never finished
    kept = []
    for turn in turns:
        if is_answered(turn):
            kept.append(turn)
        else:
            forget.extend(turn)

    # A retry of the last question (e.g. core-service could not save the answer) is kept once
    if len(kept) >= 2 and question_text(kept[-1]) == question_text(kept[-2]):
        forget.extend(kept.pop(-2))

    # Drop the oldest turns beyond the limit
    while len(kept) > max_turns:
        forget.extend(kept.pop(0))

    return forget


def earlier_messages(messages):
    """The messages before the newest citizen message: the conversation so far."""

    # Walk back to the newest citizen message
    for index in range(len(messages) - 1, -1, -1):
        if isinstance(messages[index], HumanMessage):
            history = list(messages[:index])
            return history

    history = []

    return history


def turns_to_messages(turns):
    """Stored turns as chat messages, oldest first: each question followed by its answer."""

    messages = []
    for turn in turns:
        messages.append(HumanMessage(content=turn.question))
        messages.append(AIMessage(content=turn.answer))

    return messages


# Helper functions

def split_turns(messages):

    # Start a new turn at each citizen message; anything before the first one is its own turn
    turns = []
    for message in messages:
        if isinstance(message, HumanMessage) or not turns:
            turns.append([message])
        else:
            turns[-1].append(message)

    return turns


def is_answered(turn):

    # A citizen message followed by at least one assistant reply
    answered = isinstance(turn[0], HumanMessage) and any(isinstance(message, AIMessage) for message in turn[1:])

    return answered


def question_text(turn):

    # The citizen's words, with spacing normalised
    question = " ".join(str(turn[0].text).split())

    return question
