//go:build integration

package conversation_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/conversation"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/database"
)

// Run against a migrated local database with the runtime login:
//
//	CORE_TEST_DATABASE_URL=postgres://govai_core_app:...@127.0.0.1:5432/govai_core?sslmode=disable \
//	  go test -tags integration ./internal/conversation/
func TestPostgresConversationOwnership(t *testing.T) {
	dsn := os.Getenv("CORE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CORE_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}

	defer pool.Close()
	if err := database.Ready(ctx, pool); err != nil {
		t.Fatal("database not migrated or runtime login too privileged")
	}

	store, err := conversation.NewPostgresStore(pool)
	if err != nil {
		t.Fatal(err)
	}

	service, err := conversation.NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	// Unique synthetic owners, so the test can run repeatedly (rows cannot be deleted)
	suffix := time.Now().Format("20060102150405.000000000")
	citizenA, citizenB := "synthetic-a-"+suffix, "synthetic-b-"+suffix

	id, err := service.Resolve(ctx, citizenA, "")
	if err != nil {
		t.Fatal(err)
	}

	if again, err := service.Resolve(ctx, citizenA, id); err != nil || again != id {
		t.Fatal("existing conversation not found for its owner")
	}

	if _, err := service.Resolve(ctx, citizenB, id); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatal("another citizen reached the conversation")
	}

	if _, err := service.Resolve(ctx, citizenA, "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatal("unknown conversation accepted")
	}

	// The runtime role may not rewrite ownership
	if _, err := pool.Exec(ctx, "UPDATE core.conversations SET citizen_subject = $1 WHERE id = $2", citizenB, id); err == nil {
		t.Fatal("runtime role can change conversation owners")
	}
}
