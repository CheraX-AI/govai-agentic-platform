package permissionlab

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/app/wiring"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/recovery"
	"github.com/gin-gonic/gin"
)

func TestRecoveryLive(t *testing.T) {
	if os.Getenv("GA91_RECOVERY_LIVE") != "1" {
		t.Skip("GA91 recovery live opt-in disabled")
	}

	state := os.Getenv("GA91_RECOVERY_STATE_DIR")
	if state == "" {
		t.Fatal("RECOVERY_STATE BLOCKED")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(old)
	err := guardedCombined(func() error {
		return runWithScenario(ctx, state, os.Stdout, "recovery", func(ctx context.Context, l *lab) error {
			return l.recoveryScenario(ctx, filepath.Join(state, "ca.pem"))
		})
	}, os.Stdout)
	if err != nil {
		t.Fatal("RECOVERY_LIVE FAIL_OR_BLOCKED")
	}

	if logs.Len() != 0 {
		t.Fatal("RECOVERY_LOGS FAIL")
	}

	fmt.Println("RECOVERY_LOGS PASS")
}

func (l *lab) recoveryCheck(label string, ok bool) {
	l.lifeCheck(label, ok)
	if !ok {
		panic(errSafe)
	}
}

func (l *lab) recoveryMailToken(ctx context.Context, email string, exclude ...string) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		var inbox struct {
			Messages []struct {
				ID      string
				Subject string
				To      []struct{ Address string }
			}
		}
		if l.mailJSON(ctx, "/api/v1/messages", &inbox) != nil {
			return "", errSafe
		}

		for _, message := range inbox.Messages {
			if message.Subject != "Reset your GovAI password" || !messageID.MatchString(message.ID) {
				continue
			}

			for _, recipient := range message.To {
				if recipient.Address != email {
					continue
				}

				var detail struct{ Text string }
				if l.mailJSON(ctx, "/api/v1/message/"+message.ID, &detail) != nil {
					return "", errSafe
				}

				for _, link := range httpsLink.FindAllString(detail.Text, -1) {
					u, err := url.Parse(link)
					if err == nil &&
						u.Scheme == "https" &&
						u.Host == "recovery.example.test" &&
						u.Path == "/reset" &&
						u.RawQuery == "" &&
						strings.HasPrefix(u.Fragment, "token=") {
						token, err := recovery.ParseToken(strings.TrimPrefix(u.Fragment, "token="))
						if err == nil {
							seen := false
							for _, previous := range exclude {
								if previous == token.Reveal() {
									seen = true
								}
							}

							if seen {
								continue
							}

							return token.Reveal(), nil
						}
					}
				}
			}
		}

		select {
		case <-ctx.Done():
			return "", errSafe
		case <-time.After(200 * time.Millisecond):
		}
	}

	return "", errSafe
}

