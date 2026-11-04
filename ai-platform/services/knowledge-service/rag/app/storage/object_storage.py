"""Store and read original government PDFs in SeaweedFS through its S3 API.

Design:

- Adapter: DocumentStore wraps the boto3 S3 client. The rest of the RAG service
  calls its methods and never uses boto3 directly, so the object storage can be
  replaced by changing only this module.
- Factory: create_document_store() builds the client from the settings.
- Dependency injection: scripts create one DocumentStore and pass it on, and
  tests can build one for a separate test bucket.

Storage keys come from the file content (its SHA-256 document_id), e.g.
documents/IRD/TIN_REGISTRATION/<document_id>.pdf. The same PDF always gets the
same key, so uploading it twice never creates a duplicate, and a revised PDF
gets a new key instead of overwriting the old version.
"""

import logging
import re
from pathlib import Path
from urllib.parse import quote, unquote

import boto3
from botocore.config import Config
from botocore.exceptions import ClientError, HTTPClientError
from botocore.exceptions import ConnectionError as BotoConnectionError
from pydantic import BaseModel

from app.processing.pdf_extractor import MAX_FILE_SIZE_BYTES, compute_document_id, validate_pdf_file
from app.storage.exceptions import (
    BucketNotFoundError,
    DocumentNotFoundError,
    InvalidObjectKeyError,
    StorageAuthError,
    StorageConnectionError,
    StorageError,
)

logger = logging.getLogger(__name__)

DOCUMENTS_PREFIX = "documents"

# Department guidelines have no service, so they share one folder per department
DEPARTMENT_FOLDER = "_department"

PDF_CONTENT_TYPE = "application/pdf"
DOWNLOAD_BLOCK_SIZE = 1024 * 1024

# IDs become folder names in the key; "/" or ".." would change the key layout
SAFE_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]+$")

# Bounded waits and retries, so a stopped container fails fast instead of hanging
CONNECT_TIMEOUT_SECONDS = 5
READ_TIMEOUT_SECONDS = 60
MAX_ATTEMPTS = 3

# S3 error codes that mean the credentials were rejected
AUTH_ERROR_CODES = {"AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch"}

# S3 error codes that mean the bucket or the object does not exist
NOT_FOUND_ERROR_CODES = {"404", "NoSuchBucket", "NoSuchKey", "NotFound"}


# Models

class StoredDocument(BaseModel):
    """The result of uploading one PDF."""

    object_key: str
    document_id: str
    source_file: str
    uploaded: bool


# Adapter

