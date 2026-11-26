# Notification service foundation (GA-267)

This checkpoint provides process startup/shutdown, validated configuration,
Go/Gin health endpoints and a reconnecting AMQP connection with separate security
and normal channels. No event is consumed, acknowledged or published.

## Structure and data flow

- cmd/api/main.go: local environment loading and operating-system shutdown signal.
- internal/config/: validate settings and safely encode broker credentials.
- internal/app/: connect dependencies, start HTTP and drain on shutdown.
- internal/handlers/: health endpoints only.
- internal/messaging/: AMQP lifecycle, separate channels and bounded reconnects.
- Dockerfile: dependency, validation, build and non-root runtime stages.
- .env.example: documented local settings without working credentials.

No database schema/migrations are introduced: GA-208 belongs to the database owner.
Business processing, templates, inbox APIs and delivery folders will be added when
their implementation begins instead of creating empty placeholders now.

Flow: main -> configuration -> app -> broker supervisor + health HTTP.
The supervisor authenticates as the notification consumer, opens two channels,
sets independent prefetch limits and reconnects if a channel/connection closes.
It deliberately never calls Consume, Get, Ack or Nack.

A connected channel is not an active worker. Reserved security worker capacity
and priority-aware email dispatch remain to be implemented with durable storage.
This service does not perform login, reset passwords or send verification emails.

## Run locally with Docker (PowerShell, repository root)

Start the existing broker first:

    docker compose -f infrastructure/rabbitmq/compose.yaml up -d --wait rabbitmq

Compose loads the ignored infrastructure/rabbitmq/.env created by
setup.ps1 for interpolation. Only consumer credentials enter the container;
administrator credentials are not
needed by the service. Start with both files and the explicit profile:

    $composeFiles = @(
        '-f', 'infrastructure/rabbitmq/compose.yaml',
        '-f', 'infrastructure/rabbitmq/compose.notification.yaml'
    )
    $credentials = 'infrastructure/rabbitmq/.env'
    $startup = @(
        '--env-file', $credentials
    ) + $composeFiles + @(
        '--profile', 'notifications', 'up', '-d', '--build', '--wait'
    )
    docker compose @startup

Only broker and notification-service are included. Ports 5003, 5672 and 15672 are
bound to localhost. Existing database/auth/mobile/AI services are not started.

    Invoke-RestMethod http://localhost:5003/health
    Invoke-RestMethod http://localhost:5003/api/health

- /health: 200 if HTTP is alive.
- /api/health: 200 when both broker channels are open; otherwise 503.
  Always includes processing_enabled: false and stage: foundation.
- /api/ready: intentionally 503, persistence_not_implemented, even with RabbitMQ up.
  The full pipeline cannot be marked ready until database integration exists.
- No citizen notification endpoints are mounted. JWT/cookie/CSRF integration
  belongs to GA-270 and must precede any authenticated data endpoint.

Container health checks use /health, not full pipeline readiness.
Foundation HTTP is for local/internal probes only. Add the existing HTTPS/auth
conventions before exposing citizen APIs.

## Local Go tools (optional)

The repository uses Go 1.26.0. If installed, run from this service folder:

    go test ./...
    go vet ./...
    go run ./cmd/api

For direct Windows execution, copy .env.example to .env and populate the local
consumer password privately from infrastructure/rabbitmq/.env. Leave broker host
127.0.0.1 and HTTP bind 127.0.0.1. Process environment overrides .env.

## Container checks (repository root)

    docker build --target checks -f backend/services/notification-service/Dockerfile backend

The checks stage verifies gofmt, tests configuration boundaries and health
semantics, runs go test and go vet. The actual non-root runtime is built separately.

## Recovery and shutdown checks

With the foundation running, stop ONLY RabbitMQ. /health should stay 200 while
/api/health changes to 503. Restart RabbitMQ and wait for /api/health to recover.
Consumers must remain zero and no queued event should be removed.

