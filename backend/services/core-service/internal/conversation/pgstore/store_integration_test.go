//go:build integration

package pgstore_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/conversation"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/conversation/pgstore"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Run against a migrated database with the runtime login:
//
//	CORE_TEST_DATABASE_URL=postgres://govai_core_app:...@127.0.0.1:5432/govai_core?sslmode=disable \
//	  go test -tags integration ./internal/conversation/pgstore/

// Synthetic key and texts only.
var key = bytes.Repeat([]byte{0x11}, 32)

const (
	question = "Synthetic question about passport renewal"
	answer   = "Synthetic answer only."
)

type live struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	service  *conversation.Service
	messages *conversation.Messages
	owner    string
	id       string
}

// connect opens the database and starts a conversation for a unique synthetic owner (rows cannot be deleted).
func connect(t *testing.T, inFlight time.Duration) live {
	t.Helper()
	dsn := os.Getenv("CORE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CORE_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)
	if err := database.Ready(ctx, pool); err != nil {
		t.Fatal("database not migrated or runtime login too privileged")
	}

	store, err := pgstore.New(pool)
	if err != nil {
		t.Fatal(err)
	}

	service, err := conversation.NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	messages, err := conversation.NewMessages(store, key, inFlight)
	if err != nil {
		t.Fatal(err)
	}

	owner := "synthetic-" + time.Now().Format("20060102150405.000000000")
	id, err := service.Resolve(ctx, owner, "")
	if err != nil {
		t.Fatal(err)
	}

	return live{ctx, pool, service, messages, owner, id}
}

type row struct {
	id, role, status, replyTo string
	sealed                    []byte
}

func (l live) rows(t *testing.T) []row {
	t.Helper()
	rows, err := l.pool.Query(l.ctx, `SELECT message_id::text, role, status, coalesce(reply_to::text, ''), content_ciphertext
 FROM core.conversation_messages WHERE conversation_id = $1 ORDER BY ordinal`, l.id)
	if err != nil {
		t.Fatal(err)
	}

	result, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var m row
		return m, r.Scan(&m.id, &m.role, &m.status, &m.replyTo, &m.sealed)
	})
	if err != nil {
		t.Fatal(err)
	}

	return result
}

// open decrypts a row independently of the service: nonce || AES-256-GCM ciphertext || tag.
func open(t *testing.T, conversationID string, m row) string {
	t.Helper()
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	aad, _ := json.Marshal([]string{"govai-message-v1", conversationID, m.id, m.role})
	plain, err := aead.Open(nil, m.sealed[:12], m.sealed[12:], aad)
	if err != nil {
		t.Fatalf("stored %s message does not decrypt", m.role)
	}

	return string(plain)
}

func TestConversationOwnership(t *testing.T) {
	l := connect(t, time.Minute)
	if again, err := l.service.Resolve(l.ctx, l.owner, l.id); err != nil || again != l.id {
		t.Fatal("existing conversation not found for its owner")
	}

	if _, err := l.service.Resolve(l.ctx, l.owner+"-other", l.id); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatal("another citizen reached the conversation")
	}

	if _, err := l.service.Resolve(l.ctx, l.owner, "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatal("unknown conversation accepted")
	}

	if _, err := l.pool.Exec(l.ctx, "UPDATE core.conversations SET citizen_subject = $1 WHERE id = $2", l.owner+"-other", l.id); err == nil {
		t.Fatal("runtime role can change conversation owners")
	}
}

func TestExchangeIsStoredEncrypted(t *testing.T) {
	l := connect(t, time.Minute)
	turn, err := l.messages.Start(l.ctx, l.owner, l.id, question)
	if err != nil {
		t.Fatal(err)
	}

	if rows := l.rows(t); len(rows) != 1 || rows[0].status != "pending" {
		t.Fatalf("question not stored as pending: %+v", rows)
	}

	if err := l.messages.Complete(l.ctx, turn, answer); err != nil {
		t.Fatal(err)
	}

	rows := l.rows(t)
	if len(rows) != 2 || rows[0].role != "citizen" || rows[0].status != "completed" ||
		rows[1].role != "assistant" || rows[1].status != "completed" || rows[1].replyTo != rows[0].id {
		t.Fatalf("unexpected rows: %+v", rows)
	}

	for i, want := range []string{question, answer} {
		if bytes.Contains(rows[i].sealed, []byte("Synthetic")) || open(t, l.id, rows[i]) != want {
			t.Fatalf("stored %s is plaintext or does not decrypt to the original", rows[i].role)
		}
	}

	retry, err := l.messages.Start(l.ctx, l.owner, l.id, question)
	if got, replayed := retry.Answered(); err != nil || !replayed || got != answer || len(l.rows(t)) != 2 {
		t.Fatal("retry of an answered question duplicated the exchange")
	}
}

func TestFailedCallIsRetriedInPlace(t *testing.T) {
	l := connect(t, time.Minute)
	turn, err := l.messages.Start(l.ctx, l.owner, l.id, question)
	if err != nil {
		t.Fatal(err)
	}

	if err := l.messages.Fail(l.ctx, turn); err != nil {
		t.Fatal(err)
	}

	if rows := l.rows(t); len(rows) != 1 || rows[0].status != "failed" {
		t.Fatalf("failed call not recorded: %+v", rows)
	}

	retry, err := l.messages.Start(l.ctx, l.owner, l.id, question)
	if err != nil {
		t.Fatal(err)
	}

	if err := l.messages.Complete(l.ctx, retry, answer); err != nil {
		t.Fatal(err)
	}

	if rows := l.rows(t); len(rows) != 2 || rows[0].status != "completed" || rows[1].replyTo != rows[0].id {
		t.Fatalf("retry added a turn: %+v", rows)
	}

	if l.messages.Fail(l.ctx, retry) == nil || l.messages.Complete(l.ctx, retry, "Second synthetic answer.") == nil {
		t.Fatal("completed question changed")
	}
}

