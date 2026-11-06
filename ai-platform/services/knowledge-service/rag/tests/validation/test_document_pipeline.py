"""GA-73: validate the chunking and embedding pipeline with the real uploaded documents.

The PDFs listed in documents.json are taken from data/input, run through
extraction, chunking, and the real NVIDIA embedding API, and saved to
data/output. A report is written to data/output/reports/validation_report.md.

Skipped when no listed PDF is in data/input or NVIDIA_API_KEY is missing.
Re-runs reuse the saved embeddings, so they do not send the document again.
"""

import pytest

from tests.helpers import make_embedder, make_invalid_key_embedder
from tests.validation.pipeline_checks import (
    MANIFEST_PATH,
    check_document,
    check_error_cases,
    load_manifest,
    run_document,
    write_report,
)


# Fixtures

@pytest.fixture(scope="module")
def validation(tmp_path_factory):
    """Process every uploaded document once, run all checks, and write the report."""

    # Documents from documents.json that were uploaded to data/input
    entries = load_manifest()
    if not entries:
        pytest.skip(f"none of the PDFs in {MANIFEST_PATH.name} is in data/input")

    # Real NVIDIA clients built from rag/.env
    embedder = make_embedder()
    invalid_key_embedder = make_invalid_key_embedder()

    # Process and check each document
    runs = []
    results = []
    for entry in entries:
        run = run_document(entry, embedder)
        runs.append(run)
        results.extend(check_document(run, embedder))

    # Run invalid inputs and failures through the same pipeline
    error_results = check_error_cases(tmp_path_factory.mktemp("error_cases"), embedder, invalid_key_embedder)

    # Write the report before any test can fail
    report_path = write_report(runs, results, error_results, embedder.settings.model)

    validation = {
        "runs": runs,
        "results": results + error_results,
        "report_path": report_path,
    }

    return validation


# Helper functions

def assert_criterion(validation, criterion):

    # Collect the checks for this acceptance criterion
    checks = []
    for result in validation["results"]:
        if result.criterion == criterion:
            checks.append(result)

    # List every failed check with its detail
    failed = []
    for check in checks:
        if not check.passed:
            failed.append(f"{check.document} - {check.name}: {check.detail}")

    assert checks, f"no checks ran for '{criterion}'"
    assert not failed, "\n".join(failed)


# Tests: one per GA-73 acceptance criterion

def test_documents_are_divided_into_meaningful_chunks(validation):
    assert_criterion(validation, "Meaningful chunks")


def test_important_information_is_preserved(validation):
    assert_criterion(validation, "Information preserved")


def test_chunks_keep_their_identifier_and_source_metadata(validation):
    assert_criterion(validation, "Identifiers and metadata")


def test_embeddings_are_generated_for_valid_chunks(validation):
    assert_criterion(validation, "Embeddings generated")


def test_vectors_have_consistent_dimensions_and_valid_numbers(validation):
    assert_criterion(validation, "Valid vectors")


def test_invalid_inputs_and_errors_are_handled(validation):
    assert_criterion(validation, "Errors handled")


def test_pipeline_is_validated_with_the_uploaded_documents(validation):
    assert_criterion(validation, "Pipeline validated")


def test_embedding_vectors_and_report_are_saved_to_output(validation):

    # The embeddings of every document are in data/output/embeddings
    for run in validation["runs"]:
        assert run.processed is not None, run.failure
        assert run.processed.embeddings_path is not None
        assert run.processed.embeddings_path.is_file()

    # The report is in data/output/reports
    assert validation["report_path"].is_file()