class DocumentStore:
    """Upload, download, and list original PDF documents in one bucket."""

    def __init__(self, client, bucket):

        self.client = client
        self.bucket = bucket

    def check_bucket(self):
        """Fail early, with a clear error, when storage is down, the keys are wrong, or the bucket is missing."""

        try:
            # One HEAD request checks the connection, the credentials, and the bucket
            self.client.head_bucket(Bucket=self.bucket)

        except ClientError as error:
            if is_not_found(error):
                raise BucketNotFoundError(f"Bucket not found: {self.bucket}")
            raise to_storage_error(error)

        except (BotoConnectionError, HTTPClientError):
            raise StorageConnectionError("Object storage is not reachable")

    def document_exists(self, object_key):

        try:
            # Read only the object's headers, not its content
            self.client.head_object(Bucket=self.bucket, Key=object_key)

        except ClientError as error:
            if is_not_found(error):
                return False
            raise to_storage_error(error)

        except (BotoConnectionError, HTTPClientError):
            raise StorageConnectionError("Object storage is not reachable")

        return True

    def upload_document(self, pdf_path, source):
        """Upload one PDF, or skip it when the same PDF is already stored.

        Raises a PDFExtractionError subclass when the file is not a valid PDF.
        """

        # Only real PDFs within the size limit are stored as official documents
        pdf_path = Path(pdf_path)
        validate_pdf_file(pdf_path, MAX_FILE_SIZE_BYTES)

        # Build the key from the file content
        document_id = compute_document_id(pdf_path)
        object_key = object_key_for(document_id, source)

        stored = StoredDocument(
            object_key=object_key,
            document_id=document_id,
            source_file=pdf_path.name,
            uploaded=False,
        )

        # The same content is already stored; uploading it again would change nothing
        if self.document_exists(object_key):
            logger.info("document already stored", extra={"object_key": object_key})
            return stored

        try:
            # Upload the PDF with its ownership IDs as object metadata
            with open(pdf_path, "rb") as file:
                self.client.put_object(
                    Bucket=self.bucket,
                    Key=object_key,
                    Body=file,
                    ContentType=PDF_CONTENT_TYPE,
                    Metadata=object_metadata(document_id, source, pdf_path.name),
                )

        except ClientError as error:
            raise to_storage_error(error)

        except (BotoConnectionError, HTTPClientError):
            raise StorageConnectionError("Object storage is not reachable")

        logger.info("document uploaded", extra={"object_key": object_key, "source_file": pdf_path.name})
        stored.uploaded = True

        return stored

    def download_document(self, object_key, target_dir):
        """Download a stored PDF into its own folder inside target_dir and return its path.

        Each document gets a folder named by its document_id, so two versions
        with the same file name never overwrite each other.
        """

        # The key holds the content hash, used below to check the download
        document_id = document_id_from_key(object_key)

        try:
            # Request the object
            response = self.client.get_object(Bucket=self.bucket, Key=object_key)

        except ClientError as error:
            if is_not_found(error):
                raise DocumentNotFoundError(f"Document not found: {object_key}")
            raise to_storage_error(error)

        except (BotoConnectionError, HTTPClientError):
            raise StorageConnectionError("Object storage is not reachable")

        # Create the document's folder and use the original file name
        document_dir = Path(target_dir) / document_id
        document_dir.mkdir(parents=True, exist_ok=True)
        pdf_path = document_dir / original_file_name(response, document_id)

        # Write to a temporary file first so a broken download never looks complete
        temporary_path = pdf_path.with_suffix(".tmp")

        try:
            # Copy the content to disk in blocks to keep memory use low
            with open(temporary_path, "wb") as file:
                for block in response["Body"].iter_chunks(chunk_size=DOWNLOAD_BLOCK_SIZE):
                    file.write(block)

        except (BotoConnectionError, HTTPClientError):
            temporary_path.unlink(missing_ok=True)
            raise StorageConnectionError("Object storage connection lost during download")

        # Check that the downloaded content is the document the key names
        if compute_document_id(temporary_path) != document_id:
            temporary_path.unlink()
            raise StorageError(f"Downloaded document does not match its key: {object_key}")

        # Give the file its final name
        temporary_path.replace(pdf_path)

        return pdf_path

    def list_documents(self, source):
        """List the keys of the PDFs stored for a department and service, sorted by key."""

        prefix = prefix_for(source)
        object_keys = []

        try:
            # A listing returns at most 1000 keys, so read every page
            paginator = self.client.get_paginator("list_objects_v2")
            for page in paginator.paginate(Bucket=self.bucket, Prefix=prefix):
                for item in page.get("Contents", []):
                    if item["Key"].endswith(".pdf"):
                        object_keys.append(item["Key"])

        except ClientError as error:
            if is_not_found(error):
                raise BucketNotFoundError(f"Bucket not found: {self.bucket}")
            raise to_storage_error(error)

        except (BotoConnectionError, HTTPClientError):
            raise StorageConnectionError("Object storage is not reachable")

        object_keys.sort()

        return object_keys


# Factory

