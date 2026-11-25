package postgres

import (
	"context"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
)

type LoginEventRecorder struct {
	store  *Store
	outbox *NotificationOutbox
}

func NewLoginEventRecorder(store *Store, issuer string) (*LoginEventRecorder, error) {
	outbox, err := NewNotificationOutbox(issuer)
	if store == nil || err != nil {
		return nil, ErrOutbox
	}
	return &LoginEventRecorder{store: store, outbox: outbox}, nil
}

// RecordLogin is called only after credential, JWT, profile and expiry checks.
// Its commit is the native auth success checkpoint; no broker connection is needed.

func (recorder *LoginEventRecorder) RecordLogin(
	ctx context.Context, issuer, subject string,
) error {
	if issuer != recorder.outbox.issuer {
		return ErrOutbox
	}
	event, err := notificationevents.NewEvent(contract.LoginSucceeded, issuer, subject)
	if err != nil {
		return ErrOutbox
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := recorder.store.pool.Begin(ctx)
	if err != nil {
		return ErrOutbox
	}
	defer rollbackRecovery(tx)
	if _, err := recorder.outbox.EnqueueTx(ctx, tx, event.ID, event); err != nil {
		return err
	}
	if tx.Commit(ctx) != nil {
		return ErrOutbox
	}
	return nil
}
