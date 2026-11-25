package postgres

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationDispatchStore struct {
	pool   *pgxpool.Pool
	issuer string
}

func NewNotificationDispatchStore(
	pool *pgxpool.Pool, issuer string,
) (*NotificationDispatchStore, error) {
	if pool == nil || contract.ValidateIssuer(issuer) != nil {
		return nil, ErrOutbox
	}
	return &NotificationDispatchStore{pool: pool, issuer: issuer}, nil
}

// Claim uses a short SQL lease, not a transaction held open during network I/O.

func (store *NotificationDispatchStore) Claim(
	ctx context.Context, priority contract.Priority,
) (notificationevents.Job, bool, error) {
	var job notificationevents.Job
	value := 10
	if priority == contract.Normal {
		value = 1
	} else if priority != contract.High {
		return job, false, ErrOutbox
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return job, false, ErrOutbox
	}
	token[6] = token[6]&0x0f | 0x40
	token[8] = token[8]&0x3f | 0x80
	job.LeaseToken = fmt.Sprintf("%x-%x-%x-%x-%x",
		token[:4], token[4:6], token[6:8], token[8:10], token[10:])
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := store.pool.QueryRow(ctx, `
WITH candidate AS (
 SELECT event_id FROM auth.notification_outbox
 WHERE issuer=$1 AND priority=$2 AND published_at IS NULL
 AND next_attempt_at <= clock_timestamp()
 AND (lease_expires_at IS NULL OR lease_expires_at <= clock_timestamp())
 ORDER BY next_attempt_at, created_at, event_id
 LIMIT 1 FOR UPDATE SKIP LOCKED
)
UPDATE auth.notification_outbox AS event
SET lease_token=$3, lease_expires_at=clock_timestamp() + interval '30 seconds',
 attempt_count=LEAST(attempt_count::bigint + 1, 2147483647)::integer
FROM candidate WHERE event.event_id=candidate.event_id
RETURNING event.event_id::text, event.event_type, event.event_version,
 event.issuer, event.subject, event.occurred_at`, store.issuer, value, job.LeaseToken,
	).Scan(&job.Event.ID, &job.Event.Type, &job.Event.Version,
		&job.Event.Issuer, &job.Event.Subject, &job.Event.OccurredAt)
	if err == pgx.ErrNoRows {
		return notificationevents.Job{}, false, nil
	}
	if err != nil || job.Event.Validate(store.issuer) != nil {
		return notificationevents.Job{}, false, ErrOutbox
	}
	return job, true, nil
}

// Finish rejects stale workers. An uncertain/lost database update permits replay.

func (store *NotificationDispatchStore) Finish(
	ctx context.Context, job notificationevents.Job, outcome notificationevents.Outcome,
) error {
	if job.Event.Validate(store.issuer) != nil {
		return ErrOutbox
	}
	probe := job.Event
	probe.ID = job.LeaseToken
	if probe.Validate(store.issuer) != nil {
		return ErrOutbox
	}
	switch outcome {
	case notificationevents.Confirmed, notificationevents.Unavailable,
		notificationevents.Unknown, notificationevents.Unroutable:
	default:
		return ErrOutbox
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := store.pool.Exec(ctx, `
UPDATE auth.notification_outbox
SET published_at=CASE WHEN $3='confirmed' THEN clock_timestamp() ELSE NULL END,
 next_attempt_at=CASE WHEN $3='confirmed' THEN next_attempt_at ELSE
  clock_timestamp() + LEAST(60, power(2, LEAST(attempt_count-1, 6))) * interval '1 second'
 END,
 last_error_code=CASE WHEN $3='confirmed' THEN '' ELSE $3 END,
 lease_token=NULL, lease_expires_at=NULL
WHERE event_id=$1 AND issuer=$4 AND lease_token=$2 AND published_at IS NULL
 AND lease_expires_at > clock_timestamp()`, job.Event.ID, job.LeaseToken,
		string(outcome), store.issuer)
	if err != nil || result.RowsAffected() != 1 {
		return ErrOutbox
	}
	return nil
}
