# Knowledge Service RAG

LangChain-based document processing for the GovAI knowledge service. It reads PDF
documents, extracts their text, splits it into chunks, and creates embeddings ready
for the vector database.

## Folder structure

```text
rag/
├── app/
│   ├── processing/
│   │   ├── pdf_extractor.py      # Extracts text from PDF pages
│   │   ├── chunker.py            # Splits extracted text into chunks
│   │   └── pipeline.py           # Runs extraction, chunking, and embedding in order
│   ├── embeddings/
│   │   ├── config.py             # NVIDIA settings read from .env
│   │   ├── exceptions.py         # Errors and NVIDIA API error classification
│   │   ├── models.py             # Pydantic models
│   │   ├── validators.py         # Chunk, metadata, and vector checks
│   │   ├── storage.py            # Chunk and embeddings files, vector-store output
│   │   └── embedding_service.py  # NVIDIA client, batching, retries, re-run reuse
│   └── storage/
│       ├── config.py             # SeaweedFS settings read from .env
│       ├── exceptions.py         # Storage errors
│       └── object_storage.py     # DocumentStore: upload, download, list PDFs (S3 API)
├── data/
│   ├── input/                    # Only for the GA-73 validation tests
│   └── output/                   # Generated chunks and embeddings
├── scripts/
│   ├── upload_documents.py       # Uploads PDF files to SeaweedFS
│   └── process_documents.py      # Processes the PDFs stored in SeaweedFS
├── tests/
│   ├── unit/                     # Unit tests (embedding tests call the real NVIDIA API)
│   ├── integration/              # PDF → chunks → embeddings, the scripts, and real SeaweedFS
│   ├── validation/               # GA-73: real documents from data/input, report in data/output/reports
├── .env.example                  # Template for .env (NVIDIA and SeaweedFS settings, empty secrets)
├── pytest.ini
└── requirements.txt
```

## Dependencies

| Package | Purpose |
| --- | --- |
| `langchain-community` | `PyPDFLoader` for reading PDF pages |
| `pypdf` | PDF parser used by `PyPDFLoader` |
| `langchain-text-splitters` | `RecursiveCharacterTextSplitter` for chunking |
| `langchain-core` | Shared LangChain `Document` type |
| `langchain-nvidia-ai-endpoints` | `NVIDIAEmbeddings` for the NVIDIA API |
| `numpy` | Vector checks (NaN and infinity) and similarity maths |
| `boto3` | S3 client for the SeaweedFS object storage |
| `python-dotenv` | Loads embedding and storage settings from `.env` |
| `pytest` | Running tests |

## Installation

### Prerequisites

- Python 3.10 or newer

Check your version:

```powershell
python --version
```

If `python` points to an older version, use the full path to your Python 3
installation in the commands below.

### 1. Go to the RAG folder

From the repository root:

```powershell
cd ai-platform/services/knowledge-service/rag
```

### 2. Create a virtual environment

```powershell
python -m venv .venv
```

The `.venv` folder is ignored by Git.

### 3. Activate the virtual environment

```powershell
.venv\Scripts\Activate.ps1
```

On macOS or Linux, use `source .venv/bin/activate`.

