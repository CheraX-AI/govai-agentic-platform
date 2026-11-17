// Package database connects core-service to PostgreSQL, applies its migrations,
// and checks at startup that the schema is ready.
package database

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"io/fs"
	"net"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationFiles holds migrations/NNN_description.sql, applied in name order.
// Never edit a migration that has been applied; add a new file instead.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// ErrDatabase hides connection and SQL details; they can hold credentials or data.
var ErrDatabase = errors.New("core database unavailable")

// migrationLock serialises concurrent migrate runs (an arbitrary, fixed key).
const migrationLock = 71480261032

// Open accepts explicit URL connection strings only. Unencrypted connections
// are restricted to loopback; other databases require verify-full TLS.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	u, err := url.Parse(dsn)
	if err != nil ||
		(u.Scheme != "postgres" && u.Scheme != "postgresql") ||
		u.Hostname() == "" ||
		u.Fragment != "" {
		return nil, ErrDatabase
	}

	mode := u.Query().Get("sslmode")
	for key, values := range u.Query() {
		if (key != "sslmode" && key != "sslrootcert") || len(values) != 1 {
			return nil, ErrDatabase
		}
	}

	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if mode != "verify-full" && !(local && mode == "disable") {
		return nil, ErrDatabase
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, ErrDatabase
	}

	if cfg.ConnConfig.Host != u.Hostname() ||
		(mode == "verify-full" && (cfg.ConnConfig.TLSConfig == nil || cfg.ConnConfig.TLSConfig.InsecureSkipVerify)) {
		return nil, ErrDatabase
	}

	cfg.MaxConns = 8
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.RuntimeParams["application_name"] = "govai-core"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	// Do not install pgx tracers: SQL parameters hold citizen identifiers.
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, ErrDatabase
	}

	ping, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if pool.Ping(ping) != nil {
		pool.Close()
		return nil, ErrDatabase
	}

	return pool, nil
}

// Migrate applies every NNN_*.sql file not yet applied, in one transaction.
// It is run explicitly by cmd/migrate with the owner login; cmd/api never
// changes the schema. An applied file whose checksum changed is an error.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	scripts, err := shippedMigrations()
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return ErrDatabase
	}

	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", migrationLock); err != nil {
		return ErrDatabase
	}

	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.govai_core_migrations
 (version integer PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT clock_timestamp());
 REVOKE ALL ON public.govai_core_migrations FROM PUBLIC;
 GRANT SELECT ON public.govai_core_migrations TO govai_core_app;`); err != nil {
		return ErrDatabase
	}

	for _, script := range scripts {
		var applied string
		err = tx.QueryRow(ctx, "SELECT checksum FROM public.govai_core_migrations WHERE version=$1", script.version).
			Scan(&applied)
		switch {
		case err == nil:
			if applied != script.checksum {
				return ErrDatabase
			}

		case errors.Is(err, pgx.ErrNoRows):
			if _, err = tx.Exec(ctx, script.sql); err != nil {
				return ErrDatabase
			}

			if _, err = tx.Exec(
				ctx,
				"INSERT INTO public.govai_core_migrations(version,checksum) VALUES($1,$2)",
				script.version,
				script.checksum,
			); err != nil {
				return ErrDatabase
			}

		default:
			return ErrDatabase
		}
	}

	if tx.Commit(ctx) != nil {
		return ErrDatabase
	}

	return nil
}

// Ready checks, with the runtime login, that it is not a superuser and that
// every embedded migration is applied unchanged. cmd/api refuses to start otherwise.
func Ready(ctx context.Context, pool *pgxpool.Pool) error {
	scripts, err := shippedMigrations()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var restricted bool
	if pool.QueryRow(ctx, `SELECT NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolbypassrls
 FROM pg_roles WHERE rolname = current_user`).Scan(&restricted) != nil || !restricted {
		return ErrDatabase
	}

	for _, script := range scripts {
		var applied string
		if pool.QueryRow(ctx, "SELECT checksum FROM public.govai_core_migrations WHERE version=$1", script.version).
			Scan(&applied) != nil || applied != script.checksum {
			return ErrDatabase
		}
	}

	return nil
}

type migration struct {
	version  int
	checksum string
	sql      string
}

// shippedMigrations loads the migrations embedded in this package.
func shippedMigrations() ([]migration, error) {
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return nil, ErrDatabase
	}

	return load(files)
}

// load reads the NNN_*.sql files in name order and checks their numbering.
func load(files fs.FS) ([]migration, error) {
	names, err := fs.Glob(files, "*.sql")
	if err != nil || len(names) == 0 {
		return nil, ErrDatabase
	}

	sort.Strings(names)
	var scripts []migration
	for index, name := range names {
		if len(name) < 5 || name[3] != '_' {
			return nil, ErrDatabase
		}

		version, err := strconv.Atoi(name[:3])
		if err != nil || version != index+1 {
			return nil, ErrDatabase
		}

		sql, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, ErrDatabase
		}

		sum := sha256.Sum256(sql)
		scripts = append(scripts, migration{version: version, checksum: hex.EncodeToString(sum[:]), sql: string(sql)})
	}

	return scripts, nil
}
