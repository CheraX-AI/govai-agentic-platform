# RabbitMQ account notifications (GA-266)

## Files

- compose.yaml: pinned RabbitMQ 4.4.0 management image, loopback ports,
  named data volume, private Docker network and health check.
- rabbitmq.conf: automatic startup import.
- definitions.json: shared durable exchanges, queues, bindings and poison policy.
- setup.ps1: generate independent local passwords and salted password hashes.
- smoke-test.ps1: verify routing, retry isolation and restart persistence.
- .env.example: configuration names only; no working credentials.

The root docker-compose.yml includes this service. Use the standalone commands
below to start only RabbitMQ without configuring unrelated services.

## First-time setup (PowerShell, repository root)

    .\infrastructure\rabbitmq\setup.ps1
    docker compose -f infrastructure/rabbitmq/compose.yaml config --quiet
    docker compose -f infrastructure/rabbitmq/compose.yaml up -d --wait rabbitmq

Open http://localhost:15672 and select the govai virtual host.
Read the administrator login from infrastructure/rabbitmq/.env in your editor.
Never paste that file or generated .env.definitions.json into chat or commits.
Both files are ignored by the existing Git rules.

setup.ps1 preserves existing passwords. It merges their salted hashes with the
shared topology into .env.definitions.json, which is imported at broker startup.
After topology edits, rerun setup.ps1 and restart RabbitMQ. Credential rotation
requires deliberate coordination with connected services.

## Connections and permissions

- Host applications: localhost:5672, virtual host govai.
- Containers on notifications-network: rabbitmq:5672, virtual host govai.
- Management dashboard: localhost:15672, administrator credentials.
- govai_auth: publish to notifications; no configure or consume permission.
- govai_notifications: consume notifications.account.v1 / notifications.security.v1 and publish to the
  notification/retry/dead exchanges; no topology configuration permission.
- No guest user is seeded. Only the local administrator has management access.

The .env values are inputs to setup.ps1. They are not automatically loaded by
future Go services: explicitly wire them in each service's local configuration.

## Topology

notifications is a durable topic exchange with two processing queues:

| Queue | Events | Intended worker allocation |
| --- | --- | --- |
| notifications.security.v1 | Login and password-change success | Reserved security workers |
| notifications.account.v1 | Confirmed email verification | Normal account workers |

Separate queues isolate backlogs; they do not by themselves enforce worker or
SMTP scheduling. GA-267 must attach independent consumers/channels with bounded
prefetch and reserved security concurrency. The database email dispatcher must
also reserve capacity for pending security deliveries. This is not a strict global
ordering guarantee, and work already being processed cannot be preempted.

Priorities come from a trusted event-type allowlist, not caller-supplied values.
Future reminders require approved event types and producers before adding bindings.

Both queues are durable quorum queues on one development node: persistence is
provided, but this is not a highly available cluster.

notifications.retry has two direct bindings:
- key security -> notifications.security.retry.v1 -> after 30 seconds, returns
  through delivery.security.retry.v1 to notifications.security.v1.
- key retry -> notifications.retry.v1 -> after 30 seconds, returns through
  delivery.retry.v1 to notifications.account.v1.

notifications.dead routes key failed to notifications.dead.v1 for operator
inspection. A shared dead-letter queue does not participate in active scheduling.

Both processing queues have a 20-redelivery poison-message safety ceiling.
This is NOT the SMTP budget. GA-269 must persist up to three total eligible email
attempts, distinguish permanent/unknown outcomes, and publish exhausted deliveries
to notifications.dead. The retry queue TTL cannot implement this budget by itself.

For an existing development broker with the original combined routing, stop all
producers/consumers, ensure the old account/retry queues are drained, then run:

    .\infrastructure\rabbitmq\apply-topology.ps1

The script checks the old queues, imports new definitions, waits for the security
bindings, and removes only the superseded login/password bindings from the normal
queue. No queue, volume or message is deleted. Keep producers stopped until it
finishes. Importing definitions alone would leave old bindings and duplicate routes.

Production publishers must use persistent messages, mandatory routing checks
and AMQP publisher confirms. Production consumers must ACK only after durable
database commit. Those Go integrations belong to GA-267/GA-268/GA-269.

## Verify and restart

    .\infrastructure\rabbitmq\smoke-test.ps1 -Restart

Run only with all notification queues empty and no producers/consumers attached.
The test publishes synthetic events for login, password change, email verification,
both retry paths and the dead-letter route. It verifies exact destination queues,
retry return paths, persistence through restart, and HTTP authentication.
Inspected messages are requeued until validation passes; only then are the test
messages removed. Run on an idle local broker to avoid concurrent message changes.
This uses the management HTTP API, not the future AMQP publisher-confirm client.

Inspect exchanges and queues in the dashboard under virtual host govai.

    docker compose -f infrastructure/rabbitmq/compose.yaml ps
    docker compose -f infrastructure/rabbitmq/compose.yaml stop rabbitmq
    docker compose -f infrastructure/rabbitmq/compose.yaml up -d --wait rabbitmq

Never use down -v for ordinary stops: it deletes persistent volumes.
If using the root Compose file, keep the project name govai-agentic-platform
(the standalone default) so both entry points use the same volume and service.

## Deployment boundary

This is a local development configuration with plaintext AMQP/HTTP bound to
loopback. Before production, configure TLS/AMQPS, restricted management access,
external secret provisioning, separate service identities, disk monitoring and
a multi-node broker/recovery plan. Do not expose these development ports publicly.
RabbitMQ persistence does not replace GA-208 PostgreSQL or GA-210 backups.

## References

- https://www.rabbitmq.com/docs/definitions
- https://www.rabbitmq.com/docs/passwords
- https://www.rabbitmq.com/release-information