If PowerShell blocks the script, allow scripts for the current session and try again:

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
```

### 4. Install the dependencies

```powershell
pip install -r requirements.txt
```

### 5. Verify the installation

```powershell
python -c "from langchain_community.document_loaders import PyPDFLoader; from langchain_text_splitters import RecursiveCharacterTextSplitter; print('ok')"
```

The command prints `ok` when the dependencies are installed correctly.

### Deactivate

```powershell
deactivate
```

## Documents

The original PDFs are stored in SeaweedFS (see [Document storage](#document-storage-seaweedfs)),
and processing reads them only from there. Generated output is written to
`data/output/`. Do not commit government documents or generated output to Git.

## Embedding settings

All embedding settings, including the NVIDIA API key and every model detail, live in
`rag/.env`. The code has no default values, so each variable below is required.

### Set up `.env`

1. Copy the template:

   ```powershell
   Copy-Item .env.example .env
   ```

2. Open `.env` and set `NVIDIA_API_KEY` to your key from
   [build.nvidia.com](https://build.nvidia.com). Keep the other values or change them.

| Variable | Example | Purpose |
| --- | --- | --- |
| `NVIDIA_API_KEY` | `nvapi-...` | Secret API key; must start with `nvapi-` |
| `EMBEDDING_MODEL` | `nvidia/nemotron-3.5-embed-1b` | NVIDIA embedding model |
| `EMBEDDING_BATCH_SIZE` | `50` | Chunks per API request, 1–50 |
| `EMBEDDING_MAX_RETRIES` | `2` | Retries for rate limits (408, 429) and server errors (5xx), 0–5 |
| `EMBEDDING_RETRY_DELAY_SECONDS` | `1.0` | Wait before the first retry; doubles on each further retry |
| `EMBEDDING_TRUNCATE` | `NONE` | Over-long chunks: `NONE` fails them (no text lost), `START` or `END` cuts text |

### How the settings are loaded

- `load_settings()` reads `rag/.env` directly; nothing else needs to load it.
- Real environment variables override values in `.env`, which is useful in CI.
- The script can use another file with `--env-file`.
- A missing or blank value stops the run before any PDF is processed, with a message
  that names the variable, for example `NVIDIA_API_KEY is required in .env`.

### Keeping the key secret

- `.env` is ignored by Git. Never commit it, and never put the key in code, logs, or chat.
- The key is held as a Pydantic `SecretStr`, so it is hidden when settings are printed,
  and error messages never include it.
- Only `.env.example`, with an empty key, is committed.

Every chunk is sent to NVIDIA's cloud. Use it only with documents that are approved
for that. Changing the model means every document must be embedded again; each
embeddings file records its model so vectors are never mixed.

## Document storage (SeaweedFS)

The original PDFs are kept in SeaweedFS, an S3-compatible object store that runs in
Docker. `process_documents.py` reads PDFs only from SeaweedFS, so every document must
be uploaded first.

```text
PDF on your computer ──upload_documents.py──► SeaweedFS ──process_documents.py──► data/output/
```

### Start SeaweedFS

Follow [infrastructure/object-storage/README.md](../../../../infrastructure/object-storage/README.md):
create the root `.env`, run `docker compose up -d seaweedfs`, and create the
`govai-knowledge` bucket once.

### Storage settings

Add these to `rag/.env`, with the same values as the `OBJECT_STORAGE_*` entries in the
repository root `.env` (docker-compose creates the SeaweedFS user from those):

| Variable | Example | Purpose |
| --- | --- | --- |
| `OBJECT_STORAGE_ENDPOINT` | `http://localhost:8333` | S3 API address; inside Docker use `http://seaweedfs:8333` |
| `OBJECT_STORAGE_BUCKET` | `govai-knowledge` | Bucket for verified government documents |
| `OBJECT_STORAGE_REGION` | `ap-south-1` | Any region name; only used for request signing |
| `OBJECT_STORAGE_ACCESS_KEY` | `govai_access_key` | Secret; same as the root `.env` |
| `OBJECT_STORAGE_SECRET_KEY` | | Secret; same as the root `.env` |

Both keys are held as `SecretStr` and never appear in logs or error messages.

### Upload documents

Every PDF must be uploaded with `scripts/upload_documents.py` before it can be processed.
The PDF can be in any folder on your computer.

#### Step 1: Check that SeaweedFS is running

From the repository root:

```powershell
docker compose ps seaweedfs
```

The `STATUS` column must show `Up`. If it does not, run `docker compose up -d seaweedfs`.

#### Step 2: Go to the RAG folder and activate the virtual environment

```powershell
cd ai-platform/services/knowledge-service/rag
.venv\Scripts\Activate.ps1
```

#### Step 3: Upload the PDF

Give the full path of the PDF with `--file`. Put the path in quotes if it has spaces.

```powershell
python scripts/upload_documents.py --department-id IRD --service-id TIN_REGISTRATION --document-type service_guideline --file "C:\Users\me\Downloads\TIN_guide.pdf"
```

| Option | Purpose |
| --- | --- |
| `--department-id` | Department that owns the PDF, e.g. `IRD` (required) |
| `--service-id` | Service the PDF belongs to, e.g. `TIN_REGISTRATION` (required for `service_guideline`) |
| `--document-type` | `service_guideline` or `department_guideline` (required) |
| `--file` | One or more PDF paths, separated by spaces (required) |
| `--env-file` | Settings file, default `rag/.env` |

