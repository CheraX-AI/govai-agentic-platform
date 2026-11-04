"""Integration tests: real PDF files through extraction, chunking, and embedding, in memory."""

import io

from pypdf import PdfWriter

from app.processing.pdf_extractor import DocumentSource, DocumentType
from app.processing.pipeline import process_document, process_documents
from tests.helpers import TOO_LONG_TEXT, make_embedder, make_fake_embedder, make_invalid_key_embedder


SERVICE_SOURCE = DocumentSource(
    department_id="IRD",
    service_id="TIN_REGISTRATION",
    document_type=DocumentType.SERVICE_GUIDELINE
)

DEPARTMENT_SOURCE = DocumentSource(
    department_id="IRD",
    document_type=DocumentType.DEPARTMENT_GUIDELINE
)

PAGE_ONE_LINES = [
    "TIN registration is required for every taxpayer.",
    "Applicants must submit the registration form.",
    "A copy of the national identity card is required.",
] * 10

PAGE_TWO_LINES = [
    "Registration is completed within five working days.",
]


# Helper functions

def write_text_pdf(pdf_path, pages):

    # Build PDF objects: catalog, page tree, then a page and content stream per page
    font_number = 3 + len(pages) * 2
    page_refs = " ".join(f"{3 + index * 2} 0 R" for index in range(len(pages)))

    objects = [
        "<< /Type /Catalog /Pages 2 0 R >>",
        f"<< /Type /Pages /Kids [{page_refs}] /Count {len(pages)} >>",
    ]

    for index, lines in enumerate(pages):

        # Draw each line 14 points below the previous one
        commands = ["BT", "/F1 12 Tf", "14 TL", "72 740 Td"]
        for line in lines:
            commands.append(f"({line}) Tj T*")
        commands.append("ET")
        stream = "\n".join(commands)

        objects.append(
            f"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "
            f"/Contents {4 + index * 2} 0 R "
            f"/Resources << /Font << /F1 {font_number} 0 R >> >> >>"
        )
        objects.append(f"<< /Length {len(stream)} >>\nstream\n{stream}\nendstream")

    objects.append("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

    # Write the objects and record where each one starts
    output = io.BytesIO()
    output.write(b"%PDF-1.4\n")
    offsets = []
    for number, body in enumerate(objects, start=1):
        offsets.append(output.tell())
        output.write(f"{number} 0 obj\n{body}\nendobj\n".encode())

    # Write the cross-reference table so readers can find the objects
    xref_offset = output.tell()
    output.write(f"xref\n0 {len(objects) + 1}\n0000000000 65535 f \n".encode())
    for offset in offsets:
        output.write(f"{offset:010d} 00000 n \n".encode())
    output.write(
        f"trailer\n<< /Size {len(objects) + 1} /Root 1 0 R >>\n"
        f"startxref\n{xref_offset}\n%%EOF".encode()
    )

    pdf_path.write_bytes(output.getvalue())
    return pdf_path


def write_blank_pdf(pdf_path, password=None):

    # A blank page behaves like a scanned page: it has no text layer
    writer = PdfWriter()
    writer.add_blank_page(width=612, height=792)

    if password:
        writer.encrypt(password)

    with open(pdf_path, "wb") as file:
        writer.write(file)

    return pdf_path


# Tests

def test_service_guideline_is_extracted_chunked_and_embedded(tmp_path):

    # Process a two-page service guideline with a fake embedder (no NVIDIA call)
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES, PAGE_TWO_LINES])

    result = process_document(pdf_path, SERVICE_SOURCE, chunk_size=300, chunk_overlap=50, embedder=make_fake_embedder())

    assert result.source_file == "tin_guide.pdf"
    assert result.total_pages == 2
    assert result.needs_ocr is False
    assert result.chunk_count > 2
    assert result.embedded_chunk_count == result.chunk_count

    # Every chunk keeps the metadata, points to the right page, and is in chunk order
    records = result.embeddings.records

    assert len(records) == result.chunk_count
    for index, record in enumerate(records):
        assert record.chunk_id == f"{result.document_id}:{index}"
        assert record.metadata["department_id"] == "IRD"
        assert record.metadata["service_id"] == "TIN_REGISTRATION"
        assert record.metadata["document_type"] == "service_guideline"
        assert len(record.text) <= 300

    assert records[0].metadata["page_number"] == 1
    assert records[-1].metadata["page_number"] == 2
    assert "TIN registration is required" in records[0].text
    assert "five working days" in records[-1].text


def test_department_guideline_has_no_service_id(tmp_path):

    # Department guidelines are not tied to a service
    pdf_path = write_text_pdf(tmp_path / "ird_policy.pdf", [PAGE_TWO_LINES])

    result = process_document(pdf_path, DEPARTMENT_SOURCE, embedder=make_fake_embedder())

    metadata = result.embeddings.records[0].metadata
    assert metadata["document_type"] == "department_guideline"
    assert "service_id" not in metadata


