# Current local Keycloak development environment

This environment uses synthetic test identities only. Keycloak runs at
http://localhost:8180; the realm identifier is **GovAI**, case-sensitive. The Admin
Console is http://localhost:8180/admin/ and the built-in account console is
http://localhost:8180/realms/GovAI/account/ . Do not substitute lowercase `govai`.

## Configuration recorded on 2026-10-27

| Setting | Current value |
| --- | --- |
| Keycloak image | quay.io/keycloak/keycloak:26.8.1 |
| Mode/database | start-dev, built-in dev-file |
| Published Keycloak port | 127.0.0.1:8180 -> container 8080 |
| Persistence | Named volume govai-keycloak-dev_keycloak_dev_data at /opt/keycloak/data |
| Realm | GovAI |
| Citizen authorization role | Realm role citizen; manually assigned to test users |
| Automatic default citizen role | Not configured |
| Minimum password length | 8; replaces the previous value of 15 |
| Access token lifespan | 5 minutes |
| SSO session idle / maximum | 30 minutes / 10 hours |
| Client session idle / maximum | 0 / 0; inherit the realm SSO values |
| Realm Verify email | Enabled |
| SMTP host / port | govai-mailpit / 1025 |
| Sender | noreply@govai.test (synthetic local sender) |
| SMTP SSL / STARTTLS / authentication | All disabled for this isolated local setup only |
| Mailpit inbox | http://localhost:8025 |

The operator confirmed rejection of a 7-character password and acceptance of an
8-character password followed by successful login. Configured token and session
lifetimes are not evidence that expiry, refresh or logout semantics have
been tested. The existing realm settings were configured through Keycloak; the
Compose file does not automatically recreate them on a fresh volume.

PostgreSQL integration is deferred to the database teammate. This setup does not
create or modify PostgreSQL databases, users, schemas or migrations. Dev-file is
local development storage, not PostgreSQL persistence or a production database.
The named volume contains identity data; use no real citizen information.

## Actual standalone Mailpit setup

Read-only Docker inspection on 2026-10-27 confirmed:

- Container name: govai-mailpit; image reference: axllent/mailpit:latest.
- Network: govai-keycloak-dev_default, shared with govai-keycloak-dev-keycloak-1.
  Keycloak reaches SMTP through Docker DNS as govai-mailpit:1025.
- Published inbox: 127.0.0.1:8025 -> 8025/tcp. SMTP port 1025 is not published
  to the host; it is reached on the Docker network.
- Restart policy: no. Mailpit must be started explicitly after it stops.
- No mounted storage was present. Durable inbox persistence is not configured;
  do not rely on retaining messages across restarts or container replacement.
- Mailpit was running and healthy. It is a standalone container, not a service
  in compose.yaml; Keycloak Compose commands do not start or stop it.

The latest image tag is mutable and does not identify an immutable Mailpit build.
This documents the existing setup; it does not change or repin it. Do not recreate
or delete the existing container to follow these instructions. If it is missing,
coordinate its restoration rather than assuming the Compose file provides it.

## Startup (PowerShell)

Start Docker Desktop with the Linux engine. From the repository root:

```powershell
cd backend/dev/keycloak
docker version
docker compose version
# First setup only, if .env does not already exist:
if (-not (Test-Path .env)) { Copy-Item .env.example .env }
notepad .env
```

Keep bootstrap admin credentials only in the ignored local .env. Choose local
synthetic credentials; do not reuse real passwords. Single-quote values containing
$ or # to avoid interpolation/comment interpretation. Process variables override
--env-file values. Never print full rendered Compose configuration or share .env.
Changing bootstrap values does not reset an admin already stored in the volume.

Before starting, inspect the existing project rather than replacing it:

```powershell
docker compose --env-file .env -f compose.yaml config --quiet
docker compose --env-file .env -f compose.yaml ps -a
docker ps -a --filter name=govai-mailpit --format '{{.Names}} {{.Image}} {{.Status}}'
docker compose --env-file .env -f compose.yaml up -d
docker start govai-mailpit
docker network inspect govai-keycloak-dev_default --format '{{range .Containers}}{{.Name}} {{end}}'
docker compose --env-file .env -f compose.yaml logs --tail 100 keycloak
```

Missing/empty bootstrap values intentionally fail configuration validation. Use
this explicit Compose file, not the root docker-compose.yml. Do not override the
project name govai-keycloak-dev or attach a different existing volume. No other
project services need to start. Mailpit should appear on the same network as
Keycloak; if it does not, investigate before testing mail delivery.

Open the Admin Console and select GovAI. Reuse the existing realm and users;
do not recreate or overwrite them. Verify the table above under Realm settings
(Login, Email, Tokens and Sessions), Authentication password policies, and each
test user's Role mapping. Assign the citizen realm role manually as needed;
never assign administrative roles. Do not mark email verified manually to bypass
the verification flow. SSL/STARTTLS/authentication being disabled applies only
to this local Mailpit setup, not deployed email services.

## Discovery and Go connectivity verification

From backend/dev/keycloak:

```powershell
$discovery = Invoke-RestMethod -TimeoutSec 5 http://localhost:8180/realms/GovAI/.well-known/openid-configuration
if ($discovery.issuer -cne 'http://localhost:8180/realms/GovAI') { throw 'Unexpected issuer' }
$discovery | Select-Object issuer, authorization_endpoint, token_endpoint, jwks_uri
```

