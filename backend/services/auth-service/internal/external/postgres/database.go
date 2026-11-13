package postgres

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open accepts explicit URL connection strings only. Unencrypted connections
// are restricted to loopback; deployed databases require verify-full TLS.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	u, e := url.Parse(dsn)
	if e != nil ||
		(u.Scheme != "postgres" &&
			u.Scheme != "postgresql") ||
		u.Hostname() == "" ||
		u.Fragment != "" {
		return nil, ErrStore
	}

	mode := u.Query().Get("sslmode")
	for key, values := range u.Query() {
		if (key != "sslmode" && key != "sslrootcert") || len(values) != 1 {
			return nil, ErrStore
		}
	}

	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if mode != "verify-full" && !(local && mode == "disable") {
		return nil, ErrStore
	}

	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		return nil, ErrStore
	}

	if cfg.ConnConfig.Host != u.Hostname() ||
		(mode == "verify-full" &&
			(cfg.ConnConfig.TLSConfig == nil ||
				cfg.ConnConfig.TLSConfig.InsecureSkipVerify)) {
		return nil, ErrStore
	}

	cfg.MaxConns = 12
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.RuntimeParams["application_name"] = "govai-auth"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	// Do not install pgx tracers: SQL parameters contain restricted data.
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		return nil, ErrStore
	}

	ping, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if pool.Ping(ping) != nil {
		pool.Close()
		return nil, ErrStore
	}

	return pool, nil
}

// Migrate is explicit: cmd/api never changes schema. One transactional migration
// with a checksum and transaction advisory lock is currently shipped.
func Migrate(ctx context.Context, p *pgxpool.Pool) error {
	tx, e := p.Begin(ctx)
	if e != nil {
		return ErrStore
	}

	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(71480261031)"); e != nil {
		return ErrStore
	}

	if _, e = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.govai_auth_migrations
 (version integer PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT clock_timestamp());
 REVOKE ALL ON public.govai_auth_migrations FROM PUBLIC;
 GRANT USAGE ON SCHEMA public TO govai_auth_app;
 GRANT SELECT ON public.govai_auth_migrations TO govai_auth_app;`); e != nil {
		return ErrStore
	}

	sql, _ := migrations.ReadFile("migrations/001_registration.sql")
	sum := sha256.Sum256(sql)
	checksum := hex.EncodeToString(sum[:])
	var old string
	e = tx.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=1").Scan(&old)
	switch e {
	case nil:
		if old != checksum {
			return ErrStore
		}
	case pgx.ErrNoRows:
		if _, e = tx.Exec(ctx, string(sql)); e != nil {
			return ErrStore
		}
		if _, e = tx.Exec(ctx, "INSERT INTO public.govai_auth_migrations(version,checksum) VALUES(1,$1)", checksum); e != nil {
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

// Ready checks connectivity AND the expected migration checksum, without
// requiring migration credentials or returning database details to the caller.
func Ready(ctx context.Context, p *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var restricted bool
	if p.QueryRow(ctx, `SELECT current_user = 'govai_auth_app'
 AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls
 AND NOT has_schema_privilege(current_user,'auth','CREATE')
 AND NOT has_schema_privilege(current_user,'public','CREATE')
 AND NOT has_table_privilege(current_user,'auth.citizen_profiles','UPDATE,DELETE,TRUNCATE')
 AND NOT has_table_privilege(current_user,'auth.registration_operations','DELETE,TRUNCATE')
 AND NOT has_column_privilege(current_user,'auth.registration_operations','nic_digest','UPDATE')
 AND NOT has_table_privilege(current_user,'public.govai_auth_migrations','INSERT,UPDATE,DELETE,TRUNCATE')
 FROM pg_roles WHERE rolname=current_user`).Scan(&restricted) != nil || !restricted {
		return ErrStore
	}

	var checksum string
	if p.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=1").Scan(&checksum) != nil {
		return ErrStore
	}

	sql, _ := migrations.ReadFile("migrations/001_registration.sql")
	sum := sha256.Sum256(sql)
	if !strings.EqualFold(checksum, hex.EncodeToString(sum[:])) {
		return ErrStore
	}

	return nil
}