def test_nothing_is_written_to_disk(tmp_path):

    # The pipeline works in memory: the folder holds only the PDF afterwards
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES])

    process_document(pdf_path, SERVICE_SOURCE, embedder=make_fake_embedder())

    assert [path.name for path in tmp_path.iterdir()] == ["tin_guide.pdf"]


def test_scanned_pdf_is_flagged_and_not_chunked(tmp_path):

    # A PDF without text produces no chunks and nothing to embed
    pdf_path = write_blank_pdf(tmp_path / "scanned.pdf")

    result = process_document(pdf_path, DEPARTMENT_SOURCE, embedder=make_fake_embedder())

    assert result.needs_ocr is True
    assert result.chunk_count == 0
    assert result.embeddings is None


def test_reprocessing_gives_the_same_ids(tmp_path):

    # The same PDF always gets the same document and chunk IDs, so ChromaDB updates instead of duplicating
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES])

    first = process_document(pdf_path, SERVICE_SOURCE, embedder=make_fake_embedder())
    second = process_document(pdf_path, SERVICE_SOURCE, embedder=make_fake_embedder())

    first_ids = [record.chunk_id for record in first.embeddings.records]
    second_ids = [record.chunk_id for record in second.embeddings.records]
    assert first.document_id == second.document_id
    assert first_ids == second_ids


def test_dry_run_chunks_without_embedding(tmp_path):

    # Without an embedder the PDF is only extracted and chunked
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES])

    result = process_document(pdf_path, SERVICE_SOURCE)

    assert result.chunk_count > 0
    assert result.embeddings is None
    assert result.embedded_chunk_count == 0


def test_batch_continues_after_failed_documents(tmp_path):

    # One valid PDF, one file that is not a PDF, and one password-protected PDF
    valid_pdf = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES])

    not_a_pdf = tmp_path / "notes.pdf"
    not_a_pdf.write_text("This is not a PDF")

    locked_pdf = write_blank_pdf(tmp_path / "locked.pdf", password="secret")

    report = process_documents(
        [
            (not_a_pdf, SERVICE_SOURCE),
            (valid_pdf, SERVICE_SOURCE),
            (locked_pdf, DEPARTMENT_SOURCE),
        ]
    )

    # The valid PDF is processed even though it comes after a failure
    assert [document.source_file for document in report.processed] == ["tin_guide.pdf"]

    # Each failure is recorded with its error type
    failures = {failure.source_file: failure.error_type for failure in report.failed}

    assert failures == {
        "notes.pdf": "InvalidPDFError",
        "locked.pdf": "EncryptedPDFError",
    }


# Embedding (real NVIDIA API, settings from rag/.env)

def test_pipeline_embeds_every_chunk(tmp_path):

    # Process a PDF and embed its chunks with NVIDIA
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES, PAGE_TWO_LINES])

    result = process_document(
        pdf_path,
        SERVICE_SOURCE,
        chunk_size=300,
        chunk_overlap=50,
        embedder=make_embedder(batch_size=3),
    )

    assert result.embedded_chunk_count == result.chunk_count
    assert result.failed_chunk_ids == []

    # Every vector has the document's dimension
    embeddings = result.embeddings
    for record in embeddings.records:
        assert len(record.embedding) == embeddings.dimension


def test_scanned_pdf_is_not_sent_for_embedding(tmp_path):

    # A PDF without text has no chunks; with a key NVIDIA rejects, any call would fail
    pdf_path = write_blank_pdf(tmp_path / "scanned.pdf")

    report = process_documents([(pdf_path, DEPARTMENT_SOURCE)], embedder=make_invalid_key_embedder())

    assert report.failed == []
    assert report.processed[0].embeddings is None


def test_rejected_chunks_are_reported_per_document(tmp_path):

    # One page longer than NVIDIA accepts, kept as a single chunk
    pdf_path = write_text_pdf(tmp_path / "long_guide.pdf", [[TOO_LONG_TEXT]])

    report = process_documents(
        [(pdf_path, SERVICE_SOURCE)],
        chunk_size=100_000,
        chunk_overlap=0,
        embedder=make_embedder(),
    )

    # The PDF is still processed and chunked; only its embeddings are missing
    document = report.processed[0]

    assert report.failed == []
    assert document.chunk_count == 1
    assert document.embeddings is None
    assert len(document.failed_chunk_ids) == 1
    assert report.failed_chunk_count == 1


def test_rejected_key_stops_embedding_but_keeps_chunking(tmp_path):

    # NVIDIA rejects the key on the first PDF
    first_pdf = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES])
    second_pdf = write_text_pdf(tmp_path / "tin_faq.pdf", [PAGE_TWO_LINES])

    report = process_documents(
        [(first_pdf, SERVICE_SOURCE), (second_pdf, SERVICE_SOURCE)],
        embedder=make_invalid_key_embedder(),
    )

    # The second PDF is chunked without being sent to NVIDIA
    assert report.embedding_stopped is True
    assert [failure.error_type for failure in report.failed] == ["EmbeddingAuthError"]
    assert [document.source_file for document in report.processed] == ["tin_faq.pdf"]
    assert report.processed[0].chunk_count > 0
    assert report.processed[0].embeddings is None
