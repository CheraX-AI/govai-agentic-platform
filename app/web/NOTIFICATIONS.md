# Web notification UI (GA-286)

## What is implemented

The bottom-left sidebar bell opens recent account notifications with an unread count. The full
inbox has All/Unread views, date groups, cursor pagination and message details.
Opening an unread message requests mark-as-read; the count refreshes only after
that request succeeds. Errors do not claim the message was saved. The settings
page edits email/in-app flags and explains mandatory security inbox alerts.

Protected routes:

- /app/notifications
- /app/notifications/settings

The existing cookie-session guard still controls both pages. Notification state
is held in memory under a provider keyed to the authenticated citizen, and is
discarded when the guard unmounts or identity changes. No browser token storage,
new authentication scheme, backend changes or new frontend packages are added.

## Where to edit

- src/components/notifications/: dropdown, rows, detail dialog and state messages.
- src/pages/NotificationsPage.tsx: full inbox layout and filters.
- src/pages/NotificationSettingsPage.tsx: channel controls and save feedback.
- src/styles/notifications.css: spacing, responsive sizing, notification colors
  and heading typography. Shared brand/fonts still come from index.css.
- src/notifications/: shared types, request lifecycle and provider state.
- src/services/notificationService.ts: production service boundary, currently
  unavailable until the browser BFF contract and persistent runtime are ready.
- src/dev/NotificationsPreview.tsx: separate development-only sample screen.

Data flow: protected session -> owner-keyed NotificationProvider -> service methods
-> controller state -> bell/inbox/details. Settings load/save through the same
service boundary. Replaced, unmounted and paginated requests are cancelled or
ignored when stale. Message text is rendered as React text, never injected HTML.

## Honest integration boundary

The default production service currently reports unavailable and sends no guessed
HTTP requests. No sample data is installed into the signed-in UI. The bearer-only
GA-270 handlers are not a browser BFF; do not expose access tokens to make them
work. Replace the service with an approved HttpOnly-cookie/CSRF BFF adapter when
its endpoints are available. Validate response shapes, timestamps, page bounds,
error/status behavior and cursor progression at that boundary.

The service interface matches list/count/read/channel operations. Rendering does
not supply issuer/subject selectors. Backend owner isolation, persisted read state,
preference defaults and mandatory-security delivery remain backend responsibilities.
No bulk mark-read, request/payment categories, announcements or arbitrary action
links are added. English-only wording follows the agreed current sprint scope.

## Preview and checks

With the normal Vite development server, open /preview/notifications. The small footer
note identifies sample data without a development toolbar. The preview supports
loaded, empty, loading and unavailable states via ?state=ready|empty|loading|error. Preview data resets on reload or
state changes and does not call authentication, inbox or email APIs. Production
returns Page not found for this path and excludes the sample service from assets.

    npm run build
    npm run lint
    node scripts/check-notifications.mjs
    node scripts/check-auth-state.mjs

The standalone notification checks cover fixture read/count consistency,
pagination, preferences, cancellation, no-network behavior and safe return routes.
Local browser checks/screenshots are retained under ignored
.local/notification-preview/. They use generated sample data and block external
API requests. They are not evidence of live authentication or notification flow.

## Handoff

Live acceptance belongs to the assigned teammates after the real adapters and
runtime are connected. Check two citizens, logout/re-login, stale request cleanup,
read/count consistency, preference opt-out and mandatory alerts, pagination,
expired sessions, CSRF, database outages and responsive/keyboard use. The separate
native mobile app and custom admin monitoring dashboard are outside this change.

## Updated shared shell

The sidebar, account footer, slim theme bar and shared shell styles are selectively
reconciled with origin/main at 63a39a7. This is a focused frontend reconciliation,
not a merge of unrelated chat/API/backend work. The notification bell sits beside
the profile and settings gear at bottom-left; on narrow browsers it is inside the
navigation drawer. The popover opens upward when needed and fits the viewport.
This increment stays light. Dark mode is deferred at the user's request.

The sign-in alert is titled Successful sign-in. It reports confirmed account
access, not a new user or simultaneous devices. The underlying
citizen.login.succeeded event type, priority, publishing and auth rules are
unchanged. Single-session enforcement remains an authentication responsibility.