IDs may contain only letters, digits, `_` and `-`, because they become folder names in
the bucket.

To upload several PDFs for the same service at once, list them all after `--file`:

```powershell
python scripts/upload_documents.py --department-id IRD --service-id TIN_REGISTRATION --document-type service_guideline --file "C:\docs\TIN_guide.pdf" "C:\docs\TIN_faq.pdf"
```

For a department guideline, leave out `--service-id`:

```powershell
python scripts/upload_documents.py --department-id IRD --document-type department_guideline --file "C:\docs\IRD_overview.pdf"
```

#### Step 4: Read the result

The script prints one line for each file:

```text
  UPLOADED TIN_guide.pdf -> documents/IRD/TIN_REGISTRATION/619b6495...0085c.pdf
  SKIPPED  TIN_faq.pdf (already stored as documents/IRD/TIN_REGISTRATION/3f2a...91d.pdf)
  FAILED   notes.pdf: File is not a PDF: notes.pdf

PDFs: 3, failed: 1
```

| Result | Meaning |
| --- | --- |
| `UPLOADED` | The PDF is now stored in SeaweedFS |
| `SKIPPED` | The same PDF was already stored, so nothing changed |
| `FAILED` | The file was not uploaded; the reason is shown after it |

Exit codes: `0` success, `1` some files failed, `2` invalid options, settings, or
storage (for example SeaweedFS not running).

#### Step 5: Check the file in the browser (optional)

Open http://localhost:8888/buckets/govai-knowledge/documents/ and go to the department
and service folder. The PDF is listed under its `document_id`.

Use the Filer page only to look at files, not to upload them. A PDF uploaded there gets
no `document_id` name or metadata, so processing reports it as failed.

#### Step 6: Process the uploaded PDFs

