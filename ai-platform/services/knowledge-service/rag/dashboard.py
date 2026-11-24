"""Development dashboard: browse SeaweedFS folders, upload PDFs, and watch them being embedded into ChromaDB."""

import logging
import re
import tempfile
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import streamlit as st

from app.embeddings.embedding_service import (
    EmbeddingAuthError,
    EmbeddingConfigError,
    InvalidChunkError,
    InvalidEmbeddingError,
    create_embedder,
    embed_document,
    load_settings,
    validate_chunks,
)
from app.processing.chunker import chunk_documents, to_chunk_records
from app.processing.pdf_extractor import (
    CorruptedPDFError,
    DocumentSource,
    DocumentType,
    EncryptedPDFError,
    InvalidPDFError,
    PDFExtractionError,
    PDFTooLargeError,
    extract_pdf,
    to_langchain_documents,
)
from app.storage.config import load_storage_settings
from app.storage.exceptions import StorageError
from app.storage.object_storage import DEPARTMENT_FOLDER, create_document_store, document_id_from_key
from app.vector_store.config import load_vector_store_settings
from app.vector_store.exceptions import VectorStoreError
from app.vector_store.vector_store import create_vector_store

logger = logging.getLogger(__name__)

# Same rule as the storage keys: letters, digits, "_" and "-"
ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]+$")

REFRESH_SECONDS = 2

STATUS_ICONS = {"queued": "⏳", "running": "⏳", "succeeded": "✅", "failed": "❌", "ready": "✅", "not_embedded": "⚪"}
STATUS_LABELS = {
    "queued": "Waiting",
    "running": "Processing",
    "succeeded": "Ready",
    "failed": "Failed",
    "ready": "Ready",
    "not_embedded": "Not embedded",
}
STEP_ICONS = {"running": "⏳", "done": "✅", "failed": "❌"}

EMBED_STEP = "Create embeddings"

# More specific error classes come first
ERROR_MESSAGES = [
    (EncryptedPDFError, "This PDF is password protected."),
    (PDFTooLargeError, "This PDF is too large."),
    (CorruptedPDFError, "This PDF is damaged and cannot be read."),
    (InvalidPDFError, "This file is not a valid PDF."),
    (PDFExtractionError, "This PDF could not be read."),
    (EmbeddingConfigError, "The embedding service is not available. Contact an administrator."),
    (EmbeddingAuthError, "The embedding service is not available. Contact an administrator."),
    (InvalidChunkError, "This document could not be prepared for search."),
    (InvalidEmbeddingError, "This document could not be prepared for search."),
    (StorageError, "The document storage is not available. Try again later."),
    (VectorStoreError, "The search database is not available. Try again later."),
]

# Original file names of stored PDFs never change for a key, so they are read once
FILE_NAMES = {}


# Jobs

class JobFailed(Exception):
    """A failure whose message is already safe to show to staff."""


class Job:
    """One PDF going through the steps, with the progress the page shows."""

    def __init__(self, department_id, service_id, file_name, pdf_bytes=None, object_key=None):

        self.department_id = department_id
        self.service_id = service_id
        self.file_name = file_name
        self.pdf_bytes = pdf_bytes
        self.object_key = object_key
        self.status = "queued"
        self.steps = []
        self.message = ""
        self.started_at = None
        self.finished_at = None

    @property
    def folder(self):

        return self.service_id or DEPARTMENT_FOLDER

    @property
    def job_id(self):

        return f"{self.department_id}/{self.folder}/{self.file_name}"

    def is_active(self):

        return self.status in ("queued", "running")

    def begin(self, name, detail=""):

        self.steps.append({"name": name, "state": "running", "detail": detail, "started": time.time(), "seconds": None})

    def update(self, detail):

        self.steps[-1]["detail"] = detail

    def end(self, detail=None):

        step = self.steps[-1]
        step["state"] = "done"
        step["seconds"] = time.time() - step["started"]
        if detail is not None:
            step["detail"] = detail

    def fail(self, message):

        # Mark the step that was running as failed
        if self.steps and self.steps[-1]["state"] == "running":
            self.steps[-1]["state"] = "failed"
            self.steps[-1]["seconds"] = time.time() - self.steps[-1]["started"]

        self.status = "failed"
        self.message = message
        self.finished_at = time.time()