Expect HTTP 200 and the exact case-sensitive issuer. For the Go check, from the
repository root in another terminal:

```powershell
cd backend/services/auth-service
$env:OIDC_ISSUER_URL = 'http://localhost:8180/realms/GovAI'
$env:OIDC_CHECK_TIMEOUT = '5s'
go run ./cmd/oidc-check
```

This bounded read-only check verifies discovery issuer and JWKS connectivity;
it does not validate JWTs or authenticate citizens. The public Go health endpoints
remain independent of Keycloak availability. Do not put bootstrap credentials in
the Go service's environment file.

## Synthetic-user login and email verification

1. Use existing synthetic test users, or create a separate synthetic user with a
   unique local test password satisfying current policy. Assign citizen manually.
   Do not add real email addresses, NICs, addresses or other personal information.
2. In a private browser window, open the GovAI account console. Sign in using
   the synthetic account. An unverified user must encounter email verification.
3. Open http://localhost:8025 locally, find the synthetic user's verification
   message, and complete its action in the browser. Never copy the message body,
   verification URL, password or tokens into source control, screenshots or tickets.
4. Confirm completion in the browser and the user's Email verified status in the
   Admin Console. Use a fresh browser session to confirm the resulting login.
5. For a second unverified synthetic user, first confirm the verification step is
   enforced. Record delivery and final completion separately; enforcement alone
   does not prove that the second user completed verification.

Do not bypass verification with an admin toggle. Do not enable password/direct
access grants to obtain tokens for a test. This uses Keycloak's built-in account
client; application/test-client coverage remains future work. Invalid/expired
verification links still need separate tests. Password boundary results are
recorded below.

## Persistence, logs, shutdown and resume

From backend/dev/keycloak, restart only Keycloak to repeat the persistence check:

```powershell
docker compose --env-file .env -f compose.yaml restart keycloak
docker compose --env-file .env -f compose.yaml ps
docker compose --env-file .env -f compose.yaml logs --tail 100 keycloak
```

Wait for startup, rerun discovery, then inspect GovAI, citizen, test users and
role mappings. Repeat synthetic login in a fresh browser session. The named volume
preserves local Keycloak data; do not infer inbox persistence from this result.

```powershell
# Follow Keycloak logs; Ctrl+C ends log following only.
docker compose --env-file .env -f compose.yaml logs -f keycloak
# Metadata-only inspection avoids dumping messages, credentials or tokens.
docker inspect govai-mailpit --format '{{.State.Status}} {{.Config.Image}}'
docker port govai-mailpit
# Stop each component without deleting containers or volumes.
docker compose --env-file .env -f compose.yaml stop keycloak
docker stop govai-mailpit
# Resume the existing components.
docker compose --env-file .env -f compose.yaml start keycloak
docker start govai-mailpit
```

Do not run down -v, volume rm, system prune or reset Docker data. Do not commit
realm exports containing passwords, signing keys or secrets. Mailpit messages and
Keycloak logs can contain sensitive verification data; inspect locally and redact
before sharing. No container/volume deletion is required.

## Verification record and remaining work

The operator confirmed the following checks. This documentation edit did not
rerun identity flows or modify runtime settings:

| Check | Evidence/status |
| --- | --- |
| Synthetic test-user login | Confirmed |
| Keycloak data after restart | Confirmed |
| Go discovery/JWKS connectivity | Confirmed; Go check also passed on 2026-10-27 |
| Verification-email delivery to Mailpit | Confirmed |
| First user's verification completion | Confirmed |
| Verification enforced for second unverified user | Confirmed |
| Second user's final verification completion | Not explicitly confirmed |
| 7-character password rejection | Operator-confirmed on 2026-10-27 |
| 8-character password acceptance and subsequent login | Operator-confirmed on 2026-10-27 |

The password boundary results were explicitly confirmed by the operator for the
current 8-character minimum policy. They were not rerun during this documentation
update; no passwords or other credentials are recorded here.

Read-only Docker inspection during this update confirmed the Keycloak and Mailpit
containers are running and Mailpit shares the Keycloak network. This replaces the
initial 2026-10-26 runtime-blocked note. Realm settings above reflect the operator's
reported configuration; they were not independently queried with admin credentials.

Remaining work:

- Configure application/test clients, separate web/mobile public clients, exact
  callbacks, PKCE S256 and intended Go API audience. No client secrets in public clients.
- Configure automatic default citizen assignment deliberately; manual assignment
  is the current behavior.
- Test intended-client token issuance, audience, expiry, refresh rotation/rejection
  and OIDC logout/session termination. Do not promise immediate invalidation of
  signed access tokens validated offline.
- Implement and test Go JWT validation and role/verified-email authorization later.
- Complete the second user's verification and test invalid/expired verification
  links.
- Coordinate PostgreSQL integration with the database teammate in a separate step;
  no databases, users, schemas or migrations were changed here.

The entire authentication subtask is not complete. Frontend/mobile/Python changes,
Go API behavior changes and RabbitMQ configuration are outside this update.

## Official implementation references

- Pinned container guide: https://github.com/keycloak/keycloak/blob/26.8.1/docs/guides/server/containers.adoc
- Pinned database guide: https://github.com/keycloak/keycloak/blob/26.8.1/docs/guides/server/db.adoc
- Administration guide source: https://github.com/keycloak/keycloak/tree/26.8.1/docs/documentation/server_admin/topics
