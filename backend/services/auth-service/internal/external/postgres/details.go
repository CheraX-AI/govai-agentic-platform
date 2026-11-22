package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/jackc/pgx/v5"
)

// Identity arguments are INTERNAL trusted inputs from verified authentication.
// These methods do not authenticate an HTTP caller; GA-253 must derive both
// issuer and subject from the verified session/JWT, never from request JSON.
func validDetailsOwner(issuer, subject string) bool {
	return issuer != "" && len(issuer) <= 2048 && subject != "" && len(subject) <= 128
}

const detailsColumns = `d.details_ciphertext,COALESCE(d.photo_object_key,''),d.revision,d.created_at,d.updated_at`

func (s *Store) scanDetails(row pgx.Row, issuer, subject string) (profile.Details, error) {
	var result profile.Details
	var ciphertext []byte
	err := row.Scan(&ciphertext, &result.PhotoObjectKey, &result.Revision, &result.CreatedAt, &result.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.Details{}, profile.ErrNotFound
	}
	if err != nil {
		return profile.Details{}, ErrStore
	}
	result.Fields, err = s.crypt.openDetails(ciphertext, issuer, subject)
	if err != nil {
		return profile.Details{}, ErrStore
	}
	return result, nil
}

func (s *Store) CitizenDetails(ctx context.Context, issuer, subject string) (profile.Details, error) {
	if !validDetailsOwner(issuer, subject) {
		return profile.Details{}, profile.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.scanDetails(s.pool.QueryRow(ctx, `SELECT `+detailsColumns+`
 FROM auth.citizen_details d
 JOIN auth.citizen_profiles p ON p.issuer=d.issuer AND p.subject=d.subject
 JOIN auth.registration_operations o ON o.operation_id=p.operation_id
   AND o.issuer=p.issuer AND o.subject=p.subject AND o.nic_digest=p.nic_digest
 WHERE d.issuer=$1 AND d.subject=$2
   AND o.phase IN ('enabled','email_sending','completed','attention')`, issuer, subject), issuer, subject)
}

// SaveCitizenDetails atomically checks revision and updates only permitted
// fields. Photo keys and registered identity fields are preserved. No upsert can
// introduce another owner or silently overwrite a newer revision.
func (s *Store) SaveCitizenDetails(ctx context.Context, issuer, subject string, revision int64, fields profile.EditableDetails) (profile.Details, error) {
	if !validDetailsOwner(issuer, subject) {
		return profile.Details{}, profile.ErrNotFound
	}
	if revision < 0 || fields.Validate(time.Now()) != nil {
		return profile.Details{}, profile.ErrInvalid
	}
	ciphertext, err := s.crypt.sealDetails(fields, issuer, subject)
	if err != nil {
		return profile.Details{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result, err := s.scanDetails(s.pool.QueryRow(ctx, `UPDATE auth.citizen_details d
 SET details_ciphertext=$4,revision=d.revision+1
 FROM auth.citizen_profiles p JOIN auth.registration_operations o
 ON o.operation_id=p.operation_id AND o.issuer=p.issuer AND o.subject=p.subject AND o.nic_digest=p.nic_digest
 WHERE d.issuer=$1 AND d.subject=$2 AND d.revision=$3
 AND p.issuer=d.issuer AND p.subject=d.subject
 AND o.phase IN ('enabled','email_sending','completed','attention')
 RETURNING `+detailsColumns, issuer, subject, revision, ciphertext), issuer, subject)
	if !errors.Is(err, profile.ErrNotFound) {
		return result, err
	}
	// Distinguish a stale revision from a missing/non-ready profile without
	// returning any other citizen's record. No write happened in either case.
	if _, readErr := s.CitizenDetails(ctx, issuer, subject); readErr != nil {
		return profile.Details{}, readErr
	}
	return profile.Details{}, profile.ErrConflict
}
