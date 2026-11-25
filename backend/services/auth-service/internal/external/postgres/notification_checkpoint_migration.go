package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const notificationCheckpointMigration = "migrations/008_notification_checkpoints.sql"

func applyNotificationCheckpoints(ctx context.Context, tx pgx.Tx) error {
	sql, err := migrations.ReadFile(notificationCheckpointMigration)
	if err != nil {
		return ErrOutbox
	}
	sum := sha256.Sum256(sql)
	expected := hex.EncodeToString(sum[:])
	var checksum string
	err = tx.QueryRow(ctx,
		"SELECT checksum FROM public.govai_auth_migrations WHERE version=8",
	).Scan(&checksum)
	if err == nil {
		if checksum != expected {
			return ErrOutbox
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ErrOutbox
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		return ErrOutbox
	}
	_, err = tx.Exec(ctx, `
INSERT INTO public.govai_auth_migrations(version, checksum) VALUES (8, $1)`, expected)
	if err != nil {
		return ErrOutbox
	}
	return nil
}

func notificationCheckpointsReady(ctx context.Context, pool *pgxpool.Pool) error {
	sql, err := migrations.ReadFile(notificationCheckpointMigration)
	if err != nil {
		return ErrOutbox
	}
	sum := sha256.Sum256(sql)
	var checksum string
	err = pool.QueryRow(ctx,
		"SELECT checksum FROM public.govai_auth_migrations WHERE version=8",
	).Scan(&checksum)
	if err != nil || checksum != hex.EncodeToString(sum[:]) {
		return ErrOutbox
	}
	var safe bool
	err = pool.QueryRow(ctx, `
SELECT has_table_privilege(current_user, 'auth.notification_checkpoints', 'SELECT')
 AND has_column_privilege(current_user, 'auth.notification_checkpoints', 'event_id', 'INSERT')
 AND has_column_privilege(current_user, 'auth.notification_checkpoints', 'state', 'UPDATE')
 AND NOT has_column_privilege(current_user,'auth.notification_checkpoints','source_key','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.notification_checkpoints','issuer','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.notification_checkpoints','subject','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.notification_checkpoints','event_id','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.notification_checkpoints','state','INSERT')
 AND NOT has_table_privilege(current_user,'auth.notification_checkpoints','DELETE,TRUNCATE')
 AND has_table_privilege(current_user,'auth.notification_observer_settings','SELECT')
 AND NOT has_column_privilege(
 current_user,'auth.notification_observer_settings','observe_since','INSERT,UPDATE')
 AND NOT has_table_privilege(current_user,'auth.notification_observer_settings','DELETE,TRUNCATE')
 AND has_table_privilege(current_user,'auth.notification_email_watch','SELECT')
 AND NOT has_column_privilege(current_user,'auth.notification_email_watch','operation_id','UPDATE')
 AND NOT has_table_privilege(current_user,'auth.notification_email_watch','DELETE,TRUNCATE')
`).Scan(&safe)
	if err != nil || !safe {
		return ErrOutbox
	}
	return nil
}
