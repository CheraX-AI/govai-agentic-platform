package postgres

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/recovery"
)

func TestRecoverySessionFencesDatabase(t *testing.T) {
	a, b, admin := testDatabase(t)
	ctx := context.Background()
	if MigrateRecoverySessions(ctx, admin) != nil {
		t.Fatal("fence migration/replay failed")
	}
	if MigrateRecoverySessions(ctx, admin) != nil {
		t.Fatal("fence migration/replay failed")
	}
	if RecoveryReady(ctx, a.pool) != nil {
		t.Fatal("recovery readiness failed")
	}
	makeStore := func(s *Store) *RecoveryWebStore {
		web, err := NewWebSessionStore(s.pool, bytes.Repeat([]byte{9}, 32))
		if err != nil {
			t.Fatal(err)
		}
		bound, err := NewBoundWebSessionStore(web, testIssuer, "web-client")
		if err != nil {
			t.Fatal(err)
		}
		guarded, err := NewRecoveryWebStore(bound, bytes.Repeat([]byte{6}, 32))
		if err != nil {
			t.Fatal(err)
		}
		return guarded
	}
	one, two := makeStore(a), makeStore(b)
	credentials := WebCredentials{Issuer: testIssuer, Subject: "subject-1", AccessToken: "synthetic-access", RefreshToken: "synthetic-refresh", CSRF: base64.RawURLEncoding.EncodeToString(make([]byte, 32))}
	expires := time.Now().Add(time.Hour)
	mark, err := one.AuthenticationMark(ctx)
	if err != nil {
		t.Fatal("mark failed")
	}
	cookie, err := one.CreateFenced(ctx, credentials, expires, mark)
	if err != nil {
		t.Fatal("guarded create failed")
	}
	if _, err = two.Load(ctx, cookie); err != nil {
		t.Fatal("guarded load failed")
	}
	pendingLogin, err := one.AuthenticationMark(ctx)
	if err != nil {
		t.Fatal("mark failed")
	}
	snap, err := one.BeginRefresh(ctx, cookie)
	if err != nil {
		t.Fatal("begin refresh failed")
	}
	if two.Revoke(ctx, recovery.Binding{Issuer: testIssuer, Subject: "subject-1"}) != nil {
		t.Fatal("revoke failed")
	}
	if _, err = one.Load(ctx, cookie); !errors.Is(err, ErrWebSessionMissing) {
		t.Fatal("revoked session accepted")
	}
	if err = one.FinishRefresh(ctx, cookie, snap, credentials); err == nil {
		t.Fatal("refresh resurrected revoked session")
	}
	if _, err = one.CreateFenced(ctx, credentials, expires, pendingLogin); err == nil {
		t.Fatal("in-flight old login accepted")
	}
	newer, _ := two.AuthenticationMark(ctx)
	fresh, err := two.CreateFenced(ctx, credentials, expires, newer)
	if err != nil {
		t.Fatal("new authentication rejected")
	}
	if _, err = one.Load(ctx, fresh); err != nil {
		t.Fatal("new session rejected")
	}
	if one.Revoke(ctx, recovery.Binding{Issuer: testIssuer, Subject: "other-subject"}) != nil {
		t.Fatal("other identity revoke failed")
	}
	if _, err = two.Load(ctx, fresh); err != nil {
		t.Fatal("other identity affected")
	}
	legacy, err := one.WebSessionStore.Create(ctx, credentials, expires)
	if err != nil {
		t.Fatal("legacy fixture failed")
	}
	if _, err = two.Load(ctx, legacy); !errors.Is(err, ErrWebSessionMissing) {
		t.Fatal("unbound legacy cookie accepted")
	}
	if _, err = one.Create(ctx, credentials, expires); err == nil {
		t.Fatal("unguarded creation allowed")
	}
	// An already-created identity session must also be rejected after the final
	// reset cutoff, not just requests that were awaiting creation/refresh.
	if one.Revoke(ctx, recovery.Binding{Issuer: testIssuer, Subject: "subject-1"}) != nil {
		t.Fatal("second revoke failed")
	}
	if _, err = two.Load(ctx, fresh); !errors.Is(err, ErrWebSessionMissing) {
		t.Fatal("reset-window session survived")
	}
	// Race refresh completion against cutoff commit through independent pools.
	for i := 0; i < 8; i++ {
		mark, _ := one.AuthenticationMark(ctx)
		raw, err := one.CreateFenced(ctx, credentials, expires, mark)
		if err != nil {
			t.Fatal("race session create")
		}
		snapshot, err := one.BeginRefresh(ctx, raw)
		if err != nil {
			t.Fatal("race refresh start")
		}
		start := make(chan struct{})
		finish := make(chan error, 1)
		revoked := make(chan error, 1)
		go func() { <-start; finish <- one.FinishRefresh(ctx, raw, snapshot, credentials) }()
		go func() {
			<-start
			revoked <- two.Revoke(ctx, recovery.Binding{Issuer: testIssuer, Subject: "subject-1"})
		}()
		close(start)
		_ = <-finish
		if err := <-revoked; err != nil {
			t.Fatal("concurrent revoke failed")
		}
		if _, err := two.Load(ctx, raw); !errors.Is(err, ErrWebSessionMissing) {
			t.Fatal("concurrent refresh resurrected session")
		}
	}
}
