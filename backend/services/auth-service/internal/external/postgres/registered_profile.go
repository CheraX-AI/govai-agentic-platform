package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
	"github.com/jackc/pgx/v5"
)

// RegisteredProfile reads the existing authoritative encrypted registration
// data for a verified caller. It never searches by a user-supplied NIC/email.
func (s *Store) RegisteredProfile(ctx context.Context, issuer, subject string) (registration.ProfileData, error) {
	if !validDetailsOwner(issuer, subject) {
		return registration.ProfileData{}, profile.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var operation string
	var ciphertext []byte
	err := s.pool.QueryRow(ctx, `SELECT p.operation_id,p.profile_ciphertext
 FROM auth.citizen_profiles p JOIN auth.registration_operations o
 ON o.operation_id=p.operation_id AND o.issuer=p.issuer AND o.subject=p.subject AND o.nic_digest=p.nic_digest
 WHERE p.issuer=$1 AND p.subject=$2 AND o.phase IN ('enabled','email_sending','completed','attention')`, issuer, subject).Scan(&operation, &ciphertext)
	if errors.Is(err, pgx.ErrNoRows) {
		return registration.ProfileData{}, profile.ErrNotFound
	}
	if err != nil {
		return registration.ProfileData{}, ErrStore
	}
	return s.crypt.open(ciphertext, issuer, operation)
}
