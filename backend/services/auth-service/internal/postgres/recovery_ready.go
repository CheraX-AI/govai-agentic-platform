package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

// Readiness never changes schema or calls Keycloak/SMTP.
func RecoveryReady(ctx context.Context, p *pgxpool.Pool) error {
	if p == nil {
		return ErrStore
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if WebSessionsReady(ctx, p) != nil {
		return ErrStore
	}
	for _, migration := range []struct {
		version int
		file    string
	}{{4, "004_password_recovery.sql"}, {5, "005_recovery_bindings.sql"}, {6, "006_recovery_session_fences.sql"}} {
		sql, err := migrations.ReadFile("migrations/" + migration.file)
		if err != nil {
			return ErrStore
		}
		sum := sha256.Sum256(sql)
		var checksum string
		if p.QueryRow(ctx, "SELECT checksum FROM public.govai_auth_migrations WHERE version=$1", migration.version).Scan(&checksum) != nil || checksum != hex.EncodeToString(sum[:]) {
			return ErrStore
		}
	}
	var safe bool
	err := p.QueryRow(ctx, `SELECT
 has_table_privilege(current_user,'auth.password_recovery','SELECT')
 AND has_column_privilege(current_user,'auth.password_recovery','state','UPDATE')
 AND NOT has_table_privilege(current_user,'auth.password_recovery','DELETE,TRUNCATE')
 AND NOT has_column_privilege(current_user,'auth.password_recovery','expires_at','UPDATE')
 AND NOT has_column_privilege(current_user,'auth.password_recovery','subject','UPDATE')
 AND NOT has_table_privilege(current_user,'auth.recovery_bindings','UPDATE,DELETE,TRUNCATE')
 AND NOT has_table_privilege(current_user,'auth.web_recovery_sessions','UPDATE,DELETE,TRUNCATE')
 AND NOT has_table_privilege(current_user,'auth.web_recovery_fences','DELETE,TRUNCATE')
 AND has_sequence_privilege(current_user,'auth.recovery_fence_order','USAGE')`).Scan(&safe)
	if err != nil || !safe {
		return ErrStore
	}
	for _, table := range []string{"auth.password_recovery", "auth.recovery_bindings", "auth.web_recovery_fences", "auth.web_recovery_sessions"} {
		// Table names are fixed source constants, never configuration/request input.
		var ok bool
		if p.QueryRow(ctx, fmt.Sprintf("SELECT has_table_privilege(current_user,'%s','SELECT')", table)).Scan(&ok) != nil || !ok {
			return ErrStore
		}
	}
	return nil
}
