package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationCheckpointStatus struct {
	Prepared, Uncertain, Confirmed, Cancelled int64
}

// Status exposes aggregate counts only, never reset hashes or citizen identities.

func NotificationPasswordStatus(
	ctx context.Context, pool *pgxpool.Pool,
) (NotificationCheckpointStatus, error) {
	var status NotificationCheckpointStatus
	if pool == nil {
		return status, ErrOutbox
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := pool.QueryRow(ctx, `
SELECT count(*) FILTER (WHERE state='prepared'),
 count(*) FILTER (WHERE state='uncertain'),
 count(*) FILTER (WHERE state='confirmed'),
 count(*) FILTER (WHERE state='cancelled')
FROM auth.notification_checkpoints WHERE event_type='citizen.password.changed'`,
	).Scan(&status.Prepared, &status.Uncertain, &status.Confirmed, &status.Cancelled)
	if err != nil {
		return NotificationCheckpointStatus{}, ErrOutbox
	}
	return status, nil
}
