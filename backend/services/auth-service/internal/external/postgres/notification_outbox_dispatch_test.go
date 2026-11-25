package postgres

import (
	"context"
	"sync"
	"testing"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The storage fixture calls this after initializing its isolated database.

func verifyNotificationDispatchStore(
	t *testing.T, ctx context.Context, owner, app *pgxpool.Pool,
) {
	t.Helper()
	store, err := NewNotificationDispatchStore(app, outboxTestIssuer)
	if err != nil {
		t.Fatal(err)
	}
	event := outboxTestEvent()
	event.ID = "123e4567-e89b-42d3-a456-426614174010"
	event.Type = contract.EmailVerified
	outbox, _ := NewNotificationOutbox(outboxTestIssuer)
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal("dispatch fixture transaction unavailable")
	}
	defer tx.Rollback(context.Background())
	if _, err := outbox.EnqueueTx(ctx, tx, event.ID, event); err != nil ||
		tx.Commit(ctx) != nil {
		t.Fatal("dispatch fixture enqueue failed")
	}
	defer owner.Exec(context.Background(),
		"DELETE FROM auth.notification_outbox WHERE event_id=$1", event.ID)
	jobs := make(chan notificationevents.Job, 12)
	failures := make(chan error, 12)
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			job, found, err := store.Claim(ctx, contract.Normal)
			if err != nil {
				failures <- err
			} else if found {
				jobs <- job
			}
		}()
	}
	workers.Wait()
	if len(failures) != 0 || len(jobs) != 1 {
		t.Fatal("concurrent claims did not choose exactly one worker")
	}
	first := <-jobs
	if first.Event.ID != event.ID {
		t.Fatal("normal lane claimed the wrong event")
	}
	if _, found, err := store.Claim(ctx, contract.Normal); err != nil || found {
		t.Fatal("active lease was claimed again or crossed priority lanes")
	}
	if store.Finish(ctx, first, notificationevents.Unknown) != nil {
		t.Fatal("unconfirmed publication was not retained")
	}
	var retry bool
	err = owner.QueryRow(ctx, `
SELECT published_at IS NULL AND lease_token IS NULL AND attempt_count=1
 AND last_error_code='publish_unknown' AND next_attempt_at > clock_timestamp()
FROM auth.notification_outbox WHERE event_id=$1`, event.ID).Scan(&retry)
	if err != nil || !retry {
		t.Fatal("pending retry metadata invalid")
	}
	if _, found, err := store.Claim(ctx, contract.Normal); err != nil || found {
		t.Fatal("retry backoff ignored")
	}
	if _, err := owner.Exec(ctx, `
UPDATE auth.notification_outbox SET next_attempt_at=clock_timestamp()-interval '1 second'
WHERE event_id=$1`, event.ID); err != nil {
		t.Fatal("retry fixture adjustment failed")
	}
	second, found, err := store.Claim(ctx, contract.Normal)
	if err != nil || !found || second.Event.ID != first.Event.ID ||
		second.LeaseToken == first.LeaseToken {
		t.Fatal("retry did not preserve event ID with a new lease")
	}
	if store.Finish(ctx, first, notificationevents.Confirmed) == nil {
		t.Fatal("stale lease completed a newer attempt")
	}
	if _, err := owner.Exec(ctx, `
UPDATE auth.notification_outbox SET lease_expires_at=clock_timestamp()-interval '1 second'
WHERE event_id=$1`, event.ID); err != nil {
		t.Fatal("expiry fixture adjustment failed")
	}
	third, found, err := store.Claim(ctx, contract.Normal)
	if err != nil || !found || third.LeaseToken == second.LeaseToken {
		t.Fatal("expired worker lease was not recovered")
	}
	if store.Finish(ctx, second, notificationevents.Confirmed) == nil ||
		store.Finish(ctx, third, notificationevents.Confirmed) != nil {
		t.Fatal("lease ownership was not enforced")
	}
	if _, found, err := store.Claim(ctx, contract.Normal); err != nil || found {
		t.Fatal("confirmed event was claimed again")
	}
}