class JobBoard:
    """All jobs of this dashboard, run one at a time in the background."""

    def __init__(self):

        self.jobs = {}
        self.lock = threading.Lock()
        self.executor = ThreadPoolExecutor(max_workers=1)

    def submit(self, job, services):
        """Queue a job. Returns False when the same file is already being processed."""

        with self.lock:
            existing = self.jobs.get(job.job_id)
            if existing is not None and existing.is_active():
                return False
            self.jobs[job.job_id] = job

        self.executor.submit(run_job, job, services)

        return True

    def get(self, job_id):

        return self.jobs.get(job_id)

    def remove(self, job_id):

        self.jobs.pop(job_id, None)

    def all(self):

        return list(self.jobs.values())

    def has_active(self):

        return any(job.is_active() for job in self.all())


class Services:
    """The storage client, plus the embedder and vector database created when first needed."""

    def __init__(self, store):

        self.store = store
        self.lock = threading.Lock()
        self.embedder = None
        self.vector_store = None

    def get_embedder(self):

        with self.lock:
            if self.embedder is None:
                self.embedder = create_embedder(load_settings())

        return self.embedder

    def get_vector_store(self):

        with self.lock:
            if self.vector_store is None:
                self.vector_store = create_vector_store(load_vector_store_settings())

        return self.vector_store


@st.cache_resource
def get_board():

    return JobBoard()


@st.cache_resource
def get_services():

    # Fails (and is not cached) when SeaweedFS is down or the settings are wrong
    store = create_document_store(load_storage_settings())
    store.check_bucket()
    services = Services(store)

    return services


# Job steps

def run_job(job, services):
    """Run every step for one PDF. Runs in a background thread, so it never calls Streamlit."""

    job.status = "running"
    job.steps = []
    job.message = ""
    job.started_at = time.time()

    try:
        # The temporary folder is deleted afterwards, so no PDF stays on disk
        with tempfile.TemporaryDirectory() as work_dir:
            run_steps(job, services, work_dir)

    except Exception as error:
        logger.exception("document job failed", extra={"job_id": job.job_id})
        job.fail(friendly_error(error))
        return

    job.status = "succeeded"
    job.finished_at = time.time()
    job.pdf_bytes = None

    embed_step = next(step for step in job.steps if step["name"] == EMBED_STEP)
    job.message = f"Embedding finished in {format_seconds(embed_step['seconds'])}"



def run_steps(job, services, work_dir):

    source = make_source(job)

    # Step 1: put the PDF in its folder, or fetch it again when retrying
    job.begin("Upload to storage")
    if job.pdf_bytes is not None:
        pdf_path = Path(work_dir) / job.file_name
        pdf_path.write_bytes(job.pdf_bytes)
        stored = services.store.upload_document(pdf_path, source)
        job.object_key = stored.object_key
        job.end("Uploaded" if stored.uploaded else "Already in storage")
    else:
        pdf_path = services.store.download_document(job.object_key, work_dir)
        job.end("Already in storage")

    # Step 2: read the text of every page
    job.begin("Read text")
    source = source.model_copy(update={"object_key": job.object_key})
    document = extract_pdf(pdf_path, source)
    if document.needs_ocr:
        raise JobFailed("This PDF has no readable text. It looks like a scanned document.")
    job.end(f"{document.total_pages} pages")

    # Step 3: split the text into sections
    job.begin("Split into sections")
    chunks = chunk_documents(to_langchain_documents(document))
    if not chunks:
        raise JobFailed("This PDF has no readable text.")
    records = to_chunk_records(chunks)
    job.end(f"{len(records)} sections")

    # Step 4: embed the sections, showing progress after each batch
    job.begin(EMBED_STEP, f"0 / {len(records)} sections")
    embedder = services.get_embedder()
    models = validate_chunks(records, document.source_file)
    result = embed_document(models, embedder, lambda done, total: job.update(f"{done} / {total} sections"))
    if result.failed or result.embeddings is None:
        raise JobFailed("Some sections could not be embedded. Press Retry.")
    job.end(f"{len(result.embeddings.records)} sections")

    # Step 5: save in the vector database, then drop older copies of the same file
    job.begin("Save to search database")
    stored_result = services.get_vector_store().store_document(result.embeddings)
    remove_older_copies(services.store, source, job)
    job.end(f"{stored_result.stored_count} sections saved")


