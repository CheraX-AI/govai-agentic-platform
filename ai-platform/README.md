# GovAI AI Platform: commands

Everyday commands for the AI platform: the knowledge service (RAG) and the agent
service. Run every command from the **repository root** in PowerShell, unless a step
says otherwise.

```text
Browser ──► agent-chat (Chainlit, :8502) ──► knowledge-service (internal) ──► ChromaDB
                    └──► NVIDIA LLM                          └──► NVIDIA embeddings
```

Details: [agent-service/README.md](services/agent-service/README.md) and
[knowledge-service/rag/README.md](services/knowledge-service/rag/README.md).

## 1. One-time setup

```powershell
# Settings files (fill in the keys; never commit them)
Copy-Item .env.example .env
Copy-Item ai-platform/services/agent-service/.env.example ai-platform/services/agent-service/.env
Copy-Item ai-platform/services/knowledge-service/rag/.env.example ai-platform/services/knowledge-service/rag/.env
```

You need an NVIDIA API key (build.nvidia.com) in both service `.env` files, and the
`LLM_MODEL` you use in `agent-service/.env`.

## 2. Use both compose files (once per terminal)

The AI services live in `ai-platform/docker-compose.ai.yml`, next to the shared root
`docker-compose.yml`. Tell Docker Compose to use both, so every command below stays short:

```powershell
$env:COMPOSE_FILE = "docker-compose.yml;ai-platform/docker-compose.ai.yml"
```

On macOS or Linux: `export COMPOSE_FILE=docker-compose.yml:ai-platform/docker-compose.ai.yml`

## 3. Run the platform

| Task | Command |
| --- | --- |
| Start (build if needed) | `docker compose up -d --build knowledge-service agent-chat` |
| Open the chat | http://127.0.0.1:8502 |
| Status | `docker compose ps` |
| Logs | `docker compose logs -f agent-chat knowledge-service` |
| Restart after a code change | `docker compose up -d --build knowledge-service agent-chat` |
| Stop | `docker compose stop agent-chat knowledge-service` |
| Start LangGraph Studio too | `docker compose --profile studio up -d agent-studio`, then open https://smith.langchain.com/studio/?baseUrl=http://127.0.0.1:2024 |

ChromaDB and SeaweedFS start automatically with `up`.

## 4. Add documents

Documents go in three steps: **upload** the PDF to SeaweedFS, **process** it (extract,
chunk, embed, store in ChromaDB), then **search** to check it. Processing works in memory
and writes no files.

Upload runs on your machine, because it reads the PDF from your disk. In the
`ai-platform/services/knowledge-service/rag` folder, with its `.venv` active:

```powershell
python scripts/upload_documents.py --department-id IRD --service-id TIN_REGISTRATION --document-type service_guideline --file "C:\docs\TIN_guide.pdf"
```

Process and search run in Docker, from the repository root:

| Task | Command |
| --- | --- |
| Process (dry run: count only) | `docker compose run --rm knowledge-service python scripts/process_documents.py --department-id IRD --service-id TIN_REGISTRATION --document-type service_guideline --dry-run` |
| Process (embed and store) | `docker compose run --rm knowledge-service python scripts/process_documents.py --department-id IRD --service-id TIN_REGISTRATION --document-type service_guideline --title "Obtaining a Taxpayer Identification Number (TIN)"` |
| Search | `docker compose run --rm knowledge-service python scripts/search_documents.py "How do I register for a TIN?" --top-k 3` |

Replace `IRD`, `TIN_REGISTRATION`, and the title with the department, service, and title
of your document.

## 5. Check that it works

| Check | Command | Expected |
| --- | --- | --- |
| Knowledge service is healthy | `docker compose ps knowledge-service` | `(healthy)` |
| Chat is up | `(Invoke-WebRequest http://127.0.0.1:8502 -UseBasicParsing).StatusCode` | `200` |
| Knowledge service is internal | `curl.exe -s -m 3 http://127.0.0.1:8020/health` | fails (not reachable from your machine) |
| Agent reaches the knowledge service | `docker exec govai-agent-chat python -c "import httpx; print(httpx.get('http://knowledge-service:8020/health').text)"` | `{"status":"ok"}` |

Then ask a full question in the chat, for example "What documents do I need for a TIN?".
The answer lists its sources.

## 6. Run the tests

Each service has its own virtual environment. From the service folder, with its `.venv`
active:

| Service | Folder | Command |
| --- | --- | --- |
| Agent | `ai-platform/services/agent-service` | `pytest` |
| Knowledge (RAG) | `ai-platform/services/knowledge-service/rag` | `pytest` |
| Offline only (agent) | `ai-platform/services/agent-service` | `pytest --ignore tests/test_llm_live.py --ignore tests/test_rag_live.py` |
| Offline only (RAG) | `ai-platform/services/knowledge-service/rag` | `pytest tests/unit` |

Tests that need NVIDIA, SeaweedFS, or ChromaDB are skipped when those are not available;
`pytest -rs` shows why.

## Troubleshooting

| Problem | Fix |
| --- | --- |
| `docker compose` does not know `agent-chat` | Run step 2 in this terminal |
| Chat says "not configured: ... is required in .env" | Add the named setting to `agent-service/.env`, then restart |
| Chat says "cannot answer right now" | `docker compose logs knowledge-service`; usually ChromaDB is down or `rag/.env` is incomplete |
| `knowledge-service` never becomes healthy | `docker compose logs knowledge-service` |
| Port 8502 is in use | Stop the other program, or `$env:AGENT_CHAT_PORT = "8503"` before `up` |
| Upload fails with `InternalError` | SeaweedFS has no free volume slots; ask the infrastructure owner to raise its volume limit |
