//go:build integration

package database

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Run against a local server where internal/database/setup/create_database.sql has
// created the roles. The login must be able to create databases and SET ROLE
// govai_core_app; each test uses its own throwaway database and drops it:
//
//	CORE_TEST_ADMIN_DATABASE_URL=postgres://postgres:...@127.0.0.1:5432/postgres?sslmode=disable \
//	  go test -tags integration ./internal/database/
func TestMigrateEmptyDatabase(t *testing.T) {
	ctx, pool := freshDatabase(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal("migration of an empty database failed")
	}

	if err := Migrate(ctx, pool); err != nil {
		t.Fatal("a second migrate run must be a no-op")
	}

	checkVersions(t, ctx, pool)
	checkNoConversationData(t, ctx, pool)
}

func TestMigrateDropsExistingConversations(t *testing.T) {
	ctx, pool := freshDatabase(t)
	scripts, err := shippedMigrations()
	if err != nil {
		t.Fatal(err)
	}

	// A database from before the move, holding a conversation with one exchange
	if err := apply(ctx, pool, scripts[:3]); err != nil {
		t.Fatal("001-003 failed")
	}

	conversation, question := uuid(t), uuid(t)
	if _, err := pool.Exec(ctx, "INSERT INTO core.conversations (id, citizen_subject) VALUES ($1, 'synthetic-citizen')", conversation); err != nil {
		t.Fatal("cannot add a conversation")
	}

	if _, err := pool.Exec(ctx, `INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext)
 VALUES ($1, $2, 'citizen', $3)`, question, conversation, sealed(t, 40)); err != nil {
		t.Fatal("cannot add a message")
	}

	if err := Migrate(ctx, pool); err != nil {
		t.Fatal("upgrade of a database with conversations failed")
	}

	checkVersions(t, ctx, pool)
	checkNoConversationData(t, ctx, pool)
}

// checkVersions verifies every shipped migration is recorded with its checksum.
func checkVersions(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	scripts, err := shippedMigrations()
	if err != nil {
		t.Fatal(err)
	}

	for _, script := range scripts {
		var applied string
		if pool.QueryRow(ctx, "SELECT checksum FROM public.govai_core_migrations WHERE version=$1", script.version).
			Scan(&applied) != nil || applied != script.checksum {
			t.Fatalf("migration %d is not recorded unchanged", script.version)
		}
	}
}

// checkNoConversationData verifies the conversation tables and trigger are gone and the schema stays.
func checkNoConversationData(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var tables, functions int
	var schema bool
	if err := pool.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM pg_tables WHERE schemaname = 'core'),
 (SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = 'core'),
 EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'core')`).Scan(&tables, &functions, &schema); err != nil {
		t.Fatal("cannot inspect the schema")
	}

	if tables != 0 || functions != 0 || !schema {
		t.Fatalf("core should keep its schema with no conversation tables (tables=%d functions=%d schema=%v)", tables, functions, schema)
	}

	// The runtime role can still use the schema for core's other features
	if err := asApp(ctx, pool, func(tx pgx.Tx) error {
		var usable bool
		return tx.QueryRow(ctx, "SELECT has_schema_privilege('core', 'USAGE')").Scan(&usable)
	}); err != nil {
		t.Fatalf("runtime role lost access to the core schema: %v", err)
	}
}

// freshDatabase creates an empty throwaway database and drops it after the test.
func freshDatabase(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("CORE_TEST_ADMIN_DATABASE_URL")
	if dsn == "" {
		t.Skip("CORE_TEST_ADMIN_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal("admin connection failed")
	}

	t.Cleanup(admin.Close)
	var roles bool
	if err := admin.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'govai_core_app')").
		Scan(&roles); err != nil || !roles {
		t.Fatal("run internal/database/setup/create_database.sql first: role govai_core_app is missing")
	}

	name := "govai_core_test_" + strings.ReplaceAll(uuid(t)[:13], "-", "")
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal("cannot create a test database")
	}

	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	pool, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal("test database connection failed")
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	})

	return ctx, pool
}

// asApp runs fn as the runtime role and commits only if it succeeds.
func asApp(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SET LOCAL ROLE govai_core_app"); err != nil {
		return err
	}

	if err = fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// sealed returns random bytes shaped like nonce + ciphertext + tag; no real text is stored.
func sealed(t *testing.T, size int) []byte {
	t.Helper()
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}

	return b
}

func uuid(t *testing.T) string {
	t.Helper()
	b := sealed(t, 16)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
