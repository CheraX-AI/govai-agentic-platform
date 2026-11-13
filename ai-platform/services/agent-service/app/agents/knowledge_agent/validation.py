"""Cleans and checks a citizen question before it is searched."""

# Question limits, the same as the knowledge service's
MIN_QUESTION_LENGTH = 3
MAX_QUESTION_LENGTH = 1000

# Replies for questions that cannot be searched
EMPTY_QUESTION_REPLY = "Please type a question."
SHORT_QUESTION_REPLY = "Please ask a full question, for example: What documents do I need?"
LONG_QUESTION_REPLY = f"Please shorten your question to at most {MAX_QUESTION_LENGTH} characters."


# Errors

class InvalidQuestionError(ValueError):
    """The question is empty, too short, or too long. The message can be shown to the citizen."""


# Public functions

def clean_question(question):
    """Join runs of spaces and line breaks, then check the length."""

    # Normalise the whitespace, so a question of only spaces counts as empty
    question = " ".join(str(question).split())

    # Check the length of the cleaned question
    if not question:
        raise InvalidQuestionError(EMPTY_QUESTION_REPLY)
    if len(question) < MIN_QUESTION_LENGTH:
        raise InvalidQuestionError(SHORT_QUESTION_REPLY)
    if len(question) > MAX_QUESTION_LENGTH:
        raise InvalidQuestionError(LONG_QUESTION_REPLY)

    return question
