package permissionlab

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/application"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/http/handlers"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/postgres"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRefreshLive(t *testing.T) {
	if os.Getenv("GA50_REFRESH_LIVE") != "1" {
		t.Skip("refresh live opt-in disabled")
	}
	state := os.Getenv("GA50_REFRESH_STATE_DIR")
	if state == "" {
		t.Fatal("REFRESH_STATE BLOCKED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if err := guardedCombined(func() error {
		return runWithScenario(ctx, state, os.Stdout, "combined", func(ctx context.Context, l *lab) error {
			return l.loginScenarioWithSessions(ctx, filepath.Join(state, "ca.pem"), true)
		})
	}, os.Stdout); err != nil {
		t.Fatal("REFRESH_LIVE FAIL_OR_BLOCKED")
	}
}

func (l *lab) refreshScenario(ctx context.Context, caFile string, store *postgres.Store, issuer, clientID, secret, email, password string) error {
	p := "/admin/realms/" + l.realm
	// Only the disposable test realm is changed. Existing GovAI is untouched.
	l.must(ctx, "PUT", p, object{"revokeRefreshToken": true, "refreshTokenMaxReuse": 0}, 204)
	client := decode[object](l.must(ctx, "GET", p+"/clients/"+clientID, nil, 200))
	client["attributes"] = object{"use.refresh.tokens": "true", "access.token.lifespan": "300"}
	l.must(ctx, "PUT", p+"/clients/"+clientID, client, 204)
	h, sessions, closeLogin, err := application.LoginWithSessions(config.Login{Enabled: true, Provider: login.KeycloakConfig{Issuer: issuer, ClientID: "lab-login-mobile", ClientSecret: secret, Audience: "govai-api", EnableRefresh: true}, RateKey: freshKey(), CAFile: caFile}, store)
	if err != nil {
		return errSafe
	}
	defer closeLogin()
	server := httptest.NewTLSServer(handler.NewRouterWithSessions(nil, h, nil, sessions))
	defer server.Close()
	httpClient := server.Client()
	httpClient.Timeout = 10 * time.Second
	defer httpClient.CloseIdleConnections()
	post := func(path string, fields map[string]string) (int, map[string]json.RawMessage) {
		body, _ := json.Marshal(fields)
		req, e := http.NewRequestWithContext(ctx, "POST", server.URL+path, bytes.NewReader(body))
		if e != nil {
			panic(errSafe)
		}
		req.Header.Set("Content-Type", "application/json")
		r, e := httpClient.Do(req)
		if e != nil {
			panic(errSafe)
		}
		defer r.Body.Close()
		b, e := io.ReadAll(io.LimitReader(r.Body, 65537))
		if e != nil || len(b) > 65536 || r.Header.Get("Cache-Control") != "no-store" || len(r.Cookies()) != 0 {
			panic(errSafe)
		}
		result := map[string]json.RawMessage{}
		if len(b) > 0 && json.Unmarshal(b, &result) != nil {
			panic(errSafe)
		}
		return r.StatusCode, result
	}
	token := func(m map[string]json.RawMessage, key string) string {
		var s string
		_ = json.Unmarshal(m[key], &s)
		return s
	}
	status, initial := post("/api/v1/auth/login", map[string]string{"email": email, "password": password})
	first := token(initial, "refresh_token")
	l.lifeCheck("REFRESH_LOGIN_ISSUANCE", status == 200 && first != "" && len(initial["id_token"]) == 0)
	status, next := post("/api/v1/auth/refresh", map[string]string{"refresh_token": first})
	second := token(next, "refresh_token")
	l.lifeCheck("REFRESH_ROTATES", status == 200 && second != "" && second != first && token(next, "access_token") != "")
	status, _ = post("/api/v1/auth/refresh", map[string]string{"refresh_token": first})
	l.lifeCheck("REFRESH_REUSE_REJECTED", status == 401)
	// A fresh independent login avoids relying on reuse-detection session effects.
	status, fresh := post("/api/v1/auth/login", map[string]string{"email": email, "password": password})
	current := token(fresh, "refresh_token")
	l.lifeCheck("REFRESH_FRESH_LOGIN", status == 200 && current != "")
	status, _ = post("/api/v1/auth/revoke", map[string]string{"refresh_token": current})
	l.lifeCheck("REFRESH_REVOCATION_ACCEPTED", status == 204)
	status, _ = post("/api/v1/auth/refresh", map[string]string{"refresh_token": current})
	l.lifeCheck("REFRESH_REVOKED_REJECTED", status == 401)
	status, _ = post("/api/v1/auth/refresh", map[string]string{"refresh_token": "synthetic-invalid"})
	l.lifeCheck("REFRESH_INVALID_REJECTED", status == 401)
	return nil
}
