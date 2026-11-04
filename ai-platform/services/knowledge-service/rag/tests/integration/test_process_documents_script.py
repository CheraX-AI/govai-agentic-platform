"""Tests for the process_documents command-line script.

The script reads PDFs only from SeaweedFS and stores embeddings in ChromaDB, so
these tests upload synthetic PDFs to the real test bucket first and store into
the test collection (see conftest.py). Tests that embed call the real NVIDIA API.
"""

import pytest

from app.processing.pdf_extractor import DocumentSource, DocumentType
from app.storage.config import REQUIRED_VARIABLES as STORAGE_VARIABLES
from app.vector_store.config import REQUIRED_VARIABLES as CHROMA_VARIABLES
from scripts import process_documents as script
from tests.helpers import (
    TEST_NVIDIA_KEY,
    chroma_env_values,
    make_env,
    storage_env_values,
    write_env_file,
    write_real_env_file,
)
from tests.integration.test_pipeline import PAGE_ONE_LINES, PAGE_TWO_LINES, write_text_pdf

EMBEDDING_VARIABLES = list(make_env())

# Nothing listens on port 1, so connecting there always fails
UNREACHABLE_ENDPOINT = "http://127.0.0.1:1"

SERVICE_SOURCE = DocumentSource(
    department_id="IRD",
    service_id="TIN_REGISTRATION",
    document_type=DocumentType.SERVICE_GUIDELINE,
)


# Fixtures

@pytest.fixture
def folders(tmp_path, monkeypatch):

    # Remove real settings so the tests use only the .env file they write
    for name in EMBEDDING_VARIABLES + STORAGE_VARIABLES + CHROMA_VARIABLES:
        monkeypatch.delenv(name, raising=False)

    pdf_dir = tmp_path / "pdfs"
    pdf_dir.mkdir()

    return {"pdfs": pdf_dir, "env": tmp_path / ".env"}


# Helper functions

def run_script(folders, *extra):

    # Run main() with the test folders and a service guideline source
    exit_code = script.main(
        [
            "--department-id", "IRD",
            "--service-id", "TIN_REGISTRATION",
            "--document-type", "service_guideline",
            "--env-file", str(folders["env"]),
            *extra,
        ]
    )

    return exit_code


def upload_pdf(store, folders, file_name, lines=PAGE_ONE_LINES):

    # Write a synthetic PDF and store it in the test bucket
    pdf_path = write_text_pdf(folders["pdfs"] / file_name, [lines])
    stored = store.upload_document(pdf_path, SERVICE_SOURCE)

    return stored


def write_full_env(folders, test_settings, chroma_settings, **overrides):

    # Real NVIDIA settings, plus the test bucket and the test collection
    values = storage_env_values(test_settings)
    values.update(chroma_env_values(chroma_settings))
    values.update(overrides)
    env_file = write_real_env_file(folders["env"], **values)

    return env_file


def stored_document_ids(vector_store):

    # The document IDs of every record in the test collection
    response = vector_store.collection.get(include=["metadatas"])
    document_ids = set()
    for metadata in response["metadatas"]:
        document_ids.add(metadata["document_id"])

    return document_ids


def write_storage_only_env(folders, test_settings, **overrides):

    # Storage settings without any NVIDIA settings
    env_file = write_env_file(folders["env"], storage_env_values(test_settings, **overrides))

    return env_file


# Embedding settings

def test_missing_key_fails_before_any_processing(store, test_settings, folders, capsys):

    # No NVIDIA key is set, so the script stops before downloading the PDFs
    upload_pdf(store, folders, "tin_guide.pdf")
    write_storage_only_env(folders, test_settings)

    exit_code = run_script(folders)

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "NVIDIA_API_KEY is required" in capsys.readouterr().err


def test_invalid_key_is_not_printed(store, test_settings, folders, capsys):

    # A wrong key in the .env file is reported without showing its value
    upload_pdf(store, folders, "tin_guide.pdf")
    values = make_env(NVIDIA_API_KEY="sk-secret-value-123456")
    values.update(storage_env_values(test_settings))
    write_env_file(folders["env"], values)

    exit_code = run_script(folders)

    output = capsys.readouterr()
    assert exit_code == script.EXIT_INVALID_INPUT
    assert "sk-secret-value-123456" not in output.out + output.err


