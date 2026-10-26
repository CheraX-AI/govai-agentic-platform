from pathlib import Path

import pytest
from pypdf import PdfWriter

from app.processing.pdf_extractor import (
    extract_pdf,
    DocumentSource,
    DocumentType,
    InvalidPDFError,
)


def test_valid_pdf(tmp_path):

    pdf_path = tmp_path / "test.pdf"

    writer = PdfWriter()
    writer.add_blank_page(width=600, height=800)

    with open(pdf_path, "wb") as file:
        writer.write(file)

    source = DocumentSource(
        department_id="IRD",
        service_id="TIN_REGISTRATION",
        document_type=DocumentType.SERVICE_GUIDELINE
    )

    document = extract_pdf(pdf_path, source)

    assert document.total_pages == 1
    assert document.source.department_id == "IRD"
    assert document.pages[0].has_text is False


def test_invalid_pdf(tmp_path):

    pdf_path = tmp_path / "invalid.pdf"
    pdf_path.write_text("This is not a PDF")

    source = DocumentSource(
        department_id="IRD",
        service_id="TIN_REGISTRATION",
        document_type=DocumentType.SERVICE_GUIDELINE
    )

    with pytest.raises(InvalidPDFError):
        extract_pdf(pdf_path, source)