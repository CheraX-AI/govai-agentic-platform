"""Checks that validate real documents run through the pipeline, and the report writer.

Each check returns a CheckResult instead of raising, so the report lists every
check even when some fail. The tests in test_document_pipeline.py then assert
on the results, grouped by acceptance criterion.
"""

import json
from datetime import datetime
from pathlib import Path

import numpy as np
from pydantic import BaseModel

from app.embeddings import ChunkFileError, METADATA_VALUE_TYPES, load_embeddings, to_vector_store_batch
from app.embeddings.embedding_service import embedding_text
from app.embeddings.storage import load_chunks
from app.processing.chunker import CHUNK_OVERLAP, CHUNK_SIZE
from app.processing.pdf_extractor import DocumentSource, DocumentType
from app.processing.pipeline import process_documents
from tests.helpers import TOO_LONG_TEXT
from tests.integration.test_pipeline import write_blank_pdf, write_text_pdf

RAG_ROOT = Path(__file__).resolve().parents[2]
INPUT_DIR = RAG_ROOT / "data" / "input"
OUTPUT_DIR = RAG_ROOT / "data" / "output"
REPORT_PATH = OUTPUT_DIR / "reports" / "validation_report.md"
MANIFEST_PATH = Path(__file__).resolve().parent / "documents.json"

# Metadata every chunk must carry to trace it back to its source
REQUIRED_METADATA = [
    "document_id",
    "source_file",
    "department_id",
    "document_type",
    "page_number",
    "total_pages",
    "chunk_id",
    "chunk_index",
    "start_index",
]

# Embedding the same text twice must give almost the same vector
CONSISTENCY_THRESHOLD = 0.99


# Models

class CheckResult(BaseModel):
    criterion: str
    document: str
    name: str
    passed: bool
    detail: str


class DocumentRun(BaseModel):
    """Everything produced by one document, for the checks and the report."""

    entry: dict
    processed: object = None
    failure: str = ""
    extracted: dict = {}
    chunks: list = []
    embedding_file: object = None
    stats: dict = {}


# Public functions

def load_manifest():
    """Read documents.json and keep only the entries whose PDF is in data/input."""

    # Open and read the document list
    with open(MANIFEST_PATH, "r", encoding="utf-8") as file:
        entries = json.load(file)

    # Keep the documents that were uploaded
    available = []
    for entry in entries:
        if (INPUT_DIR / entry["file"]).is_file():
            available.append(entry)

    return available


def run_document(entry, embedder):
    """Run one PDF through extraction, chunking, and embedding into data/output."""

    run = DocumentRun(entry=entry)

    # Process the PDF with its source details from documents.json
    source = DocumentSource(
        department_id=entry["department_id"],
        service_id=entry.get("service_id"),
        document_type=DocumentType(entry["document_type"]),
        title=entry.get("title"),
    )
    report = process_documents([(INPUT_DIR / entry["file"], source)], OUTPUT_DIR, embedder=embedder)

    # A PDF that could not be processed has nothing more to check
    if report.failed:
        run.failure = f"{report.failed[0].error_type}: {report.failed[0].message}"
        return run

    run.processed = report.processed[0]

    # Read back the saved pages, chunks, and embeddings
    with open(run.processed.extracted_path, "r", encoding="utf-8") as file:
        run.extracted = json.load(file)

    if run.processed.chunks_path is not None:
        run.chunks = load_chunks(run.processed.chunks_path)

    if run.processed.embeddings_path is not None:
        run.embedding_file = load_embeddings(run.processed.embeddings_path)

    # Collect the numbers shown in the report
    run.stats = document_stats(run)

    return run


