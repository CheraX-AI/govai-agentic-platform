package login

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type loginRecorderDouble struct {
	after           func()
	calls           int
	err             error
	issuer, subject string
}

func (recorder *loginRecorderDouble) RecordLogin(
	_ context.Context, issuer, subject string,
) error {
	recorder.calls++
	recorder.issuer, recorder.subject = issuer, subject
	if recorder.after != nil {
		recorder.after()
	}
	return recorder.err
}

func TestNativeLoginNotificationCheckpoint(t *testing.T) {
	for _, test := range []struct {
		name                                string
		providerError, recordError          error
		unverified, missingProfile, expired bool
		wantCalls                           int
		wantSuccess                         bool
	}{
		{name: "success", wantCalls: 1, wantSuccess: true},
		{name: "wrong password", providerError: ErrFailed},
		{name: "unverified email", unverified: true},
		{name: "missing profile", missingProfile: true},
		{name: "expired token", expired: true},
		{name: "outbox failure", recordError: errors.New("private failure"), wantCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := Claims{
				Issuer: "https://issuer.test", Subject: "synthetic-subject",
				Verified: !test.unverified, Citizen: true,
				Expires: time.Now().Add(time.Minute),
			}
			if test.expired {
				claims.Expires = time.Now().Add(-time.Minute)
			}
			provider := &identifierProviderDouble{err: test.providerError}
			profiles := &profileDouble{ready: !test.missingProfile}
			recorder := &loginRecorderDouble{err: test.recordError}
			service, err := NewServiceWithNotifications(provider,
				identifierVerifierDouble{claims}, profiles, recorder)
			if err != nil {
				t.Fatal(err)
			}
			request, _ := Parse(strings.NewReader(
				`{"email":"synthetic@example.test","password":"synthetic-password"}`,
			))
			access, err := service.Login(context.Background(), request)
			if (err == nil) != test.wantSuccess || recorder.calls != test.wantCalls {
				t.Fatal("login notification checkpoint violated")
			}
			if err != nil && access.Token != "" {
				t.Fatal("tokens returned without a durable success checkpoint")
			}
			if recorder.calls > 0 &&
				(recorder.issuer != claims.Issuer || recorder.subject != claims.Subject) {
				t.Fatal("recorder received the wrong identity")
			}
		})
	}
}

func TestOrdinaryLoginDoesNotRecordNativeEvent(t *testing.T) {
	claims := Claims{
		Issuer: "https://issuer.test", Subject: "synthetic-subject",
		Verified: true, Citizen: true, Expires: time.Now().Add(time.Minute),
	}
	service, err := NewService(&identifierProviderDouble{},
		identifierVerifierDouble{claims}, &profileDouble{ready: true})
	if err != nil || service.notifications != nil {
		t.Fatal("ordinary browser credential service unexpectedly records native events")
	}
}

func TestNativeLoginRechecksExpiryAfterOutbox(t *testing.T) {
	now := time.Now()
	claims := Claims{
		Issuer: "https://issuer.test", Subject: "synthetic-subject",
		Verified: true, Citizen: true, Expires: now.Add(2 * time.Second),
	}
	recorder := &loginRecorderDouble{after: func() { now = now.Add(3 * time.Second) }}
	service, err := NewServiceWithNotifications(&identifierProviderDouble{},
		identifierVerifierDouble{claims}, &profileDouble{ready: true}, recorder)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	request, _ := Parse(strings.NewReader(
		`{"email":"synthetic@example.test","password":"synthetic-password"}`,
	))
	access, err := service.Login(context.Background(), request)
	if err == nil || access.Token != "" || recorder.calls != 1 {
		t.Fatal("expired credentials released after outbox checkpoint")
	}
}
