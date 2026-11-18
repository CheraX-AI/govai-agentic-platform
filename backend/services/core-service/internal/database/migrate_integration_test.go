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

	checkSchema(t, ctx, pool)
	checkMessageRules(t, ctx, pool, insertConversation(t, ctx, pool, "synthetic-citizen-a"))
}

func TestMigrateWithExistingConversations(t *testing.T) {
	ctx, pool := freshDatabase(t)
	scripts, err := shippedMigrations()
	if err != nil {
		t.Fatal(err)
	}

	// A database that only has 001 and already holds conversations
	if err := apply(ctx, pool, scripts[:1]); err != nil {
		t.Fatal("001 failed")
	}

	first := insertConversation(t, ctx, pool, "synthetic-citizen-a")
	second := insertConversation(t, ctx, pool, "synthetic-citizen-b")
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal("upgrade of a database with conversations failed")
	}

	var kept int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM core.conversations
 WHERE (id = $1 AND citizen_subject = 'synthetic-citizen-a') OR (id = $2 AND citizen_subject = 'synthetic-citizen-b')`,
		first, second).Scan(&kept); err != nil || kept != 2 {
		t.Fatal("existing conversations or their owners changed during the upgrade")
	}

	checkSchema(t, ctx, pool)
	checkMessageRules(t, ctx, pool, first)
}

// checkSchema verifies the recorded versions, the foreign key and the indexes.
func checkSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	scripts, _ := shippedMigrations()
	var versions int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.govai_core_migrations").Scan(&versions); err != nil ||
		versions != len(scripts) {
		t.Fatalf("recorded %d migrations, shipped %d", versions, len(scripts))
	}

	rows, err := pool.Query(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint
 WHERE conrelid = 'core.conversation_messages'::regclass AND contype = 'f' ORDER BY conname`)
	if err != nil {
		t.Fatal(err)
	}

	foreignKeys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || strings.Join(foreignKeys, "\n") != "FOREIGN KEY (conversation_id) REFERENCES core.conversations(id)\n"+
		"FOREIGN KEY (conversation_id, reply_to) REFERENCES core.conversation_messages(conversation_id, message_id)" {
		t.Fatalf("unexpected foreign keys %q", foreignKeys)
	}

	rows, err = pool.Query(ctx, `SELECT indexdef FROM pg_indexes
 WHERE schemaname = 'core' AND tablename = 'conversation_messages' ORDER BY indexname`)
	if err != nil {
		t.Fatal(err)
	}

	indexes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(indexes, "\n")
	for _, want := range []string{
		"UNIQUE INDEX conversation_messages_order ON core.conversation_messages USING btree (conversation_id, ordinal)",
		"UNIQUE INDEX conversation_messages_pkey ON core.conversation_messages USING btree (message_id)",
		"UNIQUE INDEX conversation_messages_reply_to_key ON core.conversation_messages USING btree (reply_to)",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing index %q in:\n%s", want, joined)
		}
	}

	// Ordered reads of one conversation can come straight from the index with no
	// sort. A new table has no statistics, so the other plans are switched off.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SET LOCAL enable_seqscan = off; SET LOCAL enable_bitmapscan = off; SET LOCAL enable_sort = off"); err != nil {
		t.Fatal(err)
	}

	rows, err = tx.Query(ctx, `EXPLAIN (COSTS OFF) SELECT message_id FROM core.conversation_messages
 WHERE conversation_id = '3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34' ORDER BY ordinal`)
	if err != nil {
		t.Fatal(err)
	}

	lines, err := pgx.CollectRows(rows, pgx.RowTo[string])
	plan := strings.Join(lines, "\n")
	if err != nil || !strings.Contains(plan, "conversation_messages_order") || strings.Contains(plan, "Sort") {
		t.Fatalf("ordered read does not use conversation_messages_order:\n%s", plan)
	}
}

