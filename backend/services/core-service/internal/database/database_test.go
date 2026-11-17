package database

import (
	"context"
	"errors"
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

func TestShippedMigrationsLoad(t *testing.T) {
	scripts, err := shippedMigrations()
	if err != nil || len(scripts) == 0 || scripts[0].version != 1 || len(scripts[0].checksum) != 64 {
		t.Fatal("shipped migrations are not numbered from 001")
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
