package devsetup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const mobileLoginClient = "govai-login-mobile-dev"

func (a *localAdmin) configureLogin(ctx context.Context, root string) error {
	if a.check(ctx) != nil {
		return ErrSetup
	}
	id, secret := a.ensureClient(ctx, mobileLoginClient, object{"enabled": true, "protocol": "openid-connect", "publicClient": false, "serviceAccountsEnabled": false, "standardFlowEnabled": false, "directAccessGrantsEnabled": true, "implicitFlowEnabled": false, "fullScopeAllowed": false, "secret": randomSecret(), "attributes": object{"use.refresh.tokens": "false", "access.token.lifespan": "300"}})
	client := a.get(ctx, realmPath+"/clients/"+id)
	if client["publicClient"] != false || client["serviceAccountsEnabled"] != false || client["standardFlowEnabled"] != false || client["directAccessGrantsEnabled"] != true || client["fullScopeAllowed"] != false {
		return ErrSetup
	}
	// Scope mapping limits token role visibility. It assigns NO role or privilege
	// to the client/user; a user's citizen role still comes from actual membership.
	citizen := a.get(ctx, realmPath+"/roles/citizen")
	a.must(ctx, "POST", realmPath+"/clients/"+id+"/scope-mappings/realm", []object{citizen}, 204)
	mapper := object{"name": "govai-login-audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper", "consentRequired": false, "config": object{"included.custom.audience": "govai-api", "access.token.claim": "true", "id.token.claim": "false"}}
	a.ensureLoginMapper(ctx, id, mapper)
	verified := object{"name": "govai-login-email-verified", "protocol": "openid-connect", "protocolMapper": "oidc-usermodel-property-mapper", "consentRequired": false, "config": object{"user.attribute": "emailVerified", "claim.name": "email_verified", "jsonType.label": "boolean", "access.token.claim": "true", "id.token.claim": "false", "userinfo.token.claim": "false"}}
	a.ensureLoginMapper(ctx, id, verified)
	state := filepath.Join(root, "..", "..", "backend", "services", "auth-service", ".env.login")
	env, e := godotenv.Read(state)
	if os.IsNotExist(e) {
		env = map[string]string{"LOGIN_RATE_KEY": base64Key()}
	} else if e != nil {
		return ErrSetup
	}
	for k, v := range map[string]string{"LOGIN_ENABLED": "true", "LOGIN_ISSUER_URL": base + "/realms/GovAI", "LOGIN_CLIENT_ID": mobileLoginClient, "LOGIN_CLIENT_SECRET": secret, "LOGIN_AUDIENCE": "govai-api", "LOGIN_CA_FILE": filepath.ToSlash(filepath.Join(root, "tls", "ca.pem"))} {
		env[k] = v
	}
	text, e := godotenv.Marshal(env)
	if e != nil || os.WriteFile(state, []byte(text+"\n"), 0600) != nil {
		return ErrSetup
	}
	fmt.Fprintln(a.out, "DEV_AUTH_MOBILE_LOGIN_CLIENT PASS; no service account, no admin roles, separate secret")
	fmt.Fprintln(a.out, "DEV_AUTH_PRIVATE_LOGIN_SETTINGS PASS; access-only response, web sessions deferred")
	return nil
}
func (a *localAdmin) ensureLoginMapper(ctx context.Context, client string, want object) {
	base := realmPath + "/clients/" + client + "/protocol-mappers/models"
	for _, mapper := range a.list(ctx, base) {
		if mapper["name"] == want["name"] {
			want["id"] = mapper["id"]
			a.must(ctx, "PUT", base+"/"+mapper["id"].(string), want, 204)
			return
		}
	}
	a.must(ctx, "POST", base, want, 201)
}

// Validate the actual local route without requesting or printing a real user's
// password/token. The trusted CA/hostname checks remain enabled in this client.
func (a *localAdmin) loginCheck(ctx context.Context) error {
	if a.check(ctx) != nil {
		return ErrSetup
	}
	client := a.get(ctx, realmPath+"/clients/"+a.clientID(ctx, mobileLoginClient))
	if client["serviceAccountsEnabled"] != false || client["directAccessGrantsEnabled"] != true {
		return ErrSetup
	}
	ready, _ := http.NewRequestWithContext(ctx, "GET", "https://localhost:5001/api/ready", nil)
	r, e := a.client.Do(ready)
	if e != nil {
		return ErrSetup
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 4096))
	_ = r.Body.Close()
	if r.StatusCode != 200 {
		return ErrSetup
	}
	bytesJSON, _ := json.Marshal(map[string]string{"email": "absent-" + randomSecret()[:24] + "@govai.test", "password": randomSecret()})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://localhost:5001/api/v1/auth/login", bytes.NewReader(bytesJSON))
	req.Header.Set("Content-Type", "application/json")
	r, e = a.client.Do(req)
	if e != nil {
		return ErrSetup
	}
	defer r.Body.Close()
	b, e := io.ReadAll(io.LimitReader(r.Body, 4097))
	if e != nil || len(b) > 4096 {
		return ErrSetup
	}
	var result struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(b, &result) != nil || r.StatusCode != 401 || result.Code != "login_failed" || r.Header.Get("Cache-Control") != "no-store" || len(r.Cookies()) != 0 {
		fmt.Fprintf(a.out, "DEV_AUTH_LOCAL_LOGIN status=%d BLOCKED\n", r.StatusCode)
		return ErrSetup
	}
	fmt.Fprintln(a.out, "DEV_AUTH_LOCAL_LOGIN_HTTPS_GENERIC_401 PASS; no real credentials or tokens used")
	return nil
}
