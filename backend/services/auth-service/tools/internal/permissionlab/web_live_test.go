package permissionlab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/application"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWebSessionLive(t *testing.T) {
	if os.Getenv("GA50_WEB_LIVE") != "1" {
		t.Skip("web live opt-in disabled")
	}
	state := os.Getenv("GA50_WEB_STATE_DIR")
	if state == "" {
		t.Fatal("WEB_STATE BLOCKED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if err := guardedCombined(func() error {
		return runWithScenario(ctx, state, os.Stdout, "combined", func(ctx context.Context, l *lab) error {
			return l.loginScenarioWithSessions(ctx, filepath.Join(state, "ca.pem"), true, true)
		})
	}, os.Stdout); err != nil {
		t.Fatal("WEB_LIVE FAIL_OR_BLOCKED")
	}
}

func (l *lab) webScenario(ctx context.Context, caFile string, d *durableLab, issuer, mobileID, email, password string) error {
	if postgres.MigrateWebSessions(ctx, d.admin) != nil {
		return errSafe
	}
	p := "/admin/realms/" + l.realm
	secret := randomID()
	l.must(ctx, "POST", p+"/clients", object{"clientId": "lab-login-web", "enabled": true, "protocol": "openid-connect", "publicClient": false, "serviceAccountsEnabled": false, "standardFlowEnabled": false, "directAccessGrantsEnabled": true, "implicitFlowEnabled": false, "fullScopeAllowed": false, "secret": secret, "attributes": object{"use.refresh.tokens": "true", "access.token.lifespan": "300"}}, 201)
	clientID := l.clientID(ctx, "lab-login-web")
	citizen := decode[object](l.must(ctx, "GET", p+"/roles/citizen", nil, 200))
	l.must(ctx, "POST", p+"/clients/"+clientID+"/scope-mappings/realm", []object{citizen}, 204)
	mappers := decode[[]object](l.must(ctx, "GET", p+"/clients/"+mobileID+"/protocol-mappers/models", nil, 200))
	for _, mapper := range mappers {
		delete(mapper, "id")
		l.must(ctx, "POST", p+"/clients/"+clientID+"/protocol-mappers/models", mapper, 201)
	}
	cfg := config.Web{Enabled: true, Provider: login.KeycloakConfig{Issuer: issuer, ClientID: "lab-login-web", ClientSecret: secret, Audience: "govai-api", EnableRefresh: true}, Origin: "https://govai.test", CAFile: caFile, DataKey: freshKey(), RateKey: freshKey(), Lifetime: 30 * time.Minute}
	h, closeWeb, err := application.Web(ctx, cfg, d.pool, d.store)
	if err != nil {
		return errSafe
	}
	defer closeWeb()
	server := httptest.NewTLSServer(handler.NewRouterWithWeb(nil, nil, nil, nil, h))
	defer server.Close()
	client := server.Client()
	client.Timeout = 10 * time.Second
	client.Jar, _ = cookiejar.New(nil)
	defer client.CloseIdleConnections()
	base, _ := url.Parse(server.URL)
	request := func(method, path string, body []byte, csrf, origin string) (int, map[string]json.RawMessage, []*http.Cookie) {
		req, e := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(body))
		if e != nil {
			panic(errSafe)
		}
		req.Header.Set("X-GovAI-Web", "1")
		req.Header.Set("Origin", origin)
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		response, e := client.Do(req)
		if e != nil {
			panic(errSafe)
		}
		defer response.Body.Close()
		b, e := io.ReadAll(io.LimitReader(response.Body, 65537))
		if e != nil || len(b) > 65536 || response.Header.Get("Cache-Control") != "no-store" {
			panic(errSafe)
		}
		result := map[string]json.RawMessage{}
		if len(b) > 0 && json.Unmarshal(b, &result) != nil {
			panic(errSafe)
		}
		if len(result["access_token"]) > 0 || len(result["refresh_token"]) > 0 || len(result["id_token"]) > 0 {
			panic(errSafe)
		}
		return response.StatusCode, result, response.Cookies()
	}
	loginBody, _ := json.Marshal(map[string]string{"email": email, "password": password})
	status, result, cookies := request("POST", "/api/v1/web/login", loginBody, "", cfg.Origin)
	var csrf string
	_ = json.Unmarshal(result["csrf_token"], &csrf)
	l.lifeCheck("WEB_LOGIN_COOKIE_CUSTODY", status == 200 && csrf != "" && len(cookies) == 1 && cookies[0].Secure && cookies[0].HttpOnly && cookies[0].SameSite == http.SameSiteStrictMode && cookies[0].Path == "/" && cookies[0].Domain == "")
	cookie := cookies[0].Value
	store, e := postgres.NewWebSessionStore(d.pool, cfg.DataKey)
	if e != nil {
		return errSafe
	}
	before, e := store.Load(ctx, cookie)
	if e != nil {
		return errSafe
	}
	status, _, _ = request("GET", "/api/v1/web/session", nil, "", cfg.Origin)
	l.lifeCheck("WEB_OWN_SESSION", status == 200)
	status, _, _ = request("POST", "/api/v1/web/refresh", nil, "", cfg.Origin)
	l.lifeCheck("WEB_MISSING_CSRF_DENIED", status == 403)
	status, _, _ = request("POST", "/api/v1/web/refresh", nil, csrf, "https://evil.test")
	l.lifeCheck("WEB_FOREIGN_ORIGIN_DENIED", status == 403)
	status, _, _ = request("POST", "/api/v1/web/refresh", nil, csrf, cfg.Origin)
	after, e := store.Load(ctx, cookie)
	l.lifeCheck("WEB_ROTATION_DURABLE", status == 204 && e == nil && after.Credentials.RefreshToken != before.Credentials.RefreshToken && after.Credentials.Subject == before.Credentials.Subject)
	provider, e := login.NewKeycloak(cfg.Provider, l.client)
	if e != nil {
		return errSafe
	}
	defer provider.Close()
	mobileSecret := decode[object](l.must(ctx, "GET", p+"/clients/"+mobileID+"/client-secret", nil, 200))["value"].(string)
	mobileProvider, e := login.NewKeycloak(login.KeycloakConfig{Issuer: issuer, ClientID: "lab-login-mobile", ClientSecret: mobileSecret, Audience: "govai-api", EnableRefresh: true}, l.client)
	if e != nil {
		return errSafe
	}
	defer mobileProvider.Close()
	_, e = mobileProvider.Refresh(ctx, after.Credentials.RefreshToken)
	l.lifeCheck("WEB_TOKEN_WRONG_CLIENT_DENIED", errors.Is(e, login.ErrFailed))
	credentials, e := login.Parse(bytes.NewReader(loginBody))
	if e != nil {
		return errSafe
	}
	mobileIssued, e := mobileProvider.Password(ctx, credentials)
	if e != nil {
		return errSafe
	}
	_, e = provider.Refresh(ctx, mobileIssued.RefreshToken)
	l.lifeCheck("MOBILE_TOKEN_WEB_CLIENT_DENIED", errors.Is(e, login.ErrFailed))
	rotatedMobile, e := mobileProvider.Refresh(ctx, mobileIssued.RefreshToken)
	l.lifeCheck("WRONG_CLIENT_ATTEMPT_PRESERVES_OWNER_REFRESH", e == nil && rotatedMobile.RefreshToken != "")
	if mobileProvider.RevokeRefresh(ctx, rotatedMobile.RefreshToken) != nil {
		return errSafe
	}
	// Independent service/provider reconstruction demonstrates persisted custody.
	second, closeSecond, e := application.Web(ctx, cfg, d.pool, d.store)
	if e != nil {
		return errSafe
	}
	secondServer := httptest.NewTLSServer(handler.NewRouterWithWeb(nil, nil, nil, nil, second))
	secondClient := secondServer.Client()
	secondClient.Timeout = 10 * time.Second
	reloadedRequest, e := http.NewRequestWithContext(ctx, "GET", secondServer.URL+"/api/v1/web/session", nil)
	if e != nil {
		return errSafe
	}
	reloadedRequest.Header.Set("Origin", cfg.Origin)
	reloadedRequest.Header.Set("X-GovAI-Web", "1")
	reloadedRequest.AddCookie(&http.Cookie{Name: handler.WebSessionCookie, Value: cookie})
	reloaded, e := secondClient.Do(reloadedRequest)
	if e != nil {
		secondServer.Close()
		closeSecond()
		return errSafe
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(reloaded.Body, 65537))
	_ = reloaded.Body.Close()
	l.lifeCheck("WEB_RECOMPOSITION", reloaded.StatusCode == 200)
	secondClient.CloseIdleConnections()
	secondServer.Close()
	closeSecond()
	status, _, _ = request("GET", "/api/v1/web/csrf", nil, "", cfg.Origin)
	l.lifeCheck("WEB_CSRF_RELOAD", status == 200)
	status, _, expiredCookies := request("POST", "/api/v1/web/logout", nil, csrf, cfg.Origin)
	l.lifeCheck("WEB_LOGOUT_CLEARS_COOKIE", status == 204 && len(expiredCookies) == 1 && expiredCookies[0].MaxAge == -1 && len(client.Jar.Cookies(base)) == 0)
	client.Jar.SetCookies(base, []*http.Cookie{{Name: handler.WebSessionCookie, Value: cookie, Path: "/", Secure: true}})
	status, _, _ = request("GET", "/api/v1/web/session", nil, "", cfg.Origin)
	l.lifeCheck("WEB_OLD_COOKIE_REJECTED", status == 401)
	_, e = provider.Refresh(ctx, after.Credentials.RefreshToken)
	l.lifeCheck("WEB_PROVIDER_REFRESH_REVOKED", errors.Is(e, login.ErrFailed))
	var completed int
	if d.pool.QueryRow(ctx, "SELECT count(*) FROM auth.web_revocations WHERE state='done' AND octet_length(ciphertext)=0").Scan(&completed) != nil {
		return errSafe
	}
	l.lifeCheck("WEB_LOGOUT_RECOVERY_RECORD_COMPLETED", completed == 1)
	// Simulate a process stopping after atomic logout, before provider revocation.
	fresh, e := provider.Password(ctx, credentials)
	if e != nil {
		return errSafe
	}
	recoveryCredentials := after.Credentials
	recoveryCredentials.AccessToken = fresh.Token
	recoveryCredentials.RefreshToken = fresh.RefreshToken
	recoveryCookie, e := store.Create(ctx, recoveryCredentials, time.Now().Add(time.Minute))
	if e != nil {
		return errSafe
	}
	bound, e := postgres.NewBoundWebSessionStore(store, issuer, cfg.Provider.ClientID)
	if e != nil {
		return errSafe
	}
	if _, e = bound.TerminateQueued(ctx, recoveryCookie); e != nil {
		return errSafe
	}
	// A reconstructed store claims the persisted recovery record.
	reopened, e := postgres.NewWebSessionStore(d.pool, cfg.DataKey)
	if e != nil {
		return errSafe
	}
	recoveryQueue, e := postgres.NewBoundWebSessionStore(reopened, issuer, cfg.Provider.ClientID)
	if e != nil {
		return errSafe
	}
	queueStatus, e := recoveryQueue.RevocationStatus(ctx)
	l.lifeCheck("WEB_RECOVERY_STATUS_PENDING", e == nil && queueStatus.Pending == 1 && queueStatus.Completed == 1)
	reconciled, e := application.ReconcileWeb(ctx, cfg, d.pool)
	l.lifeCheck("WEB_DURABLE_REVOCATION_RECONCILED", e == nil && reconciled.Completed == 1 && reconciled.Deferred == 0)
	_, e = provider.Refresh(ctx, fresh.RefreshToken)
	l.lifeCheck("WEB_RECOVERED_PROVIDER_TOKEN_REJECTED", errors.Is(e, login.ErrFailed))
	// Offline verification intentionally does not consult provider session state.
	_, e = provider.Verify(ctx, after.Credentials.AccessToken)
	l.lifeCheck("WEB_OFFLINE_ACCESS_RESIDUAL_DOCUMENTED", e == nil)
	if e = l.webExpiryScenario(ctx, cfg.Provider, clientID, credentials); e != nil {
		return e
	}
	return nil
}

