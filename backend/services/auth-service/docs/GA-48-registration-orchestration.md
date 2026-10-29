# GA-48 isolated registration orchestration

Subsequent evidence: the [combined integration test](GA-48-combined-integration.md)
now runs this service through the real group-aware adapter in a disposable HTTPS
realm, with explicitly synthetic stores. The original unit evidence below remains
distinct; PostgreSQL durability, distributed concurrency and restart recovery are
not established by the combined test.

Implemented 2026-10-30 as a library only. No public routes, runtime in-memory
stores, database provisioning, migrations or background workers were added.
The new state machine is tested with synthetic doubles, not a real database.
Historical permission-isolation FAIL_UNSAFE tests and the accepted prototype
group-wide administrative authority remain unchanged.

## Files and entry points

- `internal/registration/persistence.go`: password-free ProfileStore,
  RegistrationJournal and exclusive OperationSession contracts plus state DTOs.
- `internal/registration/service.go`: constructor-injected orchestration using
  the existing strict ParseRequest and Admin interface implemented by the existing
  Keycloak transport. No duplicate HTTP/credential implementation.
- `internal/registration/service_test.go`: test-only stores, identity double,
  concurrency controls and before/after-commit fault injection.

`NewService(config, admin, profiles, journal)` requires a fixed HTTPS issuer,
server-only fingerprint key (at least 32 random bytes), bounded workflow timeout,
email retry backoff and maximum attempts. No configuration or store is wired into
the runtime. Dependencies must be concrete non-nil implementations honoring their
context/deadline and ownership contracts.

`Register(ctx, idempotencyKey, reader)` reuses the bounded strict registration
parser. Key syntax is UUID-shaped and canonicalized to lowercase; the future HTTP
adapter must require random client-generated keys, rate limits and body read
deadlines. A context cannot forcibly interrupt an arbitrary io.Reader: the HTTP
transport must bound/close body reads on cancellation. No untracked goroutine is
started to read a body.

`Resume(ctx, key)` is for a trusted internal recovery worker, never an unauthenticated
status/recovery route. It loads only password-free pending data. A reserved operation
that has not attempted creation requires credential resubmission via Register.
No worker can reconstruct a password, and no background worker is implemented.

Both methods return only a generic `received` receipt on completion or duplicate
conflict, or fixed safe error codes. No operation ID, subject, NIC, email, raw
dependency errors, tokens or credentials are returned. These internal outcomes do
not define new HTTP routes or replace the reviewed public privacy/error contract.

## Durable state machine contract

| Phase | Next action and failure behavior |
| --- | --- |
| reserved | Password supplied only by the current validated request. Commit creating intent before contacting Keycloak. |
| creating | On the initiating call, create one disabled/unverified identity using the server-generated marker. On any later call, only FindOwned(marker); never repeat create blindly. |
| subject_recorded | Read back exact owned identity; persist restricted profile plus NIC binding idempotently. No enabling until this commit succeeds. |
| profile_ready | Commit enabling intent. |
| enabling | Read owned state. If already enabled, advance; otherwise enable through the ownership-checking adapter. Failure retains this phase; retry reads back before another enable. |
| enabled | Read owned state; already verified completes without requesting mail. Otherwise honor backoff/attempt cap and commit email_sending before the request. |
| email_sending | On the initiating call, request Keycloak verification mail once. On recovery, verified email resolves the outstanding need; otherwise require reconciliation/operator handling, never an automatic resend of an uncertain send. |
| completed | Replay returns the same generic receipt, with no identity/profile/enable/email effects. EmailState distinguishes request_accepted from already_verified. |
| duplicate | Generic receipt only; no lookup/adoption, credential replacement, profile linking or email to an existing account. |
| rejected | A definite, non-ambiguous policy rejection is terminal. No password reset or silent credential update on replay. |
| attention | Retry cap reached; expose operationally for reviewed handling. No automatic further sends. |

Every checkpoint is committed before its dependent effect. Subject is recorded
before profile persistence. A crash after identity creation but before subject
recording leaves creating: recovery uses only an exact protected marker match.
Zero matches, multiple/mismatched markers, or an unexpectedly enabled identity
remain unresolved. An empty search is not proof that creation never occurred.
The adapter, not email/NIC lookup, establishes marker ownership.

