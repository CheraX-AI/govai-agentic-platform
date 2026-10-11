# GovAI Agentic Platform

A secure multi-agent AI platform for Sri Lankan government services. It helps citizens find the right service, complete applications and track their progress, and gives government staff AI support for handling requests.

> **Status:** early development. The project structure and development environment are set up; features are being built.

## Features (planned)

- **Service guidance**: answers questions about government services, eligibility and required documents.
- **Application assistance**: guides citizens step by step through filling in and submitting applications.
- **Application tracking**: shows citizens the current status of their submitted applications.
- **Staff support**: helps government staff review, summarize and respond to requests.

## Architecture

```
┌──────────────┐   ┌──────────────┐
│   Web app    │   │  Mobile app  │
│ React + Vite │   │  (planned)   │
└──────┬───────┘   └──────┬───────┘
       │      REST API    │
       └────────┬─────────┘
                ▼
       ┌─────────────────┐
       │     Backend     │   Node.js + Express
       │  (API gateway)  │   auth, business logic, data
       └────────┬────────┘
                ▼
       ┌─────────────────┐
       │    AI agent     │   Python + FastAPI + LangChain
       │    service      │   multi-agent orchestration, LLM calls
       └─────────────────┘
```

The web and mobile apps talk only to the backend. The backend calls the AI agent service when a request needs AI.

## Tech stack

| Part | Technology |
| --- | --- |
| Web app | React 19, TypeScript, Vite, Oxlint |
| Mobile app | Not chosen yet |
| Backend | Node.js 24, Express 5, Helmet, CORS, Morgan |
| AI agent | Python 3.13, FastAPI, Uvicorn, LangChain, OpenAI |

## Project structure

```
govai-agentic-platform/
├── ai-agent/      # Python AI agent service (own virtual environment)
├── app/
│   ├── mobile/    # Mobile app (not started)
│   └── web/       # React + TypeScript web app
├── backend/       # Node.js + Express REST API
├── LICENSE
└── README.md
```

Each part has its own dependencies and is set up and run separately.

## Getting started

### Prerequisites

- [Git](https://git-scm.com/)
- [Node.js 24+](https://nodejs.org/) and npm
- [Python 3.13+](https://www.python.org/downloads/)

### 1. Clone the repository

```bash
git clone https://github.com/CheraX-AI/govai-agentic-platform.git
cd govai-agentic-platform
```

### 2. Backend

```bash
cd backend
npm install
cp .env.example .env      # Windows Command Prompt: copy .env.example .env
npm run dev
```

The API runs at `http://localhost:5000`. Check it with `GET http://localhost:5000/api/health`.

| Command | Description |
| --- | --- |
| `npm run dev` | Start with auto-reload |
| `npm start` | Start the server |
| `npm test` | Run tests |

See [backend/README.md](backend/README.md) for the backend folder layout.

### 3. Web app

```bash
cd app/web
npm install
npm run dev
```

The web app runs at `http://localhost:5173`.

| Command | Description |
| --- | --- |
| `npm run dev` | Start the development server |
| `npm run build` | Type-check and build for production |
| `npm run lint` | Lint the code with Oxlint |
| `npm run preview` | Preview the production build |

### 4. AI agent

The AI agent uses a Python virtual environment, so its packages stay separate from the rest of your system.

Create the environment (one time):

```bash
cd ai-agent
py -3.13 -m venv .venv        # Windows
python3 -m venv .venv         # macOS / Linux
```

Activate it (in every new terminal):

| Shell | Command |
| --- | --- |
| PowerShell | `.\.venv\Scripts\Activate.ps1` |
| Command Prompt | `.venv\Scripts\activate.bat` |
| Git Bash | `source .venv/Scripts/activate` |
| macOS / Linux | `source .venv/bin/activate` |

Install the dependencies:

```bash
pip install -r requirements.txt
```

When you add a package, save it for the rest of the team:

```bash
pip freeze > requirements.txt
```

Leave the environment with `deactivate`.

> If PowerShell says running scripts is disabled, run `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned` once.

## Environment variables

Never commit secrets. Each service reads its settings from its own `.env` file, which git ignores. Copy the `.env.example` file in that service to `.env` and fill in the values.

**Backend** (`backend/.env`)

| Variable | Default | Description |
| --- | --- | --- |
| `NODE_ENV` | `development` | `development`, `test` or `production` |
| `PORT` | `5000` | Port the API listens on |
| `CORS_ORIGIN` | `*` | Allowed frontend origin, for example `http://localhost:5173` |

## Contributing

1. Create a branch from `main`, for example `feature/service-guidance` or `fix/login-error`.
2. Make your changes and run the tests and linter for the part you changed.
3. Push the branch and open a pull request into `main`.

Do not commit `node_modules/`, `.venv/` or `.env` files.

## License

This project is licensed under the [MIT License](LICENSE).
