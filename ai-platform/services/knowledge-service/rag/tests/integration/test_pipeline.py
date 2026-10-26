"""Integration tests: real PDF files through extraction, chunking, and saving."""

import io
import json

from pypdf import PdfWriter

from app.processing.pdf_extractor import DocumentSource, DocumentType
from app.processing.pipeline import process_document, process_documents


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


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


# Tests

def test_service_guideline_is_extracted_and_chunked(tmp_path):

    # Process a two-page service guideline
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES, PAGE_TWO_LINES])

    result = process_document(
        pdf_path,
        SERVICE_SOURCE,
        tmp_path / "output",
        chunk_size=300,
        chunk_overlap=50
    )

    assert result.source_file == "tin_guide.pdf"
    assert result.total_pages == 2
    assert result.needs_ocr is False
    assert result.chunk_count > 2

    # Each page is saved separately with the document's ownership metadata
    extracted = read_json(result.extracted_path)

    assert result.extracted_path.name == f"{result.document_id}.json"
    assert extracted["source"]["department_id"] == "IRD"
    assert extracted["source"]["service_id"] == "TIN_REGISTRATION"
    assert [page["page_number"] for page in extracted["pages"]] == [1, 2]
    assert "TIN registration is required" in extracted["pages"][0]["text"]
    assert "five working days" in extracted["pages"][1]["text"]

    # Every chunk keeps the metadata and points to the right page
    chunks = read_json(result.chunks_path)

    assert len(chunks) == result.chunk_count
    for index, chunk in enumerate(chunks):
        assert chunk["chunk_id"] == f"{result.document_id}:{index}"
        assert chunk["metadata"]["department_id"] == "IRD"
        assert chunk["metadata"]["service_id"] == "TIN_REGISTRATION"
        assert chunk["metadata"]["document_type"] == "service_guideline"
        assert len(chunk["text"]) <= 300

    assert chunks[0]["metadata"]["page_number"] == 1
    assert chunks[-1]["metadata"]["page_number"] == 2
    assert "five working days" in chunks[-1]["text"]


def test_department_guideline_has_no_service_id(tmp_path):

    # Department guidelines are not tied to a service
    pdf_path = write_text_pdf(tmp_path / "ird_policy.pdf", [PAGE_TWO_LINES])

    result = process_document(pdf_path, DEPARTMENT_SOURCE, tmp_path / "output")

    chunks = read_json(result.chunks_path)

    assert chunks[0]["metadata"]["document_type"] == "department_guideline"
    assert "service_id" not in chunks[0]["metadata"]


def test_scanned_pdf_is_flagged_and_not_chunked(tmp_path):

    # A PDF without text is saved for review but produces no chunks
    pdf_path = write_blank_pdf(tmp_path / "scanned.pdf")

    result = process_document(pdf_path, DEPARTMENT_SOURCE, tmp_path / "output")

    assert result.needs_ocr is True
    assert result.chunk_count == 0
    assert result.chunks_path is None
    assert result.extracted_path.exists()
    assert not (tmp_path / "output" / "chunks").exists()


def test_reprocessing_overwrites_instead_of_duplicating(tmp_path):

    # Processing the same PDF twice gives the same ID and the same files
    pdf_path = write_text_pdf(tmp_path / "tin_guide.pdf", [PAGE_ONE_LINES])
    output_dir = tmp_path / "output"

    first = process_document(pdf_path, SERVICE_SOURCE, output_dir)
    second = process_document(pdf_path, SERVICE_SOURCE, output_dir)

    assert first.document_id == second.document_id
    assert len(list((output_dir / "extracted").iterdir())) == 1
    assert len(list((output_dir / "chunks").iterdir())) == 1


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
        ],
        tmp_path / "output"
    )

    # The valid PDF is processed even though it comes after a failure
    assert [document.source_file for document in report.processed] == ["tin_guide.pdf"]

    # Each failure is recorded with its error type
    failures = {failure.source_file: failure.error_type for failure in report.failed}

    assert failures == {
        "notes.pdf": "InvalidPDFError",
        "locked.pdf": "EncryptedPDFError",
    }
