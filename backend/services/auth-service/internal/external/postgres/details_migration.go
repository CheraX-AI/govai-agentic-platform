package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrateCitizenDetails is explicit and additive. It requires baseline 001 but
// does not enable unrelated web/recovery features or rewrite their migrations.
func MigrateCitizenDetails(ctx context.Context, p *pgxpool.Pool) error {
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
	// Require migration authority even for a checksum-only replay. Runtime
	// credentials must never be accepted by this privileged operation.
	var permitted bool
	if tx.QueryRow(ctx, `SELECT has_schema_privilege(current_user,'auth','CREATE')
 AND has_table_privilege(current_user,'public.govai_auth_migrations','INSERT')`).Scan(&permitted) != nil || !permitted {
		return ErrStore
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(71480261031)"); err != nil {
		return ErrStore
	}
	baseline, err := migrations.ReadFile("migrations/001_registration.sql")
	if err != nil {
		return ErrStore
	}
	sum := sha256.Sum256(baseline)
	var old string
	if tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=1").Scan(&old) != nil || old != hex.EncodeToString(sum[:]) {
		return ErrStore
	}
	sql, err := migrations.ReadFile("migrations/007_citizen_details.sql")
	if err != nil {
		return ErrStore
	}
	sum = sha256.Sum256(sql)
	checksum := hex.EncodeToString(sum[:])
	err = tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=7").Scan(&old)
	switch err {
	case nil:
		if old != checksum {
			return ErrStore
		}
	case pgx.ErrNoRows:
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return ErrStore
		}
		if _, err = tx.Exec(ctx, "INSERT INTO public.govai_auth_migrations(version,checksum) VALUES(7,$1)", checksum); err != nil {
			return ErrStore
		}
	default:
		return ErrStore
	}
	if tx.Commit(ctx) != nil {
		return ErrStore
	}
	return nil
}

func CitizenDetailsReady(ctx context.Context, p *pgxpool.Pool) error {
	if p == nil {
		return ErrStore
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if Ready(ctx, p) != nil {
		return ErrStore
	}
	sql, err := migrations.ReadFile("migrations/007_citizen_details.sql")
	if err != nil {
		return ErrStore
	}
	sum := sha256.Sum256(sql)
	var checksum string
	if p.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=7").Scan(&checksum) != nil || checksum != hex.EncodeToString(sum[:]) {
		return ErrStore
	}
	var safe bool
	err = p.QueryRow(ctx, `SELECT
 has_table_privilege(current_user,'auth.citizen_details','SELECT')
 AND has_column_privilege(current_user,'auth.citizen_details','issuer','INSERT')
 AND has_column_privilege(current_user,'auth.citizen_details','subject','INSERT')
 AND has_column_privilege(current_user,'auth.citizen_details','revision','UPDATE')
 AND has_column_privilege(current_user,'auth.citizen_details','details_ciphertext','UPDATE')
 AND has_column_privilege(current_user,'auth.citizen_details','photo_object_key','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.citizen_details','issuer','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.citizen_details','subject','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.citizen_details','created_at','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.citizen_details','updated_at','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.citizen_details','details_ciphertext','INSERT')
 AND NOT has_column_privilege(current_user,'auth.citizen_details','photo_object_key','INSERT')
 AND NOT has_table_privilege(current_user,'auth.citizen_details','DELETE,TRUNCATE')`).Scan(&safe)
	if err != nil || !safe {
		return ErrStore
	}
	return nil
}