Stop notification-service using the same Compose files; its signal handler drains
HTTP and closes both channels and the AMQP connection. Do not use down -v.

## Configuration and secrets

RABBITMQ_SCHEME defaults to amqps; production TLS uses system trust or
RABBITMQ_CA_FILE. Plain AMQP is accepted only with NOTIFICATION_ENV=development
and a loopback host or the explicit development service name rabbitmq.
The pinned local broker is plaintext/localhost-only; this is not production TLS.

Configuration errors and AMQP failures use generic messages. Do not log the
configuration struct, credential URI or passwords.

## Remaining acceptance work

- GA-208 database contract and migrations supplied by the database owner.
- Versioned trusted events, atomic inbox/deduplication save before manual ACK.
- Durable auth outbox, actual success checkpoints and failed-attempt alert policy.
- Email preferences, recipient lookup, English templates and SMTP delivery records.
- Three-attempt failure policy, unknown acceptance handling and DLQ integration.
- Verified identity, owner-scoped APIs, web BFF and mobile integration.
- Independent worker capacity for security events and email delivery.

## Foundation verification recorded 21 November 2026

- Formatting, configuration/health unit tests and go vet passed in the Docker
  checks stage. The Linux binary compiled and the non-root runtime started.
- /health returned 200; /api/health showed broker_connected=true and
  processing_enabled=false; /api/ready correctly returned 503.
- Management API confirmed one AMQP connection authenticated as the consumer,
  two open channels, and no active consumers. Admin/publisher secrets were absent
  from the notification-service container environment.
- Stopping RabbitMQ kept HTTP alive and changed broker health to 503.
  Restarting the broker restored connectivity automatically.
- All five queues remained empty with zero consumers after statistics refreshed.
  The immediate post-restart statistics assertion initially ran before RabbitMQ
  repopulated its metrics; the refreshed check passed.
- Notification-service stopped with exit code zero and restarted healthy.
- Compose validation and git diff --check passed.
- First runtime-image download failed with an incomplete transfer; retry succeeded.
- No account event processing, database, SMTP or citizen API acceptance is claimed.
  No commits or pushes were made.

## Event validation checkpoint

Shared event definitions and strict decoding are under backend/pkg/notificationcontract.
internal/validation/event.go additionally checks the queue and routing key against
the server-derived priority, including retry routes. See the shared package README
for the provisional event example and GA-208 transaction/storage handoff.

The Docker build includes the shared module and its tests. No consumer has been
enabled: malformed events are only exercised in unit tests, not dequeued from the
real broker. Persistence, real publisher integration, templates and SMTP remain
unfinished. This checkpoint does not complete GA-267.

## Processing checkpoint (test storage only)

internal/notifications/processor.go now validates one delivery, renders through a
template interface, derives its priority, calls Store.Save, and acknowledges only
Saved or a confirmed Duplicate result. Storage, rendering and ACK failures return
safe errors; this processor does not blindly requeue or send SMTP.

types.go defines the notification record and the storage/template boundaries.
The production PostgreSQL Store must atomically commit deduplication, inbox and
eligible pending email state; it returns Duplicate only for a matching event.
Conflicting event-ID reuse returns ErrConflict.

processor_test.go is the only location of temporary storage and synthetic renderer
implementations. Tests model save-before-ACK, lost-ACK redelivery, duplicate/conflict
handling, normal/security priority, cancellation, and dependency failures.
These tests prove processor behavior against the interface, not real transaction
durability, database concurrency, crash recovery or delivered email.

No temporary store/renderer is wired into internal/app. Live processing remains
disabled, /api/ready remains 503, and real RabbitMQ events are not consumed.
A real PostgreSQL adapter and approved English renderer must be supplied before
connecting the processor to live consumers. GA-267 remains incomplete.

To review the processing tests in the newly built checks image:

    docker run --rm govai-notification-checks:local go test -v ./internal/notifications