def check_document(run, embedder):
    """Run every check on one processed document."""

    name = run.entry["file"]

    # A document that failed to process fails every criterion
    if run.failure:
        result = CheckResult(
            criterion="Meaningful chunks", document=name, name="Processing", passed=False, detail=run.failure
        )
        return [result]

    results = []

    # Documents are divided into meaningful chunks
    results.append(check_extraction(run))
    results.append(check_clean_text(run))
    results.append(check_chunk_sizes(run))
    results.append(check_page_coverage(run))

    # Important information is preserved
    results.append(check_no_text_lost(run))
    results.append(check_overlap(run))
    for phrase in run.entry["key_phrases"]:
        results.append(check_key_phrase(run, phrase))

    # Unique identifiers and source metadata
    results.append(check_chunk_ids(run))
    results.append(check_metadata(run))

    # Embeddings are generated for valid chunks
    results.append(check_embeddings_match_chunks(run))

    # Consistent dimensions and valid numbers
    results.append(check_dimensions(run))
    results.append(check_numbers(run))
    results.append(check_consistency(run, embedder))
    results.append(check_vector_store_ready(run))

    # The pipeline is validated: questions find the right part of the document
    for question in run.entry["questions"]:
        results.append(check_retrieval(run, embedder, question))

    return results


def check_error_cases(work_dir, embedder, invalid_key_embedder):
    """Run invalid inputs and failures through the real pipeline and record how each is handled."""

    work_dir = Path(work_dir)
    source = DocumentSource(department_id="IRD", service_id="TIN_REGISTRATION", document_type=DocumentType.SERVICE_GUIDELINE)
    results = []

    # A scanned or blank PDF has no text: it is flagged for OCR and nothing is sent to NVIDIA
    blank_pdf = write_blank_pdf(work_dir / "blank.pdf")
    report = process_documents([(blank_pdf, source)], work_dir / "blank_output", embedder=invalid_key_embedder)
    document = report.processed[0]
    passed = not report.failed and document.needs_ocr and document.chunk_count == 0
    results.append(error_result("Blank (scanned) PDF", passed, f"needs OCR: {document.needs_ocr}, chunks: {document.chunk_count}, nothing sent to NVIDIA"))

    # An empty file, a file that is not a PDF, and a password-protected PDF are rejected by name
    empty_file = work_dir / "empty.pdf"
    empty_file.write_bytes(b"")
    not_a_pdf = work_dir / "notes.pdf"
    not_a_pdf.write_text("These are notes, not a PDF.", encoding="utf-8")
    locked_pdf = write_blank_pdf(work_dir / "locked.pdf", password="secret")
    expected_errors = {
        "empty.pdf": "InvalidPDFError",
        "notes.pdf": "InvalidPDFError",
        "locked.pdf": "EncryptedPDFError",
    }

    items = []
    for path in [empty_file, not_a_pdf, locked_pdf]:
        items.append((path, source))
    report = process_documents(items, work_dir / "invalid_output", embedder=embedder)

    errors = {}
    for failure in report.failed:
        errors[failure.source_file] = failure.error_type
    for file_name, error_type in expected_errors.items():
        passed = errors.get(file_name) == error_type
        results.append(error_result(f"Invalid file `{file_name}`", passed, f"recorded as {errors.get(file_name)}, the batch continued"))

    # A chunk longer than NVIDIA accepts fails alone; the PDF is still chunked
    long_pdf = write_text_pdf(work_dir / "long.pdf", [[TOO_LONG_TEXT]])
    report = process_documents(
        [(long_pdf, source)], work_dir / "long_output", chunk_size=100_000, chunk_overlap=0, embedder=embedder
    )
    document = report.processed[0]
    passed = not report.failed and len(document.failed_chunk_ids) == 1 and document.chunks_path is not None
    results.append(error_result("Chunk longer than NVIDIA accepts", passed, f"failed chunks: {len(document.failed_chunk_ids)}, chunks saved: {document.chunks_path is not None}"))

    # A rejected API key stops embedding, but the PDF is still extracted and chunked
    tin_pdf = write_text_pdf(work_dir / "tin.pdf", [["TIN can be obtained using the IRD e-services."]])
    faq_pdf = write_text_pdf(work_dir / "faq.pdf", [["Call the IRD Call Centre on 1944."]])
    report = process_documents(
        [(tin_pdf, source), (faq_pdf, source)], work_dir / "auth_output", embedder=invalid_key_embedder
    )
    error_types = []
    for failure in report.failed:
        error_types.append(failure.error_type)
    passed = report.embedding_stopped and error_types == ["EmbeddingAuthError"] and report.processed[0].chunks_path is not None
    results.append(error_result("API key rejected by NVIDIA", passed, f"errors: {error_types}, embedding stopped, remaining PDF still chunked"))

    # A damaged chunk file is rejected before anything is sent to NVIDIA
    bad_chunks = work_dir / "bad.chunks.json"
    with open(bad_chunks, "w", encoding="utf-8") as file:
        json.dump([{"chunk_id": "doc:0", "text": "", "metadata": {"document_id": "doc"}}], file)
    try:
        load_chunks(bad_chunks)
        results.append(error_result("Invalid chunk file", False, "the invalid file was accepted"))

    except ChunkFileError as error:
        results.append(error_result("Invalid chunk file", True, f"rejected with ChunkFileError: {error}"))

    return results


