package notificationevents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

type emailObservationFixture struct {
	check         EmailCheck
	identity      registration.Identity
	providerError error
	verified      []bool
	found         bool
}

func (fixture *emailObservationFixture) ClaimEmailCheck(context.Context) (EmailCheck, bool, error) {
	return fixture.check, fixture.found, nil
}

func (fixture *emailObservationFixture) FinishEmailCheck(
	_ context.Context, _ EmailCheck, verified bool,
) error {
	fixture.verified = append(fixture.verified, verified)
	return nil
}

func (fixture *emailObservationFixture) ReadOwned(
	_ context.Context, subject, operation string,
) (registration.Identity, error) {
	if subject != fixture.check.Subject || operation != fixture.check.OperationID {
		return registration.Identity{}, errors.New("incorrect owned identity")
	}
	return fixture.identity, fixture.providerError
}

func TestEmailVerificationObservation(t *testing.T) {
	for _, test := range []struct {
		name                                      string
		verified, disabled, mismatch, unavailable bool
		wantVerified                              bool
	}{
		{name: "still unverified"},
		{name: "confirmed", verified: true, wantVerified: true},
		{name: "provider failure", verified: true, unavailable: true},
		{name: "wrong subject", verified: true, mismatch: true},
		{name: "disabled account", verified: true, disabled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := &emailObservationFixture{
				check: EmailCheck{Subject: "synthetic-subject", OperationID: "synthetic-operation"},
				identity: registration.Identity{
					Subject: "synthetic-subject", Enabled: !test.disabled,
					EmailVerified: test.verified,
				},
				found: true,
			}
			if test.mismatch {
				fixture.identity.Subject = "another-subject"
			}
			if test.unavailable {
				fixture.providerError = errors.New("synthetic private upstream body")
			}
			observer, _ := NewEmailObserver(fixture, fixture)
			worked, _ := observer.CheckOne(context.Background())
			if !worked || len(fixture.verified) != 1 || fixture.verified[0] != test.wantVerified {
				t.Fatal("email state observation produced an incorrect confirmation")
			}
		})
	}
}

func TestEmailCheckRedactionAndShutdown(t *testing.T) {
	check := EmailCheck{Subject: "private-subject", LeaseToken: "private-lease"}
	encoded, _ := json.Marshal(check)
	for _, output := range []string{fmt.Sprintf("%+v", check), string(encoded)} {
		if strings.Contains(output, check.Subject) || strings.Contains(output, check.LeaseToken) {
			t.Fatal("email observation identity leaked")
		}
	}
	fixture := &emailObservationFixture{}
	observer, _ := NewEmailObserver(fixture, fixture)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(observer.Run(ctx), context.Canceled) {
		t.Fatal("observer did not stop")
	}
}
