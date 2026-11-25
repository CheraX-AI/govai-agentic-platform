package postgres

import (
	"context"
	"errors"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/jackc/pgx/v5"
)

var (
	ErrOutbox         = errors.New("authentication notification outbox unavailable")
	ErrOutboxConflict = errors.New("authentication notification event conflict")
)

type OutboxResult uint8

const (
	OutboxCreated OutboxResult = iota + 1
	OutboxExisting
)

type NotificationOutbox struct {
	issuer string
}

// NewNotificationOutbox binds event validation to the configured trusted issuer.

func NewNotificationOutbox(issuer string) (*NotificationOutbox, error) {
	if contract.ValidateIssuer(issuer) != nil {
		return nil, ErrOutbox
	}
	return &NotificationOutbox{issuer: issuer}, nil
}

// EnqueueTx joins the caller's auth checkpoint transaction; it never commits it.
// Reuse actionID and the entire event across retries, including uncertain commits.

func (outbox *NotificationOutbox) EnqueueTx(
	ctx context.Context, tx pgx.Tx, actionID string, event contract.Event,
) (OutboxResult, error) {
	if tx == nil || ctx.Err() != nil || event.Validate(outbox.issuer) != nil {
		return 0, ErrOutbox
	}
	probe := event
	probe.ID = actionID
	if probe.Validate(outbox.issuer) != nil || event.OccurredAt.Year() > 9999 {
		return 0, ErrOutbox
	}

	// PostgreSQL timestamps use microseconds; normalize before storing/comparing.

	event.OccurredAt = event.OccurredAt.UTC().Truncate(time.Microsecond)
	result, err := tx.Exec(ctx, `
INSERT INTO auth.notification_outbox (
 event_id, action_id, event_type, event_version, issuer, subject, occurred_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT DO NOTHING`, event.ID, actionID, event.Type, event.Version,
		event.Issuer, event.Subject, event.OccurredAt)
	if err != nil {
		return 0, ErrOutbox
	}
	if result.RowsAffected() == 1 {
		return OutboxCreated, nil
	}

	// A second statement sees a concurrently committed duplicate at READ COMMITTED.
	// A collision with different identity/action/type/time must never count as success.

	var matches bool
	err = tx.QueryRow(ctx, `
SELECT EXISTS (
 SELECT 1 FROM auth.notification_outbox
 WHERE event_id=$1 AND action_id=$2 AND event_type=$3 AND event_version=$4
 AND issuer=$5 AND subject=$6 AND occurred_at=$7
)`, event.ID, actionID, event.Type, event.Version,
		event.Issuer, event.Subject, event.OccurredAt).Scan(&matches)
	if err != nil {
		return 0, ErrOutbox
	}
	if !matches {
		return 0, ErrOutboxConflict
	}
	return OutboxExisting, nil
}
