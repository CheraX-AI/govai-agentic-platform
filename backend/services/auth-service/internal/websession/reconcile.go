package websession

import (
	"context"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"time"
)

type RevocationQueue interface {
	ClaimRevocation(context.Context) (postgres.WebRevocation, error)
	CompleteRevocation(context.Context, string) error
	RetryRevocation(context.Context, postgres.WebRevocation) error
}
type Revoker interface {
	RevokeNative(context.Context, string) error
}
type Reconciliation struct{ Completed, Deferred int }

// Reconcile performs a bounded, explicitly invoked recovery pass. Only RFC 7009
// revocation is retried, never password login or refresh-token exchange. Queue
// client binding and the provider must be composed from the same trusted config.
func Reconcile(ctx context.Context, q RevocationQueue, r Revoker, maximum int) (Reconciliation, error) {
	var result Reconciliation
	if q == nil || r == nil || maximum < 1 || maximum > 20 {
		return result, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for i := 0; i < maximum; i++ {
		job, err := q.ClaimRevocation(ctx)
		if errors.Is(err, postgres.ErrWebSessionMissing) {
			return result, nil
		}
		if err != nil {
			return result, ErrUnavailable
		}
		attempt, stop := context.WithTimeout(ctx, 5*time.Second)
		err = r.RevokeNative(attempt, job.RefreshToken)
		stop()
		// Queue acknowledgement may outlive cancellation briefly; the provider
		// might have completed even if its response was lost, so replay is safe.
		save, done := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		if err == nil {
			err = q.CompleteRevocation(save, job.ID)
			if err == nil {
				result.Completed++
			}
		} else {
			err = q.RetryRevocation(save, job)
			if err == nil {
				result.Deferred++
			}
		}
		done()
		if err != nil || ctx.Err() != nil {
			return result, ErrUnavailable
		}
	}
	return result, nil
}
