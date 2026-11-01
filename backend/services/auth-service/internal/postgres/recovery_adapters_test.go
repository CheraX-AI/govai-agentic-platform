package postgres

import (
	"context"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/recovery"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecoveryBindingDatabase(t *testing.T) {
	a, b, admin := testDatabase(t)
	ctx := context.Background()
	if MigrateRecovery(ctx, admin) != nil || MigrateRecovery(ctx, admin) != nil {
		t.Fatal("migration replay failed")
	}
	root, _ := NewRecoveryStore(a.pool)
	otherRoot, _ := NewRecoveryStore(b.pool)
	one, _ := NewRecoveryChallenges(root)
	two, _ := NewRecoveryChallenges(otherRoot)
	c := recovery.Challenge{Account: recovery.Binding{Issuer: testIssuer, Subject: "subject-1", OperationID: "operation-1"}, EmailTag: [32]byte{7}}
	token, err := one.Issue(ctx, c, time.Minute)
	if err != nil {
		t.Fatal("issue failed")
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store := one
			if i%2 == 1 {
				store = two
			}
			got, err := store.Claim(ctx, token)
			if err == nil {
				if got.Account.Subject != c.Account.Subject || got.Account.OperationID != c.Account.OperationID || got.EmailTag != c.EmailTag {
					t.Error("binding mismatch")
				}
				wins.Add(1)
			} else if !errors.Is(err, recovery.ErrInvalid) {
				t.Error("claim failure")
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatal("concurrent claim mismatch")
	}
	if one.Finish(ctx, token, true) != nil {
		t.Fatal("finish failed")
	}
	if _, err = two.Claim(ctx, token); !errors.Is(err, recovery.ErrInvalid) {
		t.Fatal("replayed challenge")
	}
	legacy, err := root.Issue(ctx, RecoveryIdentity{Issuer: testIssuer, Subject: "subject-1"}, time.Minute)
	if err != nil {
		t.Fatal("legacy fixture failed")
	}
	if _, err = one.Claim(ctx, legacy); !errors.Is(err, recovery.ErrInvalid) {
		t.Fatal("metadata-free challenge accepted")
	}
	if _, err = a.pool.Exec(ctx, "UPDATE auth.recovery_bindings SET email_tag=decode(repeat('00',32),'hex')"); err == nil {
		t.Fatal("runtime binding overwrite allowed")
	}
	if _, err = a.pool.Exec(ctx, "DELETE FROM auth.recovery_bindings"); err == nil {
		t.Fatal("runtime binding deletion allowed")
	}
	c.EmailTag = [32]byte{}
	if _, err = one.Issue(ctx, c, time.Minute); err == nil {
		t.Fatal("empty binding tag accepted")
	}
}
