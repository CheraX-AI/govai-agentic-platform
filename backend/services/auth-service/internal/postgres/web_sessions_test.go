package postgres

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestWebSessionEncryption(t *testing.T) {
	// Cryptographic unit tests need no database connection.
	c, err := newCryptor(bytes.Repeat([]byte{7}, 32), bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatal(err)
	}
	s := &WebSessionStore{aead: c.aead}
	credentials := WebCredentials{Issuer: "https://issuer.test", Subject: "synthetic-subject", AccessToken: "synthetic-access", RefreshToken: "synthetic-refresh", CSRF: base64.RawURLEncoding.EncodeToString(make([]byte, 32))}
	hash := bytes.Repeat([]byte{3}, 32)
	first, err := s.seal(credentials, hash)
	if err != nil {
		t.Fatal("seal failed")
	}
	second, _ := s.seal(credentials, hash)
	if bytes.Equal(first, second) || bytes.Contains(first, []byte("synthetic-refresh")) {
		t.Fatal("encryption failed")
	}
	if _, err = s.open(first, bytes.Repeat([]byte{4}, 32)); err == nil {
		t.Fatal("row binding bypass")
	}
	first[len(first)-1] ^= 1
	if _, err = s.open(first, hash); err == nil {
		t.Fatal("tampering accepted")
	}
	b, _ := json.Marshal(credentials)
	if bytes.Contains(b, []byte("synthetic")) || bytes.Contains([]byte(fmt.Sprintf("%+v", credentials)), []byte("synthetic")) {
		t.Fatal("redaction failed")
	}
}

func TestWebSessionDatabase(t *testing.T) {
	a, b, admin := testDatabase(t)
	ctx := context.Background()
	if MigrateWebSessions(ctx, admin) != nil || MigrateWebSessions(ctx, admin) != nil {
		t.Fatal("web migration replay failed")
	}
	if WebSessionsReady(ctx, a.pool) != nil {
		t.Fatal("web readiness failed")
	}
	key := bytes.Repeat([]byte{9}, 32)
	one, err := NewWebSessionStore(a.pool, key)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := NewWebSessionStore(b.pool, key)
	credentials := WebCredentials{Issuer: "https://issuer.test", Subject: "synthetic-subject", AccessToken: "synthetic-access", RefreshToken: "synthetic-refresh", CSRF: base64.RawURLEncoding.EncodeToString(make([]byte, 32))}
	raw, err := one.Create(ctx, credentials, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal("create failed")
	}
	hash, _ := webHash(raw)
	var storedHash, stored []byte
	if admin.QueryRow(ctx, "SELECT session_hash,ciphertext FROM auth.web_sessions WHERE session_hash=$1", hash).Scan(&storedHash, &stored) != nil {
		t.Fatal("row missing")
	}
	if bytes.Equal(storedHash, []byte(raw)) || bytes.Contains(stored, []byte(credentials.RefreshToken)) {
		t.Fatal("plaintext credential storage")
	}
	wrong, _ := NewWebSessionStore(b.pool, bytes.Repeat([]byte{10}, 32))
	if _, err = wrong.Load(ctx, raw); err == nil {
		t.Fatal("wrong key accepted")
	}
	loaded, err := two.Load(ctx, raw)
	if err != nil || loaded.Credentials.RefreshToken != credentials.RefreshToken {
		t.Fatal("independent instance load failed")
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	var checkpoint WebSnapshot
	for _, store := range []*WebSessionStore{one, two} {
		wg.Add(1)
		go func(store *WebSessionStore) {
			defer wg.Done()
			snap, e := store.BeginRefresh(ctx, raw)
			if e == nil {
				mu.Lock()
				wins++
				checkpoint = snap
				mu.Unlock()
			}
		}(store)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatal("concurrent refresh claim failed")
	}
	if _, err = two.Load(ctx, raw); err == nil {
		t.Fatal("in-progress session accepted")
	}
	if _, err = two.BeginRefresh(ctx, raw); err == nil {
		t.Fatal("restart replay allowed")
	}
	changed := credentials
	changed.Subject = "other"
	if one.FinishRefresh(ctx, raw, checkpoint, changed) == nil {
		t.Fatal("identity changed")
	}
	next := credentials
	next.RefreshToken = "synthetic-next"
	if one.FinishRefresh(ctx, raw, checkpoint, next) != nil {
		t.Fatal("refresh commit failed")
	}
	if two.FinishRefresh(ctx, raw, checkpoint, credentials) == nil {
		t.Fatal("stale refresh accepted")
	}
	loaded, err = two.Load(ctx, raw)
	if err != nil || loaded.Credentials.RefreshToken != "synthetic-next" {
		t.Fatal("rotated credential not durable")
	}
	checkpoint, err = one.BeginRefresh(ctx, raw)
	if err != nil {
		t.Fatal("claim failed")
	}
	if two.Terminate(ctx, raw) != nil || one.FinishRefresh(ctx, raw, checkpoint, next) == nil {
		t.Fatal("revoked session revived")
	}
	if _, err = one.Load(ctx, raw); err == nil {
		t.Fatal("revoked session loaded")
	}
	if two.Terminate(ctx, raw) != nil {
		t.Fatal("termination not idempotent")
	}
	raw, err = one.Create(ctx, credentials, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal("expiry setup")
	}
	hash, _ = webHash(raw)
	if _, err = admin.Exec(ctx, "UPDATE auth.web_sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE session_hash=$1", hash); err != nil {
		t.Fatal("expiry setup failed")
	}
	if _, err = one.Load(ctx, raw); err == nil {
		t.Fatal("expired session accepted")
	}
	if _, err = a.pool.Exec(ctx, "DELETE FROM auth.web_sessions"); err == nil {
		t.Fatal("runtime delete permission")
	}
	if _, err = a.pool.Exec(ctx, "UPDATE auth.web_sessions SET expires_at=clock_timestamp()+interval '1 day'"); err == nil {
		t.Fatal("runtime expiry extension")
	}
	if _, err = PruneWebSessions(ctx, a.pool, 24*time.Hour, 10); err == nil {
		t.Fatal("runtime pruning allowed")
	}
	if count, err := PruneWebSessions(ctx, admin, 24*time.Hour, 10); err != nil || count != 0 {
		t.Fatal("recent records pruned")
	}
	if _, err = admin.Exec(ctx, "UPDATE auth.web_sessions SET expires_at=clock_timestamp()-interval '2 days' WHERE session_hash=$1", hash); err != nil {
		t.Fatal("cleanup setup")
	}
	if count, err := PruneWebSessions(ctx, admin, 24*time.Hour, 1); err != nil || count != 1 {
		t.Fatal("bounded prune failed")
	}
	if _, err = PruneWebSessions(ctx, admin, time.Hour, 1); err == nil {
		t.Fatal("retention floor ignored")
	}
	if _, err = PruneWebSessions(ctx, admin, 24*time.Hour, 1001); err == nil {
		t.Fatal("batch limit ignored")
	}
	if _, err = two.Load(ctx, base64.RawURLEncoding.EncodeToString(make([]byte, 32))); err != ErrWebSessionMissing {
		t.Fatal("missing session error unsafe")
	}
}
