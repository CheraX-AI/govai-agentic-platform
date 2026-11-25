package postgres

import (
	"context"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/recovery"
	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyNotificationPasswordCheckpoints(
	t *testing.T, ctx context.Context, owner, app *pgxpool.Pool,
) {
	t.Helper()
	raw, _ := NewRecoveryStore(app)
	adapter, err := NewRecoveryChallengesWithNotifications(raw, outboxTestIssuer)
	if err != nil {
		t.Fatal("password notification adapter unavailable")
	}
	challenge := recovery.Challenge{
		Account: recovery.Binding{
			Issuer: outboxTestIssuer, Subject: "password-check-citizen",
			OperationID: "password-check-operation",
		},
		EmailTag: [32]byte{1},
	}
	issueAndClaim := func() recovery.Token {
		token, err := adapter.Issue(ctx, challenge, time.Minute)
		if err != nil {
			t.Fatal("synthetic reset challenge issue failed")
		}
		if _, err := adapter.Claim(ctx, token); err != nil {
			t.Fatal("synthetic reset claim/intent failed")
		}
		return token
	}
	confirmed := issueAndClaim()
	var count int
	if app.QueryRow(ctx, `
SELECT count(*) FROM auth.notification_outbox WHERE subject='password-check-citizen'
`).Scan(&count) != nil || count != 0 {
		t.Fatal("claiming a reset produced a success event")
	}
	firstConfirmation := adapter.PasswordChanged(ctx, confirmed)
	replayConfirmation := adapter.PasswordChanged(ctx, confirmed)
	if firstConfirmation != nil || replayConfirmation != nil {
		t.Fatal("confirmed password event was not idempotent")
	}
	if _, err := adapter.Claim(ctx, confirmed); err == nil {
		t.Fatal("claimed password reset was replayable")
	}
	if adapter.Finish(ctx, confirmed, false) != nil {
		t.Fatal("post-change cleanup failure fixture failed")
	}
	if adapter.PasswordChanged(ctx, confirmed) != nil {
		t.Fatal("later cleanup failure invalidated the confirmed event")
	}
	uncertain := issueAndClaim()
	if adapter.PasswordUncertain(ctx, uncertain) != nil ||
		adapter.PasswordChanged(ctx, uncertain) == nil {
		t.Fatal("uncertain provider result was automatically confirmed")
	}
	cancelled := issueAndClaim()
	if adapter.PasswordCancelled(ctx, cancelled) != nil ||
		adapter.PasswordChanged(ctx, cancelled) == nil {
		t.Fatal("cancelled reset was confirmed")
	}
	retry := issueAndClaim()
	hash, _ := retry.Digest()
	var originalID string
	if app.QueryRow(ctx, `
SELECT event_id::text FROM auth.notification_checkpoints
WHERE issuer=$1 AND event_type=$2 AND source_key=$3`,
		outboxTestIssuer, contract.PasswordChanged, hash[:]).Scan(&originalID) != nil {
		t.Fatal("durable password intent missing")
	}
	grant := `GRANT INSERT (
 event_id, action_id, event_type, event_version, issuer, subject, occurred_at
) ON auth.notification_outbox TO govai_auth_app`
	if _, err := owner.Exec(ctx, `REVOKE INSERT (
 event_id, action_id, event_type, event_version, issuer, subject, occurred_at
) ON auth.notification_outbox FROM govai_auth_app`); err != nil {
		t.Fatal("password failure fixture unavailable")
	}
	defer owner.Exec(context.Background(), grant)
	if adapter.PasswordChanged(ctx, retry) == nil {
		t.Fatal("password confirmation succeeded without an outbox write")
	}
	var state string
	if app.QueryRow(ctx, `
SELECT state FROM auth.notification_checkpoints WHERE event_id=$1`, originalID,
	).Scan(&state) != nil || state != "prepared" {
		t.Fatal("failed enqueue partially committed its checkpoint")
	}
	if _, err := owner.Exec(ctx, grant); err != nil || adapter.PasswordChanged(ctx, retry) != nil {
		t.Fatal("confirmed checkpoint could not be saved after storage recovered")
	}
	if app.QueryRow(ctx, `
SELECT count(*) FROM auth.notification_outbox WHERE event_id=$1 AND priority=10`, originalID,
	).Scan(&count) != nil || count != 1 {
		t.Fatal("retry did not retain its original security event ID")
	}
	_ = issueAndClaim()
	status, err := NotificationPasswordStatus(ctx, app)
	if err != nil || status.Prepared != 1 || status.Uncertain != 1 ||
		status.Cancelled != 1 || status.Confirmed != 2 {
		t.Fatal("unresolved password outcomes were not visible in aggregate status")
	}
}