def write_report(runs, results, error_results, model_name):
    """Write the results as a Markdown report in data/output/reports."""

    lines = []

    # Title and summary
    passed = 0
    for result in results + error_results:
        if result.passed:
            passed += 1
    total = len(results) + len(error_results)
    status = "PASSED" if passed == total else "FAILED"

    lines.append("# GA-73 Pipeline Validation Report")
    lines.append("")
    lines.append(f"- **Date:** {datetime.now().strftime('%Y-%m-%d %H:%M')}")
    lines.append(f"- **Embedding model:** `{model_name}`")
    lines.append(f"- **Chunk size / overlap:** {CHUNK_SIZE} / {CHUNK_OVERLAP} characters")
    lines.append(f"- **Result:** {status} ({passed} of {total} checks passed)")
    lines.append("")

    # One section per document
    for run in runs:
        lines.extend(document_section(run, results))

    # Error handling
    lines.append("## Invalid inputs and processing errors")
    lines.append("")
    lines.append("| Case | Result | Detail |")
    lines.append("| --- | --- | --- |")
    for result in error_results:
        lines.append(f"| {result.name} | {pass_label(result)} | {result.detail} |")
    lines.append("")

    # Save the report
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    with open(REPORT_PATH, "w", encoding="utf-8") as file:
        file.write("\n".join(lines))

    return REPORT_PATH


# Helper functions: checks

def check_extraction(run):

    # Every page must have a text layer; scanned pages would need OCR
    pages = run.extracted["pages"]
    empty_pages = []
    for page in pages:
        if not page["has_text"]:
            empty_pages.append(page["page_number"])

    passed = not run.processed.needs_ocr and not empty_pages
    detail = f"{len(pages)} pages, pages without text: {empty_pages or 'none'}"

    result = make_result(run, "Meaningful chunks", "Text extracted from every page", passed, detail)

    return result


def check_clean_text(run):

    # PDF bullet glyphs are removed during extraction and must not reach the chunks
    with_glyphs = []
    for chunk in run.chunks:
        if "" in chunk.text or "" in chunk.text:
            with_glyphs.append(chunk.chunk_id)

    passed = not with_glyphs
    detail = f"chunks containing PDF bullet glyphs: {len(with_glyphs)}"

    result = make_result(run, "Meaningful chunks", "No PDF bullet glyphs in chunk text", passed, detail)

    return result


def check_chunk_sizes(run):

    # Chunks must exist, contain text, and stay within the chunk size
    sizes = []
    for chunk in run.chunks:
        sizes.append(len(chunk.text))

    too_large = []
    for chunk in run.chunks:
        if len(chunk.text) > CHUNK_SIZE:
            too_large.append(chunk.chunk_id)

    passed = len(sizes) > 0 and min(sizes) > 0 and not too_large
    detail = f"{len(sizes)} chunks, size {min(sizes, default=0)}-{max(sizes, default=0)} characters"

    result = make_result(run, "Meaningful chunks", f"Chunk sizes within {CHUNK_SIZE} characters", passed, detail)

    return result


def check_page_coverage(run):

    # Every page with text must produce at least one chunk
    chunked_pages = set()
    for chunk in run.chunks:
        chunked_pages.add(chunk.metadata["page_number"])

    missing = []
    for page in run.extracted["pages"]:
        if page["has_text"] and page["page_number"] not in chunked_pages:
            missing.append(page["page_number"])

    passed = not missing
    detail = f"pages with chunks: {sorted(chunked_pages)}, missing: {missing or 'none'}"

    result = make_result(run, "Meaningful chunks", "Every page is chunked", passed, detail)

    return result


