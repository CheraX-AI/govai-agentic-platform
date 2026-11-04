package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRecoveryDatabase(t *testing.T) {
	a, b, admin := testDatabase(t)
	ctx := context.Background()
	if MigrateRecovery(ctx, admin) != nil {
		t.Fatal("migration/replay failed")
	}
	if MigrateRecovery(ctx, admin) != nil {
		t.Fatal("migration/replay failed")
	}
	one, _ := NewRecoveryStore(a.pool)
	two, _ := NewRecoveryStore(b.pool)
	identity := RecoveryIdentity{Issuer: testIssuer, Subject: "synthetic-recovery"}
	token, err := one.Issue(ctx, identity, time.Minute)
	if err != nil {
		t.Fatal("issue failed")
	}
	h, _ := token.Digest()
	var count int
	if admin.QueryRow(ctx, "SELECT count(*) FROM auth.password_recovery WHERE token_hash=$1", h[:]).Scan(&count) != nil || count != 1 {
		t.Fatal("hash row missing")
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store := one
			if i%2 == 1 {
				store = two
			}
			got, e := store.Claim(ctx, token)
			if e == nil {
				if got.Issuer != identity.Issuer || got.Subject != identity.Subject {
					t.Error("identity changed")
				}
				mu.Lock()
				wins++
				mu.Unlock()
			} else if !errors.Is(e, ErrRecoveryUnavailable) {
				t.Error("unexpected claim failure")
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("claim winners=%d", wins)
	}
	if one.Finish(ctx, token, false) != nil {
		t.Fatal("attention transition failed")
	}
	if _, err = two.Claim(ctx, token); !errors.Is(err, ErrRecoveryUnavailable) {
		t.Fatal("attention token replayed")
	}
	if err = one.Finish(ctx, token, true); !errors.Is(err, ErrRecoveryUnavailable) {
		t.Fatal("attention completed blindly")
	}
	// Expiry is enforced by the database clock, including at the claim boundary.
	expired, err := one.Issue(ctx, identity, time.Minute)
	if err != nil {
		t.Fatal("issue failed")
	}
	eh, _ := expired.Digest()
	// Controlled owner-only fixture adjustment, never application-user credentials.
	if _, err = admin.Exec(ctx, "ALTER TABLE auth.password_recovery DISABLE TRIGGER recovery_transition"); err != nil {
		t.Fatal("fixture setup failed")
	}
	if _, err = admin.Exec(ctx, "UPDATE auth.password_recovery SET created_at=clock_timestamp()-interval '2 minutes',expires_at=clock_timestamp()-interval '1 minute' WHERE token_hash=$1", eh[:]); err != nil {
		t.Fatal("expiry fixture failed")
	}
	if _, err = admin.Exec(ctx, "ALTER TABLE auth.password_recovery ENABLE TRIGGER recovery_transition"); err != nil {
		t.Fatal("fixture reset failed")
	}
	if _, err = one.Claim(ctx, expired); !errors.Is(err, ErrRecoveryUnavailable) {
		t.Fatal("expired token accepted")
	}
	fresh, err := two.Issue(ctx, identity, time.Minute)
	if err != nil {
		t.Fatal("issue failed")
	}
	// Close only the independent adapter, then build a new adapter over the pool.
	reopened, _ := NewRecoveryStore(a.pool)
	if _, err = reopened.Claim(ctx, fresh); err != nil {
		t.Fatal("persisted claim failed")
	}
	if reopened.Finish(ctx, fresh, true) != nil {
		t.Fatal("completion failed")
	}
	if _, err = two.Claim(ctx, fresh); !errors.Is(err, ErrRecoveryUnavailable) {
		t.Fatal("completed token replayed")
	}
	if _, err = a.pool.Exec(ctx, "DELETE FROM auth.password_recovery"); err == nil {
		t.Fatal("runtime delete allowed")
	}
	if _, err = a.pool.Exec(ctx, "UPDATE auth.password_recovery SET expires_at=clock_timestamp()+interval '1 hour'"); err == nil {
		t.Fatal("runtime expiry mutation allowed")
	}
}
