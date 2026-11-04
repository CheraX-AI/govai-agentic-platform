# ChromaDB Dashboard

## Purpose

A simple, read-only Streamlit dashboard for inspecting the GovAI ChromaDB instance.
It shows:

- Available collections and the number of records in each
- Collection metadata
- Document chunks and their metadata, page by page

The dashboard only reads data. It never adds, updates or deletes records.

## Files

| File | Purpose |
| --- | --- |
| `app.py` | The Streamlit dashboard |
| `requirements.txt` | Python dependencies |
| `Dockerfile` | Container image for the dashboard |

## Configuration

| Variable | Description | Default |
| --- | --- | --- |
| `CHROMA_HOST` | ChromaDB host name | `localhost` (`chromadb` in Docker) |
| `CHROMA_PORT` | ChromaDB port | `8001` (`8000` in Docker) |

Outside Docker, use the host port from `CHROMA_PORT` in the root `.env` (default `8001`).
Inside the Docker network, use the service name `chromadb` and its container port `8000`.

## Run locally

From this directory, with ChromaDB already running:

```powershell
python -m venv .venv
.venv\Scripts\Activate.ps1
pip install -r requirements.txt
streamlit run app.py
```

Open http://localhost:8501.

## Docker Compose

Add this service under `services:` in the root `docker-compose.yml`:

```yaml
  chromadb-dashboard:
    build: ./infrastructure/chromadb/dashboard
    container_name: govai-chromadb-dashboard
    restart: unless-stopped
    environment:
      CHROMA_HOST: chromadb
      CHROMA_PORT: 8000
    ports:
      - "8501:8501"
    depends_on:
      - chromadb
    networks:
      - govai-network
```

Then open http://localhost:8501.

To start it only on request, like pgAdmin and RedisInsight, add `profiles: [tools]`
and start it with `docker compose --profile tools up`.