After an ambiguous create, invalid Location, untyped create error, or a create
policy rejection marked UnknownOutcome, the service leaves creating. The existing
transport conservatively marks create 400 as uncertain; a successful isolated
password-policy test does not weaken that rule. No blind retry or automatic
compensating delete/release occurs.

Only an adapter outcome explicitly identifying definite non-creation (for example
token acquisition failure or rejected authorization before user mutation) permits
returning to reserved. A subsequent Register can resubmit transient credentials.
The first actual successful creation determines the identity credential. After
creation/uncertainty, a changed password on same-key replay is ignored, never used
to overwrite an identity. Password comparison is deliberately impossible: there
is no persisted password or password-derived digest. Earlier draft wording that
implied detecting a changed password before creation is not an implemented guarantee.

Verification request success records request_accepted, not inbox delivery or email
verification. A request may skip email when readback already shows verification.
Definite failures before email acceptance return to enabled, preserving an attempt
count and a bounded linear backoff (attempt number times configured backoff).
UnknownOutcome/untyped failures and crashes while email_sending stay uncertain.
No SMTP delivery oracle or exactly-once guarantee exists. Explicit operator resend
policy and provider-side deduplication, if needed, are future work, not a hidden
automatic retry. Service-account/group privileges retain the accepted prototype
limitation; they do not become public-user authorization.

## Database teammate: exact atomicity and constraints

### Reservation and normalized NIC claim

RegistrationJournal.Reserve must perform one transaction that:

1. Establishes unique `(issuer, idempotency_key)` ownership and a server-generated
   operation ID, immutable issuer/key/fingerprint and encrypted pending profile data.
2. Atomically claims the parsed NIC's **NormalizedNumber** in the shared GovAI
   citizen namespace, including pending/completed/uncertain claims, regardless of
   email or Keycloak subject/issuer. Use database uniqueness, not check-then-insert.
3. Commits reserved before returning an exclusive operation session.

Same key/fingerprint returns the original operation and original pending data;
the newly proposed operation ID must never replace it. Different canonical input
under the same key returns ErrRequestConflict. Another operation's claim on the
same normalized NIC returns ErrNICConflict; the service returns a generic receipt
without creating an identity. Do not query/adopt an account by matching NIC/email.

Normalization is exactly the existing approved parser contract: uppercase only an
old NIC's V/X suffix, preserve digits/leading zeros, keep V and X distinct, and do
not map old/new numbers to one person. Pending claims do not expire merely because
a request timed out, SMTP failed or a lease ended. Cleanup/reuse/retention requires
an explicit reviewed policy; the service never deletes or releases claims.

The HMAC request fingerprint covers only fixed issuer, full name, normalized NIC,
canonical international phone and canonical email. It deliberately excludes the
password and does not serve as the NIC unique index: different emails with the
same number must still conflict. If sensitive NIC indexes use a keyed digest,
derive it separately from the exact normalized number, with managed secret keys.
Unsalted hashes are unsuitable. Key rotation must preserve uniqueness across old
and new keys; do not create separate uniqueness domains. Request-fingerprint key
rotation also needs a versioned comparison/migration contract before changing keys.
No key-rotation adapter is implemented.

### Exclusive session and committed checkpoints

OperationSession must serialize operations across processes/instances for the
entire workflow. A dedicated session/advisory lock is one possible design; do not
hold an open SQL transaction across HTTP calls. Every Save commits independently,
using revision CAS and enforcing immutable reservation fields, one-time subject
binding and allowed state transitions. Snapshot advances only after commit.

If lock/connection ownership is lost, Session.Context must be canceled and all
further commits rejected. The service combines that cancellation with the request
deadline. Never let a simple expired lease admit two active owners that both issue
mutations; Keycloak does not honor a database fencing token. Creating/email_sending
intent checkpoints protect against takeover replay, but do not authorize stale
owners to continue. Close must release resources using the bounded cleanup context,
even after the request was canceled. Use bounded SQL pools/deadlines and independent
connections for session ownership and short transactional work where required.

