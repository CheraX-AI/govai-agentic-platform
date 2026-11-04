"""Tests for the process_documents command-line script.

The script reads PDFs only from SeaweedFS, so these tests upload synthetic PDFs
to the real test bucket first (see conftest.py).
"""

import json

import pytest

from app.processing.pdf_extractor import DocumentSource, DocumentType
from app.storage.config import REQUIRED_VARIABLES as STORAGE_VARIABLES
from scripts import process_documents as script
from tests.helpers import (
    TEST_NVIDIA_KEY,
    make_env,
    storage_env_values,
    write_env_file,
    write_real_env_file,
)
from tests.integration.test_pipeline import PAGE_ONE_LINES, write_text_pdf

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
    for name in EMBEDDING_VARIABLES + STORAGE_VARIABLES:
        monkeypatch.delenv(name, raising=False)

    pdf_dir = tmp_path / "pdfs"
    pdf_dir.mkdir()

    return {"pdfs": pdf_dir, "output": tmp_path / "output", "env": tmp_path / ".env"}


# Helper functions

def run_script(folders, *extra):

    # Run main() with the test folders and a service guideline source
    exit_code = script.main(
        [
            "--department-id", "IRD",
            "--service-id", "TIN_REGISTRATION",
            "--document-type", "service_guideline",
            "--output-dir", str(folders["output"]),
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
    assert not folders["output"].exists()


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


def test_settings_come_from_the_env_file(store, test_settings, folders, monkeypatch):

    # The script reads the model details from the --env-file it is given
    upload_pdf(store, folders, "tin_guide.pdf")
    write_real_env_file(folders["env"], EMBEDDING_BATCH_SIZE="7", **storage_env_values(test_settings))

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

def test_skip_embeddings_only_extracts_and_chunks(store, test_settings, folders):

    # No NVIDIA key is needed when embedding is skipped
    upload_pdf(store, folders, "tin_guide.pdf")
    write_storage_only_env(folders, test_settings)

    exit_code = run_script(folders, "--skip-embeddings")

    assert exit_code == script.EXIT_OK
    assert len(list((folders["output"] / "chunks").iterdir())) == 1
    assert not (folders["output"] / "embeddings").exists()


def test_full_run_embeds_every_stored_pdf(store, test_settings, folders, capsys):

    # Two stored PDFs, one with an upper-case extension in its original name
    upload_pdf(store, folders, "tin_guide.pdf")
    upload_pdf(store, folders, "tin_faq.PDF", PAGE_ONE_LINES[:3])
    write_real_env_file(folders["env"], **storage_env_values(test_settings))

    exit_code = run_script(folders)

    assert exit_code == script.EXIT_OK
    assert len(list((folders["output"] / "embeddings").glob("*.embeddings.json"))) == 2
    assert "Chunks failed:   0" in capsys.readouterr().out


def test_every_chunk_points_to_its_stored_pdf(store, test_settings, folders):

    stored = upload_pdf(store, folders, "tin_guide.pdf")
    write_storage_only_env(folders, test_settings)

    exit_code = run_script(folders, "--skip-embeddings")

    with open(folders["output"] / "chunks" / f"{stored.document_id}.chunks.json", "r", encoding="utf-8") as file:
        chunks = json.load(file)

    # The object_key links each chunk (and later each ChromaDB record) to the original PDF
    assert exit_code == script.EXIT_OK
    assert chunks
    for chunk in chunks:
        assert chunk["metadata"]["object_key"] == stored.object_key
        assert chunk["metadata"]["source_file"] == "tin_guide.pdf"


def test_rejected_key_gives_exit_code_one(store, test_settings, folders, capsys):

    # NVIDIA rejects the key in the .env file
    upload_pdf(store, folders, "tin_guide.pdf")
    write_real_env_file(folders["env"], NVIDIA_API_KEY=TEST_NVIDIA_KEY, **storage_env_values(test_settings))

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
    assert not folders["output"].exists()


# Invalid input and storage problems

def test_service_guideline_needs_service_id(folders, capsys):

    # The source is checked before storage is contacted
    exit_code = script.main(
        [
            "--department-id", "IRD",
            "--document-type", "service_guideline",
            "--env-file", str(folders["env"]),
            "--skip-embeddings",
        ]
    )

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "service_id is required" in capsys.readouterr().err


def test_missing_storage_settings_are_reported(folders, capsys):

    folders["env"].write_text("", encoding="utf-8")

    exit_code = run_script(folders, "--skip-embeddings")

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "OBJECT_STORAGE_ENDPOINT is required" in capsys.readouterr().err


def test_unreachable_storage_is_reported(test_settings, folders, capsys):

    write_storage_only_env(folders, test_settings, OBJECT_STORAGE_ENDPOINT=UNREACHABLE_ENDPOINT)

    exit_code = run_script(folders, "--skip-embeddings")

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "not reachable" in capsys.readouterr().err
