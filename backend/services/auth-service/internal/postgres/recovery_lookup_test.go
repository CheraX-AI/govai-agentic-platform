package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

func TestRecoveryLookupDatabase(t *testing.T) {
	a, b, _ := testDatabase(t)
	ctx := context.Background()
	for index, number := range []string{"000000001v", "200000000001"} {
		r := testReservation(number, fmt.Sprintf("%08d-1111-1111-1111-111111111111", index+1))
		session, err := a.Reserve(ctx, r)
		if err != nil {
			t.Fatal("reservation failed")
		}

		if _, err := b.RecoveryByNIC(ctx, r.Issuer, r.Pending.NIC); !errors.Is(err, ErrRecoveryUnavailable) {
			t.Fatal("pending profile accepted")
		}

		advance(t, session, registration.Creating, "")
		subject := fmt.Sprintf("synthetic-%d", index)
		advance(t, session, registration.SubjectRecorded, subject)
		if err = b.Persist(
			ctx,
			registration.ProfileRecord{OperationID: r.OperationID, Issuer: r.Issuer, Subject: subject, Data: r.Pending},
		); err != nil {
			t.Fatal("profile failed")
		}

		advance(t, session, registration.ProfileReady, "")
		if _, err := b.RecoveryByNIC(ctx, r.Issuer, r.Pending.NIC); !errors.Is(err, ErrRecoveryUnavailable) {
			t.Fatal("not-enabled profile accepted")
		}

		advance(t, session, registration.Enabling, "")
		advance(t, session, registration.Enabled, "")
		upper, _ := registration.ParseNIC(strings.ToUpper(number))
		got, err := b.RecoveryByNIC(ctx, r.Issuer, upper)
		if err != nil ||
			got.Issuer != r.Issuer ||
			got.Subject != subject ||
			got.OperationID != r.OperationID {
			t.Fatal("saved binding mismatch")
		}

		if _, err := b.RecoveryByNIC(ctx, "https://other.example.test/realms/GovAI", upper); !errors.Is(err, ErrRecoveryUnavailable) {
			t.Fatal("issuer isolation failed")
		}

		output, _ := json.Marshal(got)
		if strings.Contains(string(output)+fmt.Sprintf("%+v", got), subject) {
			t.Fatal("binding leaked")
		}

		closeSession(t, session)
	}

	unknown, _ := registration.ParseNIC("000000009V")
	if _, err := b.RecoveryByNIC(ctx, testIssuer, unknown); !errors.Is(err, ErrRecoveryUnavailable) {
		t.Fatal("unknown NIC accepted")
	}

	if _, err := b.RecoveryByNIC(ctx, testIssuer, registration.NIC{}); !errors.Is(err, ErrRecoveryUnavailable) {
		t.Fatal("empty NIC accepted")
	}
}
