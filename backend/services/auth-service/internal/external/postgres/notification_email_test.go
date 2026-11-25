package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sync"
	"testing"
	"time"
)

type verifiedAccountFixture struct {
	verified bool
}

func (fixture *verifiedAccountFixture) ReadOwned(
	_ context.Context, subject, operation string,
) (registration.Identity, error) {
	if subject != "email-watch-new" || operation != "email-watch-new" {
		return registration.Identity{}, errors.New("unexpected account observation")
	}
	return registration.Identity{
		Subject: subject, Enabled: true, EmailVerified: fixture.verified,
	}, nil
}

func seedEmailObservationProfile(
	t *testing.T, ctx context.Context, owner *pgxpool.Pool, operation string, created time.Time,
) {
	t.Helper()
	id, _ := notificationevents.NewID()
	digest := sha256.Sum256([]byte("synthetic-profile-key/" + operation))
	tx, err := owner.Begin(ctx)
	if err != nil {
		t.Fatal("profile fixture transaction unavailable")
	}
	defer rollbackRecovery(tx)
	_, err = tx.Exec(ctx, `
INSERT INTO auth.registration_operations (
 operation_id, issuer, idempotency_key, fingerprint, nic_digest, pending_ciphertext,
 phase, subject, created_at
) VALUES ($1, $2, $3, $4, $4, $5, 'enabled', $1, $6)`,
		operation, outboxTestIssuer, id, digest[:], make([]byte, 29), created)
	if err != nil {
		t.Fatal("profile journal fixture insert failed")
	}
	_, err = tx.Exec(ctx, `
INSERT INTO auth.citizen_profiles (
 operation_id, issuer, subject, nic_digest, profile_ciphertext, created_at
) VALUES ($1, $2, $1, $3, $4, $5)`,
		operation, outboxTestIssuer, digest[:], make([]byte, 29), created)
	if err != nil || tx.Commit(ctx) != nil {
		t.Fatal("profile fixture commit failed")
	}
}