Processing verification (23 November 2026): current-source service tests, shared
contract tests and Go analysis passed in Docker with source mounted read-only.
The processor tests cover save-before-ACK ordering, matching redelivery, conflict
rejection, safe errors and cancellation. Line-length and comment-block spacing
review passed. This evidence is against test storage, not PostgreSQL transactions.
The checks-image rebuild is tracked separately from source test results.

## SMTP adapter checkpoint (GA-267)

internal/delivery/ contains transport configuration, safe MIME encoding and the
SMTP adapter. internal/config/smtp.go validates optional environment settings.
No live delivery worker is wired into internal/app yet. EMAIL_ENABLED defaults to
false, and setting it does not activate sending in the current foundation.

The adapter accepts a trusted recipient, rendered plain-text subject/body and a
stable UUID delivery identifier. It sends one message per call with no internal
retry loop. Header injection, invalid addresses, malformed UTF-8 and oversized
content are rejected before connecting. Bodies use quoted-printable encoding;
subjects use MIME encoding when needed, and Message-ID uses the stable delivery ID.
SMTP requires a terminating line break, which is not part of template wording.

Supported transports:
- starttls: require STARTTLS, validate the certificate/hostname and require TLS 1.2+.
- tls: verified implicit TLS, with the same minimum TLS version.
- plain: test/development only, literal loopback address and no credentials.

No TLS downgrade or certificate-validation bypass is provided. Third-party
providers use system certificate trust; tests inject their own private trust pool.
Credentials, recipient and message structs are redacted for fmt, slog and JSON.

Send returns safe status/code/retryability and a generic error:
- submitted: final positive SMTP DATA acceptance; not confirmed inbox delivery.
- failed: known rejection or failure before submission, with safe classification.
- unknown: acceptance could not be confirmed after DATA; do not blindly resend.

Explicit 4xx replies are retryable; explicit 5xx replies are permanent. TLS policy
failures, canceled requests and local validation errors are not retryable.
Once DATA acceptance is confirmed, a failed QUIT does not change submitted status.
Timeouts are bounded and cancellation closes the connection promptly.

GA-269 will persist attempt budgets and decide retry/DLQ behavior. An adapter call
does not update the inbox, delivery records, channel preferences or read state.
Trusted recipient lookup and durable job scheduling remain future integration.
No final English template copy was added; GA-271 remains separate.

Local SMTP fixture tests cover STARTTLS, implicit TLS, authenticated submission,
untrusted certificates, missing STARTTLS, SMTP 4xx/5xx errors, lost acceptance,
lost QUIT, blocked connections, cancellation, MIME content and header injection.
They send synthetic content only to an in-process server inside the test container.
At this checkpoint, Mailtrap submission had not yet run; the later verification
record below documents the completed sandbox check.

To review adapter tests in the rebuilt checks image:

    docker run --rm govai-notification-checks:local go test -v ./internal/delivery

SMTP behavior references:
- https://pkg.go.dev/net/smtp
- https://www.rfc-editor.org/rfc/rfc5321

## Manual Mailtrap sandbox check

cmd/smtp-check is a separate diagnostic executable, not a citizen API, consumer
or delivery worker. It sends no message by default. It accepts only the exact
sandbox.smtp.mailtrap.io host with TLS and explicit -send. Live SMTP hosts are
rejected. It always uses synthetic recipient/content and never reads account data.

From the repository root, create the ignored local settings file:

    .\backend\services\notification-service\scripts\setup-smtp-sandbox.ps1

Open backend/services/notification-service/.env.smtp-sandbox in your editor.
Use the Mailtrap Email Sandbox Integration tab to populate SMTP_USERNAME and
SMTP_PASSWORD privately. Verify host/port/TLS against that tab. Keep the suggested
synthetic sender for sandbox use, and set EMAIL_ENABLED=true when ready.
Do not use live-provider credentials and do not paste credentials into chat.