def test_settings_come_from_the_env_file(store, test_settings, vector_store, chroma_settings, folders, monkeypatch):

    # The script reads the model details from the --env-file it is given
    upload_pdf(store, folders, "tin_guide.pdf")
    write_full_env(folders, test_settings, chroma_settings, EMBEDDING_BATCH_SIZE="7")

    # Record the settings passed to the real create_embedder
    received = []
    real_create_embedder = script.create_embedder

    def recording_create_embedder(settings):
        received.append(settings)
        return real_create_embedder(settings)

    monkeypatch.setattr(script, "create_embedder", recording_create_embedder)

    assert run_script(folders) == script.EXIT_OK
    assert received[0].batch_size == 7


# Processing

def test_dry_run_only_extracts_and_chunks(store, test_settings, folders, capsys):

    # No NVIDIA key and no ChromaDB are needed for a dry run
    upload_pdf(store, folders, "tin_guide.pdf")
    write_storage_only_env(folders, test_settings)

    exit_code = run_script(folders, "--dry-run")

    output = capsys.readouterr().out
    assert exit_code == script.EXIT_OK
    assert "Processed PDFs:  1" in output
    assert "Model:" not in output
    assert "ChromaDB:" not in output


def test_full_run_embeds_and_stores_every_stored_pdf(
    store, test_settings, vector_store, chroma_settings, folders, capsys
):

    # Two stored PDFs, one with an upper-case extension in its original name
    first = upload_pdf(store, folders, "tin_guide.pdf")
    second = upload_pdf(store, folders, "tin_faq.PDF", PAGE_ONE_LINES[:3])
    write_full_env(folders, test_settings, chroma_settings)

    exit_code = run_script(folders)

    output = capsys.readouterr().out
    assert exit_code == script.EXIT_OK
    assert "Chunks failed:   0" in output
    assert "ChromaDB:        2 documents" in output
    assert stored_document_ids(vector_store) == {first.document_id, second.document_id}


def test_chromadb_records_point_to_the_stored_pdf(store, test_settings, vector_store, chroma_settings, folders):

    stored = upload_pdf(store, folders, "tin_guide.pdf")
    write_full_env(folders, test_settings, chroma_settings)

    run_script(folders)

    # Every record keeps its text, its source, and the key of the original PDF
    chunks = vector_store.get_document_chunks(stored.document_id)
    assert chunks
    for chunk in chunks:
        assert chunk.text
        assert chunk.metadata["object_key"] == stored.object_key
        assert chunk.metadata["source_file"] == "tin_guide.pdf"
        assert chunk.metadata["service_id"] == "TIN_REGISTRATION"


def test_documents_are_stored_as_published_by_default(store, test_settings, vector_store, chroma_settings, folders):

    stored = upload_pdf(store, folders, "tin_guide.pdf")
    write_full_env(folders, test_settings, chroma_settings)

    run_script(folders)

    # Admins upload only verified documents, so search can find them
    for chunk in vector_store.get_document_chunks(stored.document_id):
        assert chunk.metadata["publication_status"] == "published"


def test_status_change_updates_the_records(
    store, test_settings, vector_store, chroma_settings, folders, capsys
):

    stored = upload_pdf(store, folders, "tin_guide.pdf")
    write_full_env(folders, test_settings, chroma_settings)
    run_script(folders)
    first_count = vector_store.count()
    capsys.readouterr()

    # Mark the same document as a draft; the same chunk IDs are updated, not added
    exit_code = run_script(folders, "--status", "draft")

    assert exit_code == script.EXIT_OK
    assert vector_store.count() == first_count
    for chunk in vector_store.get_document_chunks(stored.document_id):
        assert chunk.metadata["publication_status"] == "draft"


def test_unknown_status_is_rejected(folders, capsys):

    # argparse stops the run before any setting is read
    with pytest.raises(SystemExit):
        run_script(folders, "--status", "archived")

    assert "invalid choice" in capsys.readouterr().err


