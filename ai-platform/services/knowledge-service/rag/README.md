# Knowledge Service RAG

LangChain-based document processing for the GovAI knowledge service. It reads PDF
documents, extracts their text and splits it into chunks for embedding.

## Folder structure

```text
rag/
├── app/
│   ├── processing/
│   │   ├── pdf_extractor.py      # Extracts text from PDF pages
│   │   ├── chunker.py            # Splits extracted text into chunks
│   │   └── pipeline.py           # Runs extraction and chunking in order
│   └── embeddings/
│       └── embedding_service.py  # Creates embeddings for chunks
├── data/
│   ├── input/                    # Original PDF documents
│   └── output/                   # Generated chunks and embeddings
├── scripts/
│   └── process_documents.py      # Processes the PDFs in data/input
├── tests/
└── requirements.txt
```

## Dependencies

| Package | Purpose |
| --- | --- |
| `langchain-community` | `PyPDFLoader` for reading PDF pages |
| `pypdf` | PDF parser used by `PyPDFLoader` |
| `langchain-text-splitters` | `RecursiveCharacterTextSplitter` for chunking |
| `langchain-core` | Shared LangChain `Document` type |
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
