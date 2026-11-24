# Notification event contract (GA-267 / GA-268)

This package is a proposed version-one wire contract shared by auth-service and
notification-service. It does not publish, consume, save or acknowledge messages.
Auth-service integration and database integration remain separate work.

## Version-one envelope

    {
      "event_id": "123e4567-e89b-42d3-a456-426614174000",
      "event_type": "citizen.login.succeeded",
      "version": 1,
      "issuer": "https://identity.example/realms/govai",
      "subject": "synthetic-subject",
      "occurred_at": "2026-11-21T05:00:00Z",
      "data": {}
    }

The example identity is synthetic; configure the real trusted Keycloak issuer.
Use a canonical lowercase RFC UUID event_id, stable issuer+subject identity,
and an RFC3339 timestamp for the actual successful account action.
Generate one ID per logical action and reuse it on outbox retry/redelivery.

Maximum serialized size is 16 KiB. Required fields use exact spelling and occur
only once. Unknown fields, duplicate keys, null required values, unsupported
versions/types and trailing JSON are rejected. data may be omitted or {}.
No additional metadata is required by the current three event types.
Add named, bounded metadata only after producer/consumer agreement; do not use
arbitrary dictionaries of personal data, templates, links or caller-authored text.

Do not include NIC, passwords, JWTs, refresh tokens, reset links, full IP addresses,
email addresses or personal profile fields. Recipient lookup must use an authorized
service contract rather than copying another service's database tables.

## Events, routing and priority

| Event | Priority | Processing queue |
| --- | --- | --- |
| citizen.login.succeeded | high | notifications.security.v1 |
| citizen.password.changed | high | notifications.security.v1 |
| citizen.email.verified | normal | notifications.account.v1 |

PriorityFor derives priority from event type. A client-supplied priority is rejected.
DecodeEvent in notification-service also checks routing key and destination queue.
Security retries use delivery.security.retry.v1; normal retries use
delivery.retry.v1, and the original event_type remains unchanged.

Only confirmed account checkpoints may publish these events. Requesting a
verification email does not mean the email is verified.
Failed-attempt alerts, welcome messages, reminders and announcements are not yet
enabled. Their checkpoints/policy must be agreed before extending the allowlist.

Validation does not prove publisher authenticity or account ownership.
Use broker credentials/permissions, trusted publisher logic and exact configured
issuer matching. Never expose this event decoder as an unauthenticated public
notification-creation API. Raw JSON still contains identity references: do not log
payloads. Generic Event formatting and slog values are redacted.

Do not expire a delayed event solely because its occurred_at is old: durable
outbox retries may arrive later. Retention and any clock-skew policy need separate
agreement; decoding does not impose them.

## Database handoff to GA-208 owner

No schema or migration is added here. Agree column types and transaction behavior
with the database owner before implementing a PostgreSQL adapter.

The future store operation must atomically:
1. Record processed_events using the stable unique event_id.
2. Insert one notifications row with issuer+subject ownership and derived priority.
3. Create a pending notification_deliveries record when policy/preferences permit.
4. Commit all records together, or roll them all back.

Return whether the event was newly saved or an already-processed duplicate.
On event_id reuse with conflicting identity/type/payload, surface a conflict
rather than silently associating it with another citizen.
The consumer ACKs only after success/confirmed duplicate; storage failure must
never be treated as a successful notification. Email submission happens later
and must not roll back saved inbox history.

The adapter must enforce owner scoping. notification_preferences and recipient
lookup must not directly query auth-service or Keycloak tables.
Dedicated connection credentials, indexes, foreign keys, migration versioning,
encryption and backups remain GA-208 responsibilities.

## Verification

The notification-service Docker checks stage runs formatting, this package's
tests/Go analysis, and service tests/analysis, including routing-isolation tests.
The tests use synthetic events and do not touch the running RabbitMQ queues.
They do not demonstrate durable deduplication: that requires PostgreSQL.
