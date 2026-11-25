package postgres

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/recovery"
	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyNotificationLoginStores(
	t *testing.T, ctx context.Context, owner, app *pgxpool.Pool,
) {
	t.Helper()
	if MigrateRecoverySessions(ctx, owner) != nil {
		t.Fatal("isolated browser schema preparation failed")
	}
	authStore, err := NewStore(app, bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal("auth fixture store unavailable")
	}
	recorder, err := NewLoginEventRecorder(authStore, outboxTestIssuer)
	if err != nil || recorder.RecordLogin(ctx, outboxTestIssuer, "native-hook-citizen") != nil {
		t.Fatal("native login checkpoint failed")
	}
	defer owner.Exec(context.Background(), `
DELETE FROM auth.notification_outbox WHERE subject IN ('native-hook-citizen', 'web-hook-citizen')`)
	if recorder.RecordLogin(ctx, "https://other.test/realms/other", "native-hook-citizen") == nil {
		t.Fatal("native recorder accepted another issuer")
	}
	web, err := NewWebSessionStoreWithNotifications(app, bytes.Repeat([]byte{3}, 32),
		outboxTestIssuer)
	if err != nil {
		t.Fatal("browser notification store unavailable")
	}
	credentials := WebCredentials{
		Issuer: outboxTestIssuer, Subject: "web-hook-citizen",
		AccessToken: "synthetic-private-access", RefreshToken: "synthetic-private-refresh",
		CSRF:         base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
		AccessUntil:  time.Now().Add(time.Minute).Unix(),
		RefreshUntil: time.Now().Add(time.Hour).Unix(),
	}
	expires := time.Now().Add(time.Hour)
	raw, err := web.Create(ctx, credentials, expires)
	if err != nil {
		t.Fatal("browser session and event commit failed")
	}
	hash, _ := webHash(raw)
	defer owner.Exec(context.Background(), "DELETE FROM auth.web_sessions WHERE session_hash=$1", hash)
	bound, _ := NewBoundWebSessionStore(web, outboxTestIssuer, "synthetic-web-client")
	guarded, _ := NewRecoveryWebStore(bound, bytes.Repeat([]byte{4}, 32))
	mark, err := guarded.AuthenticationMark(ctx)
	if err != nil {
		t.Fatal("recovery marker unavailable")
	}
	raw, err = guarded.CreateFenced(ctx, credentials, expires, mark)
	if err != nil {
		t.Fatal("guarded browser checkpoint failed")
	}
	hash, _ = webHash(raw)
	defer owner.Exec(context.Background(), "DELETE FROM auth.web_sessions WHERE session_hash=$1", hash)
	var count int
	err = app.QueryRow(ctx, `
SELECT count(*) FROM auth.notification_outbox WHERE subject='web-hook-citizen'
 AND event_type='citizen.login.succeeded' AND published_at IS NULL`).Scan(&count)
	if err != nil || count != 2 {
		t.Fatal("browser session did not create exactly one event per commit")
	}
	if guarded.Revoke(ctx, recovery.Binding{
		Issuer: outboxTestIssuer, Subject: credentials.Subject,
	}) != nil {
		t.Fatal("recovery fence fixture failed")
	}
	if _, err := guarded.CreateFenced(ctx, credentials, expires, mark); err == nil {
		t.Fatal("revoked authentication marker accepted")
	}
	err = app.QueryRow(ctx,
		"SELECT count(*) FROM auth.notification_outbox WHERE subject='web-hook-citizen'",
	).Scan(&count)
	if err != nil || count != 2 {
		t.Fatal("rejected guarded login created a success event")
	}

	// A storage failure must roll back the new session as well as the event.

	var before int
	if owner.QueryRow(ctx, "SELECT count(*) FROM auth.web_sessions").Scan(&before) != nil {
		t.Fatal("session count probe failed")
	}
	grant := `GRANT INSERT (
 event_id, action_id, event_type, event_version, issuer, subject, occurred_at
) ON auth.notification_outbox TO govai_auth_app`
	if _, err := owner.Exec(ctx, `REVOKE INSERT (
 event_id, action_id, event_type, event_version, issuer, subject, occurred_at
) ON auth.notification_outbox FROM govai_auth_app`); err != nil {
		t.Fatal("storage failure fixture failed")
	}
	defer owner.Exec(context.Background(), grant)
	if _, err := web.Create(ctx, credentials, expires); err == nil {
		t.Fatal("browser login succeeded without an outbox event")
	}
	if recorder.RecordLogin(ctx, outboxTestIssuer, "native-hook-citizen") == nil {
		t.Fatal("native checkpoint succeeded without outbox permissions")
	}
	if owner.QueryRow(ctx, "SELECT count(*) FROM auth.web_sessions").Scan(&count) != nil ||
		count != before {
		t.Fatal("failed outbox write left a new browser session")
	}
}
