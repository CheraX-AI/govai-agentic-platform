package postgres

import (
	"context"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	"github.com/jackc/pgx/v5"
)

func prepareNotificationCheckpoint(
	ctx context.Context, tx pgx.Tx, eventType string, key []byte, issuer, subject string,
) error {
	id, err := notificationevents.NewID()
	if err != nil || len(key) != 32 || contract.ValidateIssuer(issuer) != nil {
		return ErrOutbox
	}
	_, err = tx.Exec(ctx, `
INSERT INTO auth.notification_checkpoints(event_id, event_type, source_key, issuer, subject)
VALUES ($1, $2, $3, $4, $5)`, id, eventType, key, issuer, subject)
	if err != nil {
		return ErrOutbox
	}
	return nil
}

// Confirmation and enqueue share a transaction and the original event identity.

func confirmNotificationCheckpoint(
	ctx context.Context, tx pgx.Tx, outbox *NotificationOutbox, eventType string, key []byte,
) error {
	var event contract.Event
	var state string
	var confirmed *time.Time
	err := tx.QueryRow(ctx, `
SELECT event_id::text, issuer, subject, state, confirmed_at
FROM auth.notification_checkpoints
WHERE issuer=$1 AND event_type=$2 AND source_key=$3 FOR UPDATE`,
		outbox.issuer, eventType, key,
	).Scan(&event.ID, &event.Issuer, &event.Subject, &state, &confirmed)
	if err != nil || (state != "prepared" && state != "confirmed") {
		return ErrOutbox
	}
	event.Type, event.Version = eventType, contract.Version
	if confirmed == nil {
		event.OccurredAt = time.Now().UTC().Truncate(time.Microsecond)
		_, err := tx.Exec(ctx, `
UPDATE auth.notification_checkpoints SET state='confirmed', confirmed_at=$2
WHERE event_id=$1 AND state='prepared'`, event.ID, event.OccurredAt)
		if err != nil {
			return ErrOutbox
		}
	} else {
		event.OccurredAt = *confirmed
	}
	if _, err := outbox.EnqueueTx(ctx, tx, event.ID, event); err != nil {
		return ErrOutbox
	}
	return nil
}
