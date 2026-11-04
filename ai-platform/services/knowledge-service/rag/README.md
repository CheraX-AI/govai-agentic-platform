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
│   └── embeddings/
│       ├── config.py             # NVIDIA settings read from .env
│       ├── exceptions.py         # Errors and NVIDIA API error classification
│       ├── models.py             # Pydantic models
│       ├── validators.py         # Chunk, metadata, and vector checks
│       ├── storage.py            # Chunk and embeddings files, vector-store output
│       └── embedding_service.py  # NVIDIA client, batching, retries, re-run reuse
├── data/
│   ├── input/                    # Original PDF documents
│   └── output/                   # Generated chunks and embeddings
├── scripts/
│   └── process_documents.py      # Processes the PDFs in data/input
├── tests/
│   ├── unit/                     # Unit tests (embedding tests call the real NVIDIA API)
│   ├── integration/              # PDF → chunks → embeddings, and the script
│   ├── validation/               # GA-73: real documents from data/input, report in data/output/reports
├── .env.example                  # Template for .env (all NVIDIA settings, empty key)
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
| `python-dotenv` | Loads embedding settings from `.env` |
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

Place PDF documents to process in `data/input/`. Generated output is written to
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

## Processing documents

Place the PDFs in `data/input/`, then run from the `rag` folder:

```powershell
python scripts/process_documents.py --department-id IRD --service-id TIN_REGISTRATION --document-type service_guideline
```

| Option | Purpose |
| --- | --- |
| `--department-id` | Department that owns the documents (required) |
| `--service-id` | Service the documents belong to (required for `service_guideline`) |
| `--title` | Document title, e.g. `"Obtaining a Taxpayer Identification Number (TIN)"`. Each chunk is embedded with the title in front, so chunks that never repeat the topic are still found; the saved text is unchanged. |
| `--document-type` | `service_guideline` or `department_guideline` (required) |
| `--input-dir`, `--output-dir` | Default `data/input` and `data/output` |
| `--env-file` | Settings file, default `rag/.env` |
| `--skip-embeddings` | Only extract and chunk |

Output in `data/output/`:

| Folder | Content |
| --- | --- |
| `extracted/` | `<document_id>.json`, page text and metadata |
| `chunks/` | `<document_id>.chunks.json`, chunks with `chunk_id` and metadata |
| `embeddings/` | `<document_id>.embeddings.json`, model, dimension, and one record per chunk |

Exit codes: `0` success, `1` some PDFs or chunks failed, `2` invalid options or settings.

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

| What | Command |
| --- | --- |
| All tests | `pytest` |
| Embedding unit tests | `pytest tests/unit -k embedding` |
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