With Docker Desktop running, validate settings without sending:

    $sandboxSettings = 'backend/services/notification-service/.env.smtp-sandbox'
    $smtpCommand = @(
        'run', '--rm', '--env-file', $sandboxSettings,
        'govai-notification-checks:local', 'go', 'run', './cmd/smtp-check'
    )
    docker @smtpCommand

Only after validation succeeds, explicitly submit one synthetic message:

    docker @smtpCommand -send

This command uses the rebuilt checks image. Rebuild that image after code changes
using the Docker checks command above. SMTP credentials are supplied at container
startup and are not built into the image. They are still visible to local Docker
administrators, so use sandbox-only credentials.

A successful command prints submitted/smtp_accepted and a synthetic Message-ID.
Verify the captured subject/body in Mailtrap's sandbox and record that result.
An unknown result must be investigated before resending; the command never retries.
Do not interpret synthetic SMTP submission as database/event/inbox integration.

The setup helper preserves an existing local settings file. The shareable
smtp-sandbox.env.example contains placeholders; .env.smtp-sandbox is ignored.
Enabling this separate test file does not enable the running notification worker.
No account notifications or real citizen email are involved in this check.

Mailtrap instructions:
https://docs.mailtrap.io/email-sandbox/setup/sandbox-smtp-integration

## PostgreSQL connection checkpoint (GA-267)

internal/external/postgres/ contains validated connection settings, a bounded pgx
pool, startup connectivity probe, health probe and pool shutdown. The pinned pgx
version matches auth-service. Configuration formatting, structured logging and
JSON serialization redact credentials; connection errors remain generic.

internal/config/postgres.go loads separate NOTIFICATION_DB_* settings only when
NOTIFICATION_DATABASE_ENABLED=true. This loader is not yet called by internal/app:
the foundation still reports processing_enabled=false and /api/ready stays 503.

Only govai_notifications is accepted. Production connections require verify-full
TLS with hostname/certificate verification and TLS 1.2 or newer. Plain PostgreSQL
is allowed only in explicit development mode on loopback or the dedicated local
notification-postgres hostname. Pool size is bounded to 1-32 and operation/connect
timeout to 100ms-30s. A failed startup probe closes its pool.

This checkpoint does not create schemas, roles, migrations or service databases.
No INSERT, event consumption, preference lookup or email dispatch is implemented.
The GA-208 owner must provide the actual migration files before Store.Save SQL can
be written and tested. Agree exact identity columns, event conflict comparison,
atomic inbox/deduplication/pending-delivery behavior and preference defaults.

Keep NOTIFICATION_DATABASE_ENABLED=false until deliberate integration. A successful
database Ping proves connectivity only, not migration correctness, least-privilege
grants, durable notifications, cross-citizen isolation or full pipeline readiness.

Connection integration tests are opt-in via NOTIFICATION_TEST_DB_PASSWORD and are
intended only for an isolated disposable PostgreSQL fixture on loopback:5432 with
database govai_notifications and role notification_test. Never supply real service
credentials to that test. It checks identity, Ping, bad credentials and closed-pool
failure; it does not test the future notification tables or their migrations.

Mailtrap manual verification (24 November 2026): configuration validation passed,
one synthetic SMTP submission was accepted, and the user confirmed the expected
message body in the sandbox. This is adapter evidence, not event/database flow.

pgx pool reference: https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool

Connection verification (24 November 2026): full Go tests, Go analysis, formatting
and both checks/runtime Docker builds passed. An isolated PostgreSQL 16.14 server
verified connection identity, health probing, wrong-password rejection and closed
pool errors. The initial fixture trusted loopback; enforcing SCRAM authentication
fixed that fixture and the rerun passed. No notification tables were created.

To repeat after rebuilding the checks image, run from the repository root:

    .\backend\services\notification-service\scripts\test-postgres.ps1

The helper uses a generated password and RAM-backed disposable storage, publishes
no host port, removes its own test container and restores process environment.
It does not read local service secrets or modify the project database containers.
## GA-269: email retry policy checkpoint