func verifyNotificationEmailObservations(
	t *testing.T, ctx context.Context, owner, app *pgxpool.Pool,
) {
	t.Helper()
	store, _ := NewEmailObservationStore(app, outboxTestIssuer)
	if store.Initialize(ctx) != nil {
		t.Fatal("observer cutover initialization failed")
	}
	var since time.Time
	if app.QueryRow(ctx, `
SELECT observe_since FROM auth.notification_observer_settings WHERE issuer=$1`,
		outboxTestIssuer).Scan(&since) != nil {
		t.Fatal("observer cutover missing")
	}
	seedEmailObservationProfile(t, ctx, owner, "email-watch-old", since.Add(-time.Hour))
	seedEmailObservationProfile(t, ctx, owner, "email-watch-new", since.Add(time.Millisecond))
	source := &verifiedAccountFixture{}
	observer, _ := notificationevents.NewEmailObserver(store, source)
	if worked, err := observer.CheckOne(ctx); !worked || err != nil {
		t.Fatal("unverified account observation failed")
	}
	var count int
	if app.QueryRow(ctx, `
SELECT count(*) FROM auth.notification_outbox WHERE event_type='citizen.email.verified'
`).Scan(&count) != nil || count != 0 {
		t.Fatal("unverified account produced a notification")
	}
	if _, err := owner.Exec(ctx, `
UPDATE auth.notification_email_watch SET next_check_at=clock_timestamp()-interval '1 second'
WHERE operation_id='email-watch-new'`); err != nil {
		t.Fatal("observation retry fixture failed")
	}
	source.verified = true
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := observer.CheckOne(ctx); err != nil {
				t.Error("concurrent verification observation failed")
			}
		}()
	}
	workers.Wait()
	if app.QueryRow(ctx, `
SELECT count(*) FROM auth.notification_outbox
WHERE event_type='citizen.email.verified' AND subject='email-watch-new' AND priority=1
`).Scan(&count) != nil || count != 1 {
		t.Fatal("verification did not create one normal-priority notification")
	}
	if app.QueryRow(ctx, `
SELECT count(*) FROM auth.notification_email_watch WHERE operation_id='email-watch-old'
`).Scan(&count) != nil || count != 0 {
		t.Fatal("observer backfilled an older account")
	}
	seedEmailObservationProfile(t, ctx, owner, "email-watch-next", since.Add(2*time.Millisecond))
	first, found, err := store.ClaimEmailCheck(ctx)
	if err != nil || !found || first.OperationID != "email-watch-next" {
		t.Fatal("second observation fixture unavailable")
	}
	wrongOwner := first
	wrongOwner.Subject = "another-citizen"
	if store.FinishEmailCheck(ctx, wrongOwner, true) == nil {
		t.Fatal("verification confirmation crossed citizen ownership")
	}
	grant := `GRANT INSERT (
 event_id, action_id, event_type, event_version, issuer, subject, occurred_at
) ON auth.notification_outbox TO govai_auth_app`
	if _, err := owner.Exec(ctx, `REVOKE INSERT (
 event_id, action_id, event_type, event_version, issuer, subject, occurred_at
) ON auth.notification_outbox FROM govai_auth_app`); err != nil {
		t.Fatal("verification failure fixture unavailable")
	}
	defer owner.Exec(context.Background(), grant)
	if store.FinishEmailCheck(ctx, first, true) == nil {
		t.Fatal("verification completed without an outbox event")
	}
	var pending bool
	if app.QueryRow(ctx, `
SELECT verified_at IS NULL FROM auth.notification_email_watch
WHERE operation_id='email-watch-next'`).Scan(&pending) != nil || !pending {
		t.Fatal("failed verification enqueue partially completed its watch")
	}
	if _, err := owner.Exec(ctx, grant); err != nil {
		t.Fatal("verification fixture permissions restore failed")
	}
	if _, err := owner.Exec(ctx, `
UPDATE auth.notification_email_watch
SET lease_expires_at=clock_timestamp()-interval '1 second'
WHERE operation_id='email-watch-next'`); err != nil {
		t.Fatal("observation expiry fixture failed")
	}
	second, found, err := store.ClaimEmailCheck(ctx)
	if err != nil || !found || second.LeaseToken == first.LeaseToken {
		t.Fatal("expired observation lease was not recovered")
	}
	if store.FinishEmailCheck(ctx, first, true) == nil ||
		store.FinishEmailCheck(ctx, second, true) != nil {
		t.Fatal("verification lease ownership was not enforced")
	}
	if _, found, err := store.ClaimEmailCheck(ctx); err != nil || found {
		t.Fatal("verified accounts were observed again")
	}
	if store.Initialize(ctx) != nil {
		t.Fatal("observer reinitialization failed")
	}
	var reloaded time.Time
	if app.QueryRow(ctx, `
SELECT observe_since FROM auth.notification_observer_settings WHERE issuer=$1`,
		outboxTestIssuer).Scan(&reloaded) != nil || !reloaded.Equal(since) {
		t.Fatal("observer restart changed the registration cutover")
	}
}

func TestNotificationAccountCheckpointsRestart(t *testing.T) {
	if os.Getenv("GOVAI_OUTBOX_RESTART_PROBE") != "1" {
		t.Skip("opt-in: account checkpoint restart fixture required")
	}
	app := outboxTestPool(t, "govai_auth_app")
	ctx := context.Background()
	status, err := NotificationPasswordStatus(ctx, app)
	if err != nil || status.Prepared != 1 || status.Uncertain != 1 || status.Confirmed != 2 {
		t.Fatal("password checkpoint states did not survive restart")
	}
	var before time.Time
	if app.QueryRow(ctx, `
SELECT observe_since FROM auth.notification_observer_settings WHERE issuer=$1`,
		outboxTestIssuer).Scan(&before) != nil {
		t.Fatal("registration cutover did not survive restart")
	}
	store, _ := NewEmailObservationStore(app, outboxTestIssuer)
	if store.Initialize(ctx) != nil {
		t.Fatal("observer restart initialization failed")
	}
	var after time.Time
	if app.QueryRow(ctx, `
SELECT observe_since FROM auth.notification_observer_settings WHERE issuer=$1`,
		outboxTestIssuer).Scan(&after) != nil || !after.Equal(before) {
		t.Fatal("observer restart moved the persisted cutover")
	}
	if _, found, err := store.ClaimEmailCheck(ctx); err != nil || found {
		t.Fatal("observer restart re-created a completed verification")
	}
	var count int
	if app.QueryRow(ctx, `
SELECT count(*) FROM auth.notification_outbox WHERE event_type='citizen.email.verified'
`).Scan(&count) != nil || count != 2 {
		t.Fatal("verification outbox history did not survive restart")
	}
}
