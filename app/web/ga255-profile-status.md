# GA-255 web personal information

Implemented from the desktop mockup approved by the user on 25 November 2026,
with the existing GovAI header/sidebar, colours, fonts and assets. This is the
web screen only; app/mobile is unchanged.

## Open the page

After login, choose Settings in the sidebar or Personal information in the
account menu. The protected URL is `/app/profile` on the existing web origin
(normally `https://localhost:5173/app/profile` when local HTTPS is configured).
Use the normal web development/deployment instructions; no new server is added.
The live auth-service must have the GA-252 migration and GA-253 profile feature
enabled. Photos additionally require GA-254 configuration, a private bucket and
working storage credentials. Existing private configuration and feature flags
were not changed by this UI task. Disabled/unreachable APIs show an error with
retry instead of invented citizen data.

## Implemented behaviour

Registered full name, NIC, mobile and Keycloak email are read-only. The form
edits only Sinhala/Tamil names, DOB, Male/Female gender and address. Optional
values remain clearable. Date input presentation follows browser locale; the
API receives YYYY-MM-DD. The server remains authoritative for NIC/DOB matching,
field rules, ownership, encryption and persistence.

Save sends the explicit permitted-field allowlist plus the current revision.
Cancel discards unsaved local edits. Loading, validation, network and session
errors are handled. A 409 or uncertain server/network result keeps the draft,
blocks further writes, and offers explicit load-latest/keep-edits or discard
choices. There is no automatic retry of writes. Repeated submit is guarded.

Photo selection previews a JPEG/PNG up to 2 MiB, then requires Upload photo.
Photo upload is separate from Save changes, so a photo failure cannot silently
change the details form. Both operations share the current profile revision.
The backend validates actual image content. Photo reads use the authenticated
browser adapter header and temporary blob URLs, revoked on replacement/unmount.
Personal data is held in component memory, never browser storage or chat/AI.

Other settings categories are disabled placeholders. Existing unavailable
services, subscription UI and chat history are not implemented by this task.
The header retains the existing signed-in account presentation. Sample names,
addresses, recent chats and plan details from the mockup are not production data.

## Verification

In a separate copy using the repository dependencies, the production build,
TypeScript checks, lint (no warnings), profile API request tests and existing
authentication return-path checks passed. Headless Edge tests with synthetic
HTTP fixtures passed: desktop rendering, read-only fields, permitted writes,
duplicate-submit guard, persistence across simulated refresh, 409 reconciliation,
422/503 errors, authenticated photo upload/display, no browser storage, expired
session protection and no horizontal overflow at 1440/1024/768/390 widths.
The desktop screenshot was visually inspected. Test server and browser use only
synthetic data. The 390px check verifies responsive web layout, not GA-288 mobile.

Commands from app/web:

```
npm run build
npm run lint
node scripts/integration/check-profile-api.mjs
node scripts/check-auth-state.mjs
```

For the browser fixture test, run Vite locally and set
`GOVAI_FIXTURE_FRONTEND_URL` to its loopback HTTP address before running
`node scripts/integration/check-profile-browser.mjs`. The harness uses a separate
headless Edge profile and an intercepted HTTPS browser origin; it does not relax
certificate verification or contact a real account API.

Live Keycloak/PostgreSQL/SeaweedFS end-to-end persistence, cross-device behaviour,
screen-reader and physical-device checks are not claimed. Backend operational
flags remain unchanged. No commit or push was made by this task.


## Repository structure review

Reviewed against the existing web chat/auth and backend feature patterns. Web
profile presentation is in `src/components/profile`; state and lifecycle are in
`src/profile/useProfile.ts`; input feedback is in `src/validation/profileValidation.ts`.
`ProfilePage` assembles those parts and the existing auth context. API transport
remains in services, protected routes in routing, and CSS in styles. The profile
settings icon component moved from the shared component root into its feature
folder; its old import/path was removed. These are organizational changes and
retain the approved layout and API contract.

GA-252/253/254 already follow the auth-service layers: feature rules in
`internal/features/profile`, HTTP adapters in `internal/http/handlers`, database
code/migration 007 in `internal/external/postgres`, SeaweedFS S3 adapter in
`internal/external/objectstorage`, and configuration/wiring in their existing
folders. Their changed Go files passed gofmt inspection; no backend files or
existing migrations were moved or rewritten during this structure review.
