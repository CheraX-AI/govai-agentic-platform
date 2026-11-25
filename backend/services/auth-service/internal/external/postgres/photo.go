package postgres

import (
	"context"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"time"
)

// This is an internal, server-owned key update. HTTP callers cannot choose keys.
func (s *Store) SetCitizenPhoto(ctx context.Context, issuer, subject string, revision int64, key string) (profile.Details, error) {
	if !validDetailsOwner(issuer, subject) {
		return profile.Details{}, profile.ErrNotFound
	}
	if revision < 0 || revision > profile.MaxClientRevision || !profile.ValidPhotoObjectKey(key) {
		return profile.Details{}, profile.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result, err := s.scanDetails(s.pool.QueryRow(ctx, `UPDATE auth.citizen_details d
 SET photo_object_key=$4,revision=d.revision+1
 FROM auth.citizen_profiles p JOIN auth.registration_operations o
 ON o.operation_id=p.operation_id AND o.issuer=p.issuer AND o.subject=p.subject AND o.nic_digest=p.nic_digest
 WHERE d.issuer=$1 AND d.subject=$2 AND d.revision=$3
 AND p.issuer=d.issuer AND p.subject=d.subject
 AND o.phase IN ('enabled','email_sending','completed','attention') RETURNING `+detailsColumns, issuer, subject, revision, key), issuer, subject)
	if !errors.Is(err, profile.ErrNotFound) {
		return result, err
	}
	if _, err = s.CitizenDetails(ctx, issuer, subject); err != nil {
		return profile.Details{}, err
	}
	return profile.Details{}, profile.ErrConflict
}