internal/delivery/policy.go evaluates the SMTP adapter's result without sending,
consuming messages or changing database state. Three total SMTP attempts are
allowed (initial attempt plus two retries), following the current implementation
plan. Temporary failures schedule the next attempt 30 seconds later in UTC.
Permanent SMTP rejections, invalid messages and unsafe transport/authentication
configuration stop immediately. Successful SMTP acceptance remains submitted;
it does not mean the recipient received or read the email.

A lost final DATA response has an unknown outcome. It requires review and never
automatically authorizes another SMTP submission. Cancellation is also held for
review. Contradictory results, unsupported error codes and invalid attempt counts
fail closed with a generic error; provider text is never copied into decisions.

The three-attempt budget is distinct from RabbitMQ's poison-message delivery
limit. Queue redelivery is not evidence that another SMTP attempt occurred.

Before this policy can be enabled, GA-208 integration must provide:

- Atomic delivery claiming with a per-attempt lease/fence and a persisted count
  reserved before SMTP starts, shared across restarts and concurrent workers.
- Safe recovery of interrupted submissions: an expired in-flight attempt cannot
  simply be resent, because SMTP may already have accepted it.
- Persisted submitted/failed/unknown outcomes and retry-due timestamps, independent
  of the inbox row's read status. Preferences must be checked before sending.
- A durable dead-letter publication intent for terminal failures, with only safe
  delivery/event identifiers and an allowlisted reason, never recipient or body.
- A mandatory, publisher-confirmed DLQ dispatcher; failed/uncertain publication
  keeps that intent pending with a stable ID for safe broker redelivery.
- Separate security/normal worker capacity, plus real PostgreSQL and isolated
  RabbitMQ tests for concurrency, restart recovery, exhaustion and DLQ visibility.

Dead is a decision to persist a dead-letter intent, not proof of broker delivery.
Review is a worker action; it does not introduce a new database status. Map it to
unknown/manual investigation in the agreed schema. Do not delete an inbox message
or reset its read state when email delivery fails.

This checkpoint adds policy and tests only. The application still has
processing_enabled=false. No delivery table migrations, live workers, automatic
retries, DLQ publications or dashboard changes are enabled. GA-269 remains open.

To inspect the policy tests after rebuilding govai-notification-checks:local:

    docker run --rm govai-notification-checks:local go test -v ./internal/delivery

## GA-271: English account templates

The templates/ directory now contains a validated renderer and three embedded
English catalogs, with separate email and in-app wording for successful login,
password change and confirmed email verification. Body lines are kept short for
review. See templates/README.md for editing, scope and acceptance details.

The renderer validates the trusted issuer and event envelope, uses fixed text,
and never includes event identity references or dynamic links. Tests exercise
all three events, invalid envelopes, unsafe text, the real inbox processor with
an in-memory store, and the SMTP MIME encoder without network connections.

This is an implementation checkpoint awaiting human wording review. It does not
wire the foundation API to a consumer or delivery worker. Persistent processing,
preferences and real queue/email integration still depend on the storage work.
No Sinhala/Tamil translations or native-speaker approvals are claimed.

## GA-270: citizen API implementation checkpoint

Authenticated handlers are prepared for listing, marking read, unread counts and
atomic email/in-app preference replacement. They reuse the shared authn JWT/TLS
verifier and pass issuer+subject to owner-scoped storage interfaces. Input limits,
strict JSON, keyset pagination, security inbox policy and local authentication/
owner-propagation tests are included. See [API handoff](API.md)
for request/response behavior, SQL requirements and teammate live-test steps.

The foundation application does not register these routes yet. GA-208 adapters,
API JWT/TLS runtime configuration and the browser BFF proxy remain integration
work. No in-memory store is installed into the application and readiness remains
false. The tests establish API behavior against local fixtures, not persistent
SQL isolation or live acceptance. No new private configuration is required here.
