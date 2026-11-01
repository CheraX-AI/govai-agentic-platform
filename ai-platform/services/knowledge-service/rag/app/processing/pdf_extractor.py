"""Extract page text and ownership metadata from PDF documents."""

import hashlib
import logging
import re
import unicodedata
from enum import Enum
from pathlib import Path

from langchain_core.documents import Document
from pydantic import BaseModel, Field, computed_field, model_validator
from pypdf import PasswordType, PdfReader

logger = logging.getLogger(__name__)

PDF_SIGNATURE = b"%PDF-"
MAX_FILE_SIZE_BYTES = 50 * 1024 * 1024
MAX_PAGES = 1000

# Zero-width joiners (U+200C, U+200D) are kept: Sinhala and Tamil text needs them.
HORIZONTAL_WHITESPACE = re.compile(r"[ \t\f\v ]+")
CONTROL_CHARACTERS = re.compile(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]")
EXCESS_BLANK_LINES = re.compile(r"\n{3,}")

# Bullet glyphs from the Symbol and Wingdings fonts (U+F0B7, U+F0A7). Only these are removed:
# the rest of the private-use range can hold real text in legacy Sinhala and Tamil fonts.
PDF_BULLET_GLYPHS = re.compile(r"[]")


# Errors

class PDFExtractionError(Exception):
    """Base error for a PDF that cannot be extracted."""


class InvalidPDFError(PDFExtractionError):
    """The file is missing, empty, not a PDF, or has no pages."""


class PDFTooLargeError(PDFExtractionError):
    """The file exceeds the size or page limit."""


class EncryptedPDFError(PDFExtractionError):
    """The PDF is password protected."""


class CorruptedPDFError(PDFExtractionError):
    """The PDF structure cannot be read."""


# Models

class DocumentType(str, Enum):
    DEPARTMENT_GUIDELINE = "department_guideline"
    SERVICE_GUIDELINE = "service_guideline"


class DocumentSource(BaseModel):
    """Who owns the document. Supplied by the caller, never read from the PDF."""

    department_id: str = Field(min_length=1)
    service_id: str | None = Field(default=None, min_length=1)
    document_type: DocumentType
    title: str | None = Field(default=None, min_length=1)

    @model_validator(mode="after")
    def check_service_matches_type(self):

        # Service guidelines must belong to a service
        if self.document_type == DocumentType.SERVICE_GUIDELINE and self.service_id is None:
            raise ValueError("service_id is required for a service guideline")

        # Department guidelines must not belong to a service
        if self.document_type == DocumentType.DEPARTMENT_GUIDELINE and self.service_id is not None:
            raise ValueError("service_id must be empty for a department guideline")

        return self


class ExtractedPage(BaseModel):
    page_number: int = Field(ge=1)
    text: str
    char_count: int = Field(ge=0)
    has_text: bool
    extraction_failed: bool = False


class ExtractedDocument(BaseModel):
    document_id: str
    source_file: str
    source: DocumentSource
    total_pages: int = Field(ge=1)
    pages: list[ExtractedPage]

    @computed_field
    @property
    def needs_ocr(self) -> bool:
        return not any(page.has_text for page in self.pages)


# Public functions

def extract_pdf(pdf_path, source, max_file_size_bytes=MAX_FILE_SIZE_BYTES, max_pages=MAX_PAGES):

    # Check the file before parsing it
    pdf_path = Path(pdf_path)
    validate_pdf_file(pdf_path, max_file_size_bytes)

    # Identify the document by its content
    document_id = compute_document_id(pdf_path)

    # Open the PDF and count its pages
    reader = open_pdf(pdf_path)
    total_pages = count_pages(reader, pdf_path, max_pages)

    # Extract every page separately
    pages = []
    for index in range(total_pages):
        pages.append(extract_page(reader, index, pdf_path))

    # Build the extracted document
    document = ExtractedDocument(
        document_id=document_id,
        source_file=pdf_path.name,
        source=source,
        total_pages=total_pages,
        pages=pages,
    )

    # Log a summary without document content
    logger.info(
        "pdf extracted",
        extra={
            "document_id": document_id,
            "source_file": pdf_path.name,
            "total_pages": total_pages,
            "empty_pages": sum(not page.has_text for page in pages),
            "failed_pages": sum(page.extraction_failed for page in pages),
        },
    )

    # Warn when the PDF is scanned and has no text layer
    if document.needs_ocr:
        logger.warning(
            "pdf has no extractable text; OCR is required",
            extra={"document_id": document_id, "source_file": pdf_path.name},
        )

    return document