func TestOnlyTheOwnerCanAddMessages(t *testing.T) {
	l := connect(t, time.Minute)
	if _, err := l.messages.Start(l.ctx, l.owner+"-other", l.id, question); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatal("another citizen wrote to the conversation")
	}

	if len(l.rows(t)) != 0 {
		t.Fatal("message stored for another citizen")
	}
}

func TestConcurrentDuplicatesMakeOneTurn(t *testing.T) {
	l := connect(t, time.Minute)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := l.messages.Start(l.ctx, l.owner, l.id, question)
			results <- err
		}()
	}

	wg.Wait()
	close(results)
	var started, busy int
	for err := range results {
		switch {
		case err == nil:
			started++
		case errors.Is(err, conversation.ErrInProgress):
			busy++
		default:
			t.Fatal(err)
		}
	}

	if started != 1 || busy != 7 || len(l.rows(t)) != 1 {
		t.Fatalf("started=%d busy=%d rows=%d", started, busy, len(l.rows(t)))
	}
}

func TestLostRequestCanBeRetried(t *testing.T) {
	l := connect(t, 500*time.Millisecond)
	lost, err := l.messages.Start(l.ctx, l.owner, l.id, question)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(700 * time.Millisecond)
	retry, err := l.messages.Start(l.ctx, l.owner, l.id, question)
	if err != nil {
		t.Fatal("stale question blocked the retry")
	}

	if err := l.messages.Complete(l.ctx, retry, answer); err != nil {
		t.Fatal(err)
	}

	// The lost request finishing late cannot add a second answer
	if l.messages.Complete(l.ctx, lost, "Late synthetic answer.") == nil || len(l.rows(t)) != 2 {
		t.Fatal("question answered twice")
	}
}

// reconnect opens a new pool, store and service, as after a service restart.
func (l live) reconnect(t *testing.T) live {
	t.Helper()
	pool, err := database.Open(l.ctx, os.Getenv("CORE_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)
	store, err := pgstore.New(pool)
	if err != nil {
		t.Fatal(err)
	}

	messages, err := conversation.NewMessages(store, key, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	l.pool, l.messages = pool, messages
	return l
}

func TestRecentAfterReconnectIsOrderedAndCompleteOnly(t *testing.T) {
	l := connect(t, time.Minute)
	for _, n := range []string{"1", "2", "3"} {
		turn, err := l.messages.Start(l.ctx, l.owner, l.id, "Synthetic question "+n)
		if err != nil {
			t.Fatal(err)
		}

		if err := l.messages.Complete(l.ctx, turn, "Synthetic answer "+n); err != nil {
			t.Fatal(err)
		}

		// A failed question after each turn must never be read back
		failed, err := l.messages.Start(l.ctx, l.owner, l.id, "Synthetic failed "+n)
		if err != nil || l.messages.Fail(l.ctx, failed) != nil {
			t.Fatal("failed question not recorded")
		}
	}

	if _, err := l.messages.Start(l.ctx, l.owner, l.id, "Synthetic pending"); err != nil {
		t.Fatal(err)
	}

	// Only ciphertext is stored
	for _, r := range l.rows(t) {
		if bytes.Contains(r.sealed, []byte("Synthetic")) {
			t.Fatal("plaintext stored")
		}
	}

	l.pool.Close()
	l = l.reconnect(t)
	got, err := l.messages.Recent(l.ctx, l.owner, l.id, 10)
	if err != nil {
		t.Fatal(err)
	}

	var texts []string
	for _, m := range got {
		texts = append(texts, m.Role+": "+m.Text)
	}

	want := []string{
		"citizen: Synthetic question 1", "assistant: Synthetic answer 1",
		"citizen: Synthetic question 2", "assistant: Synthetic answer 2",
		"citizen: Synthetic question 3", "assistant: Synthetic answer 3",
	}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Fatalf("recent window after reconnect:\n got %q\nwant %q", texts, want)
	}

	// The window keeps the newest turns
	got, err = l.messages.Recent(l.ctx, l.owner, l.id, 1)
	if err != nil || len(got) != 2 || got[0].Text != "Synthetic question 3" || got[1].Text != "Synthetic answer 3" {
		t.Fatalf("limit not applied: %+v %v", got, err)
	}
}

func TestRecentChecksOwnership(t *testing.T) {
	l := connect(t, time.Minute)
	turn, err := l.messages.Start(l.ctx, l.owner, l.id, question)
	if err != nil || l.messages.Complete(l.ctx, turn, answer) != nil {
		t.Fatal("exchange not stored")
	}

	if got, err := l.messages.Recent(l.ctx, l.owner+"-other", l.id, 10); !errors.Is(err, conversation.ErrNotFound) || got != nil {
		t.Fatal("another citizen read the conversation")
	}

	if _, err := l.messages.Recent(l.ctx, l.owner, "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34", 10); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatal("unknown conversation readable")
	}
}
