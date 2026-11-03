"""Tests for the process_documents command-line script."""

import pytest

from scripts import process_documents as script
from tests.helpers import TEST_NVIDIA_KEY, make_env, write_env_file, write_real_env_file
from tests.integration.test_pipeline import PAGE_ONE_LINES, write_text_pdf

EMBEDDING_VARIABLES = list(make_env())


# Fixtures

@pytest.fixture
def folders(tmp_path, monkeypatch):

    # Remove real settings so the tests never use the developer's .env or key
    for name in EMBEDDING_VARIABLES:
        monkeypatch.delenv(name, raising=False)

    input_dir = tmp_path / "input"
    input_dir.mkdir()
    env_file = tmp_path / ".env"
    env_file.write_text("", encoding="utf-8")

    return {"input": input_dir, "output": tmp_path / "output", "env": env_file}


# Helper functions

def run_script(folders, *extra):

    # Run main() with the test folders and a service guideline source
    return script.main(
        [
            "--department-id", "IRD",
            "--service-id", "TIN_REGISTRATION",
            "--document-type", "service_guideline",
            "--input-dir", str(folders["input"]),
            "--output-dir", str(folders["output"]),
            "--env-file", str(folders["env"]),
            *extra,
        ]
    )


# Tests

def test_missing_key_fails_before_any_processing(folders, capsys):

    # No key is set, so the script stops before touching the PDFs
    write_text_pdf(folders["input"] / "tin_guide.pdf", [PAGE_ONE_LINES])

    exit_code = run_script(folders)

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "NVIDIA_API_KEY is required" in capsys.readouterr().err
    assert not folders["output"].exists()


def test_invalid_key_is_not_printed(folders, monkeypatch, capsys):

    # A wrong key in the .env file is reported without showing its value
    write_env_file(folders["env"], make_env(NVIDIA_API_KEY="sk-secret-value-123456"))
    write_text_pdf(folders["input"] / "tin_guide.pdf", [PAGE_ONE_LINES])

    exit_code = run_script(folders)

    output = capsys.readouterr()
    assert exit_code == script.EXIT_INVALID_INPUT
    assert "sk-secret-value-123456" not in output.out + output.err


def test_settings_come_from_the_env_file(folders, monkeypatch):

    # The script reads the model details from the --env-file it is given
    write_text_pdf(folders["input"] / "tin_guide.pdf", [PAGE_ONE_LINES])
    write_real_env_file(folders["env"], EMBEDDING_BATCH_SIZE="7")

    # Record the settings passed to the real create_embedder
    received = []
    real_create_embedder = script.create_embedder

    def recording_create_embedder(settings):
        received.append(settings)
        return real_create_embedder(settings)

    monkeypatch.setattr(script, "create_embedder", recording_create_embedder)

    assert run_script(folders) == script.EXIT_OK
    assert received[0].batch_size == 7


def test_skip_embeddings_only_extracts_and_chunks(folders):

    # No key is needed when embedding is skipped
    write_text_pdf(folders["input"] / "tin_guide.pdf", [PAGE_ONE_LINES])

    exit_code = run_script(folders, "--skip-embeddings")

    assert exit_code == script.EXIT_OK
    assert len(list((folders["output"] / "chunks").iterdir())) == 1
    assert not (folders["output"] / "embeddings").exists()


def test_full_run_embeds_every_pdf(folders, capsys):

    # Two PDFs, one with an upper-case extension
    write_text_pdf(folders["input"] / "tin_guide.pdf", [PAGE_ONE_LINES])
    write_text_pdf(folders["input"] / "tin_faq.PDF", [PAGE_ONE_LINES[:3]])
    write_real_env_file(folders["env"])

    exit_code = run_script(folders)

    assert exit_code == script.EXIT_OK
    assert len(list((folders["output"] / "embeddings").glob("*.embeddings.json"))) == 2
    assert "Chunks failed:   0" in capsys.readouterr().out


def test_rejected_key_gives_exit_code_one(folders, capsys):

    # NVIDIA rejects the key in the .env file
    write_text_pdf(folders["input"] / "tin_guide.pdf", [PAGE_ONE_LINES])
    write_real_env_file(folders["env"], NVIDIA_API_KEY=TEST_NVIDIA_KEY)

    exit_code = run_script(folders)

    output = capsys.readouterr().out
    assert exit_code == script.EXIT_FAILURES
    assert "FAILED tin_guide.pdf: EmbeddingAuthError" in output
    assert "NVIDIA rejected the API key" in output


def test_service_guideline_needs_service_id(folders, capsys):

    exit_code = script.main(
        [
            "--department-id", "IRD",
            "--document-type", "service_guideline",
            "--input-dir", str(folders["input"]),
            "--skip-embeddings",
        ]
    )

    assert exit_code == script.EXIT_INVALID_INPUT
    assert "service_id is required" in capsys.readouterr().err


def test_missing_input_folder_is_rejected(folders, tmp_path):

    folders["input"] = tmp_path / "does-not-exist"

    assert run_script(folders, "--skip-embeddings") == script.EXIT_INVALID_INPUT


def test_empty_input_folder_does_nothing(folders, capsys):

    exit_code = run_script(folders)

    assert exit_code == script.EXIT_OK
    assert "No PDF files found" in capsys.readouterr().out
    assert not folders["output"].exists()
