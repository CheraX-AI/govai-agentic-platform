"""Tests for the on_progress callback of embed_document, with a fake embedder."""

from app.embeddings.embedding_service import embed_document, validate_chunks
from tests.helpers import make_chunk_records, make_fake_embedder


def test_progress_is_reported_after_every_batch():

    chunks = validate_chunks(make_chunk_records(5), "guide.pdf")
    embedder = make_fake_embedder(batch_size=2)
    progress = []

    embed_document(chunks, embedder, lambda done, total: progress.append((done, total)))

    assert progress == [(2, 5), (4, 5), (5, 5)]


def test_embed_document_works_without_a_progress_callback():

    chunks = validate_chunks(make_chunk_records(3), "guide.pdf")

    result = embed_document(chunks, make_fake_embedder())

    assert len(result.embeddings.records) == 3
