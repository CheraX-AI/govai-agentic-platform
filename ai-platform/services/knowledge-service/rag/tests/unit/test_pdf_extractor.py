from pathlib import Path

import pytest
from pypdf import PdfWriter

from app.processing.pdf_extractor import (
    extract_pdf,
    normalize_page_text,
    to_langchain_documents,
    DocumentSource,
    DocumentType,
    ExtractedDocument,
    ExtractedPage,
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

def test_pdf_bullet_glyphs_are_removed():

    # Symbol and Wingdings bullets become plain list lines
    text = normalize_page_text("For Individuals\n National Identity Card\n Utility Bill")

    assert text == "For Individuals\nNational Identity Card\nUtility Bill"


def test_title_is_added_to_page_metadata(tmp_path):

    # A document title from the caller is added to every page's metadata
    source = DocumentSource(
        department_id="IRD",
        service_id="TIN_REGISTRATION",
        document_type=DocumentType.SERVICE_GUIDELINE,
        title="Obtaining a Taxpayer Identification Number (TIN)",
    )
    document = ExtractedDocument(
        document_id="doc001",
        source_file="tin_guide.pdf",
        source=source,
        total_pages=1,
        pages=[ExtractedPage(page_number=1, text="TIN steps.", char_count=10, has_text=True)],
    )

    pages = to_langchain_documents(document)

    assert pages[0].metadata["document_title"] == "Obtaining a Taxpayer Identification Number (TIN)"


def test_no_title_means_no_title_metadata():

    # ChromaDB cannot store None, so a missing title is left out
    source = DocumentSource(
        department_id="IRD",
        service_id="TIN_REGISTRATION",
        document_type=DocumentType.SERVICE_GUIDELINE,
    )
    document = ExtractedDocument(
        document_id="doc001",
        source_file="tin_guide.pdf",
        source=source,
        total_pages=1,
        pages=[ExtractedPage(page_number=1, text="TIN steps.", char_count=10, has_text=True)],
    )

    pages = to_langchain_documents(document)

    assert "document_title" not in pages[0].metadata
