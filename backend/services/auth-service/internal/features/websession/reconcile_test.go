package websession

import (
	"context"
	"errors"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
)

type reconciliationDouble struct {
	pending, claims, revokes, completed, retried int
	providerFail, saveFail                       bool
}

func (d *reconciliationDouble) ClaimRevocation(ctx context.Context) (postgres.WebRevocation, error) {
	if ctx.Err() != nil {
		return postgres.WebRevocation{}, ctx.Err()
	}

	d.claims++
	if d.pending == 0 {
		return postgres.WebRevocation{}, postgres.ErrWebSessionMissing
	}

	d.pending--
	return postgres.WebRevocation{ID: "synthetic-job", RefreshToken: "synthetic-refresh", Revision: 1, Attempts: 1}, nil
}

func (d *reconciliationDouble) CompleteRevocation(context.Context, string) error {
	d.completed++
	if d.saveFail {
		return errors.New("private SQL")
	}

	return nil
}

func (d *reconciliationDouble) RetryRevocation(context.Context, postgres.WebRevocation) error {
	d.retried++
	if d.saveFail {
		return errors.New("private SQL")
	}

	return nil
}

func (d *reconciliationDouble) RevokeNative(context.Context, string) error {
	d.revokes++
	if d.providerFail {
		return errors.New("private provider")
	}

	return nil
}

func TestRevocationReconciliationBounds(t *testing.T) {
	for _, mode := range []string{"success", "provider failure", "save failure", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			d := &reconciliationDouble{pending: 5, providerFail: mode == "provider failure", saveFail: mode == "save failure"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}

			result, err := Reconcile(ctx, d, d, 2)
			switch mode {
			case "success":
				if err != nil || result.Completed != 2 || d.revokes != 2 || d.pending != 3 {
					t.Fatal("batch bound ignored")
				}
			case "provider failure":
				if err != nil || result.Deferred != 2 || d.retried != 2 || d.completed != 0 {
					t.Fatal("failed revocation not deferred")
				}
			case "save failure":
				if err != ErrUnavailable || d.revokes != 1 {
					t.Fatal("acknowledgement failure ignored")
				}
			case "cancelled":
				if err != ErrUnavailable || d.revokes != 0 {
					t.Fatal("provider called after cancellation")
				}
			}
		})
	}

	d := &reconciliationDouble{}
	if _, err := Reconcile(context.Background(), d, d, 21); err != ErrUnavailable {
		t.Fatal("oversized batch accepted")
	}
}
