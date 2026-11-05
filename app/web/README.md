# GovAI web app

React, TypeScript, Vite and plain CSS. The web app talks only to the real backend:
there are no mock services, demo sessions or preview pages any more.

GovAI is a university prototype. Use synthetic test accounts only.

## How it connects

The browser calls **auth-service** only (`VITE_AUTH_API_URL`, default
`https://localhost:5001`). auth-service holds the Keycloak tokens on the server and
gives the browser an HttpOnly session cookie, so no token ever reaches JavaScript.

| Screen | Backend call | File |
| --- | --- | --- |
| Create an account | `POST /api/v1/registrations` (`Idempotency-Key`, no cookie) | `src/services/registrationApi.ts` |
| Log in, stay logged in, log out | `/api/v1/web/login`, `/session`, `/csrf`, `/refresh`, `/logout` (cookie + `X-GovAI-Web`, CSRF token on POST) | `src/services/browserAuthAdapter.ts` |
| Chat on Home | `GET /api/v1/web/csrf`, then `POST /api/v1/web/chat` | `src/services/chatApi.ts` |
| Forgot / reset password | `POST /api/v1/auth/forgot-password`, `/reset-password` | `src/services/recoveryApi.ts` |

Chat continues auth-service → core-service (`/api/chat/message`) → agent-api
(`/api/v1/chat`) → knowledge-service. A chat answer is
`{"answer": "...", "sources": [{"number": 1, "title": "...", "page": 3}]}`;
`page` and `url` are optional. Home shows sources as `[1] Title, p. 3`, linked only
when a source has a `url`.

### Backend settings the web app depends on

In `backend/services/auth-service` (private `.env` files):

- `CORS_ALLOWED_ORIGINS=https://localhost:5173` (registration)
- `WEB_SESSION_ENABLED=true` and `WEB_ORIGIN=https://localhost:5173` (login, chat)
- `WEB_CHAT_ENABLED=true` (chat; needs web sessions)
- `RECOVERY_ENABLED=true` (password reset; needs migrations 004-006 and SMTP)

Open the app at **`https://localhost:5173`**, not `127.0.0.1`: the session cookie is
`SameSite=Strict`, and `localhost:5173` and `localhost:5001` are the same site.

## Pages

URLs are normal paths (no `#`). Old `/#/login` style links are redirected.

| Path | Page |
| --- | --- |
| `/`, `/login` | Log in (email or NIC + password) |
| `/register` | Create an account |
| `/forgot-password` → `/check-email` | Request a password reset link |
| `/reset-password#token=...` | Choose a new password (link from the email; the token is read and removed from the address bar) |
| `/verify-code` | Asks the user to open a valid reset link |
| `/app/home`, `/app/recent` | Signed-in pages; guests are sent to `/login?returnTo=...` |
| `/home`, `/recent` | Redirect to `/app/home`, `/app/recent` |

Not available yet (no backend): the user's name (`/web/session` returns only an ID),
chat history, phone login, CAPTCHA, notifications, language switching, My requests,
Services, Help and Settings. These stay hidden or disabled.

## Run it

### On your machine

```powershell
npm ci
npm run dev        # npm.cmd run dev where script execution is restricted
```

Vite serves HTTPS on `https://localhost:5173` when `infrastructure/keycloak/tls/`
has `api-cert.pem` and `api-key.pem`.

### In Docker

`Dockerfile` has two targets; both serve `https://localhost:5173` with the same
local certificate (mounted read-only, never copied into an image). From the
repository root:

```powershell
# Development: Vite with live reload (src/, public/ and index.html are mounted).
# Also starts with a plain "docker compose up -d".
docker compose up -d --build web

# Production check: npm run build served by nginx (nginx.conf)
docker compose stop web
docker compose --profile prod up -d --build web-prod
```

Only one of `web`, `web-prod` and `npm run dev` can use port 5173 at a time. After
changing `package.json` or `vite.config.ts`, rebuild with `--build`. The production
image bakes in `VITE_AUTH_API_URL` at build time (build argument, default
`https://localhost:5001`). A server hosting the built app must return `index.html`
for unknown paths, as `nginx.conf` does.

## Source structure

| Responsibility | Files |
| --- | --- |
| Composition | `src/App.tsx` (AuthProvider + AppRouter), `src/main.tsx` |
| Session state | `src/auth/AuthProvider.tsx`, `AuthContext.ts`, `types.ts`, `ProtectedRoute.tsx` |
| Routing | `src/routing/routes.ts` (paths, return allowlist, titles, `navigate`, link handling), `AppRouter.tsx` (page selection, recovery state) |
| API calls | `src/services/` (see the table above) |
| Validation | `src/validation/` (format checks only; the backend and Keycloak decide) |
| Pages | `src/pages/` |
| Shared controls and layout | `src/components/`, `src/components/auth/AuthLayout.tsx`, `src/components/layout/` |
| Styles | `src/index.css` (globals, focus ring), `src/styles/` |

Focus: keyboard users get the 3px focus ring on buttons and links. Text fields and
the chat box show focus with their own border instead, and headings or the content
area focused by code (for screen readers on page change) show no ring.

## Design

Source: https://www.figma.com/design/IuwQ09eZ1P5ctrc1vkVgEz/?node-id=613-2372

- Fonts: locally hosted Inter for text and controls, Montserrat for the GovAI
  wordmark; font faces in `src/index.css`, files and OFL licences in
  `public/assets/fonts/` (see `ga51-design-assets.md`).
- Authentication photo: `public/assets/figma/login/auth-coast.jpg`, cropped from the
  original `login-photo.jpg` at x0/y620/w985/h977; `styles/login.css` anchors it
  top-right with cover. Change this shared asset for all auth screens; do not
  resize forms to fit the photo. Below 900px the photo is hidden.
- Shell assets: `public/assets/figma/shell/`; login and registration icons are
  recorded in `figma-login-assets.md`.
- Below 900px the desktop sidebar becomes a modal menu (a proposed adaptation;
  mobile frames were not supplied).

## Checks

```powershell
npm run build                                   # TypeScript + production bundle
npm run lint                                    # Oxlint
node scripts/check-auth-state.mjs               # routes, return allowlist
node scripts/check-login.mjs                    # email/NIC login validation
node scripts/check-registration.mjs             # registration rules + registration API client
node scripts/check-password-reset.mjs           # shared password policy
node scripts/check-recovery.mjs                 # recovery NIC validation
node scripts/integration/check-chat-api.mjs     # chat client: CSRF, sources, errors
```

The browser scripts (`scripts/*-browser.mjs`, `scripts/integration/*browser*.mjs`)
use headless Edge and still expect the old `#/` URLs and demo text; they need
updating before use.