def create_document_store(settings):
    """Create a DocumentStore for the bucket in the settings."""

    # Client options for SeaweedFS
    config = Config(
        connect_timeout=CONNECT_TIMEOUT_SECONDS,
        read_timeout=READ_TIMEOUT_SECONDS,
        retries={"max_attempts": MAX_ATTEMPTS, "mode": "standard"},
        # SeaweedFS serves buckets as http://host/bucket, not as http://bucket.host
        s3={"addressing_style": "path"},
        # Newer boto3 adds extra checksums by default that S3-compatible stores may
        # reject; the download is checked against its SHA-256 key instead
        request_checksum_calculation="when_required",
        response_checksum_validation="when_required",
    )

    # Build the S3 client with the keys from .env
    client = boto3.client(
        "s3",
        endpoint_url=settings.endpoint,
        region_name=settings.region,
        aws_access_key_id=settings.access_key.get_secret_value(),
        aws_secret_access_key=settings.secret_key.get_secret_value(),
        config=config,
    )

    store = DocumentStore(client=client, bucket=settings.bucket)

    return store


# Storage keys

def prefix_for(source):
    """The folder holding a department's or service's documents, e.g. documents/IRD/TIN_REGISTRATION/."""

    # Department guidelines have no service ID
    service_folder = source.service_id or DEPARTMENT_FOLDER

    # Both IDs become folder names, so they must be safe in a key
    check_safe_id(source.department_id, "department_id")
    check_safe_id(service_folder, "service_id")

    prefix = f"{DOCUMENTS_PREFIX}/{source.department_id}/{service_folder}/"

    return prefix


def object_key_for(document_id, source):

    # Place the document in its department or service folder
    check_safe_id(document_id, "document_id")
    object_key = f"{prefix_for(source)}{document_id}.pdf"

    return object_key


def document_id_from_key(object_key):

    # The file name in the key is <document_id>.pdf
    file_name = object_key.rsplit("/", 1)[-1]
    if not file_name.endswith(".pdf"):
        raise InvalidObjectKeyError(f"Not a document key: {object_key}")

    document_id = file_name.removesuffix(".pdf")
    check_safe_id(document_id, "document_id")

    return document_id


# Helper functions

def check_safe_id(value, name):

    # Allow only letters, digits, "_" and "-"
    if not SAFE_ID_PATTERN.match(value):
        raise InvalidObjectKeyError(f"{name} may contain only letters, digits, '_' and '-'")


def object_metadata(document_id, source, source_file):
    """Metadata stored with the PDF. S3 metadata must be ASCII, so the file name is URL-encoded."""

    metadata = {
        "document-id": document_id,
        "department-id": source.department_id,
        "document-type": source.document_type.value,
        "source-file": quote(source_file),
    }

    # Department guidelines have no service ID
    if source.service_id is not None:
        metadata["service-id"] = source.service_id

    return metadata


def original_file_name(response, document_id):

    # Decode the stored file name; fall back to the key's name when it is missing
    encoded_name = response.get("Metadata", {}).get("source-file", "")
    file_name = unquote(encoded_name)

    # Keep only the last part of the name, so metadata can never write outside the folder
    file_name = Path(file_name).name

    if not file_name.lower().endswith(".pdf"):
        file_name = f"{document_id}.pdf"

    return file_name


def error_code(error):

    # S3 errors carry a code such as "NoSuchBucket", or only the HTTP status for HEAD requests
    code = error.response.get("Error", {}).get("Code", "")

    return code


def is_not_found(error):

    # 404 from a HEAD request, or a NoSuch* code from other requests
    not_found = error_code(error) in NOT_FOUND_ERROR_CODES

    return not_found


def to_storage_error(error):
    """Turn an S3 client error into a project error with a short message."""

    code = error_code(error)
    status_code = error.response.get("ResponseMetadata", {}).get("HTTPStatusCode")

    # 403 or a credential code means SeaweedFS rejected the keys
    if code in AUTH_ERROR_CODES or status_code == 403:
        storage_error = StorageAuthError("Object storage rejected the access key or secret key")
        return storage_error

    storage_error = StorageError(f"Object storage request failed: {code or status_code}")

    return storage_error
