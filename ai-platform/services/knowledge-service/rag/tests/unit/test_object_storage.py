"""Tests for app/storage/object_storage.py that need no running SeaweedFS:
storage keys, object metadata, and error classification."""

from datetime import datetime, timezone

import pytest
from botocore.exceptions import ClientError

from app.processing.pdf_extractor import DocumentSource, DocumentType
from app.storage.exceptions import InvalidObjectKeyError, StorageAuthError, StorageError
from app.storage.object_storage import (
    document_id_from_key,
    is_not_found,
    object_key_for,
    object_metadata,
    original_file_name,
    prefix_for,
    stored_file_name,
    to_storage_error,
    upload_time,
)

DOCUMENT_ID = "a" * 64

SERVICE_SOURCE = DocumentSource(
    department_id="IRD",
    service_id="TIN_REGISTRATION",
    document_type=DocumentType.SERVICE_GUIDELINE,
)

DEPARTMENT_SOURCE = DocumentSource(
    department_id="IRD",
    document_type=DocumentType.DEPARTMENT_GUIDELINE,
)


# Helper functions

def make_client_error(code, status_code):

    # The error boto3 raises for a failed S3 request
    error = ClientError(
        {"Error": {"Code": code, "Message": "test"}, "ResponseMetadata": {"HTTPStatusCode": status_code}},
        "HeadObject",
    )

    return error


# Storage keys

def test_service_document_key_uses_department_and_service():

    object_key = object_key_for(DOCUMENT_ID, SERVICE_SOURCE)

    assert object_key == f"documents/IRD/TIN_REGISTRATION/{DOCUMENT_ID}.pdf"


def test_department_document_key_uses_the_department_folder():

    object_key = object_key_for(DOCUMENT_ID, DEPARTMENT_SOURCE)

    assert object_key == f"documents/IRD/_department/{DOCUMENT_ID}.pdf"


def test_key_starts_with_the_listing_prefix():

    # list_documents() finds documents by this prefix
    assert object_key_for(DOCUMENT_ID, SERVICE_SOURCE).startswith(prefix_for(SERVICE_SOURCE))


@pytest.mark.parametrize("department_id", ["IRD/../other", "IRD DEPT", "..", "IRD/"])
def test_unsafe_ids_are_rejected(department_id):

    # "/" or ".." would change the folder layout of the bucket
    source = DocumentSource(department_id=department_id, document_type=DocumentType.DEPARTMENT_GUIDELINE)

    with pytest.raises(InvalidObjectKeyError):
        object_key_for(DOCUMENT_ID, source)


def test_document_id_is_read_back_from_the_key():

    object_key = object_key_for(DOCUMENT_ID, SERVICE_SOURCE)

    assert document_id_from_key(object_key) == DOCUMENT_ID


def test_key_that_is_not_a_document_is_rejected():

    with pytest.raises(InvalidObjectKeyError):
        document_id_from_key("documents/IRD/TIN_REGISTRATION/notes.txt")


# Object metadata

def test_metadata_holds_only_ascii_ids_and_an_encoded_file_name():

    # S3 metadata must be ASCII; a Sinhala file name is URL-encoded
    metadata = object_metadata(DOCUMENT_ID, SERVICE_SOURCE, "බදු මාර්ගෝපදේශය.pdf")

    assert metadata["document-id"] == DOCUMENT_ID
    assert metadata["department-id"] == "IRD"
    assert metadata["service-id"] == "TIN_REGISTRATION"
    assert metadata["document-type"] == "service_guideline"
    for value in metadata.values():
        assert value.isascii()


def test_department_metadata_has_no_service_id():

    metadata = object_metadata(DOCUMENT_ID, DEPARTMENT_SOURCE, "guide.pdf")

    assert "service-id" not in metadata


def test_original_file_name_is_decoded():

    metadata = object_metadata(DOCUMENT_ID, SERVICE_SOURCE, "බදු මාර්ගෝපදේශය.pdf")

    file_name = original_file_name({"Metadata": metadata}, DOCUMENT_ID)

    assert file_name == "බදු මාර්ගෝපදේශය.pdf"


def test_file_name_from_metadata_cannot_leave_the_folder():

    # Stored metadata is untrusted; a path in it must not choose where the file is written
    response = {"Metadata": {"source-file": "..%2F..%2Fevil.pdf"}}

    file_name = original_file_name(response, DOCUMENT_ID)

    assert file_name == "evil.pdf"


def test_missing_file_name_falls_back_to_the_document_id():

    file_name = original_file_name({}, DOCUMENT_ID)

    assert file_name == f"{DOCUMENT_ID}.pdf"


# Error classification

@pytest.mark.parametrize("code", ["404", "NoSuchKey", "NoSuchBucket"])
def test_not_found_codes_are_recognised(code):

    assert is_not_found(make_client_error(code, 404))


@pytest.mark.parametrize("code, status_code", [("InvalidAccessKeyId", 403), ("SignatureDoesNotMatch", 403), ("403", 403)])
def test_rejected_credentials_become_an_auth_error(code, status_code):

    storage_error = to_storage_error(make_client_error(code, status_code))

    assert isinstance(storage_error, StorageAuthError)


def test_other_errors_keep_their_code():

    storage_error = to_storage_error(make_client_error("InternalError", 500))

    assert type(storage_error) is StorageError
    assert "InternalError" in str(storage_error)


# Newest version helpers

def test_metadata_records_the_upload_time():

    metadata = object_metadata(DOCUMENT_ID, SERVICE_SOURCE, "guide.pdf")

    uploaded_at = datetime.fromisoformat(metadata["uploaded-at"])

    assert uploaded_at.tzinfo is not None


def test_upload_time_prefers_the_saved_time():

    # The saved time has microseconds; the storage time only has seconds
    headers = {
        "Metadata": {"uploaded-at": "2026-11-04T09:01:15.250000+00:00"},
        "LastModified": datetime(2026, 11, 4, 9, 1, 15, tzinfo=timezone.utc),
    }

    assert upload_time(headers) == datetime(2026, 11, 4, 9, 1, 15, 250000, tzinfo=timezone.utc)


def test_upload_time_falls_back_to_the_storage_time():

    # PDFs stored without the upload script have no saved time
    last_modified = datetime(2026, 11, 4, 9, 1, 15, tzinfo=timezone.utc)

    assert upload_time({"Metadata": {}, "LastModified": last_modified}) == last_modified
    assert upload_time({"Metadata": {"uploaded-at": "not a time"}, "LastModified": last_modified}) == last_modified


def test_stored_file_name_is_decoded_or_falls_back_to_the_key():

    with_name = {"Metadata": {"source-file": "TIN%20guide.pdf"}}

    assert stored_file_name(with_name, "documents/IRD/x.pdf") == "TIN guide.pdf"
    assert stored_file_name({}, "documents/IRD/x.pdf") == "documents/IRD/x.pdf"
