# GovAI Backend

Node.js + Express API for the GovAI agentic platform.

## Requirements

- Node.js 24+ (see `.nvmrc`; run `nvm use` if you use nvm)

## Setup

```bash
cd backend
npm install
cp .env.example .env
npm run dev      # start with auto-reload on http://localhost:5000
```

| Script          | Description                          |
| --------------- | ------------------------------------ |
| `npm run dev`   | Start the server with `--watch`      |
| `npm start`     | Start the server                     |
| `npm test`      | Run tests with the built-in test runner |

## Structure

```
backend/
├── src/
│   ├── config/        # Environment-based configuration
│   ├── controllers/   # Request handlers
│   ├── middlewares/   # Express middleware (errors, auth, ...)
│   ├── models/        # Data models / schemas
│   ├── routes/        # Route definitions, mounted under /api
│   ├── services/      # Business logic
│   ├── utils/         # Shared helpers
│   ├── validators/    # Request validation
│   ├── app.js         # Express app setup
│   └── server.js      # Entry point
├── tests/
├── logs/
├── .env.example
└── package.json
```

## Endpoints

- `GET /api/health`: health check