// checkMessageRules exercises the table as the runtime role.
func checkMessageRules(t *testing.T, ctx context.Context, pool *pgxpool.Pool, conversationID string) {
	t.Helper()
	question, answer := uuid(t), uuid(t)
	if err := asApp(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO core.conversation_messages
 (message_id, conversation_id, role, content_ciphertext) VALUES ($1, $2, 'citizen', $3)`,
			question, conversationID, sealed(t, 40)); err != nil {
			return err
		}

		_, err := tx.Exec(ctx, `INSERT INTO core.conversation_messages
 (message_id, conversation_id, role, content_ciphertext, status, reply_to) VALUES ($1, $2, 'assistant', $3, 'completed', $4)`,
			answer, conversationID, sealed(t, 120), question)
		return err
	}); err != nil {
		t.Fatalf("runtime role cannot add messages: %v", err)
	}

	// Messages come back in the order they were added, defaults applied
	var order []string
	var status string
	if err := asApp(ctx, pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT m.message_id::text FROM core.conversation_messages m
 JOIN core.conversations c ON c.id = m.conversation_id
 WHERE m.conversation_id = $1 AND c.citizen_subject = 'synthetic-citizen-a' ORDER BY m.ordinal`, conversationID)
		if err != nil {
			return err
		}

		if order, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
			return err
		}

		return tx.QueryRow(ctx, "SELECT status FROM core.conversation_messages WHERE message_id = $1", question).Scan(&status)
	}); err != nil || len(order) != 2 || order[0] != question || order[1] != answer || status != "pending" {
		t.Fatalf("ordered read failed: %v %v %q", err, order, status)
	}

	// A pending message may complete once; the trigger moves updated_at
	var moved bool
	if err := asApp(ctx, pool, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `UPDATE core.conversation_messages SET status = 'completed'
 WHERE message_id = $1 RETURNING updated_at > created_at`, question).Scan(&moved)
	}); err != nil || !moved {
		t.Fatalf("pending message could not complete: %v", err)
	}

	for name, statement := range map[string]string{
		"unknown conversation":    fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext) VALUES ('%s', '%s', 'citizen', '\x%s')`, uuid(t), uuid(t), strings.Repeat("ab", 40)),
		"unknown role":            fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext) VALUES ('%s', '%s', 'system', '\x%s')`, uuid(t), conversationID, strings.Repeat("ab", 40)),
		"unknown status":          fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext, status) VALUES ('%s', '%s', 'citizen', '\x%s', 'sent')`, uuid(t), conversationID, strings.Repeat("ab", 40)),
		"too short to be sealed":  fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext) VALUES ('%s', '%s', 'citizen', '\x%s')`, uuid(t), conversationID, strings.Repeat("ab", 28)),
		"duplicate message_id":    fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext) VALUES ('%s', '%s', 'citizen', '\x%s')`, question, conversationID, strings.Repeat("ab", 40)),
		"chosen ordinal":          fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, ordinal, role, content_ciphertext) OVERRIDING SYSTEM VALUE VALUES ('%s', '%s', 1, 'citizen', '\x%s')`, uuid(t), conversationID, strings.Repeat("ab", 40)),
		"answer without question": fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext, status) VALUES ('%s', '%s', 'assistant', '\x%s', 'completed')`, uuid(t), conversationID, strings.Repeat("ab", 40)),
		"second answer":           fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext, status, reply_to) VALUES ('%s', '%s', 'assistant', '\x%s', 'completed', '%s')`, uuid(t), conversationID, strings.Repeat("ab", 40), question),
		"pending answer":          fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext, reply_to) VALUES ('%s', '%s', 'assistant', '\x%s', '%s')`, uuid(t), conversationID, strings.Repeat("ab", 40), question),
		"answer to unknown":       fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext, status, reply_to) VALUES ('%s', '%s', 'assistant', '\x%s', 'completed', '%s')`, uuid(t), conversationID, strings.Repeat("ab", 40), uuid(t)),
		"citizen with reply_to":   fmt.Sprintf(`INSERT INTO core.conversation_messages (message_id, conversation_id, role, content_ciphertext, reply_to) VALUES ('%s', '%s', 'citizen', '\x%s', '%s')`, uuid(t), conversationID, strings.Repeat("ab", 40), question),
		"final status reopened":   fmt.Sprintf(`UPDATE core.conversation_messages SET status = 'pending' WHERE message_id = '%s'`, answer),
		"content rewritten":       fmt.Sprintf(`UPDATE core.conversation_messages SET content_ciphertext = '\x%s' WHERE message_id = '%s'`, strings.Repeat("cd", 40), answer),
		"message moved":           fmt.Sprintf(`UPDATE core.conversation_messages SET conversation_id = conversation_id WHERE message_id = '%s'`, answer),
		"message deleted":         fmt.Sprintf(`DELETE FROM core.conversation_messages WHERE message_id = '%s'`, answer),
		"conversation deleted":    fmt.Sprintf(`DELETE FROM core.conversations WHERE id = '%s'`, conversationID),
		"owner rewritten":         fmt.Sprintf(`UPDATE core.conversations SET citizen_subject = 'synthetic-citizen-b' WHERE id = '%s'`, conversationID),
	} {
		err := asApp(ctx, pool, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, statement); return err })
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}

		t.Logf("%s: rejected (%v)", name, err)
	}

	// Even the owner cannot delete a conversation that still has messages
	if _, err := pool.Exec(ctx, "DELETE FROM core.conversations WHERE id = $1", conversationID); err == nil {
		t.Fatal("foreign key did not protect the conversation's messages")
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

func insertConversation(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner string) string {
	t.Helper()
	id := uuid(t)
	if err := asApp(ctx, pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO core.conversations (id, citizen_subject) VALUES ($1, $2)", id, owner)
		return err
	}); err != nil {
		t.Fatalf("runtime role cannot create a conversation: %v", err)
	}

	return id
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