def remove_older_copies(store, source, job):

    # A revised PDF has a new key, so older PDFs with the same file name are deleted
    for object_key in store.list_documents(source):
        if object_key != job.object_key and store.read_file_name(object_key) == job.file_name:
            store.delete_document(object_key)


def make_source(job):

    # A folder named _department holds department guidelines, other folders hold service guidelines
    if job.service_id is None:
        document_type = DocumentType.DEPARTMENT_GUIDELINE
    else:
        document_type = DocumentType.SERVICE_GUIDELINE

    source = DocumentSource(
        department_id=job.department_id,
        service_id=job.service_id,
        document_type=document_type,
        title=Path(job.file_name).stem,
    )

    return source


# Names and messages

def clean_id(text):
    """Upper-case a department or service ID. Returns (id, error); error is None when it is valid."""

    # Check the name, upper-casing it to match IDs like TIN_REGISTRATION
    value = (text or "").strip().upper()
    error = None
    if not value:
        error = "Enter a name."
    elif not ID_PATTERN.match(value):
        error = "Use only letters, digits, '_' and '-'."
    elif value == DEPARTMENT_FOLDER.upper():
        error = "This name is reserved."

    result = (None, error) if error else (value, None)

    return result


def parse_file_name(file_name):
    """Read Department_Service.pdf. Returns (department, service or None), or None when the name does not fit."""

    if not file_name.lower().endswith(".pdf"):
        return None

    # Split on the first "_": the department, then the rest is the service
    department_text, _, service_text = file_name[:-4].partition("_")
    department_id, error = clean_id(department_text)
    if error:
        return None

    # No "_" in the name means a department-wide guideline
    if not service_text:
        return department_id, None

    service_id, error = clean_id(service_text)
    if error:
        return None

    target = (department_id, service_id)

    return target


def friendly_error(error):
    """A short message for staff. Technical details stay in the server log."""

    # A failure that already carries a safe message
    if isinstance(error, JobFailed):
        return str(error)

    # The first matching error class decides the message
    message = "Something went wrong. Try again, or contact an administrator."
    for error_class, text in ERROR_MESSAGES:
        if isinstance(error, error_class):
            message = text
            break

    return message


def format_seconds(seconds):

    # One decimal under 10 s, then whole seconds, then minutes
    if seconds is None:
        text = "-"
    elif seconds < 10:
        text = f"{seconds:.1f} s"
    elif seconds < 60:
        text = f"{seconds:.0f} s"
    else:
        text = f"{int(seconds // 60)} min {int(seconds % 60)} s"

    return text


def folder_label(folder):

    label = "Department guidelines" if folder == DEPARTMENT_FOLDER else folder

    return label


# Page

def main():

    st.set_page_config(page_title="GovAI Documents", layout="wide")
    st.title("GovAI Documents")
    st.caption("Upload PDFs into department folders. They are embedded and searchable once they show Ready.")

    try:
        services = get_services()
    except StorageError as error:
        logger.error("storage unavailable: %s", error)
        st.error(friendly_error(error))
        st.stop()

    board = get_board()

    render_folder_forms(services)
    render_smart_upload(services, board)
    st.divider()

    # Refresh by itself while a PDF is being processed
    active = board.has_active()
    folders_view = st.fragment(run_every=REFRESH_SECONDS if active else None)(render_folders)
    folders_view(services, board)


def render_folder_forms(services):

    with st.expander("Create a folder"):
        left, right = st.columns(2)

        with left.form("new-department", clear_on_submit=True):
            name = st.text_input("New department", placeholder="IRD")
            if st.form_submit_button("Create department"):
                department_id, error = clean_id(name)
                if error:
                    st.error(error)
                else:
                    create_folder(services, department_id, None)

        with right.form("new-service", clear_on_submit=True):
            folders = list_folders(services)
            department = st.selectbox("Department", sorted(folders))
            name = st.text_input("New service folder", placeholder="TIN_REGISTRATION")
            if st.form_submit_button("Create service folder"):
                service_id, error = clean_id(name)
                if error:
                    st.error(error)
                elif department is None:
                    st.error("Create a department first.")
                else:
                    create_folder(services, department, service_id)


def create_folder(services, department_id, service_id):

    try:
        services.store.create_folder(department_id, service_id)
    except StorageError as error:
        logger.error("folder creation failed: %s", error)
        st.error(friendly_error(error))
        return

    st.rerun()


