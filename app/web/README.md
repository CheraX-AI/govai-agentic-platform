# GovAI web app

React, TypeScript, Vite and plain CSS. The authenticated application uses the real
backend and has no demo authentication or simulated-success fallback.
`/preview/chat` is an isolated development-only fixture preview; `/app/chat`
remains protected by the existing session checks. The preview is unavailable in production.

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
| `/app/home`, `/app/chat`, `/app/recent` | Signed-in pages; guests are sent to `/login?returnTo=...` |
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
node scripts/check-recovery.mjs                 # recovery NIC validation
node scripts/integration/check-chat-api.mjs     # chat client: CSRF, sources, errors
node scripts/integration/check-web-auth-api.mjs # session client transport and errors
node scripts/integration/check-recovery-api.mjs # recovery client and receipt validation
node scripts/integration/check-reset-api.mjs    # current reset policy, token handling and client
```

## Frontend reconciliation

The existing `Modify_Code-structure` branch retains its commits and main's real authentication, pathname routing, recovery, and API services. GA-21's presentation was selectively ported: `ChatComposer`, `ChatMessage`, `ConversationView`, `ChatPage` and `styles/chat.css`. Its mock authentication, demo controls, synthetic user, mock services, hash router and full-history request contract were not imported.

`routing/ApplicationRoutes.tsx` owns one local `chat/useConversation.ts` state for Home, Chat and Recent. Home sends into `/app/chat`; Recent links to the current in-tab conversation without claiming saved history. New chat resets it. Logout, identity change, confirmed session failure, public departure and reload clear local messages and cancel pending work. Existing session checking withholds protected UI; a successful same-identity recheck retains the in-memory conversation. The hook calls the destination's unchanged `services/chatApi.ts` with just the newest message. No request/response mapper, new API endpoint, backend setup or auth changes were added by this reconciliation.

The UI distinguishes citizen/assistant messages, retains messages on errors, permits explicit retry with a duplicate-processing warning, supports cancellation/Enter/Shift+Enter/IME, follows new messages without forcing readers away from older messages, and preserves the reading anchor when the existing GA-21 header collapses. Sources use the existing service's numbered title/page/optional-HTTPS-URL contract. Missing URLs are plain text. Text is rendered by React, not raw HTML. There is no fallback to simulated answers.

Layout, typography, assets, login and account behavior retain main's behavior alongside the pushed GA-21 conversation presentation and the requested shared composer adjustments. Source/UI logic is in `src/chat`, `src/components/chat`, `src/pages/ChatPage.tsx`, and `src/styles/chat.css`; existing authentication code and `src/services` are unchanged. Synthetic chat replies exist only in the explicitly selected development preview under `src/dev`. Fixture-driven browser verification is maintained locally and is outside the proposed shared test set. Neither the preview nor local test responses are a fallback for the authenticated application.

### Shared verification scope

The commands above are self-contained validation and API-client checks. The API-client
checks replace `fetch` within each check; they require no external fixture files or
running services and do not demonstrate live integration.

Fixture-driven browser checks are retained locally, outside the proposed shared
commit. Their coverage includes authentication/recovery navigation, protected routes,
chat loading/errors/retry/cancellation, source rendering, keyboard/IME handling,
composer geometry, scrolling and responsive menu focus. Those checks are not part
of the shared commands above.

The existing opt-in live checks are `scripts/integration/check-auth-live-browser.mjs`
and `scripts/integration/check-chat-unavailable-live.mjs`. They use the shared,
fixture-independent browser harness. They require configured HTTPS services and
create synthetic accounts when explicitly enabled. Historical route/selector
expectations still require review; live acceptance is not claimed. Keep their opt-in
flags unset during frontend-only checks; no services are started by this cleanup.

Paused API-integration work remains outside this frontend increment.

Limitations: no server-side conversation history, history CRUD, full-transcript context, attachments or voice. Live verification requires available configured services and legitimate test access, and is not part of this frontend-only reconciliation. The destination's existing API/auth behavior is intentionally unchanged; any transport/security improvements belong to the separately paused integration work.


## Isolated development chat preview

Open **http://localhost:5173/preview/chat** with the standard local Vite server (`npm.cmd run dev`; use HTTPS instead if local certificates are configured). This route is outside AuthProvider and does not check, create or require a session. It reuses the conversation UI with an in-memory synthetic service; opening or interacting with it makes no backend requests and stores no cookies or browser data. Normal Vite scripts/assets still load from the frontend server. The notice and replies explicitly identify the synthetic preview; the optional fixture result selector exercises success/error UI. It is never a fallback for real chat.

Production returns Page not found for `/preview/chat`, and its synthetic module is excluded from the built assets. `/app/chat` retains the real protected route. The preview's login link performs normal navigation back to the real application. Reload clears its local conversation.

Only the login form's visible Retry session check button was removed. Automatic session checking, session-error feedback, normal login submission, and the retry action on the protected session-error page remain unchanged. Authentication services were not edited.

### Shared composer sizing and keyboard behavior

`ChatComposer` is reused on Home, authenticated Chat and `/preview/chat`. The one-line form is 56px high with 5px padding, centered 44px controls and readable 16px text. It uses width:100%, max-width:850px and automatic horizontal margins; the disclaimer sits directly below on the same centerline. The textarea grows automatically to 160px, then scrolls internally; it has no manual resize handle. Enter submits through the same form handler as Send, Shift+Enter inserts a newline, and composition/native-IME/229 guards prevent unfinished composition from submitting. Empty/whitespace and pending submissions are blocked both at the shared control and conversation-state boundary.

Local browser verification measures sizing, targets, alignment, growth, internal
scrolling, resize and overflow at widths 1440/768/375/320, plus whitespace,
Shift+Enter and IME behavior in preview and normal application contexts.
Preview fixture success is not authenticated backend evidence.
