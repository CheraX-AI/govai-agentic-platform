package postgres

import (
	"context"
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const outboxTestIssuer = "https://identity.test/realms/outbox-test"

const outboxTestAction = "123e4567-e89b-42d3-a456-426614174001"

func outboxTestEvent() contract.Event {
	return contract.Event{
		ID:   "123e4567-e89b-42d3-a456-426614174000",
		Type: contract.LoginSucceeded, Version: contract.Version,
		Issuer: outboxTestIssuer, Subject: "synthetic-outbox-citizen",
		OccurredAt: time.Date(2026, 11, 24, 1, 2, 3, 123456789, time.UTC),
	}
}

func TestNotificationOutboxValidation(t *testing.T) {
	if _, err := NewNotificationOutbox("http://untrusted.example"); err == nil {
		t.Fatal("invalid issuer accepted")
	}
	outbox, err := NewNotificationOutbox(outboxTestIssuer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outbox.EnqueueTx(context.Background(), nil,
		outboxTestAction, outboxTestEvent()); !errors.Is(err, ErrOutbox) {
		t.Fatal("transaction is required")
	}
	if MigrateNotificationOutbox(context.Background(), nil) == nil ||
		NotificationOutboxReady(context.Background(), nil) == nil {
		t.Fatal("missing database accepted")
	}
}

func outboxTestPool(t *testing.T, username string) *pgxpool.Pool {
	t.Helper()
	password := os.Getenv("GOVAI_OUTBOX_TEST_PASSWORD")
	if password == "" {
		t.Skip("opt-in: isolated outbox test helper required")
	}
	endpoint := &url.URL{
		Scheme: "postgres", Host: "127.0.0.1:5432",
		User: url.UserPassword(username, password), Path: "/govai_auth_outbox_test",
		RawQuery: "sslmode=disable",
	}
	pool, err := Open(context.Background(), endpoint.String())
	if err != nil {
		t.Fatal("isolated outbox database unavailable")
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestNotificationOutboxDatabase(t *testing.T) {
	owner := outboxTestPool(t, "outbox_owner")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var statement string
	err := owner.QueryRow(ctx, `
SELECT format('CREATE ROLE govai_auth_app LOGIN PASSWORD %L', $1::text)`,
		os.Getenv("GOVAI_OUTBOX_TEST_PASSWORD")).Scan(&statement)
	if err != nil {
		t.Fatal("test role preparation failed")
	}
	if _, err := owner.Exec(ctx, statement); err != nil {
		t.Fatal("test role creation failed")
	}
	if Migrate(ctx, owner) != nil || MigrateNotificationOutbox(ctx, owner) != nil ||
		MigrateNotificationOutbox(ctx, owner) != nil {
		t.Fatal("outbox migration/replay failed")
	}
	app := outboxTestPool(t, "govai_auth_app")
	if NotificationOutboxReady(ctx, app) != nil {
		t.Fatal("restricted outbox readiness failed")
	}
	_, err = owner.Exec(ctx, `
CREATE TABLE public.outbox_test_checkpoint(id uuid PRIMARY KEY);
GRANT SELECT, INSERT ON public.outbox_test_checkpoint TO govai_auth_app;`)
	if err != nil {
		t.Fatal("atomicity fixture failed")
	}
	outbox, _ := NewNotificationOutbox(outboxTestIssuer)
	event := outboxTestEvent()

	t.Run("rollback includes checkpoint and event", func(t *testing.T) {
		tx, err := app.Begin(ctx)
		if err != nil {
			t.Fatal("transaction unavailable")
		}
		defer tx.Rollback(context.Background())
		if _, err := tx.Exec(ctx, "INSERT INTO public.outbox_test_checkpoint VALUES ($1)",
			event.ID); err != nil {
			t.Fatal("checkpoint insert failed")
		}
		result, err := outbox.EnqueueTx(ctx, tx, outboxTestAction, event)
		if err != nil || result != OutboxCreated || tx.Rollback(ctx) != nil {
			t.Fatal("rollback setup failed")
		}
		var count int
		err = owner.QueryRow(ctx, `
SELECT (SELECT count(*) FROM auth.notification_outbox) +
 (SELECT count(*) FROM public.outbox_test_checkpoint)`).Scan(&count)
		if err != nil || count != 0 {
			t.Fatal("rolled back data persisted")
		}
	})

	t.Run("concurrent retries create one event", func(t *testing.T) {
		var workers sync.WaitGroup
		var lock sync.Mutex
		created, existing := 0, 0
		for i := 0; i < 16; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				tx, err := app.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
				if err != nil {
					t.Error("transaction unavailable")
					return
				}
				defer tx.Rollback(context.Background())
				result, err := outbox.EnqueueTx(ctx, tx, outboxTestAction, event)
				if err != nil || tx.Commit(ctx) != nil {
					t.Error("concurrent enqueue failed")
					return
				}
				lock.Lock()
				defer lock.Unlock()
				if result == OutboxCreated {
					created++
				} else if result == OutboxExisting {
					existing++
				}
			}()
		}
		workers.Wait()
		if created != 1 || existing != 15 {
			t.Fatal("concurrent duplicate handling failed")
		}
	})

	t.Run("conflicting retries are rejected", func(t *testing.T) {
		for _, mutate := range []func(*contract.Event){
			func(e *contract.Event) { e.Subject = "another-citizen" },
			func(e *contract.Event) { e.Type = contract.PasswordChanged },
			func(e *contract.Event) { e.OccurredAt = e.OccurredAt.Add(time.Second) },
			func(e *contract.Event) { e.ID = outboxTestAction },
		} {
			changed := event
			mutate(&changed)
			tx, err := app.Begin(ctx)
			if err != nil {
				t.Fatal("transaction unavailable")
			}
			_, err = outbox.EnqueueTx(ctx, tx, outboxTestAction, changed)
			_ = tx.Rollback(ctx)
			if !errors.Is(err, ErrOutboxConflict) {
				t.Fatal("conflicting event accepted")
			}
		}
	})

	t.Run("commit includes checkpoint and event", func(t *testing.T) {
		tx, err := app.Begin(ctx)
		if err != nil {
			t.Fatal("transaction unavailable")
		}
		defer tx.Rollback(context.Background())
		second := event
		second.ID = "123e4567-e89b-42d3-a456-426614174002"
		if _, err := tx.Exec(ctx, "INSERT INTO public.outbox_test_checkpoint VALUES ($1)",
			second.ID); err != nil {
			t.Fatal("checkpoint insert failed")
		}
		result, err := outbox.EnqueueTx(ctx, tx, second.ID, second)
		if err != nil || result != OutboxCreated || tx.Commit(ctx) != nil {
			t.Fatal("atomic commit failed")
		}
		var count int
		err = owner.QueryRow(ctx, `
SELECT count(*) FROM public.outbox_test_checkpoint c
JOIN auth.notification_outbox o ON o.event_id=c.id`).Scan(&count)
		if err != nil || count != 1 {
			t.Fatal("checkpoint and event were not committed together")
		}
	})

	t.Run("runtime cannot edit identity or delete", func(t *testing.T) {
		for _, sql := range []string{
			"DELETE FROM auth.notification_outbox",
			"TRUNCATE auth.notification_outbox",
			"UPDATE auth.notification_outbox SET subject='changed'",
		} {
			if _, err := app.Exec(ctx, sql); err == nil {
				t.Fatal("restricted operation allowed")
			}
		}
	})
	t.Run("normal priority remains pending", func(t *testing.T) {
		tx, err := app.Begin(ctx)
		if err != nil {
			t.Fatal("transaction unavailable")
		}
		defer tx.Rollback(context.Background())
		normal := event
		normal.ID = "123e4567-e89b-42d3-a456-426614174003"
		normal.Type = contract.EmailVerified
		result, err := outbox.EnqueueTx(ctx, tx, normal.ID, normal)
		if err != nil || result != OutboxCreated {
			t.Fatal("normal event enqueue failed")
		}
		var pending bool
		err = tx.QueryRow(ctx, `
SELECT priority=1 AND published_at IS NULL AND attempt_count=0
 AND lease_token IS NULL AND lease_expires_at IS NULL
FROM auth.notification_outbox WHERE event_id=$1`, normal.ID).Scan(&pending)
		if err != nil || !pending {
			t.Fatal("normal event defaults invalid")
		}
	})

	t.Run("invalid events do not insert", func(t *testing.T) {
		for _, mutate := range []func(*contract.Event){
			func(e *contract.Event) { e.Issuer = "https://other.test/realms/other" },
			func(e *contract.Event) { e.Type = "untrusted.event" },
			func(e *contract.Event) { e.Version = 2 },
		} {
			tx, err := app.Begin(ctx)
			if err != nil {
				t.Fatal("transaction unavailable")
			}
			changed := event
			mutate(&changed)
			_, err = outbox.EnqueueTx(ctx, tx, outboxTestAction, changed)
			_ = tx.Rollback(ctx)
			if !errors.Is(err, ErrOutbox) {
				t.Fatal("invalid event accepted")
			}
		}
	})

	t.Run("migration checksum mismatch fails closed", func(t *testing.T) {
		var original string
		if owner.QueryRow(ctx,
			"SELECT checksum FROM public.govai_auth_migrations WHERE version=7",
		).Scan(&original) != nil {
			t.Fatal("migration checksum missing")
		}
		defer owner.Exec(context.Background(),
			"UPDATE public.govai_auth_migrations SET checksum=$1 WHERE version=7", original)
		if _, err := owner.Exec(ctx,
			"UPDATE public.govai_auth_migrations SET checksum='changed' WHERE version=7",
		); err != nil {
			t.Fatal("checksum fixture failed")
		}
		if MigrateNotificationOutbox(ctx, owner) == nil ||
			NotificationOutboxReady(ctx, app) == nil {
			t.Fatal("checksum mismatch accepted")
		}
	})
	t.Run("lease recovery and retry scheduling", func(t *testing.T) {
		verifyNotificationDispatchStore(t, ctx, owner, app)
	})

	t.Run("native and browser login checkpoints", func(t *testing.T) {
		verifyNotificationLoginStores(t, ctx, owner, app)
	})
	t.Run("confirmed password change checkpoints", func(t *testing.T) {
		verifyNotificationPasswordCheckpoints(t, ctx, owner, app)
	})
	t.Run("owned email verification observations", func(t *testing.T) {
		verifyNotificationEmailObservations(t, ctx, owner, app)
	})
	var priority int
	if app.QueryRow(ctx, "SELECT priority FROM auth.notification_outbox WHERE event_id=$1",
		event.ID).Scan(&priority) != nil || priority != 10 {
		t.Fatal("security priority missing")
	}
}

func TestNotificationOutboxRestart(t *testing.T) {
	if os.Getenv("GOVAI_OUTBOX_RESTART_PROBE") != "1" {
		t.Skip("opt-in: restart probe only after fixture initialization")
	}
	app := outboxTestPool(t, "govai_auth_app")
	ctx := context.Background()
	var count int
	if NotificationOutboxReady(ctx, app) != nil || app.QueryRow(ctx, `
SELECT count(*) FROM auth.notification_outbox WHERE published_at IS NULL
 AND event_id IN ('123e4567-e89b-42d3-a456-426614174000',
 '123e4567-e89b-42d3-a456-426614174002')
`).Scan(&count) != nil || count != 2 {
		t.Fatal("pending outbox events did not survive restart")
	}
}
