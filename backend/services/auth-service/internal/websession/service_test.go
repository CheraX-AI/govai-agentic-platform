package websession

import (
	"context"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"strings"
	"testing"
	"time"
)

type dependencies struct {
	snap                                 postgres.WebSnapshot
	pending, ended                       bool
	creates, refreshes, revokes          int
	failProvider, failCommit, failCreate bool
	failRead                             bool
}

func (d *dependencies) Login(context.Context, login.Request) (login.Access, error) {
	return login.Access{Token: "synthetic-access", RefreshToken: "synthetic-refresh", ExpiresIn: 300, RefreshExpiresIn: 1800}, nil
}
func (d *dependencies) Verify(context.Context, string) (login.Claims, error) {
	return login.Claims{Issuer: "https://issuer.test", Subject: "synthetic-subject", Expires: time.Now().Add(5 * time.Minute), Verified: true, Citizen: true}, nil
}
func (d *dependencies) HasReadyProfile(context.Context, string, string) (bool, error) {
	return true, nil
}
func (d *dependencies) Create(_ context.Context, c postgres.WebCredentials, expires time.Time) (string, error) {
	d.creates++
	if d.failCreate {
		return "", errors.New("private SQL")
	}
	d.snap = postgres.WebSnapshot{Credentials: c, Expires: expires}
	return "synthetic-cookie", nil
}
func (d *dependencies) Load(context.Context, string) (postgres.WebSnapshot, error) {
	if d.failRead {
		return postgres.WebSnapshot{}, errors.New("private database outage")
	}
	if d.pending || d.ended {
		return postgres.WebSnapshot{}, postgres.ErrWebSessionMissing
	}
	return d.snap, nil
}

func TestWebSessionDatabaseOutage(t *testing.T) {
	s, d, started := setup(t)
	d.failRead = true
	if _, _, err := s.Check(context.Background(), started.Cookie); err != ErrUnavailable {
		t.Fatal("database outage reported as logout")
	}
	if _, err := s.CSRF(context.Background(), started.Cookie); err != ErrUnavailable {
		t.Fatal("CSRF outage classification")
	}
	if s.Refresh(context.Background(), started.Cookie, started.CSRF) != ErrUnavailable || d.refreshes != 0 {
		t.Fatal("provider called during database outage")
	}
}
func (d *dependencies) LoadForTermination(context.Context, string) (postgres.WebSnapshot, error) {
	if d.failRead {
		return postgres.WebSnapshot{}, errors.New("private database outage")
	}
	if d.ended {
		return postgres.WebSnapshot{}, postgres.ErrWebSessionMissing
	}
	return d.snap, nil
}

type queuedLogoutDouble struct {
	*dependencies
	revoked     string
	completed   bool
	readFailure bool
}

