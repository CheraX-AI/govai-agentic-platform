package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const notificationOutboxMigration = "migrations/007_notification_outbox.sql"

// MigrateNotificationOutbox is explicit and optional; API startup never calls it.

func MigrateNotificationOutbox(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return ErrOutbox
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return ErrOutbox
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(71480261031)"); err != nil {
		return ErrOutbox
	}
	baseline, err := migrations.ReadFile("migrations/001_registration.sql")
	if err != nil {
		return ErrOutbox
	}
	sum := sha256.Sum256(baseline)
	var checksum string
	err = tx.QueryRow(ctx,
		"SELECT checksum FROM public.govai_auth_migrations WHERE version=1",
	).Scan(&checksum)
	if err != nil || checksum != hex.EncodeToString(sum[:]) {
		return ErrOutbox
	}
	sql, err := migrations.ReadFile(notificationOutboxMigration)
	if err != nil {
		return ErrOutbox
	}
	sum = sha256.Sum256(sql)
	expected := hex.EncodeToString(sum[:])
	err = tx.QueryRow(ctx,
		"SELECT checksum FROM public.govai_auth_migrations WHERE version=7",
	).Scan(&checksum)
	switch {
	case err == nil:
		if checksum != expected {
			return ErrOutbox
		}
	case errors.Is(err, pgx.ErrNoRows):
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return ErrOutbox
		}
		_, err = tx.Exec(ctx, `
INSERT INTO public.govai_auth_migrations(version, checksum) VALUES (7, $1)`, expected)
		if err != nil {
			return ErrOutbox
		}
	default:
		return ErrOutbox
	}
	if applyNotificationCheckpoints(ctx, tx) != nil {
		return ErrOutbox
	}
	if tx.Commit(ctx) != nil {
		return ErrOutbox
	}
	return nil
}

// NotificationOutboxReady verifies the migration and restricted runtime grants.

func NotificationOutboxReady(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return ErrOutbox
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if Ready(ctx, pool) != nil {
		return ErrOutbox
	}
	sql, err := migrations.ReadFile(notificationOutboxMigration)
	if err != nil {
		return ErrOutbox
	}
	sum := sha256.Sum256(sql)
	var checksum string
	err = pool.QueryRow(ctx,
		"SELECT checksum FROM public.govai_auth_migrations WHERE version=7",
	).Scan(&checksum)
	if err != nil || checksum != hex.EncodeToString(sum[:]) {
		return ErrOutbox
	}
	var safe bool
	err = pool.QueryRow(ctx, `
SELECT has_table_privilege(current_user, 'auth.notification_outbox', 'SELECT')
 AND has_column_privilege(current_user, 'auth.notification_outbox', 'event_id', 'INSERT')
 AND has_column_privilege(current_user, 'auth.notification_outbox', 'published_at', 'UPDATE')
 AND NOT has_table_privilege(current_user, 'auth.notification_outbox', 'DELETE,TRUNCATE')
 AND NOT has_column_privilege(current_user, 'auth.notification_outbox', 'published_at', 'INSERT')
 AND NOT has_column_privilege(current_user, 'auth.notification_outbox', 'event_id', 'UPDATE')
 AND NOT has_column_privilege(current_user, 'auth.notification_outbox', 'action_id', 'UPDATE')
 AND NOT has_column_privilege(current_user, 'auth.notification_outbox', 'issuer', 'UPDATE')
 AND NOT has_column_privilege(current_user, 'auth.notification_outbox', 'subject', 'UPDATE')
 AND NOT has_column_privilege(current_user, 'auth.notification_outbox', 'event_type', 'UPDATE')
 AND NOT has_column_privilege(current_user, 'auth.notification_outbox', 'event_version', 'UPDATE')
 AND NOT has_column_privilege(current_user, 'auth.notification_outbox', 'occurred_at', 'UPDATE')
`).Scan(&safe)
	if err != nil || !safe {
		return ErrOutbox
	}
	return notificationCheckpointsReady(ctx, pool)
}
