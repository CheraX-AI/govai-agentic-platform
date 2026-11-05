package devsetup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

const webLoginClient = "govai-login-web-dev"

// configureSessions addresses only the approved fixed local realm. It snapshots
// relevant settings privately before changing owned clients or rotation policy.
func (a *localAdmin) configureSessions(ctx context.Context, root string) error {
	if a.check(ctx) != nil {
		return ErrSetup
	}

	service := filepath.Join(root, "..", "..", "backend", "services", "auth-service")
	webFile := filepath.Join(service, ".env.web")
	loginFile := filepath.Join(service, ".env.login")
	web, e := godotenv.Read(webFile)
	if e != nil {
		return ErrSetup
	}

	mobileEnv, e := godotenv.Read(loginFile)
	if e != nil {
		return ErrSetup
	}

	if web["WEB_ORIGIN"] != "https://localhost:5001" ||
		web["WEB_CLIENT_ID"] != webLoginClient ||
		web["WEB_ISSUER_URL"] != base+"/realms/GovAI" ||
		len(web["WEB_CLIENT_SECRET"]) < 32 {
		return ErrSetup
	}

	if mobileEnv["LOGIN_CLIENT_ID"] != mobileLoginClient {
		return ErrSetup
	}

	realm := a.get(ctx, realmPath)
	mobileID := a.clientID(ctx, mobileLoginClient)
	mobile := a.get(ctx, realmPath+"/clients/"+mobileID)
	if mobile["description"] != ownerTag ||
		mobile["serviceAccountsEnabled"] != false ||
		mobile["publicClient"] != false ||
		mobile["directAccessGrantsEnabled"] != true ||
		mobile["standardFlowEnabled"] != false {
		return ErrSetup
	}

	existingWeb := a.list(ctx, realmPath+"/clients?clientId="+webLoginClient)
	if len(existingWeb) > 1 || (len(existingWeb) == 1 && existingWeb[0]["description"] != ownerTag) {
		return ErrSetup
	}

	users := a.list(ctx, realmPath+"/users?max=1000&briefRepresentation=true")
	if len(users) >= 1000 {
		return ErrSetup
	}

	snapshot := object{
		"rotation":           realm["revokeRefreshToken"],
		"max_reuse":          realm["refreshTokenMaxReuse"],
		"web_client_existed": len(existingWeb) == 1,
		"mobile_client_id":   mobileID,
		"mobile_attributes":  mobile["attributes"],
		"user_count":         len(users),
	}
	backup := filepath.Join(service, ".local", "ga50-rollout-"+randomSecret()[:16])
	if os.MkdirAll(backup, 0700) != nil {
		return ErrSetup
	}

	data, _ := json.MarshalIndent(snapshot, "", "  ")
	if writeNew(filepath.Join(backup, "settings-before.json"), data) != nil {
		return ErrSetup
	}

	for _, name := range []string{".env.web", ".env.login"} {
		data, e := os.ReadFile(filepath.Join(service, name))
		if e != nil || writeNew(filepath.Join(backup, name+".before"), data) != nil {
			return ErrSetup
		}
	}

	fmt.Fprintln(a.out, "DEV_AUTH_SESSIONS_PRIVATE_BACKUP PASS")
	id, secret := a.ensureClient(
		ctx,
		webLoginClient,
		object{
			"enabled":                   true,
			"protocol":                  "openid-connect",
			"publicClient":              false,
			"serviceAccountsEnabled":    false,
			"standardFlowEnabled":       false,
			"directAccessGrantsEnabled": true,
			"implicitFlowEnabled":       false,
			"fullScopeAllowed":          false,
			"secret":                    web["WEB_CLIENT_SECRET"],
			"attributes": object{
				"use.refresh.tokens":          "true",
				"access.token.lifespan":       "300",
				"client.session.max.lifespan": "1800",
				"client.session.idle.timeout": "1800",
			},
		},
	)
	client := a.get(ctx, realmPath+"/clients/"+id)
	if client["publicClient"] != false ||
		client["serviceAccountsEnabled"] != false ||
		client["standardFlowEnabled"] != false ||
		client["directAccessGrantsEnabled"] != true ||
		client["implicitFlowEnabled"] != false ||
		client["fullScopeAllowed"] != false {
		return ErrSetup
	}

	attrs, _ := client["attributes"].(map[string]any)
	if attrs["use.refresh.tokens"] != "true" {
		return ErrSetup
	}

	citizen := a.get(ctx, realmPath+"/roles/citizen")
	a.must(ctx, "POST", realmPath+"/clients/"+id+"/scope-mappings/realm", []object{citizen}, 204)
	for _, mapper := range []object{
		{
			"name":           "govai-web-audience",
			"protocol":       "openid-connect",
			"protocolMapper": "oidc-audience-mapper",
			"config": object{
				"included.custom.audience": "govai-api",
				"access.token.claim":       "true",
				"id.token.claim":           "false",
			},
		},
		{
			"name":           "govai-web-email-verified",
			"protocol":       "openid-connect",
			"protocolMapper": "oidc-usermodel-property-mapper",
			"config": object{
				"user.attribute":     "emailVerified",
				"claim.name":         "email_verified",
				"jsonType.label":     "boolean",
				"access.token.claim": "true",
				"id.token.claim":     "false",
			},
		},
	} {
		a.ensureLoginMapper(ctx, id, mapper)
	}

	mobileAttrs, _ := mobile["attributes"].(map[string]any)
	if mobileAttrs == nil {
		mobileAttrs = map[string]any{}
	}

	mobileAttrs["use.refresh.tokens"] = "true"
	mobileAttrs["access.token.lifespan"] = "300"
	mobile["attributes"] = mobileAttrs
	a.must(ctx, "PUT", realmPath+"/clients/"+mobileID, mobile, 204)
	// This realm-wide policy change was explicitly included in local approval.
	a.must(ctx, "PUT", realmPath, object{"revokeRefreshToken": true, "refreshTokenMaxReuse": 0}, 204)
	after := a.list(ctx, realmPath+"/users?max=1000&briefRepresentation=true")
	ids := map[any]bool{}
	for _, u := range after {
		ids[u["id"]] = true
	}

	if len(after) != len(users) {
		return ErrSetup
	}

	for _, u := range users {
		if !ids[u["id"]] {
			return ErrSetup
		}
	}

	if a.check(ctx) != nil {
		return ErrSetup
	}

	mobileEnv["LOGIN_REFRESH_ENABLED"] = "true"
	web["WEB_CLIENT_SECRET"] = secret
	web["WEB_SESSION_ENABLED"] = "true"
	for path, env := range map[string]map[string]string{loginFile: mobileEnv, webFile: web} {
		text, e := godotenv.Marshal(env)
		if e != nil || os.WriteFile(path, []byte(text+"\n"), 0600) != nil {
			return ErrSetup
		}
	}

	fmt.Fprintln(
		a.out,
		"DEV_AUTH_SESSIONS_CLIENTS_ROTATION PASS; separate non-admin clients, rotation enabled, max reuse zero",
	)
	fmt.Fprintln(a.out, "DEV_AUTH_SESSIONS_USERS_PRESERVED PASS; existing users unchanged")
	fmt.Fprintln(a.out, "DEV_AUTH_SESSIONS_PRIVATE_SETTINGS PASS; restart owned API to activate")
	return nil
}

