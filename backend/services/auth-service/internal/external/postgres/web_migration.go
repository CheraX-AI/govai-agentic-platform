package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrateWebSessions is an explicit optional migration. API startup never calls
// it, and baseline registration databases need not install web session storage.
func MigrateWebSessions(ctx context.Context, p *pgxpool.Pool) error {
	if p == nil {
		return ErrStore
	}

	tx, err := p.Begin(ctx)
	if err != nil {
		return ErrStore
	}

	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(71480261031)"); err != nil {
		return ErrStore
	}
	// Require the unchanged baseline checksum before installing the extension.
	baseline, _ := migrations.ReadFile("migrations/001_registration.sql")
	sum := sha256.Sum256(baseline)
	var old string
	if tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=1").Scan(&old) != nil ||

		old != hex.EncodeToString(sum[:]) {
		return ErrStore
	}

	sql, err := migrations.ReadFile("migrations/002_web_sessions.sql")
	if err != nil {
		return ErrStore
	}

	sum = sha256.Sum256(sql)
	checksum := hex.EncodeToString(sum[:])
	err = tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=2").Scan(&old)
	switch err {
	case nil:
		if old != checksum {
			return ErrStore
		}
	case pgx.ErrNoRows:
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return ErrStore
		}
		if _, err = tx.Exec(ctx, "INSERT INTO public.govai_auth_migrations(version,checksum) VALUES(2,$1)", checksum); err != nil {
			return ErrStore
		}
	default:
		return ErrStore
	}

	if err = applyWebRevocations(ctx, tx); err != nil {
		return err
	}

	if tx.Commit(ctx) != nil {
		return ErrStore
	}

	return nil
}

func WebSessionsReady(ctx context.Context, p *pgxpool.Pool) error {
	if p == nil {
		return ErrStore
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if Ready(ctx, p) != nil {
		return ErrStore
	}

	sql, err := migrations.ReadFile("migrations/002_web_sessions.sql")
	if err != nil {
		return ErrStore
	}

	sum := sha256.Sum256(sql)
	var checksum string
	if p.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=2").Scan(&checksum) != nil ||

		checksum != hex.EncodeToString(sum[:]) {
		return ErrStore
	}

	var restricted bool
	if p.QueryRow(
		ctx,
		`SELECT NOT has_table_privilege(current_user,'auth.web_sessions','DELETE,TRUNCATE') AND NOT has_column_privilege(current_user,'auth.web_sessions','session_hash','UPDATE') AND NOT has_column_privilege(current_user,'auth.web_sessions','expires_at','UPDATE')`,
	).Scan(&restricted) != nil ||
		!restricted {
		return ErrStore
	}

	if err := webRevocationsReady(ctx, p); err != nil {
		return err
	}

	return nil
}

func applyWebRevocations(ctx context.Context, tx pgx.Tx) error {
	sql, err := migrations.ReadFile("migrations/003_web_revocations.sql")
	if err != nil {
		return ErrStore
	}

	sum := sha256.Sum256(sql)
	checksum := hex.EncodeToString(sum[:])
	var old string
	err = tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=3").Scan(&old)
	switch err {
	case nil:
		if old != checksum {
			return ErrStore
		}
		return nil
	}

	if err != pgx.ErrNoRows {
		return ErrStore
	}

	if _, err = tx.Exec(ctx, string(sql)); err != nil {
		return ErrStore
	}

	if _, err = tx.Exec(ctx, "INSERT INTO public.govai_auth_migrations(version,checksum) VALUES(3,$1)", checksum); err != nil {
		return ErrStore
	}

	return nil
}

func webRevocationsReady(ctx context.Context, p *pgxpool.Pool) error {
	sql, err := migrations.ReadFile("migrations/003_web_revocations.sql")
	if err != nil {
		return ErrStore
	}

	sum := sha256.Sum256(sql)
	var old string
	if p.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=3").Scan(&old) != nil ||

		old != hex.EncodeToString(sum[:]) {
		return ErrStore
	}

	var restricted bool
	if p.QueryRow(
		ctx,
		`SELECT NOT has_table_privilege(current_user,'auth.web_revocations','DELETE,TRUNCATE') AND NOT has_column_privilege(current_user,'auth.web_revocations','client_hash','UPDATE')`,
	).Scan(&restricted) != nil ||
		!restricted {
		return ErrStore
	}

	return nil
}
