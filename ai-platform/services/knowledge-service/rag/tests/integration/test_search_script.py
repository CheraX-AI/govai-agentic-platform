"""Tests for the search_documents command-line script.

The end-to-end test uploads a synthetic PDF to the test bucket, processes it
into the test collection with the real NVIDIA API, then searches it.
"""

import json

import pytest

from app.processing.pdf_extractor import DocumentSource, DocumentType
from app.storage.config import REQUIRED_VARIABLES as STORAGE_VARIABLES
from app.vector_store.config import REQUIRED_VARIABLES as CHROMA_VARIABLES
from scripts import process_documents as process_script
from scripts import search_documents as search_script
from tests.helpers import chroma_env_values, make_env, storage_env_values, write_env_file, write_real_env_file
from tests.integration.test_pipeline import write_text_pdf

SERVICE_SOURCE = DocumentSource(
    department_id="IRD",
    service_id="TIN_REGISTRATION",
    document_type=DocumentType.SERVICE_GUIDELINE,
)

# Two pages on different topics, so the right page must be found
REGISTRATION_PAGE = [
    "Taxpayer registration requires a completed registration form.",
    "Applicants must submit a copy of their national identity card.",
]
OPENING_HOURS_PAGE = [
    "The head office is open from 8.30 am to 4.15 pm on working days.",
    "The office is closed on weekends and public holidays.",
]


# Fixtures

@pytest.fixture
def env_file(tmp_path, monkeypatch, test_settings, chroma_settings):

    # Remove real settings so the scripts use only this .env file
    for name in list(make_env()) + STORAGE_VARIABLES + CHROMA_VARIABLES:
        monkeypatch.delenv(name, raising=False)

    # Real NVIDIA settings, plus the test bucket and the test collection
    values = storage_env_values(test_settings)
    values.update(chroma_env_values(chroma_settings))
    env_file = write_real_env_file(tmp_path / ".env", **values)

    return env_file


# Tests

def test_empty_question_is_rejected(capsys):

    # Checked before any setting is read or service contacted
    exit_code = search_script.main(["   "])

    assert exit_code == search_script.EXIT_INVALID_INPUT
    assert "empty" in capsys.readouterr().err


def test_missing_chromadb_settings_are_reported(tmp_path, monkeypatch, capsys):

    for name in CHROMA_VARIABLES:
        monkeypatch.delenv(name, raising=False)
    env_file = write_env_file(tmp_path / ".env", make_env())

    exit_code = search_script.main(["How do I register?", "--env-file", str(env_file)])

    assert exit_code == search_script.EXIT_INVALID_INPUT
    assert "CHROMA_HOST is required" in capsys.readouterr().err


def test_search_finds_the_matching_page(store, vector_store, env_file, tmp_path, capsys):

    # Upload and process a two-page PDF into the test collection
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [REGISTRATION_PAGE, OPENING_HOURS_PAGE])
    stored = store.upload_document(pdf_path, SERVICE_SOURCE)
    process_exit = process_script.main(
        [
            "--department-id", "IRD",
            "--service-id", "TIN_REGISTRATION",
            "--document-type", "service_guideline",
            "--env-file", str(env_file),
        ]
    )
    capsys.readouterr()

    # Ask about the documents needed for registration
    search_exit = search_script.main(
        [
            "What documents must I submit to register as a taxpayer?",
            "--department-id", "IRD",
            "--service-id", "TIN_REGISTRATION",
            "--top-k", "1",
            "--env-file", str(env_file),
        ]
    )

    output = capsys.readouterr().out
    assert process_exit == process_script.EXIT_OK
    assert search_exit == search_script.EXIT_OK
    assert "page 1" in output
    assert "national identity card" in output
    assert stored.object_key in output


def test_search_in_another_service_finds_nothing(store, vector_store, env_file, tmp_path, capsys):

    # Nothing is stored for VAT, so a VAT-only search has no results
    exit_code = search_script.main(
        ["How do I register?", "--department-id", "IRD", "--service-id", "VAT_REGISTRATION", "--env-file", str(env_file)]
    )

    assert exit_code == search_script.EXIT_OK
    assert "No matching chunks found" in capsys.readouterr().out


def process_test_pdf(store, env_file, tmp_path, *extra):

    # Upload and process the two-page PDF into the test collection
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [REGISTRATION_PAGE, OPENING_HOURS_PAGE])
    stored = store.upload_document(pdf_path, SERVICE_SOURCE)
    exit_code = process_script.main(
        [
            "--department-id", "IRD",
            "--service-id", "TIN_REGISTRATION",
            "--document-type", "service_guideline",
            "--title", "Obtaining a Taxpayer Identification Number (TIN)",
            "--env-file", str(env_file),
            *extra,
        ]
    )

    return stored, exit_code


def test_json_output_has_the_full_result(store, vector_store, env_file, tmp_path, capsys):

    stored, process_exit = process_test_pdf(store, env_file, tmp_path)
    capsys.readouterr()

    search_exit = search_script.main(
        [
            "What documents must I submit to register as a taxpayer?",
            "--department-id", "IRD",
            "--service-id", "TIN_REGISTRATION",
            "--top-k", "2",
            "--json",
            "--env-file", str(env_file),
        ]
    )

    # The same JSON the Agent Orchestrator will receive
    data = json.loads(capsys.readouterr().out)
    best = data["results"][0]
    assert process_exit == process_script.EXIT_OK
    assert search_exit == search_script.EXIT_OK
    assert data["top_k"] == 2
    assert best["rank"] == 1
    assert best["page_number"] == 1
    assert best["document_id"] == stored.document_id
    assert best["chunk_id"].startswith(stored.document_id)
    assert best["document_title"] == "Obtaining a Taxpayer Identification Number (TIN)"
    assert best["object_key"] == stored.object_key
    assert 0 <= best["distance"] <= 2
    assert best["similarity"] == pytest.approx(1 - best["distance"])


def test_draft_document_is_not_found(store, vector_store, env_file, tmp_path, capsys):

    # Processed as a draft: stored in ChromaDB, but hidden from search
    _, process_exit = process_test_pdf(store, env_file, tmp_path, "--status", "draft")
    capsys.readouterr()

    search_exit = search_script.main(
        ["What documents must I submit to register as a taxpayer?", "--env-file", str(env_file)]
    )

    assert process_exit == process_script.EXIT_OK
    assert vector_store.count() > 0
    assert search_exit == search_script.EXIT_OK
    assert "No matching chunks found" in capsys.readouterr().out


def test_invalid_top_k_is_rejected_before_connecting(capsys):

    exit_code = search_script.main(["How do I register?", "--top-k", "0"])

    assert exit_code == search_script.EXIT_INVALID_INPUT
    assert "top_k" in capsys.readouterr().err