def test_running_again_creates_no_duplicates(store, test_settings, vector_store, chroma_settings, folders):

    # The second run embeds again and upserts the same chunk IDs
    upload_pdf(store, folders, "tin_guide.pdf")
    write_full_env(folders, test_settings, chroma_settings)

    run_script(folders)
    first_count = vector_store.count()
    exit_code = run_script(folders)

    assert exit_code == script.EXIT_OK
    assert vector_store.count() == first_count


def test_revised_pdf_replaces_the_old_version(store, test_settings, vector_store, chroma_settings, folders, tmp_path):

    # Version 1 is processed and stored
    first = upload_pdf(store, folders, "tin_guide.pdf")
    write_full_env(folders, test_settings, chroma_settings)
    run_script(folders)

    # Version 2 has the same file name but new content
    (tmp_path / "revised").mkdir()
    revised_path = write_text_pdf(tmp_path / "revised" / "tin_guide.pdf", [PAGE_TWO_LINES])
    second = store.upload_document(revised_path, SERVICE_SOURCE)
    exit_code = run_script(folders)

    # Only the new version is searchable; both PDFs stay in SeaweedFS
    assert exit_code == script.EXIT_OK
    assert stored_document_ids(vector_store) == {second.document_id}
    assert first.object_key in store.list_documents(SERVICE_SOURCE)


def test_rejected_key_gives_exit_code_one(store, test_settings, vector_store, chroma_settings, folders, capsys):

    # NVIDIA rejects the key in the .env file
    upload_pdf(store, folders, "tin_guide.pdf")
    write_full_env(folders, test_settings, chroma_settings, NVIDIA_API_KEY=TEST_NVIDIA_KEY)

    exit_code = run_script(folders)

    output = capsys.readouterr().out
    assert exit_code == script.EXIT_FAILURES
    assert "FAILED tin_guide.pdf: EmbeddingAuthError" in output
    assert "NVIDIA rejected the API key" in output


def test_no_stored_pdfs_does_nothing(store, test_settings, folders, capsys):

    write_storage_only_env(folders, test_settings)

    exit_code = run_script(folders)

    assert exit_code == script.EXIT_OK
    assert "No stored PDF files found" in capsys.readouterr().out


# Invalid input and storage problems

def test_service_guideline_needs_service_id(folders, capsys):

    # The source is checked before storage is contacted
    exit_code = script.main(
        [
            "--department-id", "IRD",
            "--document-type", "service_guideline",
            "--env-file", str(folders["env"]),
            "--dry-run",
        ]
    )

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "service_id is required" in capsys.readouterr().err


def test_missing_storage_settings_are_reported(folders, capsys):

    folders["env"].write_text("", encoding="utf-8")

    exit_code = run_script(folders, "--dry-run")

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "OBJECT_STORAGE_ENDPOINT is required" in capsys.readouterr().err


def test_missing_chromadb_settings_are_reported(store, test_settings, folders, capsys):

    # Embedding is on, so ChromaDB settings are needed
    upload_pdf(store, folders, "tin_guide.pdf")
    write_real_env_file(folders["env"], **storage_env_values(test_settings))

    exit_code = run_script(folders)

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "CHROMA_HOST is required" in capsys.readouterr().err


def test_unreachable_chromadb_is_reported(store, test_settings, chroma_settings, folders, capsys):

    # Nothing listens on port 1, so the script stops before downloading anything
    upload_pdf(store, folders, "tin_guide.pdf")
    write_full_env(folders, test_settings, chroma_settings, CHROMA_HOST="127.0.0.1", CHROMA_PORT="1")

    exit_code = run_script(folders)

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "ChromaDB is not reachable" in capsys.readouterr().err


def test_unreachable_storage_is_reported(test_settings, folders, capsys):

    write_storage_only_env(folders, test_settings, OBJECT_STORAGE_ENDPOINT=UNREACHABLE_ENDPOINT)

    exit_code = run_script(folders, "--dry-run")

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "not reachable" in capsys.readouterr().err
