"""Tests for app/retrieval/query.py: question cleaning and search option checks."""

import pytest

from app.retrieval.exceptions import InvalidQueryError
from app.retrieval.query import MAX_QUESTION_LENGTH, build_query, clean_question
from app.vector_store.vector_store import DEFAULT_TOP_K, MAX_TOP_K


# Cleaning

def test_spaces_and_line_breaks_are_joined():

    assert clean_question("  How do I\n\tget a   TIN? \r\n") == "How do I get a TIN?"


def test_control_characters_are_removed():

    assert clean_question("TIN\x00registration") == "TIN registration"


def test_text_is_put_in_the_same_unicode_form_as_the_documents():

    # "e" followed by a combining accent becomes the single character "é" (NFC)
    assert clean_question("café") == "café"


def test_sinhala_zero_width_joiner_is_kept():

    # Sinhala and Tamil letters need the zero-width joiner (U+200D)
    question = "ශ්‍රී ලංකා"

    assert clean_question(question) == question


# Valid queries

def test_valid_query_keeps_its_options():

    query = build_query("How do I get a TIN?", top_k=3, department_id="IRD", service_id="TIN_REGISTRATION")

    assert query.question == "How do I get a TIN?"
    assert query.top_k == 3
    assert query.department_id == "IRD"
    assert query.service_id == "TIN_REGISTRATION"
    assert query.max_distance is None


def test_defaults_search_everything_with_the_default_top_k():

    query = build_query("TIN registration")

    assert query.top_k == DEFAULT_TOP_K
    assert query.department_id is None
    assert query.service_id is None


def test_question_is_cleaned_before_it_is_checked():

    query = build_query("   TIN   ")

    assert query.question == "TIN"


# Invalid questions

@pytest.mark.parametrize("question", ["", "   ", "\n\t "])
def test_empty_question_is_rejected(question):

    with pytest.raises(InvalidQueryError, match="empty"):
        build_query(question)


def test_too_short_question_is_rejected():

    with pytest.raises(InvalidQueryError, match="at least"):
        build_query("TI")


def test_too_long_question_is_rejected():

    with pytest.raises(InvalidQueryError, match="at most"):
        build_query("a" * (MAX_QUESTION_LENGTH + 1))


def test_question_that_is_not_text_is_rejected():

    with pytest.raises(InvalidQueryError, match="text"):
        build_query(None)


def test_error_message_does_not_contain_the_question():

    # The question may hold personal details, so it never appears in an error
    question = "My NIC is 199012345678, is it too long? " * 30

    with pytest.raises(InvalidQueryError) as error:
        build_query(question)

    assert "199012345678" not in str(error.value)


# Invalid options

@pytest.mark.parametrize("top_k", [0, -1, MAX_TOP_K + 1])
def test_top_k_outside_the_range_is_rejected(top_k):

    with pytest.raises(InvalidQueryError, match="top_k"):
        build_query("TIN registration", top_k=top_k)


@pytest.mark.parametrize("department_id", ["IRD/../x", "IRD SL", "IRD;DROP", ""])
def test_unsafe_department_id_is_rejected(department_id):

    with pytest.raises(InvalidQueryError, match="department_id"):
        build_query("TIN registration", department_id=department_id)


def test_unsafe_service_id_is_rejected():

    with pytest.raises(InvalidQueryError, match="service_id"):
        build_query("TIN registration", service_id="TIN$REG")


@pytest.mark.parametrize("max_distance", [0, -0.1, 2.1])
def test_max_distance_outside_the_cosine_range_is_rejected(max_distance):

    with pytest.raises(InvalidQueryError, match="max_distance"):
        build_query("TIN registration", max_distance=max_distance)
