# GA-48 registration runtime wiring

Current workstation update: [local HTTPS setup](../../../dev/keycloak/HTTPS-SETUP.md)
is configured and the actual API smoke passes. Registration remains disabled by
default in committed examples; local activation/secrets live only in ignored files.

`cmd/api` can now compose the real PostgreSQL stores, group-aware Keycloak
adapter, orchestration, admission limits and HTTP handler. Registration remains
**disabled by default**. Existing health/database readiness development works
without Keycloak settings or TLS listener changes.

When explicitly enabled, the route is:

```text
POST https://<api-host>:<PORT>/api/v1/registrations
Content-Type: application/json
Idempotency-Key: <random client-generated UUID retained across retries>
```

Request fields and errors remain in [the HTTP contract](GA-48-registration-http.md).
The API returns the same generic 202 for accepted requests and duplicates. Never
interpret 202 as proof that a new account was created or email verified.

## Enable prerequisites

1. The [authentication PostgreSQL database](../../../dev/auth-postgres/README.md)
   is migrated and the restricted runtime URL, encryption key and NIC index key
   are available. Keep migration credentials out of the API environment.
2. Keycloak has a validated **HTTPS** address. The current
   `http://localhost:8180` environment is insufficient for this runtime adapter.
   Use a server certificate matching the host and a trusted CA, never a TLS bypass.
3. In the intended realm, provision the reviewed fixed registration group,
   protected operation-marker attribute, default citizen role, verified-email
   policy and scoped administrative client. The accepted prototype permission
   is group-wide administrative authority, not create-only isolation. Existing
   experiment results and limitations remain unchanged.
4. Configure SMTP and an actual verification OIDC client with an exact approved
   HTTPS callback. The disposable test callback is not your application's callback.
5. Disable or otherwise prevent hosted registration bypassing required profile
   creation before accepting real application registrations. This code does not
   rewrite realm settings automatically.
6. Provide the Go API's own certificate/key for **direct HTTPS**. This version
   does not trust TLS termination headers or forwarded client IPs. Reverse-proxy
   termination and multi-instance abuse controls need separate deployment work.

## Private configuration

From the auth-service directory in PowerShell:

```powershell
.\scripts\new-registration-env.ps1
notepad .env.registration
```

The script creates independent random 32-byte fingerprint/abuse HMAC keys and blank
connection/client/certificate fields. It refuses to overwrite an existing file,
never prints secrets, and leaves registration disabled. Do not rerun it to rotate
keys after durable operations exist. This file is ignored by Git.

Fill the following settings with your reviewed local/deployment values:

| Setting | Meaning |
| --- | --- |
| `REGISTRATION_ENABLED=true` | Explicit activation; use only after completing all settings |
| `REGISTRATION_KEYCLOAK_URL` | HTTPS server base URL, optional fixed deployment prefix, no trailing slash |
| `REGISTRATION_REALM` | Exact case-sensitive realm name, e.g. GovAI |
| `REGISTRATION_CLIENT_ID`, `REGISTRATION_CLIENT_SECRET` | Scoped service-account client, never a public login client |
| `REGISTRATION_GROUP_PATH` | Fixed reviewed top-level path, e.g. /ga48-registration |
| `REGISTRATION_VERIFICATION_CLIENT_ID` | Intended OIDC client for verification completion |
| `REGISTRATION_VERIFICATION_REDIRECT` | Exact HTTPS callback allowed by that client |
| `REGISTRATION_CA_FILE` | Optional PEM CA bundle for private Keycloak TLS; system roots otherwise |
| `API_TLS_CERT_FILE`, `API_TLS_KEY_FILE` | Go server certificate chain and matching private key |
| `REGISTRATION_FINGERPRINT_KEY` | Stable secret matching all instances and retained pending operations |
| `REGISTRATION_RATE_KEY` | Independent stable key for process-local abuse counters |

Do not reuse encryption, NIC-index, fingerprint or abuse keys. Preserve them in
restricted backups and secret storage. Fingerprint-key rotation requires a
coordinated plan for existing idempotency records; it is not implemented here.
The key-file generator does not provision certificates, clients, groups or SMTP.

Precedence: process environment, `.env.registration`, `.env.database`, then `.env`.
The migration-only `.env.database-admin` is never loaded by the API.
Invalid enabled configuration fails startup with a fixed safe error. Missing
database, HTTP Keycloak URL, bad group/callback, malformed/shared HMAC keys,
unreadable CA or invalid listener certificates do not silently enable a partial
route. No startup registration or administrative mutation occurs.

## Start and call

Stop the previous Go process before restarting on the same port. Port 5000 on
this workstation was observed serving another application; 5001 has been used
for the Go service.

```powershell
$env:PORT = '5001'
go run ./cmd/api
```

With registration enabled, use **https://localhost:5001/api/ready** and a client
that trusts the API certificate. Do not pass `-SkipCertificateCheck` or disable
Postman certificate verification. GET on the registration path is not a signup
form; registration is a JSON POST from the custom app/client.

The response contains only a generic receipt or safe error. Send sample data only
in local tests and do not share real passwords, tokens, administrative secrets or
NICs in logs/screenshots. Full web/mobile integration remains teammate work.

## Transport, timeouts and abuse policy

- Direct TLS is required both at the listener and registration route. A request
  without `Request.TLS` gets 426 before body parsing/service calls, even if
  X-Forwarded-Proto claims HTTPS. Trusted proxies remain disabled.
- CORS permits POST + Idempotency-Key only for the enabled registration path and
  existing exact origin allowlist. Health endpoints retain GET-only preflight.
- Keycloak calls: 5 seconds; orchestration: 25 seconds; handler: 28 seconds;
  server body/header reads: 10/5 seconds; enabled server writes: 45 seconds.
  Enabled graceful shutdown allows 30 seconds; health-only mode retains 5 seconds.
- Admission starts at IP 5/15 minutes, canonical email 3/hour, 4096 entries per
  map and 4 simultaneous requests, aligned with journal-session capacity.
- These limits are process-local, reset on restart and aggregate NAT/proxy users.
  Replicas/restarts do not share counters. Generic receipts do not prove timing
  resistance to enumeration. Distributed/edge enforcement remains pending.
- Verification links are requested with a 15-minute lifespan. This is not an
  access/refresh token lifetime or a login implementation.

## Evidence and scope

Configuration and router tests exercise explicit disablement, incomplete settings,
secret redaction, direct TLS enforcement, rejected forwarding spoofing and exact
CORS path/header rules. The opt-in [real database/Keycloak test](GA-48-postgresql-keycloak-integration.md)
also exercises the same application constructor/router with a trusted private
test listener and CA. It does not deploy settings to your existing GovAI realm
or prove a real browser/mobile signup.

The 2026-10-31 live runtime-composition run passed, including real profile commit
and account creation through this constructor/router. Full unit tests, vet,
race tests and diff checks passed separately; ordinary runs skip live infrastructure.
JWT/RBAC, login, refresh/logout, automated uncertain-operation reconciliation,
full crash/restore tests and actual client integration remain separate work.
