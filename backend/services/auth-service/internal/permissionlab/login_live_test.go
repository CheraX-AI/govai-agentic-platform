package permissionlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/application"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/handler"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/login"
	"github.com/gin-gonic/gin"
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
)

func TestLoginLive(t *testing.T) {
	if os.Getenv("GA48_LOGIN_LIVE") != "1" {
		t.Skip("login live opt-in disabled")
	}
	state := os.Getenv("GA48_LOGIN_STATE_DIR")
	if state == "" {
		t.Fatal("LOGIN_STATE BLOCKED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	err := guardedCombined(func() error {
		return runWithScenario(ctx, state, os.Stdout, "combined", func(ctx context.Context, l *lab) error { return l.loginScenario(ctx, filepath.Join(state, "ca.pem")) })
	}, os.Stdout)
	if err != nil {
		t.Fatal("LOGIN_LIVE FAIL_OR_BLOCKED")
	}
	decoder := json.NewDecoder(bytes.NewReader(logs.Bytes()))
	counts := map[string]int{}
	for {
		var entry map[string]any
		e := decoder.Decode(&entry)
		if e == io.EOF {
			break
		}
		if e != nil || entry["msg"] != "http request" || len(entry) != 8 {
			t.Fatal("LOGIN_LOGS FAIL")
		}
		for k := range entry {
			switch k {
			case "time", "level", "msg", "request_id", "method", "route", "status", "duration_ms":
			default:
				t.Fatal("LOGIN_LOG_SCHEMA FAIL")
			}
		}
		route, _ := entry["route"].(string)
		counts[route]++
	}
	if len(counts) != 3 || counts["/api/v1/auth/login"] != 6 || counts["/api/v1/me"] != 4 || counts["/api/v1/ai/chat"] != 2 {
		t.Fatal("LOGIN_LOG_COUNT FAIL")
	}
	fmt.Println("LOGIN_SAFE_LOGS PASS")
}
func (l *lab) loginScenario(ctx context.Context, caFile string) error {
	d, e := newDurableLab(ctx)
	if e != nil {
		return errSafe
	}
	defer func() { l.lifeCheck("LOGIN_SQL_CLEANUP", d.close() == nil) }()
	cfg, group, _, _ := l.combinedSetup(ctx)
	// This disposable realm's actor stays in the same tested scoped registration model.
	registration, _, closeRegistration := l.durableService(d, cfg, group, 601, false)
	defer closeRegistration()
	email := "login" + randomID() + "@example.test"
	password := randomID()
	if _, e = registration.Register(ctx, combinedKey(601), combinedBody(601, email, password)); e != nil {
		return errSafe
	}
	issuer := l.base + "/realms/" + l.realm
	op := d.operation(ctx, issuer, combinedKey(601))
	secret := randomID()
	p := "/admin/realms/" + l.realm
	l.must(ctx, "POST", p+"/clients", object{"clientId": "lab-login-mobile", "enabled": true, "protocol": "openid-connect", "publicClient": false, "serviceAccountsEnabled": false, "standardFlowEnabled": false, "directAccessGrantsEnabled": true, "implicitFlowEnabled": false, "fullScopeAllowed": false, "secret": secret, "attributes": object{"use.refresh.tokens": "false", "access.token.lifespan": "300"}}, 201)
	clientID := l.clientID(ctx, "lab-login-mobile")
	citizen := decode[object](l.must(ctx, "GET", p+"/roles/citizen", nil, 200))
	l.must(ctx, "POST", p+"/clients/"+clientID+"/scope-mappings/realm", []object{citizen}, 204)
	for _, mapper := range []object{
		{"name": "login-audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper", "config": object{"included.custom.audience": "govai-api", "access.token.claim": "true", "id.token.claim": "false"}},
		{"name": "login-verified", "protocol": "openid-connect", "protocolMapper": "oidc-usermodel-property-mapper", "config": object{"user.attribute": "emailVerified", "claim.name": "email_verified", "jsonType.label": "boolean", "access.token.claim": "true", "id.token.claim": "false"}},
	} {
		l.must(ctx, "POST", p+"/clients/"+clientID+"/protocol-mappers/models", mapper, 201)
	}
	client := decode[object](l.must(ctx, "GET", p+"/clients/"+clientID, nil, 200))
	l.lifeCheck("LOGIN_SEPARATE_NONADMIN_CLIENT", client["serviceAccountsEnabled"] == false && client["standardFlowEnabled"] == false && client["directAccessGrantsEnabled"] == true)
	h, closeLogin, e := application.Login(config.Login{Enabled: true, Provider: login.KeycloakConfig{Issuer: issuer, ClientID: "lab-login-mobile", ClientSecret: secret, Audience: "govai-api"}, RateKey: freshKey(), CAFile: caFile}, d.store)
	if e != nil {
		return errSafe
	}
	defer closeLogin()
	protected, closeAuthorization, e := application.Authorization(config.Authorization{Enabled: true, Issuer: issuer, Audience: "govai-api", Clients: []string{"lab-login-mobile"}, CAFile: caFile}, d.store)
	if e != nil {
		return errSafe
	}
	defer closeAuthorization()
	previous := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	defer gin.SetMode(previous)
	server := httptest.NewTLSServer(handler.NewRouterWithProtected(nil, h, protected))
	defer server.Close()
	httpClient := server.Client()
	httpClient.Timeout = 10 * time.Second
	defer httpClient.CloseIdleConnections()
	requestLogin := func(user, pw string) (int, []byte) {
		b, _ := json.Marshal(map[string]string{"email": user, "password": pw})
		req, e := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/v1/auth/login", bytes.NewReader(b))
		if e != nil {
			panic(errSafe)
		}
		req.Header.Set("Content-Type", "application/json")
		r, e := httpClient.Do(req)
		if e != nil {
			panic(errSafe)
		}
		defer r.Body.Close()
		body, e := io.ReadAll(io.LimitReader(r.Body, 16385))
		if e != nil || len(body) > 16384 || r.Header.Get("Cache-Control") != "no-store" || len(r.Cookies()) != 0 {
			panic(errSafe)
		}
		return r.StatusCode, body
	}
	unverified, unverifiedBody := requestLogin(email, password)
	l.lifeCheck("LOGIN_UNVERIFIED_DENIED", unverified == 401 && l.realUser(ctx, op.Subject)["emailVerified"] == false)
	link, e := l.verificationLink(ctx, email)
	if e != nil {
		return errSafe
	}
	if _, _, e = l.visitVerification(ctx, link); e != nil {
		return errSafe
	}
	l.lifeCheck("LOGIN_EMAIL_LINK_COMPLETED", l.realUser(ctx, op.Subject)["emailVerified"] == true)
	wrong, wrongBody := requestLogin(email, "wrong-credential-value")
	unknown, unknownBody := requestLogin("missing"+randomID()+"@example.test", password)
	l.lifeCheck("LOGIN_GENERIC_FAILURE_EQUIVALENCE", wrong == 401 && unknown == 401 && bytes.Equal(unverifiedBody, wrongBody) && bytes.Equal(wrongBody, unknownBody))
	status, body := requestLogin(email, password)
	var result struct {
		Access  string `json:"access_token"`
		Type    string `json:"token_type"`
		Expires int64  `json:"expires_in"`
	}
	if json.Unmarshal(body, &result) != nil {
		return errSafe
	}
	l.lifeCheck("LOGIN_VERIFIED_PROFILE_SUCCESS", status == 200 && result.Access != "" && result.Type == "Bearer" && result.Expires > 0 && result.Expires <= 300 && !strings.Contains(string(body), "refresh_token") && !strings.Contains(string(body), "id_token"))
	verifier, e := login.NewKeycloak(login.KeycloakConfig{Issuer: issuer, ClientID: "lab-login-mobile", ClientSecret: secret, Audience: "govai-api"}, l.client)
	if e != nil {
		return errSafe
	}
	defer verifier.Close()
	verified, e := verifier.Verify(ctx, result.Access)
	l.lifeCheck("LOGIN_SIGNATURE_ISSUER_AUDIENCE_CITIZEN", e == nil && verified.Subject == op.Subject && verified.Verified && verified.Citizen)
	// Verify refresh suppression in the live provider, without printing its reply.
	rawStatus, raw, e := l.request(ctx, "", "POST", "/realms/"+l.realm+"/protocol/openid-connect/token", url.Values{"grant_type": {"password"}, "client_id": {"lab-login-mobile"}, "client_secret": {secret}, "username": {email}, "password": {password}, "scope": {"email"}})
	var envelope map[string]json.RawMessage
	if e != nil || json.Unmarshal(raw, &envelope) != nil {
		return errSafe
	}
	l.lifeCheck("LOGIN_UPSTREAM_REFRESH_ID_SUPPRESSED", rawStatus == 200 && len(envelope["refresh_token"]) == 0 && len(envelope["id_token"]) == 0)
	protectedRequest := func(method, path, token string) (int, []byte) {
		req, e := http.NewRequestWithContext(ctx, method, server.URL+path, nil)
		if e != nil {
			panic(errSafe)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r, e := httpClient.Do(req)
		if e != nil {
			panic(errSafe)
		}
		defer r.Body.Close()
		b, e := io.ReadAll(io.LimitReader(r.Body, 16385))
		if e != nil || len(b) > 16384 || r.Header.Get("Cache-Control") != "no-store" {
			panic(errSafe)
		}
		return r.StatusCode, b
	}
	meStatus, meBody := protectedRequest("GET", "/api/v1/me", result.Access)
	var me struct {
		Subject  string `json:"subject"`
		Verified bool   `json:"email_verified"`
	}
	_ = json.Unmarshal(meBody, &me)
	l.lifeCheck("AUTH_VALID_TOKEN_OWN_PROFILE", meStatus == 200 && me.Subject == op.Subject && me.Verified)
	noToken, _ := protectedRequest("GET", "/api/v1/me", "")
	badToken, _ := protectedRequest("GET", "/api/v1/me", "invalid-token")
	l.lifeCheck("AUTH_MISSING_INVALID_401", noToken == 401 && badToken == 401)
	chatMissing, _ := protectedRequest("POST", "/api/v1/ai/chat", "")
	chatValid, chatBody := protectedRequest("POST", "/api/v1/ai/chat", result.Access)
	l.lifeCheck("AUTH_CHAT_GUARDED_PLACEHOLDER", chatMissing == 401 && chatValid == 501 && strings.Contains(string(chatBody), "chat_not_implemented"))
	// Remove/restore only the disposable test user's default role membership to
	// obtain a real signed non-citizen token. No existing GovAI role/user changes.
	realmConfig := decode[object](l.must(ctx, "GET", p, nil, 200))
	defaultRole := realmConfig["defaultRole"].(map[string]any)
	l.must(ctx, "DELETE", p+"/users/"+op.Subject+"/role-mappings/realm", []object{defaultRole}, 204)
	roleStatus, roleBody, e := l.request(ctx, "", "POST", "/realms/"+l.realm+"/protocol/openid-connect/token", url.Values{"grant_type": {"password"}, "client_id": {"lab-login-mobile"}, "client_secret": {secret}, "username": {email}, "password": {password}, "scope": {"email"}})
	var noRole struct {
		Access string `json:"access_token"`
	}
	if e != nil || json.Unmarshal(roleBody, &noRole) != nil || roleStatus != 200 {
		return errSafe
	}
	forbidden, _ := protectedRequest("GET", "/api/v1/me", noRole.Access)
	l.lifeCheck("AUTH_SIGNED_WRONG_ROLE_403", forbidden == 403)
	l.must(ctx, "POST", p+"/users/"+op.Subject+"/role-mappings/realm", []object{defaultRole}, 204)
	l.must(ctx, "PUT", p+"/users/"+op.Subject, object{"requiredActions": []string{"UPDATE_PASSWORD"}}, 204)
	actions, actionsBody := requestLogin(email, password)
	l.lifeCheck("LOGIN_PENDING_ACTIONS_DENIED", actions == 401 && bytes.Equal(actionsBody, unverifiedBody))
	limited, _ := requestLogin(email, password)
	l.lifeCheck("LOGIN_ATTEMPT_LIMIT", limited == 429)
	return nil
}
