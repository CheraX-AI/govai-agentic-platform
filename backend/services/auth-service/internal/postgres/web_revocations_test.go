package postgres

import (
	"bytes"
	"context"
	"encoding/base64"
	"sync"
	"testing"
	"time"
)

func TestWebRevocationDatabase(t *testing.T) {
	a, b, admin := testDatabase(t)
	ctx := context.Background()
	if MigrateWebSessions(ctx, admin) != nil {
		t.Fatal("migration failed")
	}
	if MigrateWebSessions(ctx, admin) != nil {
		t.Fatal("migration failed")
	}
	key := bytes.Repeat([]byte{21}, 32)
	one, _ := NewWebSessionStore(a.pool, key)
	two, _ := NewWebSessionStore(b.pool, key)
	q1, _ := NewBoundWebSessionStore(one, "https://issuer.test", "web")
	q2, _ := NewBoundWebSessionStore(two, "https://issuer.test", "web")
	other, _ := NewBoundWebSessionStore(two, "https://issuer.test", "mobile")
	c := WebCredentials{Issuer: "https://issuer.test", Subject: "synthetic-subject", AccessToken: "synthetic-access", RefreshToken: "synthetic-refresh", CSRF: base64.RawURLEncoding.EncodeToString(make([]byte, 32))}
	raw, err := one.Create(ctx, c, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal("setup")
	}
	// A logout's earlier read can race with a completed rotation. The queued
	// credential must come from the locked current row, not the earlier snapshot.
	checkpoint, err := one.BeginRefresh(ctx, raw)
	if err != nil {
		t.Fatal("race setup")
	}
	c.RefreshToken = "synthetic-latest-rotated"
	if one.FinishRefresh(ctx, raw, checkpoint, c) != nil {
		t.Fatal("race rotation setup")
	}
	id, err := q1.TerminateQueued(ctx, raw)
	if err != nil {
		t.Fatal("atomic logout failed")
	}
	captured, err := q2.LoadRevocation(ctx, id)
	if err != nil || captured.RefreshToken != c.RefreshToken {
		t.Fatal("stale credential queued during logout race")
	}
	if _, err = two.Load(ctx, raw); err != ErrWebSessionMissing {
		t.Fatal("logout state active")
	}
	var ciphertext []byte
	if admin.QueryRow(ctx, "SELECT ciphertext FROM auth.web_revocations WHERE job_id=$1", id).Scan(&ciphertext) != nil || bytes.Contains(ciphertext, []byte(c.RefreshToken)) {
		t.Fatal("plaintext queue")
	}
	if _, err = other.ClaimRevocation(ctx); err != ErrWebSessionMissing {
		t.Fatal("cross-client claim")
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	var claimed WebRevocation
	for _, queue := range []*BoundWebSessionStore{q1, q2} {
		wg.Add(1)
		go func(q *BoundWebSessionStore) {
			defer wg.Done()
			job, e := q.ClaimRevocation(ctx)
			if e == nil {
				mu.Lock()
				wins++
				claimed = job
				mu.Unlock()
			}
		}(queue)
	}
	wg.Wait()
	if wins != 1 || claimed.RefreshToken != c.RefreshToken {
		t.Fatal("lease concurrency failed")
	}
	if q1.RetryRevocation(ctx, claimed) != nil {
		t.Fatal("retry checkpoint failed")
	}
	if _, err = q2.ClaimRevocation(ctx); err != ErrWebSessionMissing {
		t.Fatal("backoff bypass")
	}
	if _, err = admin.Exec(ctx, "UPDATE auth.web_revocations SET next_attempt=clock_timestamp()-interval '1 second' WHERE job_id=$1", id); err != nil {
		t.Fatal("retry setup")
	}
	next, err := q2.ClaimRevocation(ctx)
	if err != nil || next.Attempts != 2 {
		t.Fatal("persistent retry claim")
	}
	if q1.RetryRevocation(ctx, claimed) == nil {
		t.Fatal("stale owner overwrote retry")
	}
	if q2.CompleteRevocation(ctx, next.ID) != nil {
		t.Fatal("completion failed")
	}
	if admin.QueryRow(ctx, "SELECT ciphertext FROM auth.web_revocations WHERE job_id=$1", id).Scan(&ciphertext) != nil || len(ciphertext) != 0 {
		t.Fatal("completed credential retained")
	}
	if _, err = q1.QueueRevocation(ctx, "https://other.test", "synthetic-next"); err == nil {
		t.Fatal("issuer binding bypass")
	}
	id, err = q1.QueueRevocation(ctx, c.Issuer, "synthetic-next")
	if err != nil {
		t.Fatal("queue setup")
	}
	for i := 1; i <= 8; i++ {
		if _, err = admin.Exec(ctx, "UPDATE auth.web_revocations SET next_attempt=clock_timestamp()-interval '1 second' WHERE job_id=$1", id); err != nil {
			t.Fatal("attempt setup")
		}
		job, e := q2.ClaimRevocation(ctx)
		if e != nil || job.Attempts != i {
			t.Fatal("attempt count")
		}
		if q2.RetryRevocation(ctx, job) != nil {
			t.Fatal("bounded retry failed")
		}
	}
	var state string
	if admin.QueryRow(ctx, "SELECT state FROM auth.web_revocations WHERE job_id=$1", id).Scan(&state) != nil || state != "attention" {
		t.Fatal("attempt cap ignored")
	}
	if _, err = q1.ClaimRevocation(ctx); err != ErrWebSessionMissing {
		t.Fatal("attention job automatically retried")
	}
	if _, err = a.pool.Exec(ctx, "DELETE FROM auth.web_revocations"); err == nil {
		t.Fatal("runtime queue deletion allowed")
	}
	if WebSessionsReady(ctx, a.pool) != nil {
		t.Fatal("queue permissions/readiness failed")
	}
	status, err := q1.RevocationStatus(ctx)
	if err != nil || status.Completed != 1 || status.Attention != 1 {
		t.Fatal("safe queue status incorrect")
	}
	foreign, err := other.RevocationStatus(ctx)
	if err != nil || foreign.Completed != 0 || foreign.Attention != 0 {
		t.Fatal("status crossed client binding")
	}
	if _, err = PruneCompletedWebRevocations(ctx, a.pool, 24*time.Hour, 10); err == nil {
		t.Fatal("runtime queue pruning allowed")
	}
	if _, err = admin.Exec(ctx, "UPDATE auth.web_revocations SET updated_at=clock_timestamp()-interval '2 days'"); err != nil {
		t.Fatal("retention setup")
	}
	if count, err := PruneCompletedWebRevocations(ctx, admin, 24*time.Hour, 10); err != nil || count != 1 {
		t.Fatal("completed job pruning failed")
	}
	status, err = q1.RevocationStatus(ctx)
	if err != nil || status.Completed != 0 || status.Attention != 1 {
		t.Fatal("attention evidence pruned")
	}
}