func (l *lab) webExpiryScenario(ctx context.Context, cfg login.KeycloakConfig, clientID string, credentials login.Request) error {
	p := "/admin/realms/" + l.realm + "/clients/" + clientID
	client := decode[object](l.must(ctx, "GET", p, nil, 200))
	client["attributes"] = object{"use.refresh.tokens": "true", "access.token.lifespan": "5", "client.session.max.lifespan": "5", "client.session.idle.timeout": "5"}
	l.must(ctx, "PUT", p, client, 204)
	provider, e := login.NewKeycloak(cfg, l.client)
	if e != nil {
		return errSafe
	}
	defer provider.Close()
	issued, e := provider.Password(ctx, credentials)
	l.lifeCheck("EXPIRY_SHORT_LIFETIME_ISSUED", e == nil && issued.ExpiresIn > 0 && issued.ExpiresIn <= 5 && issued.RefreshExpiresIn > 0 && issued.RefreshExpiresIn <= 5)
	// Use a real provider-issued lifetime, not a fabricated token or clock override.
	timer := time.NewTimer(time.Duration(issued.RefreshExpiresIn+2) * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errSafe
	case <-timer.C:
	}
	_, e = provider.Verify(ctx, issued.Token)
	l.lifeCheck("EXPIRY_REAL_ACCESS_REJECTED", e != nil)
	_, e = provider.Refresh(ctx, issued.RefreshToken)
	l.lifeCheck("EXPIRY_REAL_REFRESH_REJECTED", errors.Is(e, login.ErrFailed))
	return nil
}
