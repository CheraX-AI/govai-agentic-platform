package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

// PruneWebSessions deletes one bounded batch of long-expired custody records.
// It requires the table owner, cannot run through the API role, and neither
// refreshes nor revokes provider tokens. Operators must review retention first.
func PruneWebSessions(ctx context.Context, pool *pgxpool.Pool, retention time.Duration, batch int) (int64, error) {
	if pool == nil || retention < 24*time.Hour || retention > 365*24*time.Hour || batch < 1 || batch > 1000 {
		return 0, ErrStore
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var owner bool
	if pool.QueryRow(ctx, `SELECT current_user=pg_get_userbyid(relowner) FROM pg_class WHERE oid='auth.web_sessions'::regclass`).Scan(&owner) != nil || !owner {
		return 0, ErrStore
	}
	sql, err := migrations.ReadFile("migrations/002_web_sessions.sql")
	if err != nil {
		return 0, ErrStore
	}
	sum := sha256.Sum256(sql)
	var checksum string
	if pool.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=2").Scan(&checksum) != nil || checksum != hex.EncodeToString(sum[:]) {
		return 0, ErrStore
	}
	tag, err := pool.Exec(ctx, `DELETE FROM auth.web_sessions WHERE session_hash IN
 (SELECT session_hash FROM auth.web_sessions WHERE expires_at < clock_timestamp()-($1::bigint*interval '1 second')
 ORDER BY expires_at,session_hash LIMIT $2 FOR UPDATE SKIP LOCKED)`, int64(retention.Seconds()), batch)
	if err != nil {
		return 0, ErrStore
	}
	return tag.RowsAffected(), nil
}

// PruneCompletedWebRevocations never removes pending/running/attention work.
func PruneCompletedWebRevocations(ctx context.Context, pool *pgxpool.Pool, retention time.Duration, batch int) (int64, error) {
	if pool == nil || retention < 24*time.Hour || retention > 365*24*time.Hour || batch < 1 || batch > 1000 {
		return 0, ErrStore
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var owner bool
	if pool.QueryRow(ctx, `SELECT current_user=pg_get_userbyid(relowner) FROM pg_class WHERE oid='auth.web_revocations'::regclass`).Scan(&owner) != nil || !owner {
		return 0, ErrStore
	}
	sql, err := migrations.ReadFile("migrations/003_web_revocations.sql")
	if err != nil {
		return 0, ErrStore
	}
	sum := sha256.Sum256(sql)
	var checksum string
	if pool.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=3").Scan(&checksum) != nil || checksum != hex.EncodeToString(sum[:]) {
		return 0, ErrStore
	}
	tag, err := pool.Exec(ctx, `DELETE FROM auth.web_revocations WHERE job_id IN
 (SELECT job_id FROM auth.web_revocations WHERE state='done' AND updated_at<clock_timestamp()-($1::bigint*interval '1 second') ORDER BY updated_at,job_id LIMIT $2 FOR UPDATE SKIP LOCKED)`, int64(retention.Seconds()), batch)
	if err != nil {
		return 0, ErrStore
	}
	return tag.RowsAffected(), nil
}