def render_smart_upload(services, board):

    with st.expander("Upload by file name (Department_Service.pdf)"):
        round_number = st.session_state.get("upload_round", 0)
        files = st.file_uploader(
            "PDF files, named like IRD_TIN_REGISTRATION.pdf",
            type=["pdf"],
            accept_multiple_files=True,
            key=f"smart-upload-{round_number}",
        )

        # Show where each file will go, and which names do not fit
        targets = []
        for file in files:
            target = parse_file_name(file.name)
            if target is None:
                st.warning(f"{file.name}: the name must look like Department_Service.pdf")
                continue
            department_id, service_id = target
            st.write(f"{file.name} → {department_id} / {folder_label(service_id or DEPARTMENT_FOLDER)}")
            targets.append((file, department_id, service_id))

        if targets and st.button("Upload", key="smart-upload-button"):
            for file, department_id, service_id in targets:
                submit_upload(board, services, file, department_id, service_id)
            st.session_state["upload_round"] = round_number + 1
            st.rerun()


def submit_upload(board, services, file, department_id, service_id):

    # Keep the bytes in memory; the job writes them to a temporary file
    job = Job(department_id, service_id, Path(file.name).name, pdf_bytes=file.getvalue())
    board.submit(job, services)


def list_folders(services):

    try:
        # Read the folders and PDFs from SeaweedFS
        folders = services.store.list_folders()

    except StorageError as error:
        logger.error("folder listing failed: %s", error)
        st.error(friendly_error(error))
        folders = {}

    return folders


def render_folders(services, board):

    active = board.has_active()
    folders = list_folders(services)

    # Folders that only exist in a job (e.g. its upload failed) are still shown
    for job in board.all():
        folders.setdefault(job.department_id, {}).setdefault(job.folder, [])
    for services_in_department in folders.values():
        services_in_department.setdefault(DEPARTMENT_FOLDER, [])

    rows = build_rows(services, board, folders)
    embedded_ids = read_embedded_ids(services, rows)

    if not folders:
        st.info("No folders yet. Create a department folder above.")

    for department_id in sorted(folders):
        with st.expander(f"📁 {department_id}", expanded=True):
            for folder in sorted(folders[department_id], key=lambda name: (name != DEPARTMENT_FOLDER, name)):
                render_folder(services, board, department_id, folder, rows[(department_id, folder)], embedded_ids)

    # When the last job finishes, reload the whole page once so the refresh timer stops
    was_active = st.session_state.get("was_active", False)
    st.session_state["was_active"] = active
    if was_active and not active:
        st.rerun()


def build_rows(services, board, folders):
    """One row per PDF in each folder: stored PDFs, plus jobs that never reached storage."""

    rows = {}
    for department_id, folder_map in folders.items():
        for folder, object_keys in folder_map.items():
            folder_rows = []
            seen_names = set()

            for object_key in sorted(object_keys):
                file_name = read_file_name(services, object_key)
                seen_names.add(file_name)
                job = board.get(f"{department_id}/{folder}/{file_name}")
                folder_rows.append(
                    {
                        "file_name": file_name,
                        "object_key": object_key,
                        "document_id": document_id_from_key(object_key),
                        "job": job,
                    }
                )

            for job in board.all():
                if job.department_id == department_id and job.folder == folder and job.file_name not in seen_names:
                    folder_rows.append({"file_name": job.file_name, "object_key": None, "document_id": None, "job": job})

            rows[(department_id, folder)] = folder_rows

    return rows


def read_file_name(services, object_key):

    # Read the name once; fall back to the key's name when storage cannot be read
    if object_key not in FILE_NAMES:
        try:
            FILE_NAMES[object_key] = services.store.read_file_name(object_key)

        except StorageError:
            return object_key.rsplit("/", 1)[-1]

    file_name = FILE_NAMES[object_key]

    return file_name


def read_embedded_ids(services, rows):
    """Which stored PDFs already have chunks in ChromaDB, or None when ChromaDB cannot be read."""

    document_ids = set()
    for folder_rows in rows.values():
        for row in folder_rows:
            if row["document_id"]:
                document_ids.add(row["document_id"])

    try:
        # One query for every stored PDF
        embedded_ids = services.get_vector_store().embedded_document_ids(document_ids)

    except (VectorStoreError, EmbeddingConfigError) as error:
        logger.error("embedded check failed: %s", error)
        embedded_ids = None

    return embedded_ids