def to_langchain_documents(document):

    # Metadata shared by every page
    metadata = {
        "document_id": document.document_id,
        "source_file": document.source_file,
        "department_id": document.source.department_id,
        "document_type": document.source.document_type.value,
        "total_pages": document.total_pages,
    }

    # ChromaDB metadata cannot hold None, so optional fields are added only when present
    if document.source.service_id is not None:
        metadata["service_id"] = document.source.service_id
    if document.source.title is not None:
        metadata["document_title"] = document.source.title

    # Create one LangChain Document per page that has text
    langchain_documents = []
    for page in document.pages:
        if page.has_text:
            langchain_documents.append(
                Document(
                    page_content=page.text,
                    metadata={**metadata, "page_number": page.page_number},
                )
            )

    return langchain_documents


def save_extracted_document(document, output_dir):

    # Create output directory
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    # Create JSON file path
    output_path = output_dir / f"{document.document_id}.json"

    # Save extracted document as JSON
    output_path.write_text(
        document.model_dump_json(indent=2),
        encoding="utf-8"
    )

    return output_path


def normalize_page_text(text):

    # Use one consistent Unicode form
    text = unicodedata.normalize("NFC", text)

    # Use one consistent line ending
    text = text.replace("\r\n", "\n").replace("\r", "\n")

    # Collapse repeated spaces and tabs
    text = HORIZONTAL_WHITESPACE.sub(" ", text)

    # Remove invisible control characters
    text = CONTROL_CHARACTERS.sub("", text)

    # Remove PDF bullet glyphs; each list item already starts on its own line
    text = PDF_BULLET_GLYPHS.sub("", text)

    # Trim spaces at the start and end of each line
    lines = [line.strip() for line in text.split("\n")]
    text = "\n".join(lines)

    # Keep at most one blank line between paragraphs
    text = EXCESS_BLANK_LINES.sub("\n\n", text)

    return text.strip()


# Helper functions

def validate_pdf_file(pdf_path, max_file_size_bytes):

    # Check that the file exists
    if not pdf_path.is_file():
        raise InvalidPDFError(f"PDF file not found: {pdf_path.name}")

    # Check the file size
    file_size = pdf_path.stat().st_size
    if file_size == 0:
        raise InvalidPDFError(f"PDF file is empty: {pdf_path.name}")
    if file_size > max_file_size_bytes:
        raise PDFTooLargeError(f"PDF file exceeds {max_file_size_bytes} bytes: {pdf_path.name}")

    # Check that the file starts like a PDF, not just that it is named .pdf
    with pdf_path.open("rb") as pdf_file:
        header = pdf_file.read(1024)
    if PDF_SIGNATURE not in header:
        raise InvalidPDFError(f"File is not a PDF: {pdf_path.name}")


def compute_document_id(pdf_path):

    # Hash the file in 1 MB blocks to keep memory use low
    digest = hashlib.sha256()
    with pdf_path.open("rb") as pdf_file:
        while block := pdf_file.read(1024 * 1024):
            digest.update(block)

    return digest.hexdigest()


def open_pdf(pdf_path):

    # pypdf raises many exception types for malformed files,
    # so any parse failure is reported as a corrupted PDF
    try:
        reader = PdfReader(pdf_path)
    except Exception as error:
        raise CorruptedPDFError(f"PDF cannot be read: {pdf_path.name}") from error

    # Unencrypted PDFs can be used directly
    if not reader.is_encrypted:
        return reader

    # Many PDFs are encrypted only to restrict editing and open with an empty password
    try:
        result = reader.decrypt("")
    except Exception as error:
        raise EncryptedPDFError(f"PDF cannot be decrypted: {pdf_path.name}") from error

    if result == PasswordType.NOT_DECRYPTED:
        raise EncryptedPDFError(f"PDF is password protected: {pdf_path.name}")

    return reader


def count_pages(reader, pdf_path, max_pages):

    # Read the page count
    try:
        total_pages = len(reader.pages)
    except Exception as error:
        raise CorruptedPDFError(f"PDF pages cannot be read: {pdf_path.name}") from error

    # Check the page count is within limits
    if total_pages == 0:
        raise InvalidPDFError(f"PDF has no pages: {pdf_path.name}")
    if total_pages > max_pages:
        raise PDFTooLargeError(f"PDF exceeds {max_pages} pages: {pdf_path.name}")

    return total_pages


def extract_page(reader, index, pdf_path):

    # PDF page numbers start at 1 for people
    page_number = index + 1

    # Extract the page text; one bad page should not stop the whole document
    try:
        raw_text = reader.pages[index].extract_text() or ""
    except Exception as error:

        # Log only the error type: error messages may include document content
        logger.warning(
            "pdf page extraction failed",
            extra={
                "source_file": pdf_path.name,
                "page_number": page_number,
                "error_type": type(error).__name__,
            },
        )

        return ExtractedPage(
            page_number=page_number,
            text="",
            char_count=0,
            has_text=False,
            extraction_failed=True,
        )

    # Clean the text and record whether the page has any
    text = normalize_page_text(raw_text)

    return ExtractedPage(
        page_number=page_number,
        text=text,
        char_count=len(text),
        has_text=bool(text),
    )
