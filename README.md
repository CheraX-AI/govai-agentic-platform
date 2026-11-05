# GovAI Agentic Platform

GovAI is a university prototype providing guidance about Sri Lankan government
services. It is not an official government system. Sprint scope is informational
service guidance; government application submission, payments and tracking are
not implemented by this backend.

## Architecture and current status

React web / future mobile client -> Go + Gin APIs -> Keycloak / PostgreSQL /
teammate-owned Python AI services.

- Authentication: custom registration, email/NIC password login, email verification,
  browser sessions, native refresh/revoke and NIC-based email password recovery.
- Chat: browser → auth-service (`/api/v1/web/chat`, session cookie) → core-service
  (`/api/chat/message`, Keycloak access token) → agent-api (`/api/v1/chat`) →
  knowledge-service (RAG). Answers carry numbered sources (`number`, `title`,
  optional `page`/`url`); no generated answers are fabricated when a service is down.
- Web: registration, login, session, logout and chat use the real backend (no
  mocks). Mobile integration remains to be verified.
- Catalogue, KYC and notification service directories remain placeholders.

## Technology and folders

| Area | Technology / location |
| --- | --- |
| Backend APIs | Go 1.26+, Gin; `backend/services/` |
| Shared JWT middleware | Go; `backend/pkg/authn/` |
| Identity | Keycloak; `infrastructure/keycloak/` |
| Auth application database | PostgreSQL; `infrastructure/postgres/auth/` |
| Keycloak database | Separate PostgreSQL; `infrastructure/postgres/keycloak/` |
| Web | React, TypeScript, Vite; `app/web/`; Node/npm are frontend tools |
| Mobile | `app/mobile/`; consult its owner for current implementation |
| AI | Teammate-owned Python services; `ai-platform/services/` |
| Shared infrastructure | Root Compose; Redis/storage/ChromaDB and shared PostgreSQL |

The Go backend does not use Express or npm. Do not replace the isolated auth
PostgreSQL instances with root Compose's shared PostgreSQL by assumption.

## Getting started

Clone the repository and use the approved branch/review process:

```powershell
git clone https://github.com/CheraX-AI/govai-agentic-platform.git
cd govai-agentic-platform
```

Install Go, Git and Docker Desktop (Linux engine). Node/npm are needed for the
web app; Python requirements belong to the AI owners' service guides.

### Backend

Follow [backend guide](backend/README.md) and the individual service README.
Fresh setup requires private environment settings, TLS trust, database provisioning,
Keycloak realm/clients/SMTP setup and explicit migrations; pulling code alone is
not a complete bootstrap. Do not copy another developer's private keys or secrets.

After local setup, run from the authentication module:

```powershell
cd backend/services/auth-service
go mod download
go test ./...
go vet ./...
go run ./cmd/api
```

Configured local auth API: `https://localhost:5001`; health `/api/health`,
database readiness `/api/ready`. Bare foundation mode can default to HTTP port
5000 when authentication features are disabled; that is not the configured auth setup.

Core/chat is a separate module at `backend/services/core-service`, with configured
HTTPS port 5002. See its README before running it.

### Web

Follow [web guide](app/web/README.md). Open the app at `https://localhost:5173`
(not `127.0.0.1`: the session cookie only works on the same site as the API).
Either run it on your machine (from `app/web`: `npm ci`, then `npm run dev`) or in
Docker from the repository root:

```powershell
docker compose up -d --build web                          # Vite dev server, live reload
docker compose --profile prod up -d --build web-prod      # production build on nginx
```

`web` also starts with a plain `docker compose up -d`. Only one of `web`,
`web-prod` and `npm run dev` can use port 5173 at a time. The browser calls
auth-service on the host (`https://localhost:5001`).

### AI services

Follow the existing [RAG guide](ai-platform/services/knowledge-service/rag/README.md)
and coordinate endpoints/authentication with its owner. Go proxies questions;
it does not implement ingestion, embeddings, retrieval or model orchestration.

## Secrets and verification

Each service owns its environment configuration. Only placeholder `.env.example`
files belong in shared source; real `.env`, TLS private keys, backups and local
state must stay ignored. Run tests from each Go module separately.

Use synthetic accounts. Preserve existing named volumes and credentials; do not
use `docker compose down -v` to repair configuration. Local health checks do not
prove real AI integration, deployment readiness or government compliance.

## Contributing and license

Create a feature branch, run relevant checks and open a reviewed PR. Do not
commit secrets or generated dependency/build directories. See [LICENSE](LICENSE).
