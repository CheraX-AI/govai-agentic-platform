package postgres

import (
	"context"
	"testing"
)

func TestRejectUnsafeDatabaseURLs(t *testing.T) {
	for _, dsn := range []string{
		"", "host=127.0.0.1 user=app", "postgres://db.example/govai?sslmode=disable",
		"postgres://db.example/govai?sslmode=require", "postgres://127.0.0.1/govai",
		"postgres://127.0.0.1/govai?sslmode=disable&host=db.example",
		"postgres://127.0.0.1/govai?sslmode=disable&sslmode=verify-full",
		"postgres://127.0.0.1/govai?sslmode=disable#fragment",
	} {
		if p, e := Open(context.Background(), dsn); e == nil {
			p.Close()
			t.Fatal("unsafe database URL accepted")
		}
	}
}
