package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func id() string { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }

const testIssuer = "https://identity.test/realms/database-test"

func testReservation(nic, key string) registration.Reservation {
	n, _ := registration.ParseNIC(nic)
	return registration.Reservation{Issuer: testIssuer, Key: key, OperationID: id(), Fingerprint: [32]byte{9}, Pending: registration.ProfileData{FullName: "Synthetic Citizen", NIC: n, MobileNumber: "+94770000000"}}
}
func testDatabase(t *testing.T) (*Store, *Store, *pgxpool.Pool) {
	t.Helper()
	if os.Getenv("GOVAI_DB_TEST") != "1" {
		t.Skip("opt-in: GOVAI_DB_TEST=1; requires isolated backend/dev/auth-postgres setup")
	}
	env, e := godotenv.Read("../../.env.database")
	if e != nil {
		t.Fatal("private local database settings required")
	}
	adminEnv, e := godotenv.Read("../../.env.database-admin")
	if e != nil {
		t.Fatal("private migration settings required")
	}
	ownerURL, e := url.Parse(adminEnv["MIGRATION_DATABASE_URL"])
	if e != nil {
		t.Fatal("invalid test DSN")
	}
	// Never run destructive tests in the configured database. Create a fresh,
	// randomly named database on the explicitly local development cluster.
	if ownerURL.Hostname() != "127.0.0.1" || ownerURL.Port() != "55432" {
		t.Fatal("tests restricted to isolated loopback development cluster")
	}
	ctx := context.Background()
	owner, e := Open(ctx, ownerURL.String())
	if e != nil {
		t.Fatal(e)
	}
	name := "govai_auth_test_" + id()
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = owner.Exec(ctx, "CREATE DATABASE "+quoted); e != nil {
		owner.Close()
		t.Fatal("cannot create disposable database")
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(name, "govai_auth_test_") || len(name) != 48 {
			t.Error("invalid cleanup target")
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, e := owner.Exec(cleanup, "DROP DATABASE "+quoted+" WITH (FORCE)"); e != nil {
			t.Error("disposable database cleanup failed")
		}
		owner.Close()
	})
	ownerURL.Path = "/" + name
	admin, e := Open(ctx, ownerURL.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(admin.Close)
	if e = Migrate(ctx, admin); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(ctx, admin); e != nil {
		t.Fatal("migration replay failed")
	}
	if _, e = admin.Exec(ctx, "REVOKE ALL ON DATABASE "+quoted+" FROM PUBLIC; GRANT CONNECT ON DATABASE "+quoted+" TO govai_auth_app; REVOKE ALL ON SCHEMA public FROM PUBLIC"); e != nil {
		t.Fatal("privilege setup failed")
	}
	appURL, e := url.Parse(env["DATABASE_URL"])
	if e != nil {
		t.Fatal("invalid test app DSN")
	}
	appURL.Path = "/" + name
	p, e := Open(ctx, appURL.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	p2, e := Open(ctx, appURL.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p2.Close)
	a, e := NewStore(p, bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewStore(p2, bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	if e != nil {
		t.Fatal(e)
	}
	if Ready(ctx, p) != nil {
		t.Fatal("runtime readiness failed")
	}
	return a, b, admin
}
func closeSession(t *testing.T, s registration.OperationSession) {
	t.Helper()
	if e := s.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func advance(t *testing.T, s registration.OperationSession, phase registration.Phase, subject string) {
	t.Helper()
	o := s.Snapshot()
	o.Revision++
	o.Phase = phase
	if subject != "" {
		o.Subject = subject
	}
	if e := s.Save(context.Background(), o); e != nil {
		t.Fatal(e)
	}
}

func TestPostgresContracts(t *testing.T) {
	a, b, admin := testDatabase(t)
	ctx := context.Background()
	t.Run("privileges", func(t *testing.T) {
		if Ready(ctx, admin) == nil {
			t.Fatal("administrative connection accepted as runtime")
		}
		for _, sql := range []string{"CREATE TABLE auth.forbidden(id int)", "DELETE FROM auth.registration_operations", "TRUNCATE auth.citizen_profiles", "UPDATE auth.registration_operations SET issuer='changed'", "CREATE ROLE forbidden", "SELECT * FROM pg_authid", "UPDATE public.govai_auth_migrations SET version=2"} {
			if _, e := a.pool.Exec(ctx, sql); e == nil {
				t.Fatal("runtime privilege unexpectedly granted")
			}
		}
		if Migrate(ctx, a.pool) == nil {
			t.Fatal("runtime user migrated schema")
		}
	})
	t.Run("unique-reservation-replay-and-cancel", func(t *testing.T) {
		r := testReservation("123456789v", "11111111-1111-1111-1111-111111111111")
		s, e := a.Reserve(ctx, r)
		if e != nil {
			t.Fatal(e)
		}
		wait, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
		defer cancel()
		if _, e = b.Open(wait, r.Issuer, r.Key); e == nil {
			t.Fatal("concurrent session acquired")
		}
		closeSession(t, s)
		replay := r
		replay.OperationID = id()
		s, e = b.Reserve(ctx, replay)
		if e != nil || s.Snapshot().OperationID != r.OperationID {
			t.Fatal("replay replaced reservation")
		}
		closeSession(t, s)
		replay.Fingerprint[0]++
		if _, e = b.Reserve(ctx, replay); !errors.Is(e, registration.ErrRequestConflict) {
			t.Fatal("input conflict not rejected")
		}
		other := testReservation("123456789V", "22222222-2222-2222-2222-222222222222")
		if _, e = b.Reserve(ctx, other); !errors.Is(e, registration.ErrNICConflict) {
			t.Fatal("normalized NIC collision accepted")
		}
		var count int
		if admin.QueryRow(ctx, "SELECT count(*) FROM auth.registration_operations WHERE operation_id=$1", other.OperationID).Scan(&count) != nil || count != 0 {
			t.Fatal("orphan operation after NIC conflict")
		}
	})
	t.Run("independent-pools-concurrent-NIC", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for i, store := range []*Store{a, b} {
			wg.Add(1)
			go func(i int, store *Store) {
				defer wg.Done()
				key := []string{"33333333-3333-3333-3333-333333333333", "44444444-4444-4444-4444-444444444444"}[i]
				s, e := store.Reserve(ctx, testReservation("200000000001", key))
				if e == nil {
					e = s.Close(ctx)
				}
				results <- e
			}(i, store)
		}
		wg.Wait()
		close(results)
		success, conflict := 0, 0
		for e := range results {
			if e == nil {
				success++
			} else if errors.Is(e, registration.ErrNICConflict) {
				conflict++
			} else {
				t.Fatal(e)
			}
		}
		if success != 1 || conflict != 1 {
			t.Fatal("atomic NIC claim failed")
		}
	})
	t.Run("same-key-independent-pools", func(t *testing.T) {
		r := testReservation("200000000009", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
		s, e := a.Reserve(ctx, r)
		if e != nil {
			t.Fatal(e)
		}
		result := make(chan error, 1)
		go func() {
			replay := r
			replay.Key = strings.ToUpper(r.Key)
			replay.OperationID = id()
			other, e := b.Reserve(ctx, replay)
			if e == nil {
				if other.Snapshot().OperationID != r.OperationID {
					e = ErrStore
				}
				if ce := other.Close(ctx); e == nil {
					e = ce
				}
			}
			result <- e
		}()
		select {
		case <-result:
			t.Fatal("same UUID acquired under different case while locked")
		case <-time.After(75 * time.Millisecond):
		}
		closeSession(t, s)
		select {
		case e := <-result:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("lock was not released")
		}
	})
	t.Run("profile-commit-before-enable-and-restart", func(t *testing.T) {
		r := testReservation("200000000002", "55555555-5555-5555-5555-555555555555")
		s, e := a.Reserve(ctx, r)
		if e != nil {
			t.Fatal(e)
		}
		advance(t, s, registration.Creating, "")
		advance(t, s, registration.SubjectRecorded, "subject-test")
		o := s.Snapshot()
		bad := o
		bad.Revision++
		bad.Phase = registration.ProfileReady
		if s.Save(ctx, bad) == nil {
			t.Fatal("advanced without durable profile")
		}
		_ = s.Close(ctx)
		s, e = b.Open(ctx, r.Issuer, r.Key)
		if e != nil || s.Snapshot().Phase != registration.SubjectRecorded {
			t.Fatal("restart checkpoint missing")
		}
		p := registration.ProfileRecord{OperationID: r.OperationID, Issuer: r.Issuer, Subject: "subject-test", Data: r.Pending}
		wrong := p
		wrong.Subject = "another"
		if !errors.Is(b.Persist(ctx, wrong), registration.ErrProfileConflict) {
			t.Fatal("wrong profile owner accepted")
		}
		if b.Persist(ctx, p) != nil || b.Persist(ctx, p) != nil {
			t.Fatal("idempotent profile commit failed")
		}
		advance(t, s, registration.ProfileReady, "")
		advance(t, s, registration.Enabling, "")
		advance(t, s, registration.Enabled, "")
		if b.Persist(ctx, p) != nil {
			t.Fatal("later exact profile replay failed")
		}
		closeSession(t, s)
		var cipher []byte
		if admin.QueryRow(ctx, "SELECT pending_ciphertext FROM auth.registration_operations WHERE operation_id=$1", r.OperationID).Scan(&cipher) != nil || bytes.Contains(cipher, []byte(r.Pending.FullName)) {
			t.Fatal("plaintext storage")
		}
	})
	t.Run("database-connection-loss-cancels-owner", func(t *testing.T) {
		r := testReservation("200000000003", "66666666-6666-6666-6666-666666666666")
		ss, e := a.Reserve(ctx, r)
		if e != nil {
			t.Fatal(e)
		}
		s := ss.(*session)
		s.mu.Lock()
		pid := s.conn.Conn().PgConn().PID()
		s.mu.Unlock()
		if _, e = admin.Exec(ctx, "SELECT pg_terminate_backend($1)", pid); e != nil {
			t.Fatal("terminate synthetic owner failed")
		}
		select {
		case <-s.Context().Done():
		case <-time.After(3 * time.Second):
			t.Fatal("ownership loss not detected")
		}
		o := s.Snapshot()
		o.Phase = registration.Creating
		o.Revision++
		if s.Save(ctx, o) == nil {
			t.Fatal("stale owner committed")
		}
		_ = s.Close(ctx)
		replacement, e := b.Open(ctx, r.Issuer, r.Key)
		if e != nil {
			t.Fatal(e)
		}
		closeSession(t, replacement)
	})
}

type databaseAdmin struct {
	creates int
	user    registration.Identity
}

func (a *databaseAdmin) CreateDisabled(_ context.Context, _ registration.Request, _ string) (registration.Identity, error) {
	a.creates++
	a.user = registration.Identity{Subject: "synthetic-subject"}
	return a.user, nil
}
func (a *databaseAdmin) FindOwned(context.Context, string) (registration.Identity, error) {
	return a.user, nil
}
func (a *databaseAdmin) ReadOwned(context.Context, string, string) (registration.Identity, error) {
	return a.user, nil
}
func (a *databaseAdmin) EnableOwned(context.Context, string, string) error {
	a.user.Enabled = true
	return nil
}
func (a *databaseAdmin) SendVerification(context.Context, string, string) error { return nil }

func TestPostgresOrchestration(t *testing.T) {
	store, other, _ := testDatabase(t)
	admin := &databaseAdmin{}
	svc, e := registration.NewService(registration.ServiceConfig{Issuer: testIssuer, FingerprintKey: bytes.Repeat([]byte{3}, 32), Timeout: 5 * time.Second, EmailRetryBackoff: time.Second, MaxEmailAttempts: 3}, admin, store, store)
	if e != nil {
		t.Fatal(e)
	}
	key := "77777777-7777-7777-7777-777777777777"
	body := `{"full_name":"Synthetic Citizen","nic":"200000000004","mobile_number":"+94771234567","email":"synthetic@example.test","password":"Example8!"}`
	if _, e = svc.Register(context.Background(), key, strings.NewReader(body)); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Register(context.Background(), key, strings.NewReader(body)); e != nil || admin.creates != 1 {
		t.Fatal("registration replay created again")
	}
	s, e := other.Open(context.Background(), testIssuer, key)
	if e != nil {
		t.Fatal(e)
	}
	if s.Snapshot().Phase != registration.Completed {
		t.Fatal("workflow not durable")
	}
	closeSession(t, s)
}