func (l *lab) recoveryScenario(ctx context.Context, caFile string) error {
	fmt.Fprintln(l.out, "RECOVERY_GROUP_AUTHORITY ACCEPTED_PROTOTYPE_LIMITATION")
	d, err := newDurableLab(ctx)
	if err != nil {
		return errSafe
	}

	defer func() { l.recoveryCheck("RECOVERY_SQL_CLEANUP", d.close() == nil) }()
	if postgres.MigrateRecoverySessions(ctx, d.admin) != nil {
		return errSafe
	}

	cfg, group, _, _ := l.combinedSetup(ctx)
	registration, _, closeRegistration := l.durableService(d, cfg, group, 901, false)
	defer closeRegistration()
	email := "recovery" + randomID() + "@example.test"
	oldPassword := randomID()
	newPassword := randomID()
	if _, err = registration.Register(ctx, combinedKey(901), combinedBody(901, email, oldPassword)); err != nil {
		return errSafe
	}

	issuer := l.base + "/realms/" + l.realm
	op := d.operation(ctx, issuer, combinedKey(901))
	link, err := l.verificationLink(ctx, email)
	if err != nil {
		return errSafe
	}

	if _, _, err = l.visitVerification(ctx, link); err != nil {
		return errSafe
	}

	l.recoveryCheck("RECOVERY_INITIAL_EMAIL_VERIFIED", l.realUser(ctx, op.Subject)["emailVerified"] == true)
	admin, err := keycloak.NewRegistrationAdmin(cfg, l.client)
	if err != nil {
		return errSafe
	}

	defer admin.Close()
	provider, err := keycloak.NewRecoveryProvider(admin)
	if err != nil {
		return errSafe
	}

	accounts, _ := postgres.NewRecoveryAccounts(d.store)
	rawStore, _ := postgres.NewRecoveryStore(d.pool)
	tokens, _ := postgres.NewRecoveryChallenges(rawStore)
	smtpConfig := recovery.SMTPConfig{
		Host:                 "127.0.0.1",
		Port:                 l.mailSMTPPort,
		From:                 "noreply@govai.test",
		DevelopmentPlaintext: true,
		Timeout:              3 * time.Second,
	}
	mail, err := recovery.NewSMTPMailer(smtpConfig)
	if err != nil {
		return errSafe
	}

	webDataKey, fenceKey := freshKey(), freshKey()
	web, _ := postgres.NewWebSessionStore(d.pool, webDataKey)
	bound, _ := postgres.NewBoundWebSessionStore(web, issuer, "recovery-web-test")
	guarded, _ := postgres.NewRecoveryWebStore(bound, fenceKey)
	recoveryConfig := recovery.Config{
		Issuer:     issuer,
		ResetURL:   "https://recovery.example.test/reset",
		BindingKey: freshKey(),
		Lifetime:   15 * time.Minute,
		Timeout:    25 * time.Second,
	}
	service, err := recovery.NewService(recoveryConfig, accounts, provider, tokens, mail, guarded)
	if err != nil {
		return errSafe
	}

	p := "/admin/realms/" + l.realm
	clientSecret := randomID()
	l.must(
		ctx,
		"POST",
		p+"/clients",
		object{
			"clientId":                  "lab-recovery-login",
			"enabled":                   true,
			"protocol":                  "openid-connect",
			"publicClient":              false,
			"serviceAccountsEnabled":    false,
			"standardFlowEnabled":       false,
			"directAccessGrantsEnabled": true,
			"implicitFlowEnabled":       false,
			"secret":                    clientSecret,
			"attributes":                object{"use.refresh.tokens": "true"},
		},
		201,
	)
	loginRequest := func(password string) (int, []byte) {
		status, body, err := l.request(
			ctx,
			"",
			"POST",
			"/realms/"+l.realm+"/protocol/openid-connect/token",
			url.Values{
				"grant_type":    {"password"},
				"client_id":     {"lab-recovery-login"},
				"client_secret": {clientSecret},
				"username":      {email},
				"password":      {password},
				"scope":         {"email"},
			},
		)
		if err != nil {
			panic(errSafe)
		}

		return status, body
	}
	mark, _ := guarded.AuthenticationMark(ctx)
	status, body := loginRequest(oldPassword)
	l.recoveryCheck("RECOVERY_OLD_LOGIN_INITIAL", status == 200)
	var issued struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
	}
	if json.Unmarshal(body, &issued) != nil || issued.Access == "" || issued.Refresh == "" {
		return errSafe
	}

	cookie, err := guarded.CreateFenced(
		ctx,
		postgres.WebCredentials{
			Issuer:       issuer,
			Subject:      op.Subject,
			AccessToken:  issued.Access,
			RefreshToken: issued.Refresh,
			CSRF:         base64.RawURLEncoding.EncodeToString(freshKey()),
		},
		time.Now().Add(time.Hour),
		mark,
	)
	if err != nil {
		return errSafe
	}

	webConfig := config.Web{
		Enabled: true,
		Provider: login.KeycloakConfig{
			Issuer:        issuer,
			ClientID:      "recovery-web-test",
			ClientSecret:  clientSecret,
			Audience:      "govai-api",
			EnableRefresh: true,
		},
		Origin:   "https://recovery.example.test",
		CAFile:   caFile,
		DataKey:  webDataKey,
		RateKey:  freshKey(),
		Lifetime: 30 * time.Minute,
	}
	recoveryRuntime := config.Recovery{
		Enabled:  true,
		Service:  recoveryConfig,
		SMTP:     smtpConfig,
		FenceKey: fenceKey,
		RateKey:  freshKey(),
		Origin:   webConfig.Origin,
	}
	h, closeRecovery, err := wiring.Recovery(
		ctx,
		recoveryRuntime,
		config.Registration{Enabled: true, Admin: cfg, CAFile: caFile},
		webConfig,
		d.pool,
		d.store,
	)
	if err != nil {
		return errSafe
	}

	defer closeRecovery()
	if _, closeUnguarded, err := wiring.Web(ctx, webConfig, d.pool, d.store); err == nil {
		if closeUnguarded != nil {
			closeUnguarded()
		}

		return errSafe
	}

	_, closeGuardedWeb, err := wiring.Web(ctx, webConfig, d.pool, d.store, fenceKey)
	if err != nil {
		return errSafe
	}

	defer closeGuardedWeb()
	l.recoveryCheck("RECOVERY_RUNTIME_COMPOSITION", true)
	oldMode := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	defer gin.SetMode(oldMode)
	router := gin.New()
	router.POST("/request", h.Request)
	router.POST("/reset", h.Reset)
	server := httptest.NewTLSServer(router)
	defer server.Close()
	client := server.Client()
	client.Timeout = 30 * time.Second
	defer client.CloseIdleConnections()
	send := func(path string, payload map[string]string) (int, []byte) {
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			panic(errSafe)
		}

		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, 8193))
		if err != nil || len(data) > 8192 || response.Header.Get("Cache-Control") != "no-store" {
			panic(errSafe)
		}

		return response.StatusCode, data
	}
	status, body = send("/request", map[string]string{"nic": "000000901v"})
	var receipt recovery.Receipt
	_ = json.Unmarshal(body, &receipt)
	expectedMask, _ := recovery.MaskEmail(email)
	l.recoveryCheck(
		"RECOVERY_HTTP_REQUEST_MASKED",
		status == 202 &&
			receipt.MaskedEmail == expectedMask &&
			!bytes.Contains(body, []byte(email)),
	)
	token, err := l.recoveryMailToken(ctx, email)
	if err != nil {
		return errSafe
	}

	l.recoveryCheck("RECOVERY_REAL_SMTP_LINK", true)
	// Confirm metadata persistence without ever printing token/hash/identity values.
	parsed, _ := recovery.ParseToken(token)
	hash, _ := parsed.Digest()
	var stored int
	if d.admin.QueryRow(ctx, "SELECT count(*) FROM auth.recovery_bindings WHERE token_hash=$1", hash[:]).Scan(&stored) != nil {
		return errSafe
	}

	l.recoveryCheck("RECOVERY_SQL_BOUND_CHALLENGE", stored == 1)
	status, _ = send(
		"/reset",
		map[string]string{"token": token, "new_password": newPassword, "confirm_password": newPassword},
	)
	l.recoveryCheck("RECOVERY_HTTP_RESET", status == 204)
	status, body = loginRequest(oldPassword)
	var rejected struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &rejected)
	l.recoveryCheck("RECOVERY_OLD_PASSWORD_REJECTED", status == 400 && rejected.Error == "invalid_grant")
	status, _ = loginRequest(newPassword)
	l.recoveryCheck("RECOVERY_NEW_PASSWORD_ACCEPTED", status == 200)
	status, _, err = l.request(
		ctx,
		"",
		"POST",
		"/realms/"+l.realm+"/protocol/openid-connect/token",
		url.Values{
			"grant_type":    {"refresh_token"},
			"client_id":     {"lab-recovery-login"},
			"client_secret": {clientSecret},
			"refresh_token": {issued.Refresh},
		},
	)
	l.recoveryCheck("RECOVERY_OLD_PROVIDER_REFRESH_REJECTED", err == nil && status == 400)
	_, err = guarded.Load(ctx, cookie)
	l.recoveryCheck("RECOVERY_OLD_WEB_SESSION_REJECTED", errors.Is(err, postgres.ErrWebSessionMissing))
	status, _ = send(
		"/reset",
		map[string]string{"token": token, "new_password": randomID(), "confirm_password": newPassword},
	)
	l.recoveryCheck("RECOVERY_CONFIRMATION_MISMATCH", status == 400)
	status, _ = send(
		"/reset",
		map[string]string{"token": token, "new_password": newPassword, "confirm_password": newPassword},
	)
	l.recoveryCheck("RECOVERY_LINK_REPLAY_REJECTED", status == 400)
	status, _ = send("/request", map[string]string{"nic": "000000901V"})
	l.recoveryCheck("RECOVERY_LIVE_RESEND_LIMIT", status == 429)
	status, body = send("/request", map[string]string{"nic": "000000999V"})
	var unknown recovery.Receipt
	_ = json.Unmarshal(body, &unknown)
	l.recoveryCheck(
		"RECOVERY_LIVE_UNKNOWN_NIC",
		status == 202 &&
			unknown.Status == "received" &&
			unknown.MaskedEmail == "",
	)
	// Changed email: deliver under old address, then change ONLY this test account.
	receipt2, err := service.Request(ctx, "000000901V")
	if err != nil || receipt2.MaskedEmail == "" {
		return errSafe
	}

	changedToken, err := l.recoveryMailToken(ctx, email, token)
	if err != nil || changedToken == token {
		return errSafe
	}

	changedEmail := "changed" + randomID() + "@example.test"
	l.must(
		ctx,
		"PUT",
		p+"/users/"+op.Subject,
		object{"email": changedEmail, "emailVerified": true},
		204,
	)
	err = service.Reset(ctx, changedToken, randomID())
	l.recoveryCheck("RECOVERY_CHANGED_EMAIL_REJECTED", err != nil)
	l.must(ctx, "PUT", p+"/users/"+op.Subject, object{"email": email, "emailVerified": true}, 204)
	status, _ = loginRequest(newPassword)
	l.recoveryCheck("RECOVERY_CHANGED_EMAIL_NO_PASSWORD_MUTATION", status == 200)
	// Expiry controlled only in this newly created disposable database.
	_, err = service.Request(ctx, "000000901V")
	if err != nil {
		return errSafe
	}

	expiredToken, err := l.recoveryMailToken(ctx, email, token, changedToken)
	if err != nil || expiredToken == changedToken {
		return errSafe
	}

	expired, _ := recovery.ParseToken(expiredToken)
	expiredHash, _ := expired.Digest()
	tx, err := d.admin.Begin(ctx)
	if err != nil {
		return errSafe
	}

	if _, err = tx.Exec(ctx, "ALTER TABLE auth.password_recovery DISABLE TRIGGER recovery_transition"); err != nil {
		_ = tx.Rollback(ctx)
		return errSafe
	}

	if _, err = tx.Exec(
		ctx,
		"UPDATE auth.password_recovery SET created_at=clock_timestamp()-interval '2 minutes',expires_at=clock_timestamp()-interval '1 minute' WHERE token_hash=$1",
		expiredHash[:],
	); err != nil {
		_ = tx.Rollback(ctx)
		return errSafe
	}

	if _, err = tx.Exec(ctx, "ALTER TABLE auth.password_recovery ENABLE TRIGGER recovery_transition"); err != nil {
		_ = tx.Rollback(ctx)
		return errSafe
	}

	if tx.Commit(ctx) != nil {
		return errSafe
	}

	err = service.Reset(ctx, expiredToken, randomID())
	l.recoveryCheck("RECOVERY_EXPIRED_LINK_REJECTED", errors.Is(err, recovery.ErrInvalid))
	// Keycloak enforces its policy independently of the API's minimum bound.
	_, err = service.Request(ctx, "000000901V")
	if err != nil {
		return errSafe
	}

	policyToken, err := l.recoveryMailToken(ctx, email, token, changedToken, expiredToken)
	if err != nil || policyToken == expiredToken {
		return errSafe
	}

	l.must(ctx, "PUT", p, object{"passwordPolicy": "length(12)"}, 204)
	err = service.Reset(ctx, policyToken, "12345678")
	l.recoveryCheck("RECOVERY_PROVIDER_POLICY_REJECTED", err != nil)
	l.must(ctx, "PUT", p, object{"passwordPolicy": "length(8)"}, 204)
	status, _ = loginRequest(newPassword)
	l.recoveryCheck("RECOVERY_POLICY_NO_PASSWORD_MUTATION", status == 200)
	// SMTP failure uses a closed loopback destination, never another mail server.
	badMail, err := recovery.NewSMTPMailer(
		recovery.SMTPConfig{
			Host:                 "127.0.0.1",
			Port:                 1,
			From:                 "noreply@govai.test",
			DevelopmentPlaintext: true,
			Timeout:              time.Second,
		},
	)
	if err != nil {
		return errSafe
	}

	failedService, err := recovery.NewService(recoveryConfig, accounts, provider, tokens, badMail, guarded)
	if err != nil {
		return errSafe
	}

	var beforeAttention, afterAttention int
	if d.admin.QueryRow(ctx, "SELECT count(*) FROM auth.password_recovery WHERE state='attention'").Scan(&beforeAttention) != nil {
		return errSafe
	}

	_, err = failedService.Request(ctx, "000000901V")
	l.recoveryCheck("RECOVERY_SMTP_FAILURE_SAFE", err != nil)
	if d.admin.QueryRow(ctx, "SELECT count(*) FROM auth.password_recovery WHERE state='attention'").Scan(&afterAttention) != nil {
		return errSafe
	}

	l.recoveryCheck("RECOVERY_SMTP_FAILED_TOKEN_SPENT", afterAttention == beforeAttention+1)
	return nil
}