def check_no_text_lost(run):
    """Every non-whitespace character of every page must be inside some chunk."""

    lost = 0

    for page in run.extracted["pages"]:
        text = page["text"]
        covered = [False] * len(text)

        # Mark the characters each chunk of this page covers
        for chunk in run.chunks:
            if chunk.metadata["page_number"] != page["page_number"]:
                continue
            position = text.find(chunk.text, max(0, chunk.metadata["start_index"] - 5))
            if position < 0:
                lost += len(chunk.text)
                continue
            for index in range(position, position + len(chunk.text)):
                covered[index] = True

        # Count characters no chunk covers; whitespace between chunks does not matter
        for index, character in enumerate(text):
            if not covered[index] and not character.isspace():
                lost += 1

    passed = lost == 0
    detail = f"{lost} characters of page text are missing from the chunks"

    result = make_result(run, "Information preserved", "No text lost between chunks", passed, detail)

    return result


def check_overlap(run):

    # Neighbouring chunks on a page may overlap by at most CHUNK_OVERLAP characters
    overlaps = []
    too_large = []
    for previous, current in zip(run.chunks, run.chunks[1:]):
        if previous.metadata["page_number"] != current.metadata["page_number"]:
            continue
        overlap = previous.metadata["start_index"] + len(previous.text) - current.metadata["start_index"]
        if overlap > 0:
            overlaps.append(overlap)
        if overlap > CHUNK_OVERLAP:
            too_large.append(current.chunk_id)

    passed = not too_large
    detail = f"overlapping pairs: {len(overlaps)}, overlap sizes: {overlaps or 'none'} (clean section breaks need none)"

    result = make_result(run, "Information preserved", f"Overlap within {CHUNK_OVERLAP} characters", passed, detail)

    return result


def check_key_phrase(run, phrase):

    # The phrase must appear whole in a chunk from its expected page
    wanted = normalize_spaces(phrase["text"])
    found_on = []
    for chunk in run.chunks:
        if wanted in normalize_spaces(chunk.text):
            found_on.append(chunk.metadata["page_number"])

    passed = phrase["page"] in found_on
    detail = f"expected page {phrase['page']}, found on pages: {sorted(set(found_on)) or 'not found'}"

    result = make_result(run, "Information preserved", f'Key phrase "{phrase["text"]}"', passed, detail)

    return result


def check_chunk_ids(run):

    # IDs are unique and numbered <document_id>:0, :1, ... in order
    document_id = run.processed.document_id
    wrong = []
    for index, chunk in enumerate(run.chunks):
        if chunk.chunk_id != f"{document_id}:{index}" or chunk.metadata["chunk_index"] != index:
            wrong.append(chunk.chunk_id)

    unique_ids = set()
    for chunk in run.chunks:
        unique_ids.add(chunk.chunk_id)

    passed = not wrong and len(unique_ids) == len(run.chunks)
    detail = f"{len(unique_ids)} unique IDs, from {document_id[:12]}...:0 to :{len(run.chunks) - 1}"

    result = make_result(run, "Identifiers and metadata", "Chunk IDs unique and in order", passed, detail)

    return result


def check_metadata(run):

    # Every chunk carries its source details, matching documents.json
    entry = run.entry
    expected = {
        "document_id": run.processed.document_id,
        "source_file": entry["file"],
        "department_id": entry["department_id"],
        "document_type": entry["document_type"],
        "total_pages": run.processed.total_pages,
    }
    if entry.get("service_id"):
        expected["service_id"] = entry["service_id"]
    if entry.get("title"):
        expected["document_title"] = entry["title"]

    problems = []
    for chunk in run.chunks:

        # Required fields are present
        for field in REQUIRED_METADATA:
            if field not in chunk.metadata:
                problems.append(f"{chunk.chunk_id} missing {field}")

        # Source values are correct
        for field, value in expected.items():
            if chunk.metadata.get(field) != value:
                problems.append(f"{chunk.chunk_id} {field}={chunk.metadata.get(field)!r}")

        # The page number is a real page
        page_number = chunk.metadata.get("page_number")
        if not isinstance(page_number, int) or not 1 <= page_number <= run.processed.total_pages:
            problems.append(f"{chunk.chunk_id} page_number={page_number!r}")

    passed = not problems
    detail = "all chunks carry document, title, source file, department, service, type, and page" if passed else "; ".join(problems[:5])

    result = make_result(run, "Identifiers and metadata", "Source metadata on every chunk", passed, detail)

    return result


