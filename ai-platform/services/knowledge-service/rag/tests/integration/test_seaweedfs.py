"""Integration tests for DocumentStore and the upload script against the real SeaweedFS container.

The test bucket and its fixtures are in conftest.py. Only small synthetic PDFs
are uploaded.
"""

import pytest
from pydantic import SecretStr

from app.processing.pdf_extractor import DocumentSource, DocumentType, InvalidPDFError, compute_document_id
from app.storage.exceptions import (
    BucketNotFoundError,
    DocumentNotFoundError,
    StorageAuthError,
    StorageConnectionError,
    StorageError,
)
from app.storage.object_storage import create_document_store
from scripts import upload_documents as upload_script
from tests.helpers import storage_env_values, write_env_file
from tests.integration.conftest import TEST_BUCKET
from tests.integration.test_pipeline import PAGE_ONE_LINES, PAGE_TWO_LINES, write_text_pdf

# Nothing listens on port 1, so connecting there always fails
UNREACHABLE_ENDPOINT = "http://127.0.0.1:1"

SERVICE_SOURCE = DocumentSource(
    department_id="IRD",
    service_id="TIN_REGISTRATION",
    document_type=DocumentType.SERVICE_GUIDELINE,
)


# Fixtures

@pytest.fixture
def pdf_path(tmp_path):

    # A small synthetic two-page PDF
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES, PAGE_TWO_LINES])

    return pdf_path


# Helper functions

def run_upload_script(env_file, *files):

    # Upload the given files as TIN registration service guidelines
    exit_code = upload_script.main(
        [
            "--department-id", "IRD",
            "--service-id", "TIN_REGISTRATION",
            "--document-type", "service_guideline",
            "--env-file", str(env_file),
            "--file", *[str(file) for file in files],
        ]
    )

    return exit_code


# Connection and bucket checks

def test_bucket_check_passes_for_an_existing_bucket(store):

    store.check_bucket()


def test_missing_bucket_is_reported(test_settings, store):

    missing = create_document_store(test_settings.model_copy(update={"bucket": "govai-missing-bucket"}))

    with pytest.raises(BucketNotFoundError):
        missing.check_bucket()


def test_wrong_secret_key_is_an_auth_error(test_settings, store):

    rejected = create_document_store(test_settings.model_copy(update={"secret_key": SecretStr("wrong-secret")}))

    with pytest.raises(StorageAuthError):
        rejected.check_bucket()


def test_stopped_storage_is_a_connection_error(test_settings):

    # Needs no running SeaweedFS: nothing listens on the endpoint
    unreachable = create_document_store(test_settings.model_copy(update={"endpoint": UNREACHABLE_ENDPOINT}))

    with pytest.raises(StorageConnectionError):
        unreachable.check_bucket()


# Upload and download

def test_upload_then_download_returns_the_same_file(store, pdf_path, tmp_path):

    stored = store.upload_document(pdf_path, SERVICE_SOURCE)
    downloaded = store.download_document(stored.object_key, tmp_path / "downloads")

    assert stored.uploaded
    assert stored.object_key == f"documents/IRD/TIN_REGISTRATION/{stored.document_id}.pdf"
    assert downloaded.name == "tin_guide.pdf"
    assert downloaded.read_bytes() == pdf_path.read_bytes()


def test_uploading_the_same_pdf_twice_is_skipped(store, pdf_path):

    first = store.upload_document(pdf_path, SERVICE_SOURCE)
    second = store.upload_document(pdf_path, SERVICE_SOURCE)

    assert first.uploaded
    assert not second.uploaded
    assert second.object_key == first.object_key
    assert store.list_documents(SERVICE_SOURCE) == [first.object_key]


def test_revised_pdf_is_stored_as_a_new_version(store, pdf_path, tmp_path):

    # Same file name, different content: both versions are kept
    first = store.upload_document(pdf_path, SERVICE_SOURCE)
    (tmp_path / "revised").mkdir()
    revised_path = write_text_pdf(tmp_path / "revised" / "tin_guide.pdf", [PAGE_TWO_LINES])
    second = store.upload_document(revised_path, SERVICE_SOURCE)

    assert second.object_key != first.object_key
    assert store.list_documents(SERVICE_SOURCE) == sorted([first.object_key, second.object_key])


