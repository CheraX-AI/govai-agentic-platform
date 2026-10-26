# ChromaDB

## Purpose

Vector database for storing and searching embeddings.

## Planned use in GovAI

- Stores document embeddings and their metadata for the RAG (retrieval-augmented generation) service.

## Local setup

ChromaDB and its read-only dashboard are defined as the `chromadb` and
`chromadb-dashboard` services in the root `docker-compose.yml`.

| Interface | URL | Use |
| --- | --- | --- |
| ChromaDB API | http://localhost:8001 | Used by GovAI services to store and search embeddings |
| Dashboard | http://localhost:8501 | View collections, document chunks and metadata |

The ChromaDB API port is bound to `127.0.0.1`, so it is only reachable from your machine.
Inside the Docker network, other services reach ChromaDB at `chromadb:8000`.

### Prerequisites

- Docker Desktop installed and running
- A `.env` file in the repository root

### 1. Configure environment variables

From the repository root, create `.env` once if it does not exist:

```powershell
Copy-Item .env.example .env
```

Check the ChromaDB setting in `.env`:

```env
CHROMA_PORT=8001
```

Startup fails if `CHROMA_PORT` is missing.

### 2. Start ChromaDB and the dashboard

From the repository root:

```powershell
docker compose up -d --build chromadb chromadb-dashboard
```

`--build` builds the dashboard image. It is only needed the first time or after
changing files in `dashboard/`.

Check that both are running:

```powershell
docker compose ps chromadb chromadb-dashboard
```

No initialization is needed. Collections are created by the GovAI services when they
store embeddings.

### 3. Verify

- Run `curl.exe http://localhost:8001/api/v2/heartbeat`. A JSON response with a
  heartbeat value means ChromaDB is running.
- Open http://localhost:8501. If no data has been stored yet, the dashboard shows
  "No collections exist yet."

### Stop

```powershell
docker compose stop chromadb chromadb-dashboard
```

Stored embeddings remain in the `chroma_data` volume.

### Troubleshooting

| Problem | Fix |
| --- | --- |
| `Set CHROMA_PORT in .env` | Add `CHROMA_PORT=8001` to the root `.env` |
| Dashboard shows a connection error | Check that `chromadb` is running and view logs with `docker compose logs chromadb-dashboard` |
| Port already in use | Change `CHROMA_PORT` in `.env`, or stop the program using port 8501 |

See [dashboard/README.md](dashboard/README.md) for running the dashboard without Docker.
