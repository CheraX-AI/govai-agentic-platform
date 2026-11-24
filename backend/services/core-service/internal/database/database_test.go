package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func TestOpenRejectsUnsafeConnectionStrings(t *testing.T) {
	for _, dsn := range []string{
		"",
		"host=127.0.0.1 dbname=govai_core",
		"mysql://user@127.0.0.1/govai_core",
		"postgres://user@db.example.test/govai_core",
		"postgres://user@db.example.test/govai_core?sslmode=disable",
		"postgres://user@db.example.test/govai_core?sslmode=require",
		"postgres://user@127.0.0.1/govai_core?sslmode=disable&options=-c%20role%3Dpostgres",
		"postgres://user@127.0.0.1/govai_core?sslmode=disable#fragment",
	} {
		if _, err := Open(context.Background(), dsn); !errors.Is(err, ErrDatabase) {
			t.Fatalf("unsafe connection string accepted: %q", dsn)
		}
	}
}

// appliedChecksums pins every shipped migration: an applied file must never change,
// because databases that ran it would then refuse to start. Add new files instead.
var appliedChecksums = []string{
	"124f58984a0e06253d4819ba2024536ec56b6410cd8013f3e8d9978f7e79ac81", // 001_conversations.sql
	"43860f9d5e2ad7e885a9962d08a9c381fb30d8653384d970c81915302b14a95d", // 002_conversation_messages.sql
	"e6ba1e65f99cd906881bbdbbff37a335e01165eed30579255fb26e2e3f880150", // 003_message_turns.sql
	"619eee60d13cf0ac8d5df8b80d433b2cbebc90c652e9cb3389734a3e43d3fd87", // 004_drop_conversations.sql
}

func TestShippedMigrationsAreUnchanged(t *testing.T) {
	scripts, err := shippedMigrations()
	if err != nil || len(scripts) != len(appliedChecksums) {
		t.Fatalf("expected %d shipped migrations; pin the new file's checksum here", len(appliedChecksums))
	}

	for index, script := range scripts {
		if script.version != index+1 || script.checksum != appliedChecksums[index] {
			t.Fatalf("migration %03d changed after it was shipped; add a new migration instead", index+1)
		}
	}
}

func TestChecksumIgnoresLineEndings(t *testing.T) {
	lf, err := load(fstest.MapFS{"001_a.sql": {Data: []byte("SELECT 1;\nSELECT 2;\n")}})
	if err != nil {
		t.Fatal(err)
	}

	crlf, err := load(fstest.MapFS{"001_a.sql": {Data: []byte("SELECT 1;\r\nSELECT 2;\r\n")}})
	if err != nil || crlf[0].checksum != lf[0].checksum {
		t.Fatal("a Windows checkout and a Linux build disagree on the migration checksum")
	}
}

// The message table stores ciphertext only, keeps ownership on conversations and
// never lets the runtime role delete or rewrite messages.
func TestMessageMigrationKeepsContentEncryptedAndImmutable(t *testing.T) {
	scripts, err := shippedMigrations()
	if err != nil || len(scripts) < 2 {
		t.Fatal("message migration missing")
	}

	// Check the statements only, not the comments
	var statements []string
	for _, line := range strings.Split(strings.ToLower(scripts[1].sql), "\n") {
		code, _, _ := strings.Cut(line, "--")
		statements = append(statements, strings.Join(strings.Fields(code), " "))
	}

	sql := strings.Join(statements, " ")
	for _, want := range []string{
		"content_ciphertext bytea not null",
		"references core.conversations (id)",
		"unique (conversation_id, ordinal)",
		"revoke all on core.conversation_messages from public",
		"grant update (status) on core.conversation_messages to govai_core_app",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("message migration lacks %q", want)
		}
	}

	for _, forbidden := range []string{"content text", "citizen_subject", "token", "delete", "grant all", "grant update on"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("message migration must not contain %q", forbidden)
		}
	}
}

func TestMigrationNumberingMustBeContiguous(t *testing.T) {
	for name, files := range map[string]fstest.MapFS{
		"none":      {},
		"gap":       {"001_a.sql": {Data: []byte("SELECT 1")}, "003_c.sql": {Data: []byte("SELECT 1")}},
		"no number": {"conversations.sql": {Data: []byte("SELECT 1")}},
		"bad name":  {"001-a.sql": {Data: []byte("SELECT 1")}},
	} {
		if _, err := load(files); !errors.Is(err, ErrDatabase) {
			t.Fatalf("%s: invalid migration set accepted", name)
		}
	}
}
