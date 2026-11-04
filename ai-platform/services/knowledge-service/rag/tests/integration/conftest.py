"""Shared fixtures for tests that use the real SeaweedFS and ChromaDB containers.

They use the settings in rag/.env, but a separate test bucket and a separate
test collection, so the real govai-knowledge bucket and govai_knowledge
collection are never touched. Tests that use these fixtures are skipped with a
clear reason when the settings are missing or the container is not running.
"""

import pytest

from app.storage.config import load_storage_settings
from app.storage.exceptions import StorageConfigError
from app.storage.object_storage import create_document_store
from app.vector_store.config import load_vector_store_settings
from app.vector_store.exceptions import VectorStoreConfigError, VectorStoreError
from app.vector_store.vector_store import create_vector_store

TEST_BUCKET = "govai-knowledge-test"
TEST_COLLECTION = "govai_knowledge_test"


# Fixtures

@pytest.fixture(scope="session")
def test_settings():

    # Real settings from rag/.env, pointed at the test bucket
    try:
        settings = load_storage_settings()
    except StorageConfigError as error:
        pytest.skip(f"object storage settings are not available in rag/.env ({error})")

    settings = settings.model_copy(update={"bucket": TEST_BUCKET})

    return settings


@pytest.fixture(scope="session")
def store(test_settings):

    store = create_document_store(test_settings)

    # Create the test bucket; skip when SeaweedFS is not running
    try:
        store.client.create_bucket(Bucket=TEST_BUCKET)

    except store.client.exceptions.BucketAlreadyOwnedByYou:
        pass

    except store.client.exceptions.BucketAlreadyExists:
        pass

    except Exception as error:
        pytest.skip(f"SeaweedFS is not reachable; start it with 'docker compose up -d seaweedfs' ({error})")

    yield store

    # Delete every test object, then the test bucket
    delete_all_objects(store)
    store.client.delete_bucket(Bucket=TEST_BUCKET)


@pytest.fixture(autouse=True)
def empty_bucket(request):

    # Start each test that uses the store with an empty test bucket
    if "store" in request.fixturenames:
        delete_all_objects(request.getfixturevalue("store"))


@pytest.fixture(scope="session")
def chroma_settings():

    # Real settings from rag/.env, pointed at the test collection
    try:
        settings = load_vector_store_settings()
    except VectorStoreConfigError as error:
        pytest.skip(f"ChromaDB settings are not available in rag/.env ({error})")

    settings = settings.model_copy(update={"collection": TEST_COLLECTION})

    return settings


@pytest.fixture
def vector_store(chroma_settings):
    """A new, empty test collection for each test, deleted afterwards."""

    # Connect once to remove what an earlier test left; skip when ChromaDB is not running
    try:
        old_store = create_vector_store(chroma_settings)
    except VectorStoreError as error:
        pytest.skip(f"ChromaDB is not reachable; start it with 'docker compose up -d chromadb' ({error})")

    old_store.client.delete_collection(TEST_COLLECTION)

    # Create the collection again, empty
    vector_store = create_vector_store(chroma_settings)

    yield vector_store

    vector_store.client.delete_collection(TEST_COLLECTION)


# Helper functions

def delete_all_objects(store):

    # Remove every object in the test bucket
    paginator = store.client.get_paginator("list_objects_v2")
    for page in paginator.paginate(Bucket=TEST_BUCKET):
        for item in page.get("Contents", []):
            store.client.delete_object(Bucket=TEST_BUCKET, Key=item["Key"])