def test_only_the_newest_version_of_a_file_is_listed_for_processing(store, pdf_path, tmp_path):

    # Two versions of tin_guide.pdf, plus another file
    first = store.upload_document(pdf_path, SERVICE_SOURCE)
    (tmp_path / "revised").mkdir()
    revised = store.upload_document(
        write_text_pdf(tmp_path / "revised" / "tin_guide.pdf", [PAGE_TWO_LINES]), SERVICE_SOURCE
    )
    other = store.upload_document(write_text_pdf(tmp_path / "tin_faq.pdf", [PAGE_ONE_LINES]), SERVICE_SOURCE)

    latest = store.list_latest_documents(SERVICE_SOURCE)

    # Both versions stay stored, but only the newer one is processed
    assert len(store.list_documents(SERVICE_SOURCE)) == 3
    assert first.object_key not in latest
    assert sorted(latest) == sorted([revised.object_key, other.object_key])


def test_file_that_is_not_a_pdf_is_not_uploaded(store, tmp_path):

    text_file = tmp_path / "notes.pdf"
    text_file.write_text("not a pdf", encoding="utf-8")

    with pytest.raises(InvalidPDFError):
        store.upload_document(text_file, SERVICE_SOURCE)

    assert store.list_documents(SERVICE_SOURCE) == []


def test_missing_document_is_reported(store, tmp_path):

    missing_key = f"documents/IRD/TIN_REGISTRATION/{'0' * 64}.pdf"

    with pytest.raises(DocumentNotFoundError):
        store.download_document(missing_key, tmp_path)


def test_changed_content_is_detected_on_download(store, pdf_path, tmp_path):

    # Store other content under the key of pdf_path; the hash check must catch it
    wrong_key = f"documents/IRD/TIN_REGISTRATION/{compute_document_id(pdf_path)}.pdf"
    store.client.put_object(Bucket=TEST_BUCKET, Key=wrong_key, Body=b"%PDF-1.4 changed")

    download_dir = tmp_path / "downloads"
    with pytest.raises(StorageError, match="does not match"):
        store.download_document(wrong_key, download_dir)

    # The damaged download is deleted, not left behind
    assert list(download_dir.rglob("*")) == [download_dir / compute_document_id(pdf_path)]


# Upload script

def test_upload_script_uploads_then_skips(store, test_settings, pdf_path, tmp_path, capsys):

    env_file = write_env_file(tmp_path / ".env", storage_env_values(test_settings))

    first_exit = run_upload_script(env_file, pdf_path)
    first_output = capsys.readouterr().out
    second_exit = run_upload_script(env_file, pdf_path)
    second_output = capsys.readouterr().out

    assert first_exit == upload_script.EXIT_OK
    assert "UPLOADED tin_guide.pdf" in first_output
    assert second_exit == upload_script.EXIT_OK
    assert "SKIPPED  tin_guide.pdf" in second_output


def test_upload_script_takes_files_from_any_folder(store, test_settings, tmp_path):

    # Two PDFs in different folders, uploaded in one run
    first_dir = tmp_path / "downloads"
    second_dir = tmp_path / "desktop"
    first_dir.mkdir()
    second_dir.mkdir()
    first_pdf = write_text_pdf(first_dir / "tin_guide.pdf", [PAGE_ONE_LINES])
    second_pdf = write_text_pdf(second_dir / "tin_faq.pdf", [PAGE_TWO_LINES])
    env_file = write_env_file(tmp_path / ".env", storage_env_values(test_settings))

    exit_code = run_upload_script(env_file, first_pdf, second_pdf)

    assert exit_code == upload_script.EXIT_OK
    assert len(store.list_documents(SERVICE_SOURCE)) == 2


def test_upload_script_reports_bad_files(store, test_settings, pdf_path, tmp_path, capsys):

    # A missing file fails, but the good PDF is still uploaded
    env_file = write_env_file(tmp_path / ".env", storage_env_values(test_settings))

    exit_code = run_upload_script(env_file, tmp_path / "missing.pdf", pdf_path)

    output = capsys.readouterr().out
    assert exit_code == upload_script.EXIT_FAILURES
    assert "FAILED   missing.pdf" in output
    assert "UPLOADED tin_guide.pdf" in output


def test_upload_script_reports_unreachable_storage(test_settings, pdf_path, tmp_path, capsys):

    values = storage_env_values(test_settings, OBJECT_STORAGE_ENDPOINT=UNREACHABLE_ENDPOINT)
    env_file = write_env_file(tmp_path / ".env", values)

    exit_code = run_upload_script(env_file, pdf_path)

    assert exit_code == upload_script.EXIT_INVALID_INPUT
    assert "not reachable" in capsys.readouterr().err
