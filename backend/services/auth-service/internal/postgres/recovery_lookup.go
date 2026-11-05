package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
	"github.com/jackc/pgx/v5"
)

// RecoveryBinding is internal-only. The NIC identifies a self-declared number,
// not its human owner; this lookup alone never authorizes a password change.
type RecoveryBinding struct {
	webPrivate
	Issuer, Subject, OperationID string
}

// RecoveryByNIC uses the existing keyed NIC digest and ownership join. It never
// decrypts profiles or searches Keycloak by caller input. Issuer is server config.
// The current enabled/verified state must also be checked in Keycloak.
func (s *Store) RecoveryByNIC(ctx context.Context, issuer string, nic registration.NIC) (RecoveryBinding, error) {
	if issuer == "" || len(issuer) > 2048 {
		return RecoveryBinding{}, ErrStore
	}

	parsed, err := registration.ParseNIC(nic.NormalizedNumber())
	if err != nil {
		return RecoveryBinding{}, ErrRecoveryUnavailable
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var result RecoveryBinding
	err = s.pool.QueryRow(ctx, `SELECT p.issuer,p.subject,p.operation_id
 FROM auth.citizen_profiles p JOIN auth.registration_operations o
 ON p.operation_id=o.operation_id AND p.issuer=o.issuer AND p.subject=o.subject AND p.nic_digest=o.nic_digest
 WHERE p.issuer=$1 AND p.nic_digest=$2 AND o.phase IN ('enabled','email_sending','completed','attention')`,
		issuer,
		s.crypt.digest(parsed)).Scan(&result.Issuer, &result.Subject, &result.OperationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return RecoveryBinding{}, ErrRecoveryUnavailable
	}

	if err != nil {
		return RecoveryBinding{}, ErrStore
	}

	return result, nil
}
