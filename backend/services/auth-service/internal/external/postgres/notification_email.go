package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailObservationStore struct {
	pool   *pgxpool.Pool
	outbox *NotificationOutbox
}

func NewEmailObservationStore(pool *pgxpool.Pool, issuer string) (*EmailObservationStore, error) {
	outbox, err := NewNotificationOutbox(issuer)
	if pool == nil || err != nil {
		return nil, ErrOutbox
	}
	return &EmailObservationStore{pool: pool, outbox: outbox}, nil
}

// Initialize persists the first cutover once, never resetting it on restart.

func (store *EmailObservationStore) Initialize(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := store.pool.Exec(ctx, `
INSERT INTO auth.notification_observer_settings(issuer) VALUES ($1)
ON CONFLICT DO NOTHING`, store.outbox.issuer)
	if err != nil {
		return ErrOutbox
	}
	return nil
}

func (store *EmailObservationStore) ClaimEmailCheck(
	ctx context.Context,
) (notificationevents.EmailCheck, bool, error) {
	var check notificationevents.EmailCheck
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := store.pool.Exec(ctx, `
INSERT INTO auth.notification_email_watch(operation_id)
SELECT profile.operation_id FROM auth.citizen_profiles AS profile
JOIN auth.registration_operations AS operation
 ON operation.operation_id=profile.operation_id
 AND operation.issuer=profile.issuer AND operation.subject=profile.subject
JOIN auth.notification_observer_settings AS settings ON settings.issuer=profile.issuer
WHERE profile.issuer=$1 AND profile.created_at >= settings.observe_since
 AND operation.phase IN ('enabled','email_sending','completed','attention')
 AND NOT EXISTS (SELECT 1 FROM auth.notification_email_watch AS watch
 WHERE watch.operation_id=profile.operation_id)
ORDER BY profile.created_at, profile.operation_id LIMIT 100
ON CONFLICT DO NOTHING`, store.outbox.issuer)
	if err != nil {
		return check, false, ErrOutbox
	}
	nonce, err := notificationevents.NewID()
	if err != nil {
		return check, false, ErrOutbox
	}
	check.LeaseToken = nonce
	err = store.pool.QueryRow(ctx, `
WITH candidate AS (
 SELECT watch.operation_id FROM auth.notification_email_watch AS watch
 JOIN auth.citizen_profiles AS profile ON profile.operation_id=watch.operation_id
 WHERE profile.issuer=$1 AND watch.verified_at IS NULL
 AND watch.next_check_at <= clock_timestamp()
 AND (watch.lease_expires_at IS NULL OR watch.lease_expires_at <= clock_timestamp())
 ORDER BY watch.next_check_at, watch.operation_id LIMIT 1 FOR UPDATE OF watch SKIP LOCKED
)
UPDATE auth.notification_email_watch AS watch
SET lease_token=$2, lease_expires_at=clock_timestamp()+interval '30 seconds'
FROM candidate, auth.citizen_profiles AS profile
WHERE watch.operation_id=candidate.operation_id AND profile.operation_id=watch.operation_id
RETURNING profile.issuer, profile.subject, profile.operation_id`,
		store.outbox.issuer, check.LeaseToken,
	).Scan(&check.Issuer, &check.Subject, &check.OperationID)
	if err == pgx.ErrNoRows {
		return notificationevents.EmailCheck{}, false, nil
	}
	if err != nil {
		return notificationevents.EmailCheck{}, false, ErrOutbox
	}
	return check, true, nil
}

func (store *EmailObservationStore) FinishEmailCheck(
	ctx context.Context, check notificationevents.EmailCheck, verified bool,
) error {
	if check.Issuer != store.outbox.issuer {
		return ErrOutbox
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return ErrOutbox
	}
	defer rollbackRecovery(tx)
	var operation string
	err = tx.QueryRow(ctx, `
SELECT watch.operation_id FROM auth.notification_email_watch AS watch
JOIN auth.citizen_profiles AS profile ON profile.operation_id=watch.operation_id
WHERE watch.operation_id=$1 AND profile.issuer=$2 AND profile.subject=$3
 AND watch.lease_token=$4 AND watch.lease_expires_at > clock_timestamp()
 AND watch.verified_at IS NULL FOR UPDATE OF watch`,
		check.OperationID, check.Issuer, check.Subject, check.LeaseToken).Scan(&operation)
	if err != nil {
		return ErrOutbox
	}
	if verified {
		body, _ := json.Marshal([]string{"govai:email-verified:v1", check.Issuer, operation})
		key := sha256.Sum256(body)
		if prepareNotificationCheckpoint(ctx, tx, contract.EmailVerified, key[:],
			check.Issuer, check.Subject) != nil ||
			confirmNotificationCheckpoint(ctx, tx, store.outbox, contract.EmailVerified, key[:]) != nil {
			return ErrOutbox
		}
	}
	_, err = tx.Exec(ctx, `
UPDATE auth.notification_email_watch
SET next_check_at=clock_timestamp()+interval '30 seconds',
 verified_at=CASE WHEN $2 THEN clock_timestamp() ELSE NULL END,
 lease_token=NULL, lease_expires_at=NULL WHERE operation_id=$1`, operation, verified)
	if err != nil || tx.Commit(ctx) != nil {
		return ErrOutbox
	}
	return nil
}