Required transition graph: reserved→creating; creating→subject_recorded, duplicate,
rejected or reserved only on a definite non-creation outcome; subject_recorded→
profile_ready→enabling→enabled; enabled→email_sending, completed or attention;
email_sending→completed, or enabled only on definite non-acceptance. Never regress
completed/duplicate/rejected/attention. A save timeout may have committed: reload
under a new exclusive session rather than trusting the old in-memory snapshot.
Maintain server timestamps/operational monitoring for stuck phases and attention
states without logging personal data or request/response bodies.

### Profile persistence and owner binding

ProfileStore.Persist must atomically verify the existing operation/NIC claim and
record the profile plus its unique `(issuer, subject)`/normalized-NIC binding.
It must verify the subject already recorded by that operation and reject a
different owner or different data with ErrProfileConflict; no upsert that overwrites
an existing profile. Unique constraints must cover issuer+subject, normalized NIC,
and one binding per operation. Original NIC/name/phone are restricted encrypted
application data, not tokens, logs or queues. Profile records contain no email,
password, access/refresh token, reset link or administrative credential.

The journal profile_ready checkpoint follows the committed profile transaction.
These are intentionally separate commits: if the checkpoint fails, repeat the
exact idempotent profile write on recovery, keeping the account disabled until the
checkpoint succeeds. The prior subject_recorded journal already supplies immutable
operation/identity linkage; the profile transaction must verify it. If the teammate
also implements the earlier NICReservations.Bind port, its profile/linkage work
must use that same owner-checked transaction, not an independent non-atomic bind.
Do not implement separate NIC reservation then journal insertion calls: the new
combined journal boundary carries the earlier Reserve atomicity obligation.

No transaction spans the application database, Keycloak and SMTP. Recover forward;
do not claim rollback of an identity operation after SQL/email failure. Store no
credentials to make replay easier. Read-only marker recovery is acceptable; ambiguous
creation/email states require explicit reconciliation, not scheduled blind POSTs.

## Evidence and required real database tests

Synthetic tests cover strict validation/override rejection, same-key replay with
changed password and canonical NIC/email forms, changed-input conflict, normalized
NIC collision, simultaneous same-key requests, cancellation while waiting/after
creation and ownership loss, reservation/profile/admin failures, every checkpoint
before/after-commit failure, exact-marker reconciliation, duplicate non-adoption,
enable readback, verified-email skip, definite-send backoff/cap and uncertain-send
quarantine. DTO formatting/JSON/structured logging redact values; DTOs/ports contain
no Request/password/token field and the fingerprint is password-independent.

The mutex/channel fakes are **not** production persistence or database concurrency
evidence. They assume a single configured issuer and model only the contract paths
needed for these tests. Disposable PostgreSQL integration still must demonstrate:

- Same normalized NIC, different email/key, independent processes/connections:
  exactly one durable claim and at most one authorized create operation.
- Same-key retry/concurrent input conflict; lowercase/uppercase suffix collision;
  V/X and different old/new numbers remain separate; no digit conversion.
- Atomic reservation+journal rollback; owner binding/profile rollback; duplicate
  subject/operation conflicts; no orphan claim or overwritten profile.
- Process kill and database connection loss at every checkpoint, including uncertain
  commits, session takeover and cancellation of the prior owner; no create replay.
- Recovery after restart with no original request/password; encrypted pending data,
  stable HMAC keys/rotation, durable retry schedules/caps and retained uncertain claims.

No live Keycloak, SMTP or database integration was rerun for this orchestration
slice. Prior lifecycle evidence covers the isolated group-bearing harness, **not
this new service plus real persistence**. The runtime Keycloak create payload now
supports the required [server-configured group](GA-48-group-adapter-config.md).
Combined live integration with this service/durable stores remains pending. No new
runtime in-memory store, worker, route, migration, commit or push was added.

Final checks completed from the auth-service module on 2026-10-30: gofmt,
`go test ./...`, `go vet ./...`, `go test -race ./...` and
`git diff --check` passed. Full-module tests include the existing adapter and
validation suites. No live Keycloak, SMTP, PostgreSQL or real-client check was
run for this change, and no production persistence guarantee is claimed.