func (d *queuedLogoutDouble) TerminateQueued(context.Context, string) (string, error) {
	d.ended = true
	return "synthetic-job", nil
}
func (d *queuedLogoutDouble) LoadRevocation(context.Context, string) (postgres.WebRevocation, error) {
	if d.readFailure {
		return postgres.WebRevocation{}, postgres.ErrStore
	}
	return postgres.WebRevocation{ID: "synthetic-job", RefreshToken: "synthetic-latest-rotated"}, nil
}
func (d *queuedLogoutDouble) CompleteRevocation(context.Context, string) error {
	d.completed = true
	return nil
}
func (d *queuedLogoutDouble) QueueRevocation(context.Context, string, string) (string, error) {
	return "synthetic-job", nil
}
func (d *queuedLogoutDouble) RevokeNative(_ context.Context, raw string) error {
	d.revoked = raw
	return nil
}
func TestLogoutUsesAtomicallyCapturedCredential(t *testing.T) {
	for _, fail := range []bool{false, true} {
		s, d, started := setup(t)
		queued := &queuedLogoutDouble{dependencies: d, readFailure: fail}
		s.store = queued
		s.rotation = queued
		err := s.End(context.Background(), started.Cookie, started.CSRF)
		if !fail && (err != nil || queued.revoked != "synthetic-latest-rotated" || !queued.completed) {
			t.Fatal("logout used stale credential")
		}
		if fail && (err != ErrRevocationPending || queued.revoked != "" || queued.completed) {
			t.Fatal("uncertain queue read acknowledged")
		}
	}
}
func TestPendingSessionCanRestoreCSRFForLogout(t *testing.T) {
	s, d, started := setup(t)
	d.pending = true
	if _, _, err := s.Check(context.Background(), started.Cookie); err != ErrDenied {
		t.Fatal("pending session authorized")
	}
	if csrf, err := s.CSRF(context.Background(), started.Cookie); err != nil || csrf != started.CSRF {
		t.Fatal("pending session cannot recover logout CSRF")
	}
	if s.End(context.Background(), started.Cookie, started.CSRF) != nil || !d.ended {
		t.Fatal("pending session logout failed")
	}
}
func (d *dependencies) BeginRefresh(context.Context, string) (postgres.WebSnapshot, error) {
	if d.pending || d.ended {
		return postgres.WebSnapshot{}, postgres.ErrWebSessionMissing
	}
	d.pending = true
	d.snap.Revision++
	return d.snap, nil
}
func (d *dependencies) FinishRefresh(_ context.Context, _ string, _ postgres.WebSnapshot, next postgres.WebCredentials) error {
	if d.failCommit || d.ended {
		return errors.New("commit unknown")
	}
	d.snap.Credentials = next
	d.pending = false
	return nil
}
func (d *dependencies) Terminate(context.Context, string) error { d.ended = true; return nil }
func (d *dependencies) RefreshBound(_ context.Context, _ string, owner login.SessionIdentity) (login.SessionTokens, error) {
	d.refreshes++
	if owner.Issuer != d.snap.Credentials.Issuer || owner.Subject != d.snap.Credentials.Subject {
		return login.SessionTokens{}, login.ErrFailed
	}
	if d.failProvider {
		return login.SessionTokens{}, login.ErrUnavailable
	}
	return login.SessionTokens{AccessToken: "synthetic-next-access", RefreshToken: "synthetic-next-refresh", ExpiresIn: 300, RefreshExpiresIn: 1800}, nil
}
func (d *dependencies) RevokeNative(context.Context, string) error {
	d.revokes++
	if d.failProvider {
		return errors.New("private provider")
	}
	return nil
}
func setup(t *testing.T) (*Service, *dependencies, Started) {
	t.Helper()
	d := &dependencies{}
	s, err := New(d, d, d, d, d, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := login.Parse(strings.NewReader(`{"email":"synthetic@example.test","password":"synthetic-password"}`))
	started, err := s.Start(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return s, d, started
}

func TestWebSessionLifecycle(t *testing.T) {
	s, d, started := setup(t)
	ctx := context.Background()
	if started.Cookie == "" || len(started.CSRF) != 43 || started.Expires.After(time.Now().Add(31*time.Minute)) {
		t.Fatal("incorrect session custody/expiry")
	}
	identity, csrf, err := s.Check(ctx, started.Cookie)
	if err != nil || identity.Subject != "synthetic-subject" || csrf != started.CSRF {
		t.Fatal("session check failed")
	}
	if s.Refresh(ctx, started.Cookie, "incorrect") != ErrDenied || d.refreshes != 0 || d.pending {
		t.Fatal("CSRF bypass")
	}
	if s.Refresh(ctx, started.Cookie, csrf) != nil || d.snap.Credentials.RefreshToken != "synthetic-next-refresh" {
		t.Fatal("rotation not committed")
	}
	if s.End(ctx, started.Cookie, "incorrect") != ErrDenied || d.ended {
		t.Fatal("logout CSRF bypass")
	}
	if s.End(ctx, started.Cookie, csrf) != nil || !d.ended || d.revokes != 1 {
		t.Fatal("logout failed")
	}
	if _, _, err = s.Check(ctx, started.Cookie); err != ErrDenied {
		t.Fatal("terminated session accepted")
	}
}

func TestWebSessionAmbiguityFailsClosed(t *testing.T) {
	for _, mode := range []string{"provider", "commit"} {
		t.Run(mode, func(t *testing.T) {
			s, d, started := setup(t)
			d.failProvider = mode == "provider"
			d.failCommit = mode == "commit"
			if s.Refresh(context.Background(), started.Cookie, started.CSRF) != ErrUnavailable || !d.pending {
				t.Fatal("ambiguous refresh not pending")
			}
			if s.Refresh(context.Background(), started.Cookie, started.CSRF) != ErrDenied || d.refreshes != 1 {
				t.Fatal("old credential replay")
			}
			// Logout remains locally effective even when provider revocation fails.
			d.failProvider = true
			if s.End(context.Background(), started.Cookie, started.CSRF) != ErrRevocationPending || !d.ended {
				t.Fatal("local logout restored on provider failure")
			}
		})
	}
}

func TestWebSessionExpiry(t *testing.T) {
	s, d, started := setup(t)
	d.snap.Credentials.AccessUntil = time.Now().Add(-time.Second).Unix()
	if token, err := s.CSRF(context.Background(), started.Cookie); err != nil || token != started.CSRF {
		t.Fatal("reload CSRF unavailable after access expiry")
	}
	if _, _, err := s.Check(context.Background(), started.Cookie); err != ErrDenied {
		t.Fatal("expired access accepted")
	}
	d.snap.Credentials.RefreshUntil = time.Now().Add(-time.Second).Unix()
	if s.Refresh(context.Background(), started.Cookie, started.CSRF) != ErrDenied || d.refreshes != 0 {
		t.Fatal("expired refresh accepted")
	}
}