def row_status(row, embedded_ids):

    # A job from this session wins; otherwise ChromaDB tells whether the PDF was embedded
    if row["job"] is not None:
        status = row["job"].status
    elif embedded_ids is not None and row["document_id"] in embedded_ids:
        status = "ready"
    else:
        status = "not_embedded"

    return status


def render_folder(services, board, department_id, folder, folder_rows, embedded_ids):

    header, upload = st.columns([6, 2])
    header.markdown(f"**📂 {folder_label(folder)}** ({len(folder_rows)} PDFs)")

    # Upload straight into this folder
    service_id = None if folder == DEPARTMENT_FOLDER else folder
    round_number = st.session_state.get("upload_round", 0)
    with upload.popover("Upload PDF"):
        files = st.file_uploader(
            "PDF files",
            type=["pdf"],
            accept_multiple_files=True,
            key=f"upload-{department_id}-{folder}-{round_number}",
        )
        if files and st.button("Upload", key=f"upload-button-{department_id}-{folder}"):
            for file in files:
                submit_upload(board, services, file, department_id, service_id)
            st.session_state["upload_round"] = round_number + 1
            st.rerun()

    if not folder_rows:
        st.caption("Empty")

    for row in folder_rows:
        render_row(services, board, department_id, folder, row, embedded_ids)


def render_row(services, board, department_id, folder, row, embedded_ids):

    status = row_status(row, embedded_ids)
    job = row["job"]
    row_id = f"{department_id}/{folder}/{row['file_name']}"
    active = job is not None and job.is_active()

    name, label, icon, retry, delete = st.columns([5, 2, 1, 1, 1])
    name.write(row["file_name"])
    label.write(STATUS_LABELS[status])

    # The icon opens the full list of steps
    with icon.popover(STATUS_ICONS[status]):
        render_steps(job, status)

    if active:
        return

    # Retry runs the steps again from the stored PDF
    if status in ("failed", "not_embedded") and retry.button("Retry", key=f"retry-{row_id}"):
        retry_job(services, board, department_id, folder, row)
        st.rerun()

    # Ask once before deleting
    if st.session_state.get("confirm_delete") == row_id:
        if delete.button("Confirm", key=f"confirm-{row_id}", type="primary"):
            delete_row(services, board, row, row_id)
            st.session_state["confirm_delete"] = None
            st.rerun()
        if delete.button("Cancel", key=f"cancel-{row_id}"):
            st.session_state["confirm_delete"] = None
            st.rerun()
    elif delete.button("Delete", key=f"delete-{row_id}"):
        st.session_state["confirm_delete"] = row_id
        st.rerun()


def render_steps(job, status):

    if job is None:
        st.write("Embedded earlier." if status == "ready" else "Not embedded yet. Press Retry to embed it.")
        return

    for step in job.steps:
        seconds = step["seconds"] if step["seconds"] is not None else time.time() - step["started"]
        detail = f" - {step['detail']}" if step["detail"] else ""
        st.write(f"{STEP_ICONS[step['state']]} {step['name']}{detail} ({format_seconds(seconds)})")

    if job.status == "queued":
        st.write("⏳ Waiting for the previous document to finish.")
    elif job.status == "succeeded":
        st.success(job.message)
    elif job.status == "failed":
        st.error(job.message)


def retry_job(services, board, department_id, folder, row):

    # Reuse the file kept in memory when the upload itself failed; otherwise read it from storage
    old_job = row["job"]
    pdf_bytes = old_job.pdf_bytes if old_job is not None else None
    service_id = None if folder == DEPARTMENT_FOLDER else folder
    job = Job(department_id, service_id, row["file_name"], pdf_bytes=pdf_bytes, object_key=row["object_key"])
    board.submit(job, services)


def delete_row(services, board, row, row_id):

    try:
        # Remove the embeddings first, so a failure never leaves searchable text without a PDF
        if row["document_id"]:
            services.get_vector_store().delete_document(row["document_id"])
        if row["object_key"]:
            services.store.delete_document(row["object_key"])
            FILE_NAMES.pop(row["object_key"], None)

    except (StorageError, VectorStoreError) as error:
        logger.error("delete failed: %s", error)
        st.error(friendly_error(error))
        return

    board.remove(row_id)


if __name__ == "__main__":
    main()
