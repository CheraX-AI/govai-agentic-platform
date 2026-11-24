"""Tests for app/agents/small_talk.py: which messages get a fixed friendly reply, and in which language."""

import pytest

from app.agents.small_talk import (
    ACKNOWLEDGMENT_REPLIES,
    ENGLISH,
    GREETING_REPLIES,
    SINHALA,
    TAMIL,
    THANKS_REPLIES,
    small_talk_reply,
)

ENGLISH_WELCOME = "Welcome to GovAI. I'm here to help you with Sri Lankan government services. How can I assist you today?"


# Greetings

@pytest.mark.parametrize("message", ["Hi", "hello", "Hey!", "Hi there", "Hello GovAI 👋", "  hello  ", "hi?"])
def test_english_greetings_get_the_welcome(message):

    assert small_talk_reply(message) == f"Hello! 👋 {ENGLISH_WELCOME}"


@pytest.mark.parametrize(
    "message, opening",
    [
        ("Good morning", "Good morning! 👋"),
        ("good afternoon!", "Good afternoon! 👋"),
        ("Good evening everyone", "Good evening! 👋"),
        ("Ayubowan", "Ayubowan! 🙏"),
        ("vanakkam", "Vanakkam! 🙏"),
    ],
)
def test_the_greeting_is_said_back(message, opening):

    assert small_talk_reply(message) == f"{opening} {ENGLISH_WELCOME}"


@pytest.mark.parametrize(
    "message, language",
    [
        ("ආයුබෝවන්", SINHALA),
        ("ආයුබෝවන්!", SINHALA),
        ("வணக்கம்", TAMIL),
        ("காலை வணக்கம்", TAMIL),
    ],
)
def test_sinhala_and_tamil_greetings_get_a_reply_in_the_same_language(message, language):

    assert small_talk_reply(message) == GREETING_REPLIES[language]


# Thanks and acknowledgments

@pytest.mark.parametrize(
    "message, language",
    [
        ("Thank you", ENGLISH),
        ("thanks a lot!", ENGLISH),
        ("Ok, thank you 🙏", ENGLISH),
        ("Thanks GovAI", ENGLISH),
        ("nandri", ENGLISH),
        ("ස්තූතියි", SINHALA),
        ("බොහොම ස්තුතියි", SINHALA),
        ("நன்றி", TAMIL),
        ("மிக்க நன்றி", TAMIL),
    ],
)
def test_thanks_get_a_polite_reply(message, language):

    assert small_talk_reply(message) == THANKS_REPLIES[language]


@pytest.mark.parametrize("message, language", [("ok", ENGLISH), ("Got it.", ENGLISH), ("හරි", SINHALA), ("சரி", TAMIL)])
def test_acknowledgments_get_a_polite_reply(message, language):

    assert small_talk_reply(message) == ACKNOWLEDGMENT_REPLIES[language]


# Not small talk

@pytest.mark.parametrize(
    "message",
    [
        "Hi, how do I renew my passport?",
        "Hello what documents do I need for a NIC",
        "Thanks, and what is the fee?",
        "What is the weather today?",
        "hi " * 30,
        "",
        "   ",
        "?",
    ],
)
def test_questions_and_other_messages_are_not_small_talk(message):

    assert small_talk_reply(message) is None


def test_every_reply_language_has_every_kind():

    for replies in (GREETING_REPLIES, THANKS_REPLIES, ACKNOWLEDGMENT_REPLIES):
        assert set(replies) == {ENGLISH, SINHALA, TAMIL}
