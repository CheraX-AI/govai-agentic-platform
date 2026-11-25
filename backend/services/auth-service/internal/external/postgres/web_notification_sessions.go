package postgres

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewWebSessionStoreWithNotifications(
	pool *pgxpool.Pool, key []byte, issuer string,
) (*WebSessionStore, error) {
	store, err := NewWebSessionStore(pool, key)
	if err != nil {
		return nil, err
	}
	store.notifications, err = NewNotificationOutbox(issuer)
	if err != nil {
		return nil, err
	}
	return store, nil
}

// Prepare cookie material once; the transaction decides whether it becomes usable.

func (store *WebSessionStore) prepareSession(
	credentials WebCredentials, expires time.Time,
) (string, []byte, []byte, error) {
	if !expires.After(time.Now()) || expires.After(time.Now().Add(24*time.Hour)) {
		return "", nil, nil, ErrStore
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", nil, nil, ErrStore
	}
	raw := base64.RawURLEncoding.EncodeToString(random[:])
	hash, _ := webHash(raw)
	sealed, err := store.seal(credentials, hash)
	if err != nil {
		return "", nil, nil, ErrStore
	}
	return raw, hash, sealed, nil
}

// Save the session and optional event together, including guarded-session callers.

func (store *WebSessionStore) insertSessionTx(
	ctx context.Context, tx pgx.Tx, credentials WebCredentials,
	expires time.Time, hash, sealed []byte,
) error {
	_, err := tx.Exec(ctx, `
INSERT INTO auth.web_sessions(session_hash, ciphertext, expires_at) VALUES ($1, $2, $3)`,
		hash, sealed, expires)
	if err != nil {
		return ErrStore
	}
	if store.notifications == nil {
		return nil
	}
	event, err := notificationevents.NewEvent(
		contract.LoginSucceeded, credentials.Issuer, credentials.Subject,
	)
	if err != nil {
		return ErrStore
	}
	if _, err := store.notifications.EnqueueTx(ctx, tx, event.ID, event); err != nil {
		return ErrStore
	}
	return nil
}
