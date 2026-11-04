"""Tests for embed_query in app/embeddings/embedding_service.py, with a fake NVIDIA client."""

import pytest

from app.embeddings.embedding_service import embed_query
from app.embeddings.exceptions import EmbeddingAuthError, InvalidEmbeddingError
from tests.helpers import make_fake_embedder


def test_question_is_embedded_as_a_query():

    embedder = make_fake_embedder(vector=[0.1, 0.2, 0.3])

    vector = embed_query(embedder, "How do I get a TIN?")

    # The query input type is used, not the document (passage) one
    assert vector == [0.1, 0.2, 0.3]
    assert embedder.embedding_model.query_calls == ["How do I get a TIN?"]
    assert embedder.embedding_model.document_calls == []


def test_server_error_is_retried():

    embedder = make_fake_embedder(errors=[Exception("[503] Service Unavailable")], max_retries=2)

    vector = embed_query(embedder, "TIN registration")

    assert len(vector) > 0
    assert len(embedder.embedding_model.query_calls) == 2


def test_retries_stop_after_the_limit():

    errors = [Exception("[500] Internal Server Error")] * 3
    embedder = make_fake_embedder(errors=errors, max_retries=2)

    with pytest.raises(Exception, match="500"):
        embed_query(embedder, "TIN registration")

    # One first attempt plus two retries
    assert len(embedder.embedding_model.query_calls) == 3


def test_rejected_key_is_not_retried():

    embedder = make_fake_embedder(errors=[Exception("[403] Forbidden")], max_retries=2)

    with pytest.raises(EmbeddingAuthError):
        embed_query(embedder, "TIN registration")

    assert len(embedder.embedding_model.query_calls) == 1


def test_bad_request_is_not_retried():

    embedder = make_fake_embedder(errors=[Exception("[400] Bad Request")], max_retries=2)

    with pytest.raises(Exception, match="400"):
        embed_query(embedder, "TIN registration")

    assert len(embedder.embedding_model.query_calls) == 1


@pytest.mark.parametrize("vector", [[], [0.1, float("nan")], [0.1, float("inf")], [0.1, "0.2"]])
def test_unusable_vector_is_rejected(vector):

    embedder = make_fake_embedder(vector=vector)

    with pytest.raises(InvalidEmbeddingError):
        embed_query(embedder, "TIN registration")
