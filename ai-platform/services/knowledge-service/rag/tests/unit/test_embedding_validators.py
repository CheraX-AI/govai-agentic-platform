"""Tests for app/embeddings/validators.py: chunk, metadata, and vector checks."""

import numpy as np
import pytest

from app.embeddings.exceptions import ChunkFileError, InvalidEmbeddingError
from app.embeddings.models import Chunk
from app.embeddings.validators import (
    is_number,
    validate_chunks,
    validate_embedding_file,
    validate_vectors,
)
from tests.helpers import DIMENSION, chunk_ids, make_chunk_records, make_embedding_file

FILE_NAME = "doc001.chunks.json"


# Chunks

def test_valid_chunks_are_returned_in_order():

    chunks = validate_chunks(make_chunk_records(3), FILE_NAME)

    assert chunk_ids(chunks) == ["doc001:0", "doc001:1", "doc001:2"]
    assert isinstance(chunks[0], Chunk)
    assert chunks[0].metadata["department_id"] == "IRD"


def invalid_chunk_data():

    # Each case changes one thing about valid chunk data
    def missing_chunk_id():
        records = make_chunk_records(2)
        del records[0]["chunk_id"]
        return records

    def empty_text():
        records = make_chunk_records(2)
        records[1]["text"] = ""
        return records

    def whitespace_text():
        records = make_chunk_records(2)
        records[1]["text"] = "   \n "
        return records

    def duplicate_id():
        records = make_chunk_records(2)
        records[1]["chunk_id"] = records[0]["chunk_id"]
        return records

    def wrong_document():
        records = make_chunk_records(2)
        records[1]["chunk_id"] = "other:1"
        return records

    def missing_document_id():
        records = make_chunk_records(1)
        del records[0]["metadata"]["document_id"]
        return records

    def none_metadata():
        records = make_chunk_records(1)
        records[0]["metadata"]["service_id"] = None
        return records

    def list_metadata():
        records = make_chunk_records(1)
        records[0]["metadata"]["tags"] = ["tax"]
        return records

    def dict_metadata():
        records = make_chunk_records(1)
        records[0]["metadata"]["owner"] = {"name": "IRD"}
        return records

    def mixed_documents():
        return make_chunk_records(1, "doc001") + make_chunk_records(1, "doc002")

    return [
        ({"chunk_id": "doc001:0"}, "must contain a list of chunks"),
        ([], "has no chunks"),
        (missing_chunk_id(), "Chunk 0 in doc001.chunks.json is invalid"),
        (empty_text(), "Chunk 1 in doc001.chunks.json is invalid"),
        (whitespace_text(), "empty text"),
        (duplicate_id(), "Duplicate chunk ID"),
        (wrong_document(), "does not match its document ID"),
        (missing_document_id(), "does not match its document ID"),
        (none_metadata(), "unsupported type NoneType"),
        (list_metadata(), "unsupported type list"),
        (dict_metadata(), "unsupported type dict"),
        (mixed_documents(), "several documents"),
    ]


@pytest.mark.parametrize("data, expected_message", invalid_chunk_data())
def test_invalid_chunks_are_rejected(data, expected_message):

    # Bad chunk data is rejected with a message that names the problem
    with pytest.raises(ChunkFileError, match=expected_message):
        validate_chunks(data, FILE_NAME)


# Vectors

def test_valid_vectors_pass():

    # Ints and floats of one dimension are accepted
    validate_vectors([[0.1, 2, -3.5], [1.0, 0, 0.25]], 2, None)


@pytest.mark.parametrize(
    "bad_vectors",
    [
        [[0.1] * DIMENSION],
        [[0.1] * DIMENSION, [0.1] * (DIMENSION - 1)],
        [[0.1] * DIMENSION, [np.nan] * DIMENSION],
        [[0.1] * DIMENSION, [np.inf] * DIMENSION],
        [[0.1] * DIMENSION, []],
        [[0.1] * DIMENSION, ["0.1"] * DIMENSION],
        [[0.1] * DIMENSION, [True] * DIMENSION],
        "not a list",
    ],
)
def test_invalid_vectors_are_rejected(bad_vectors):

    # Wrong count, mixed dimensions, and non-numbers are all rejected
    with pytest.raises(InvalidEmbeddingError):
        validate_vectors(bad_vectors, 2, None)


def test_vectors_must_match_an_earlier_dimension():

    # A later batch must use the dimension set by the first batch
    with pytest.raises(InvalidEmbeddingError, match="does not match 8"):
        validate_vectors([[0.1] * 3, [0.2] * 3], 2, DIMENSION)


@pytest.mark.parametrize(
    "value, expected",
    [
        (0.5, True),
        (-3, True),
        (0, True),
        (True, False),
        (False, False),
        ("0.5", False),
        (None, False),
    ],
)
def test_is_number(value, expected):
    assert is_number(value) is expected


@pytest.mark.parametrize("bad_value", [np.nan, np.inf, -np.inf])
def test_vectors_with_nan_or_infinity_are_rejected(bad_value):

    # numpy checks the whole vector for values similarity search cannot use
    with pytest.raises(InvalidEmbeddingError, match="not a finite number"):
        validate_vectors([[0.1, bad_value, 0.3]], 1, None)


@pytest.mark.parametrize("bad_value", [True, "0.1"])
def test_values_numpy_would_convert_are_rejected(bad_value):

    # np.array would turn True into 1.0 and "0.1" into 0.1, so these are checked first
    with pytest.raises(InvalidEmbeddingError, match="not a number"):
        validate_vectors([[0.1, bad_value, 0.3]], 1, None)


# Embeddings files

def test_embedding_file_with_matching_dimensions_passes():
    validate_embedding_file(make_embedding_file(3), "doc001.embeddings.json")


def test_embedding_file_with_a_wrong_dimension_is_rejected():

    # One record whose vector does not match the recorded dimension
    embedding_file = make_embedding_file(3)
    embedding_file.records[1].embedding = [0.1, 0.2]

    with pytest.raises(InvalidEmbeddingError, match="doc001:1 in doc001.embeddings.json has the wrong dimension"):
        validate_embedding_file(embedding_file, "doc001.embeddings.json")
