package postgres

import "context"

// HasReadyProfile authorizes only validated issuer+subject ownership. It never
// looks up by caller email/NIC, decrypts profiles, or returns citizen data.
func (s *Store) HasReadyProfile(ctx context.Context, issuer, subject string) (bool, error) {
	if issuer == "" || subject == "" {
		return false, ErrStore
	}

	var ready bool
	e := s.pool.QueryRow(ctx, `SELECT EXISTS (
 SELECT 1 FROM auth.citizen_profiles p JOIN auth.registration_operations o
 ON p.operation_id=o.operation_id AND p.issuer=o.issuer AND p.subject=o.subject AND p.nic_digest=o.nic_digest
 WHERE p.issuer=$1 AND p.subject=$2 AND o.phase IN ('enabled','email_sending','completed','attention'))`,
		issuer,
		subject).Scan(&ready)
	if e != nil {
		return false, ErrStore
	}

	return ready, nil
}
