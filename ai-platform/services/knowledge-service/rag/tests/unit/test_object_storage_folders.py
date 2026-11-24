"""Tests for the folder and delete methods of DocumentStore, with an in-memory S3 client."""

import pytest

from app.storage.exceptions import InvalidObjectKeyError
from app.storage.object_storage import DocumentStore

DOCUMENT_ID = "a" * 64


# Fakes

class FakePaginator:

    def __init__(self, keys):

        self.keys = keys

    def paginate(self, Bucket, Prefix):

        # One page with the keys under the prefix, like a small S3 listing
        contents = []
        for key in sorted(self.keys):
            if key.startswith(Prefix):
                contents.append({"Key": key})

        return [{"Contents": contents}]


class FakeS3Client:
    """Keeps object keys in memory; only the calls the folder methods use."""

    def __init__(self, keys=()):

        self.keys = set(keys)

    def get_paginator(self, name):

        return FakePaginator(self.keys)

    def put_object(self, Bucket, Key, Body):

        self.keys.add(Key)

    def delete_object(self, Bucket, Key):

        self.keys.discard(Key)


# Tests

def test_list_folders_groups_pdfs_by_department_and_service():

    client = FakeS3Client(
        [
            f"documents/IRD/TIN_REGISTRATION/{DOCUMENT_ID}.pdf",
            f"documents/IRD/_department/{DOCUMENT_ID}.pdf",
            "documents/DMV/LICENSE_RENEWAL/.keep",
            "documents/EMPTY/.keep",
        ]
    )

    folders = DocumentStore(client, "bucket").list_folders()

    assert folders["IRD"]["TIN_REGISTRATION"] == [f"documents/IRD/TIN_REGISTRATION/{DOCUMENT_ID}.pdf"]
    assert folders["IRD"]["_department"] == [f"documents/IRD/_department/{DOCUMENT_ID}.pdf"]

    # A marker shows an empty folder but is never listed as a PDF
    assert folders["DMV"]["LICENSE_RENEWAL"] == []
    assert folders["EMPTY"] == {}


def test_create_folder_writes_a_marker_and_the_folder_appears():

    client = FakeS3Client()
    store = DocumentStore(client, "bucket")

    store.create_folder("IRD")
    store.create_folder("IRD", "VAT_REGISTRATION")

    assert "documents/IRD/.keep" in client.keys
    assert store.list_folders() == {"IRD": {"VAT_REGISTRATION": []}}


@pytest.mark.parametrize("department_id, service_id", [("../x", None), ("IRD", "a/b"), ("IRD", "")])
def test_create_folder_rejects_unsafe_ids(department_id, service_id):

    store = DocumentStore(FakeS3Client(), "bucket")

    with pytest.raises(InvalidObjectKeyError):
        store.create_folder(department_id, service_id)


def test_delete_document_removes_the_pdf_and_leaves_the_folder_marker():

    pdf_key = f"documents/IRD/TIN_REGISTRATION/{DOCUMENT_ID}.pdf"
    client = FakeS3Client([pdf_key, "documents/IRD/TIN_REGISTRATION/.keep"])

    DocumentStore(client, "bucket").delete_document(pdf_key)

    assert client.keys == {"documents/IRD/TIN_REGISTRATION/.keep"}


def test_delete_document_refuses_a_folder_marker():

    client = FakeS3Client(["documents/IRD/TIN_REGISTRATION/.keep"])

    with pytest.raises(InvalidObjectKeyError):
        DocumentStore(client, "bucket").delete_document("documents/IRD/TIN_REGISTRATION/.keep")