def check_embeddings_match_chunks(run):

    # Every chunk has exactly one embedding, with the same ID, text, and metadata
    if run.embedding_file is None:
        return make_result(run, "Embeddings generated", "One embedding per chunk", False, "no embeddings file was saved")

    records = run.embedding_file.records
    mismatched = []
    for chunk, record in zip(run.chunks, records):
        if chunk.chunk_id != record.chunk_id or chunk.text != record.text or chunk.metadata != record.metadata:
            mismatched.append(chunk.chunk_id)

    failed = run.processed.failed_chunk_ids
    passed = len(records) == len(run.chunks) and not mismatched and not failed
    detail = f"{len(records)} embeddings for {len(run.chunks)} chunks, failed: {len(failed)}, saved to {relative(run.processed.embeddings_path)}"

    result = make_result(run, "Embeddings generated", "One embedding per chunk", passed, detail)

    return result


def check_dimensions(run):

    # Every vector has the dimension recorded in the file
    if run.embedding_file is None:
        return make_result(run, "Valid vectors", "Consistent dimensions", False, "no embeddings file was saved")

    dimensions = set()
    for record in run.embedding_file.records:
        dimensions.add(len(record.embedding))

    passed = dimensions == {run.embedding_file.dimension}
    detail = f"file dimension {run.embedding_file.dimension}, vector dimensions found: {sorted(dimensions)}"

    result = make_result(run, "Valid vectors", "Consistent dimensions", passed, detail)

    return result


def check_numbers(run):

    # Every value is a finite number and no vector is all zeros
    if run.embedding_file is None:
        return make_result(run, "Valid vectors", "Finite, non-zero values", False, "no embeddings file was saved")

    embeddings = []
    for record in run.embedding_file.records:
        embeddings.append(record.embedding)
    vectors = np.array(embeddings, dtype=float)
    finite = bool(np.all(np.isfinite(vectors)))
    norms = np.linalg.norm(vectors, axis=1)

    passed = finite and bool(np.all(norms > 0))
    detail = f"all finite: {finite}, vector length (norm) {norms.min():.4f}-{norms.max():.4f}, values {vectors.min():.4f} to {vectors.max():.4f}"

    result = make_result(run, "Valid vectors", "Finite, non-zero values", passed, detail)

    return result