func (a *localAdmin) sessionsCheck(ctx context.Context) error {
	tests := []struct {
		method, path, origin, body string
		want                       int
	}{
		{"GET", "/api/ready", "", "", 200},
		{"POST", "/api/v1/auth/refresh", "", `{"refresh_token":"synthetic-invalid"}`, 401},
		{"POST", "/api/v1/auth/revoke", "", `{"refresh_token":"synthetic-invalid"}`, 204},
		{"GET", "/api/v1/web/session", "https://localhost:5001", "", 401},
		{"GET", "/api/v1/web/csrf", "https://localhost:5001", "", 401},
		{"POST", "/api/v1/web/refresh", "https://localhost:5001", "", 401},
		{"POST", "/api/v1/web/login", "https://evil.test", `{}`, 403},
		{
			"POST",
			"/api/v1/web/login",
			"https://localhost:5001",
			`{"email":"absent-` + randomSecret()[:16] + `@govai.test","password":"` + randomSecret() + `"}`,
			401,
		},
	}
	for _, test := range tests {
		req, e := http.NewRequestWithContext(ctx, test.method, "https://localhost:5001"+test.path, bytes.NewBufferString(test.body))
		if e != nil {
			return ErrSetup
		}

		if test.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}

		if test.origin != "" {
			req.Header.Set("Origin", test.origin)
			req.Header.Set("X-GovAI-Web", "1")
		}

		response, e := a.client.Do(req)
		if e != nil {
			return ErrSetup
		}

		body, e := io.ReadAll(io.LimitReader(response.Body, 4097))
		_ = response.Body.Close()
		if e != nil ||
			len(body) > 4096 ||
			response.StatusCode != test.want ||
			response.Header.Get("Cache-Control") != "no-store" ||
			len(response.Cookies()) != 0 {
			fmt.Fprintf(
				a.out,
				"DEV_AUTH_SESSION_ROUTE status=%d expected=%d BLOCKED\n",
				response.StatusCode,
				test.want,
			)
			return ErrSetup
		}
	}

	fmt.Fprintln(
		a.out,
		"DEV_AUTH_LOCAL_SESSION_ROUTES PASS; readiness, generic invalid credentials, revocation and browser boundaries",
	)
	return nil
}