See [Processing documents](#processing-documents).

#### Upload problems

| Message | Fix |
| --- | --- |
| `Object storage unavailable: Object storage is not reachable` | Start SeaweedFS (step 1) |
| `Object storage unavailable: Bucket not found: govai-knowledge` | Create the bucket, see [infrastructure/object-storage/README.md](../../../../infrastructure/object-storage/README.md) |
| `Object storage unavailable: ... rejected the access key or secret key` | Make the `OBJECT_STORAGE_*` keys in `rag/.env` match the root `.env` |
| `OBJECT_STORAGE_... is required in .env` | Add the storage settings to `rag/.env` (see [Storage settings](#storage-settings)) |
| `FAILED ... PDF file not found` | Check the path after `--file` |
| `FAILED ... File is not a PDF` or `exceeds ... bytes` | Only real PDFs up to 50 MB can be uploaded |

#### How the PDFs are stored

Each PDF is stored under a key built from its content (its SHA-256 `document_id`):

```text
documents/<department_id>/<service_id>/<document_id>.pdf     # service guideline
documents/<department_id>/_department/<document_id>.pdf      # department guideline
```

- The same PDF always gets the same key, so running the upload again prints `SKIPPED`
  and never creates a duplicate.
- A revised PDF has different content, so it is stored under a new key; the old version
  is kept.
- Files that are not real PDFs, or are larger than 50 MB, are not uploaded.
- Object metadata holds only the IDs and the URL-encoded file name (S3 metadata must be
  ASCII, so Sinhala and Tamil names are encoded).

### Use in code

```python
from app.storage.config import load_storage_settings
from app.storage.object_storage import create_document_store

store = create_document_store(load_storage_settings())
store.check_bucket()
stored = store.upload_document("C:/Users/me/Downloads/guide.pdf", source)
pdf_path = store.download_document(stored.object_key, "downloads")
```

`DocumentStore` is the only code that uses boto3, so the object store can be replaced by
changing `app/storage/object_storage.py` alone.

| Error | Meaning |
| --- | --- |
| `StorageConfigError` | A storage setting is missing or invalid |
| `StorageConnectionError` | SeaweedFS is not running or not reachable |
| `StorageAuthError` | SeaweedFS rejected the access key or secret key |
| `BucketNotFoundError` | The bucket has not been created |
| `DocumentNotFoundError` | No document is stored under the key |

## Processing documents

Upload the PDFs first (see [Upload documents](#upload-documents)), then run from the
`rag` folder:

```powershell
python scripts/process_documents.py --department-id IRD --service-id TIN_REGISTRATION --document-type service_guideline
```

The script processes every PDF stored for that department and service. The PDFs are
downloaded to a temporary folder that is deleted afterwards, and each download is
checked against the SHA-256 in its key. Every chunk gets an `object_key` metadata value,
so each vector stored later in ChromaDB points back to its original PDF.

| Option | Purpose |
| --- | --- |
| `--department-id` | Department that owns the documents (required) |
| `--service-id` | Service the documents belong to (required for `service_guideline`) |
| `--title` | Document title, e.g. `"Obtaining a Taxpayer Identification Number (TIN)"`. Each chunk is embedded with the title in front, so chunks that never repeat the topic are still found; the saved text is unchanged. |
| `--document-type` | `service_guideline` or `department_guideline` (required) |
| `--output-dir` | Default `data/output` |
| `--env-file` | Settings file, default `rag/.env` |
| `--skip-embeddings` | Only extract and chunk |

Output in `data/output/`:

| Folder | Content |
| --- | --- |
| `extracted/` | `<document_id>.json`, page text and metadata |
| `chunks/` | `<document_id>.chunks.json`, chunks with `chunk_id` and metadata |
| `embeddings/` | `<document_id>.embeddings.json`, model, dimension, and one record per chunk |

Exit codes: `0` success, `1` some PDFs or chunks failed, `2` invalid options, settings,
or storage.

Failures do not lose finished work. A failed batch is reported and the other batches are
saved. Running the script again embeds only the chunks that are new, changed, or failed
before. If NVIDIA rejects the API key, embedding stops, but the remaining PDFs are
still extracted and chunked.

## Tests

```powershell
pytest
```

The embedding tests call the **real NVIDIA API** with the settings in `rag/.env`; there
are no fake models. They need `NVIDIA_API_KEY` and an internet connection. Without a key
they are skipped, and pytest shows the reason. The other tests run offline.

- Only short synthetic sentences are sent, never real government documents.
- Failures are produced for real: a key NVIDIA rejects (403) and a chunk longer than
  NVIDIA accepts (400).
- Retries after server errors (5xx) and rate limits (429) cannot be triggered on demand,
  so they are not covered by tests.

The SeaweedFS tests in `tests/integration/test_seaweedfs.py` use the **real SeaweedFS
container** with the storage settings in `rag/.env`. They create a separate
`govai-knowledge-test` bucket and delete it afterwards, so the real bucket is never
touched. Without the settings or a running container they are skipped.

| What | Command |
| --- | --- |
| All tests | `pytest` |
| Embedding unit tests | `pytest tests/unit -k embedding` |
| SeaweedFS tests | `pytest tests/integration/test_seaweedfs.py tests/unit/test_object_storage.py tests/unit/test_storage_config.py` |
| One file | `pytest tests/unit/test_embedding_service.py` |
| Show skipped tests and why | `pytest -rs` |

## Validating documents (GA-73)

`tests/validation/` runs real documents through extraction, chunking, and the real NVIDIA
API, saves everything to `data/output/`, and writes a report.

1. Copy the PDF into `data/input/`.
2. Add it to `tests/validation/documents.json` with its source details, title, key phrases
   (with the page each is on), and a few questions with the page that answers them.
3. Run:

   ```powershell
   pytest tests/validation -v
   ```

4. Read `data/output/reports/validation_report.md`.

| Acceptance criterion | What is checked |
| --- | --- |
| Meaningful chunks | Every page has text and is chunked, chunks are 1–1000 characters, no PDF bullet glyphs |
| Information preserved | No page text is lost between chunks, overlap is at most 200 characters, every key phrase is whole in a chunk from its page |
| Identifiers and metadata | Chunk IDs are unique and in order; every chunk has its document ID, title, file, department, service, type, and page |
| Embeddings generated | One embedding per chunk, none failed, saved to `data/output/embeddings/` |
| Valid vectors | One dimension for all vectors, finite non-zero values, re-embedding gives the same vector (cosine ≥ 0.99), ready for ChromaDB |
| Errors handled | Blank, empty, non-PDF, and password-protected files; an over-long chunk; a rejected key; a damaged chunk file |
| Pipeline validated | Each question's closest chunk comes from the page that answers it |

The documents and the report stay in `data/`, which Git ignores. The document text is sent to
NVIDIA's cloud. A re-run reuses the saved vectors, so the document is sent only once.