def check_consistency(run, embedder):
    """Embedding the same chunks again must give almost the same vectors."""

    if run.embedding_file is None:
        return make_result(run, "Valid vectors", "Consistent results", False, "no embeddings file was saved")

    # Re-embed the first, middle, and last chunk
    records = run.embedding_file.records
    picked = []
    for index in sorted({0, len(records) // 2, len(records) - 1}):
        picked.append(records[index])

    # Send exactly what the pipeline sent: the document title and the chunk text
    texts = []
    for record in picked:
        texts.append(embedding_text(record))
    fresh_vectors = embedder.embedding_model.embed_documents(texts)

    # Compare each fresh vector with the saved one
    similarities = []
    for record, fresh in zip(picked, fresh_vectors):
        similarities.append(cosine_similarity(record.embedding, fresh))

    passed = min(similarities) >= CONSISTENCY_THRESHOLD
    rounded = []
    for similarity in similarities:
        rounded.append(round(similarity, 4))
    detail = f"cosine similarity of {len(picked)} re-embedded chunks: {rounded} (minimum {CONSISTENCY_THRESHOLD})"

    result = make_result(run, "Valid vectors", "Consistent results", passed, detail)

    return result


def check_vector_store_ready(run):

    # The ChromaDB input has four lists of equal length with storable metadata
    if run.embedding_file is None:
        return make_result(run, "Valid vectors", "Ready for the vector database", False, "no embeddings file was saved")

    batch = to_vector_store_batch(run.embedding_file)
    lengths = {len(batch.ids), len(batch.embeddings), len(batch.documents), len(batch.metadatas)}

    bad_values = 0
    for metadata in batch.metadatas:
        for value in metadata.values():
            if not isinstance(value, METADATA_VALUE_TYPES):
                bad_values += 1

    passed = len(lengths) == 1 and bad_values == 0
    detail = f"{len(batch.ids)} records with ids, embeddings, documents, and metadatas; unsupported metadata values: {bad_values}"

    result = make_result(run, "Valid vectors", "Ready for the vector database", passed, detail)

    return result


def check_retrieval(run, embedder, question):

    # Find the saved chunk closest to the question
    if run.embedding_file is None:
        return make_result(run, "Pipeline validated", f'Question "{question["question"]}"', False, "no embeddings file was saved")

    query_vector = embedder.embedding_model.embed_query(question["question"])
    best_record = None
    best_score = -1.0
    for record in run.embedding_file.records:
        score = cosine_similarity(query_vector, record.embedding)
        if score > best_score:
            best_record = record
            best_score = score

    # The closest chunk must come from the page that answers the question
    page = best_record.metadata["page_number"]
    passed = page == question["expected_page"]
    preview = normalize_spaces(best_record.text)[:70]
    detail = f"expected page {question['expected_page']}, best match page {page} (score {best_score:.3f}): \"{preview}...\""

    result = make_result(run, "Pipeline validated", f'Question "{question["question"]}"', passed, detail)

    return result


# Helper functions: report

def document_stats(run):

    # Chunk sizes
    sizes = []
    for chunk in run.chunks:
        sizes.append(len(chunk.text))

    stats = {
        "pages": run.processed.total_pages,
        "chunks": len(run.chunks),
        "smallest chunk": min(sizes, default=0),
        "average chunk": round(sum(sizes) / len(sizes)) if sizes else 0,
        "largest chunk": max(sizes, default=0),
        "embedded now": run.processed.embedded_chunk_count,
        "reused from earlier run": run.processed.reused_chunk_count,
        "failed chunks": len(run.processed.failed_chunk_ids),
    }

    # Embedding details
    if run.embedding_file is not None:
        stats["vector dimension"] = run.embedding_file.dimension
        stats["embeddings file"] = relative(run.processed.embeddings_path)
    stats["chunks file"] = relative(run.processed.chunks_path)

    return stats


def document_section(run, results):

    lines = [f"## {run.entry.get('title', run.entry['file'])}", ""]
    lines.append(f"- **File:** `{run.entry['file']}`")
    lines.append(
        f"- **Source:** department `{run.entry['department_id']}`, service `{run.entry.get('service_id')}`, "
        f"type `{run.entry['document_type']}`"
    )
    lines.append("")

    # Statistics
    if run.stats:
        lines.append("| Measure | Value |")
        lines.append("| --- | --- |")
        for measure, value in run.stats.items():
            lines.append(f"| {measure} | {value} |")
        lines.append("")

    # Checks, grouped by acceptance criterion
    lines.append("| Criterion | Check | Result | Detail |")
    lines.append("| --- | --- | --- | --- |")
    for result in results:
        if result.document == run.entry["file"]:
            lines.append(f"| {result.criterion} | {result.name} | {pass_label(result)} | {result.detail} |")
    lines.append("")

    return lines


def make_result(run, criterion, name, passed, detail):

    result = CheckResult(
        criterion=criterion,
        document=run.entry["file"],
        name=name,
        passed=bool(passed),
        detail=detail.replace("|", "/"),
    )

    return result


def error_result(name, passed, detail):

    result = CheckResult(
        criterion="Errors handled",
        document="error cases",
        name=name,
        passed=bool(passed),
        detail=detail.replace("|", "/"),
    )

    return result


def pass_label(result):

    label = "PASS" if result.passed else "**FAIL**"

    return label


def cosine_similarity(first, second):

    # Convert the vectors to numpy arrays
    first = np.array(first, dtype=float)
    second = np.array(second, dtype=float)

    # Dot product divided by the product of the lengths
    similarity = float(np.dot(first, second) / (np.linalg.norm(first) * np.linalg.norm(second)))

    return similarity


def normalize_spaces(text):

    # Line breaks and repeated spaces from the PDF layout do not change the meaning
    normalized = " ".join(text.split())

    return normalized


def relative(path):

    # Show paths relative to the rag folder
    if path is None:
        return "none"

    relative_path = Path(path).resolve().relative_to(RAG_ROOT).as_posix()

    return relative_path
