"""Tests for dashboard.py: file names, messages, and the job steps with fake services."""

import pytest

pytest.importorskip("streamlit")

import dashboard
from app.embeddings.embedding_service import EmbeddingAuthError
from app.storage.exceptions import StorageConnectionError
from app.storage.object_storage import StoredDocument
from app.vector_store.exceptions import VectorStoreConnectionError
from app.vector_store.vector_store import StoreResult
from tests.helpers import make_fake_embedder
from tests.integration.test_pipeline import PAGE_ONE_LINES, write_blank_pdf, write_text_pdf


# Fakes

class FakeStore:
    """Stands in for DocumentStore; remembers what was deleted."""

    def __init__(self, existing=None, error=None):

        self.existing = existing or {}
        self.error = error
        self.deleted = []

    def upload_document(self, pdf_path, source):

        if self.error:
            raise self.error
        key = f"documents/IRD/TIN_REGISTRATION/{'a' * 64}.pdf"

        return StoredDocument(object_key=key, document_id="a" * 64, source_file=pdf_path.name, uploaded=True)

    def list_documents(self, source):

        return sorted(self.existing)

    def read_file_name(self, object_key):

        return self.existing[object_key]

    def delete_document(self, object_key):

        self.deleted.append(object_key)


class FakeVectorStore:

    def __init__(self, error=None):

        self.error = error
        self.stored = []

    def store_document(self, embeddings):

        if self.error:
            raise self.error
        self.stored.append(embeddings)

        return StoreResult(document_id=embeddings.document_id, stored_count=len(embeddings.records))


def make_services(store=None, vector_store=None):

    services = dashboard.Services(store or FakeStore())
    services.embedder = make_fake_embedder()
    services.vector_store = vector_store or FakeVectorStore()

    return services


def make_job(pdf_bytes, file_name="IRD_TIN_REGISTRATION.pdf"):

    return dashboard.Job("IRD", "TIN_REGISTRATION", file_name, pdf_bytes=pdf_bytes)


# File names and folder names

@pytest.mark.parametrize(
    "file_name, expected",
    [
        ("IRD_TIN_REGISTRATION.pdf", ("IRD", "TIN_REGISTRATION")),
        ("ird_tin-registration.PDF", ("IRD", "TIN-REGISTRATION")),
        ("IRD.pdf", ("IRD", None)),
        ("IRD_.pdf", ("IRD", None)),
        ("_TIN.pdf", None),
        ("IRD TIN.pdf", None),
        ("IRD_TIN.docx", None),
        ("IRD_a b.pdf", None),
    ],
)
def test_parse_file_name(file_name, expected):

    assert dashboard.parse_file_name(file_name) == expected


def test_clean_id_upper_cases_and_rejects_unsafe_or_reserved_names():

    assert dashboard.clean_id(" ird ") == ("IRD", None)
    assert dashboard.clean_id("a/b")[0] is None
    assert dashboard.clean_id("")[0] is None
    assert dashboard.clean_id("_department")[0] is None


def test_format_seconds():

    assert dashboard.format_seconds(3.14) == "3.1 s"
    assert dashboard.format_seconds(42.4) == "42 s"
    assert dashboard.format_seconds(125) == "2 min 5 s"


# Messages for staff

@pytest.mark.parametrize(
    "error",
    [
        EmbeddingAuthError("nvapi-secret-key rejected"),
        StorageConnectionError("http://seaweedfs:8333 is not reachable"),
        VectorStoreConnectionError("chromadb:8000 is not reachable"),
        RuntimeError("Traceback password=hunter2"),
    ],
)
def test_friendly_error_hides_technical_details(error):

    message = dashboard.friendly_error(error)

    for secret in ("nvapi", "seaweedfs", "chromadb", "hunter2", "Traceback"):
        assert secret not in message


# Job steps

def test_job_runs_every_step_and_reports_success(tmp_path):

    pdf_path = write_text_pdf(tmp_path / "guide.pdf", [PAGE_ONE_LINES])
    services = make_services()
    job = make_job(pdf_path.read_bytes())

    dashboard.run_job(job, services)

    names = [step["name"] for step in job.steps]
    assert names == ["Upload to storage", "Read text", "Split into sections", "Create embeddings", "Save to search database"]
    assert all(step["state"] == "done" for step in job.steps)
    assert job.status == "succeeded"
    assert job.message.startswith("Embedding finished in")
    assert len(services.vector_store.stored) == 1

    # The PDF bytes are dropped once everything succeeded
    assert job.pdf_bytes is None


def test_job_deletes_an_older_copy_with_the_same_file_name(tmp_path):

    old_key = f"documents/IRD/TIN_REGISTRATION/{'b' * 64}.pdf"
    other_key = f"documents/IRD/TIN_REGISTRATION/{'c' * 64}.pdf"
    store = FakeStore(existing={old_key: "IRD_TIN_REGISTRATION.pdf", other_key: "other.pdf"})
    pdf_path = write_text_pdf(tmp_path / "guide.pdf", [PAGE_ONE_LINES])
    job = make_job(pdf_path.read_bytes())

    dashboard.run_job(job, make_services(store=store))

    assert store.deleted == [old_key]


def test_scanned_pdf_fails_with_a_plain_message_and_keeps_the_file_for_retry(tmp_path):

    pdf_path = write_blank_pdf(tmp_path / "scan.pdf")
    job = make_job(pdf_path.read_bytes())

    dashboard.run_job(job, make_services())

    assert job.status == "failed"
    assert "scanned" in job.message
    assert job.steps[-1]["state"] == "failed"
    assert job.pdf_bytes is not None


def test_storage_failure_is_reported_without_technical_details():

    job = make_job(b"%PDF-1.4")
    services = make_services(store=FakeStore(error=StorageConnectionError("http://seaweedfs:8333 down")))

    dashboard.run_job(job, services)

    assert job.status == "failed"
    assert "seaweedfs" not in job.message
    assert job.steps[0]["state"] == "failed"


def test_vector_database_failure_fails_the_last_step(tmp_path):

    pdf_path = write_text_pdf(tmp_path / "guide.pdf", [PAGE_ONE_LINES])
    job = make_job(pdf_path.read_bytes())
    services = make_services(vector_store=FakeVectorStore(error=VectorStoreConnectionError("down")))

    dashboard.run_job(job, services)

    assert job.status == "failed"
    assert job.steps[-1]["name"] == "Save to search database"
    assert job.steps[-1]["state"] == "failed"


def test_same_file_cannot_be_queued_twice_while_active():

    board = dashboard.JobBoard()
    first = make_job(b"x")
    first.status = "running"
    board.jobs[first.job_id] = first

    assert board.submit(make_job(b"x"), make_services()) is False
