package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/recovery"
)

func NewRecoveryChallengesWithNotifications(
	store *RecoveryStore, issuer string,
) (*RecoveryChallenges, error) {
	challenges, err := NewRecoveryChallenges(store)
	if err != nil {
		return nil, err
	}
	challenges.notifications, err = NewNotificationOutbox(issuer)
	if err != nil {
		return nil, err
	}
	return challenges, nil
}

// Claim and intent commit before Keycloak can receive a password change.

func (adapter *RecoveryChallenges) claimNotificationReset(
	ctx context.Context, token recovery.Token,
) (recovery.Challenge, error) {
	var challenge recovery.Challenge
	hash, err := token.Digest()
	if err != nil {
		return challenge, recovery.ErrInvalid
	}
	tx, err := adapter.store.pool.Begin(ctx)
	if err != nil {
		return challenge, recovery.ErrUnavailable
	}
	defer rollbackRecovery(tx)
	var tag []byte
	err = tx.QueryRow(ctx, `
UPDATE auth.password_recovery AS password SET state='claimed'
FROM auth.recovery_bindings AS binding
WHERE password.token_hash=$1 AND password.token_hash=binding.token_hash
 AND password.issuer=$2 AND password.state='pending'
 AND password.expires_at > clock_timestamp()
RETURNING password.issuer, password.subject, binding.operation_id, binding.email_tag`,
		hash[:], adapter.notifications.issuer,
	).Scan(&challenge.Account.Issuer, &challenge.Account.Subject,
		&challenge.Account.OperationID, &tag)
	if err == pgx.ErrNoRows {
		return recovery.Challenge{}, recovery.ErrInvalid
	}
	if err != nil || len(tag) != 32 {
		return recovery.Challenge{}, recovery.ErrUnavailable
	}
	if prepareNotificationCheckpoint(ctx, tx, contract.PasswordChanged, hash[:],
		challenge.Account.Issuer, challenge.Account.Subject) != nil || tx.Commit(ctx) != nil {
		return recovery.Challenge{}, recovery.ErrUnavailable
	}
	copy(challenge.EmailTag[:], tag)
	return challenge, nil
}

func (adapter *RecoveryChallenges) PasswordChanged(
	ctx context.Context, token recovery.Token,
) error {
	if adapter.notifications == nil {
		return nil
	}
	hash, err := token.Digest()
	if err != nil {
		return recovery.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	tx, err := adapter.store.pool.Begin(ctx)
	if err != nil {
		return recovery.ErrUnavailable
	}
	defer rollbackRecovery(tx)
	if confirmNotificationCheckpoint(ctx, tx, adapter.notifications,
		contract.PasswordChanged, hash[:]) != nil || tx.Commit(ctx) != nil {
		return recovery.ErrUnavailable
	}
	return nil
}

func (adapter *RecoveryChallenges) PasswordUncertain(
	ctx context.Context, token recovery.Token,
) error {
	return adapter.closePasswordCheckpoint(ctx, token, "uncertain")
}

func (adapter *RecoveryChallenges) PasswordCancelled(
	ctx context.Context, token recovery.Token,
) error {
	return adapter.closePasswordCheckpoint(ctx, token, "cancelled")
}

func (adapter *RecoveryChallenges) closePasswordCheckpoint(
	ctx context.Context, token recovery.Token, state string,
) error {
	if adapter.notifications == nil {
		return nil
	}
	hash, err := token.Digest()
	if err != nil || (state != "uncertain" && state != "cancelled") {
		return recovery.ErrUnavailable
	}
	_, err = adapter.store.pool.Exec(ctx, `
UPDATE auth.notification_checkpoints SET state=$4
WHERE issuer=$1 AND event_type=$2 AND source_key=$3 AND state='prepared'`,
		adapter.notifications.issuer, contract.PasswordChanged, hash[:], state)
	if err != nil {
		return recovery.ErrUnavailable
	}
	return nil
}
