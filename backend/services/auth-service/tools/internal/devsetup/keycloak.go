package devsetup

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const base = "https://localhost:8443"
const realmPath = "/admin/realms/GovAI"
const ownerTag = "govai-local-ga48-v1"
const registrationClient = "govai-registration-service-dev"
const verificationClient = "govai-registration-verify-dev"
const callback = base + "/realms/GovAI/account/"

type object = map[string]any
type localAdmin struct {
	client *http.Client
	token  string
	out    io.Writer
}

func randomSecret() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(ErrSetup)
	}
	return hex.EncodeToString(b)
}
func base64Key() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(ErrSetup)
	}
	return base64.StdEncoding.EncodeToString(b)
}
func mustJSON[T any](b []byte) T {
	var v T
	if json.Unmarshal(b, &v) != nil {
		panic(ErrSetup)
	}
	return v
}
func (a *localAdmin) request(ctx context.Context, method, path string, body any) (int, []byte) {
	if !(path == "/realms/master/protocol/openid-connect/token" || path == "/realms/GovAI/protocol/openid-connect/token" || path == "/realms/GovAI/.well-known/openid-configuration" || path == realmPath || strings.HasPrefix(path, realmPath+"/")) || strings.ContainsAny(path, "\\#") || strings.Contains(path, "..") {
		panic(ErrSetup)
	}
	var reader io.Reader
	contentType := "application/json"
	if form, ok := body.(url.Values); ok {
		reader = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	} else if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			panic(ErrSetup)
		}
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, base+path, reader)
	if e != nil {
		panic(ErrSetup)
	}
	req.Header.Set("Content-Type", contentType)
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	response, e := a.client.Do(req)
	if e != nil {
		fmt.Fprintln(a.out, "DEV_AUTH_TLS_OR_CONNECTION BLOCKED")
		panic(ErrSetup)
	}
	defer response.Body.Close()
	b, e := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if e != nil || len(b) > 1<<20 {
		panic(ErrSetup)
	}
	return response.StatusCode, b
}
func (a *localAdmin) must(ctx context.Context, method, path string, body any, want int) []byte {
	status, b := a.request(ctx, method, path, body)
	if status != want {
		fmt.Fprintf(a.out, "DEV_AUTH_HTTP_UNEXPECTED status=%d expected=%d\n", status, want)
		panic(ErrSetup)
	}
	return b
}
func (a *localAdmin) get(ctx context.Context, path string) object {
	return mustJSON[object](a.must(ctx, "GET", path, nil, 200))
}
func (a *localAdmin) list(ctx context.Context, path string) []object {
	return mustJSON[[]object](a.must(ctx, "GET", path, nil, 200))
}

func connect(ctx context.Context, root string, out io.Writer) *localAdmin {
	pem, e := os.ReadFile(filepath.Join(root, "tls", "ca.pem"))
	if e != nil {
		panic(ErrSetup)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		panic(ErrSetup)
	}
	a := &localAdmin{out: out, client: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	discovery := a.get(ctx, "/realms/GovAI/.well-known/openid-configuration")
	if discovery["issuer"] != base+"/realms/GovAI" {
		fmt.Fprintln(out, "DEV_AUTH_ISSUER BLOCKED")
		panic(ErrSetup)
	}
	env, e := godotenv.Read(filepath.Join(root, ".env"))
	if e != nil || env["KC_BOOTSTRAP_ADMIN_USERNAME"] == "" || env["KC_BOOTSTRAP_ADMIN_PASSWORD"] == "" {
		panic(ErrSetup)
	}
	token := mustJSON[object](a.must(ctx, "POST", "/realms/master/protocol/openid-connect/token", url.Values{"client_id": {"admin-cli"}, "grant_type": {"password"}, "username": {env["KC_BOOTSTRAP_ADMIN_USERNAME"]}, "password": {env["KC_BOOTSTRAP_ADMIN_PASSWORD"]}}, 200))
	a.token, _ = token["access_token"].(string)
	if a.token == "" {
		panic(ErrSetup)
	}
	return a
}

// Run never targets a supplied remote URL or realm and never emits credentials.
func Run(ctx context.Context, root, mode string, out io.Writer) (err error) {
	if mode != "inspect" && mode != "configure" && mode != "check" && mode != "smoke" && mode != "configure-login" && mode != "login-check" && mode != "configure-authorization" && mode != "authorization-check" && mode != "configure-sessions" && mode != "sessions-check" && mode != "recovery-check" && mode != "recovery-sample-check" && mode != "recovery-sample-verification" {
		return ErrSetup
	}
	defer func() {
		if recover() != nil {
			err = ErrSetup
		}
	}()
	a := connect(ctx, root, out)
	defer a.client.CloseIdleConnections()
	realm := a.get(ctx, realmPath)
	profile := a.get(ctx, realmPath+"/users/profile")
	users := a.list(ctx, realmPath+"/users?max=1000&briefRepresentation=true")
	if len(users) >= 1000 {
		fmt.Fprintln(out, "DEV_AUTH_USER_LIMIT BLOCKED")
		return ErrSetup
	}
	smtp, _ := realm["smtpServer"].(map[string]any)
	fmt.Fprintf(out, "DEV_AUTH_INSPECT users=%d verify_email=%t hosted_signup=%t smtp_configured=%t\n", len(users), realm["verifyEmail"] == true, realm["registrationAllowed"] == true, smtp["host"] != nil)
	if mode == "inspect" {
		reuse, _ := realm["refreshTokenMaxReuse"].(float64)
		fmt.Fprintf(out, "DEV_AUTH_REFRESH_POLICY rotation=%t max_reuse=%.0f\n", realm["revokeRefreshToken"] == true, reuse)
		for _, name := range []string{mobileLoginClient, "govai-login-web-dev"} {
			clients := a.list(ctx, realmPath+"/clients?clientId="+url.QueryEscape(name))
			if len(clients) != 1 {
				fmt.Fprintf(out, "DEV_AUTH_CLIENT name=%s configured=false\n", name)
				continue
			}
			attrs, _ := clients[0]["attributes"].(map[string]any)
			fmt.Fprintf(out, "DEV_AUTH_CLIENT name=%s configured=true refresh_enabled=%t service_account=%t owned=%t\n", name, attrs["use.refresh.tokens"] == "true", clients[0]["serviceAccountsEnabled"] == true, clients[0]["description"] == ownerTag)
		}
		return nil
	}
	if mode == "check" {
		if a.check(ctx) != nil {
			return ErrSetup
		}
		return a.diagnose(ctx, root)
	}
	if mode == "smoke" {
		return a.smoke(ctx, root)
	}
	if mode == "configure-sessions" {
		return a.configureSessions(ctx, root)
	}
	if mode == "recovery-sample-verification" {
		return a.recoverySampleCheck(ctx, root, true)
	}
	if mode == "recovery-sample-check" {
		return a.recoverySampleCheck(ctx, root)
	}
	if mode == "recovery-check" {
		return a.recoveryCheck(ctx)
	}
	if mode == "sessions-check" {
		return a.sessionsCheck(ctx)
	}
	if mode == "configure-login" {
		return a.configureLogin(ctx, root)
	}
	if mode == "login-check" {
		return a.loginCheck(ctx)
	}
	if mode == "configure-authorization" {
		return a.configureAuthorization(ctx, root)
	}
	if mode == "authorization-check" {
		return a.authorizationCheck(ctx)
	}
	// Snapshot only changed settings/attribute definitions, never realm secrets or
	// user representations. Files are private and outside tracked source.
	snapshot := object{"time": time.Now().UTC(), "registrationAllowed": realm["registrationAllowed"], "adminPermissionsEnabled": realm["adminPermissionsEnabled"], "verifyEmail": realm["verifyEmail"], "profile": profile, "previous_user_count": len(users)}
	state := filepath.Join(root, "..", "..", "backend", "services", "auth-service", ".local")
	if os.MkdirAll(state, 0700) != nil {
		return ErrSetup
	}
	b, _ := json.MarshalIndent(snapshot, "", "  ")
	if writeNew(filepath.Join(state, "keycloak-before-"+randomSecret()[:16]+".json"), b) != nil {
		return ErrSetup
	}
	fmt.Fprintln(out, "DEV_AUTH_PRIVATE_SETTINGS_SNAPSHOT PASS")
	if e := a.configure(ctx, root, realm, profile); e != nil {
		return e
	}
	after := a.list(ctx, realmPath+"/users?max=1000&briefRepresentation=true")
	ids := map[any]bool{}
	for _, u := range after {
		ids[u["id"]] = true
	}
	for _, u := range users {
		if !ids[u["id"]] {
			fmt.Fprintln(out, "DEV_AUTH_EXISTING_USERS_PRESERVED FAIL")
			return ErrSetup
		}
	}
	fmt.Fprintln(out, "DEV_AUTH_EXISTING_USERS_PRESERVED PASS")
	if a.check(ctx) != nil {
		return ErrSetup
	}
	runtimeFile := filepath.Join(root, "..", "..", "backend", "services", "auth-service", ".env.registration")
	env, e := godotenv.Read(runtimeFile)
	if e != nil {
		return ErrSetup
	}
	env["REGISTRATION_ENABLED"] = "true"
	text, e := godotenv.Marshal(env)
	if e != nil || os.WriteFile(runtimeFile, []byte(text+"\n"), 0600) != nil {
		return ErrSetup
	}
	fmt.Fprintln(out, "DEV_AUTH_REGISTRATION_ACTIVATION_CONFIG PASS")
	return nil
}

func (a *localAdmin) clientID(ctx context.Context, name string) string {
	rows := a.list(ctx, realmPath+"/clients?clientId="+url.QueryEscape(name))
	if len(rows) != 1 {
		panic(ErrSetup)
	}
	id, _ := rows[0]["id"].(string)
	if id == "" {
		panic(ErrSetup)
	}
	return id
}
func (a *localAdmin) ensureClient(ctx context.Context, name string, payload object) (string, string) {
	rows := a.list(ctx, realmPath+"/clients?clientId="+url.QueryEscape(name))
	if len(rows) == 0 {
		payload["clientId"] = name
		payload["description"] = ownerTag
		a.must(ctx, "POST", realmPath+"/clients", payload, 201)
	} else if len(rows) != 1 || rows[0]["description"] != ownerTag {
		fmt.Fprintln(a.out, "DEV_AUTH_EXISTING_CLIENT_NOT_OWNED BLOCKED")
		panic(ErrSetup)
	}
	id := a.clientID(ctx, name)
	if payload["publicClient"] == true {
		return id, ""
	}
	secret := a.get(ctx, realmPath+"/clients/"+id+"/client-secret")
	value, _ := secret["value"].(string)
	if value == "" {
		panic(ErrSetup)
	}
	return id, value
}

func (a *localAdmin) configure(ctx context.Context, root string, realm, profile object) error {
	fmt.Fprintln(a.out, "DEV_AUTH_REALM_SETTINGS APPLY")
	a.must(ctx, "PUT", realmPath, object{"registrationAllowed": false, "verifyEmail": true, "adminPermissionsEnabled": true}, 204)
	attrs, ok := profile["attributes"].([]any)
	if !ok {
		return ErrSetup
	}
	found := false
	for _, raw := range attrs {
		attr := raw.(map[string]any)
		if attr["name"] == "firstName" || attr["name"] == "lastName" {
			delete(attr, "required")
		}
		if attr["name"] == "govai_registration_operation" {
			found = true
			if !protectedMarker(attr) {
				fmt.Fprintln(a.out, "DEV_AUTH_EXISTING_MARKER_UNSAFE BLOCKED")
				return ErrSetup
			}
		}
	}
	if !found {
		attrs = append(attrs, object{"name": "govai_registration_operation", "multivalued": false, "permissions": object{"view": []string{"admin"}, "edit": []string{"admin"}}})
	}
	profile["attributes"] = attrs
	a.must(ctx, "PUT", realmPath+"/users/profile", profile, 200)
	status, roleBody := a.request(ctx, "GET", realmPath+"/roles/citizen", nil)
	if status == 404 {
		a.must(ctx, "POST", realmPath+"/roles", object{"name": "citizen"}, 201)
		roleBody = a.must(ctx, "GET", realmPath+"/roles/citizen", nil, 200)
	} else if status != 200 {
		return ErrSetup
	}
	citizen := mustJSON[object](roleBody)
	current := a.get(ctx, realmPath)
	defaultRole := current["defaultRole"].(map[string]any)["name"].(string)
	composites := a.list(ctx, realmPath+"/roles/"+defaultRole+"/composites/realm")
	hasCitizen := false
	for _, role := range composites {
		if role["name"] == "citizen" {
			hasCitizen = true
		}
	}
	if !hasCitizen {
		a.must(ctx, "POST", realmPath+"/roles/"+defaultRole+"/composites", []object{citizen}, 204)
	}
	groups := a.list(ctx, realmPath+"/groups?search=ga48-registration&exact=true")
	if len(groups) == 0 {
		a.must(ctx, "POST", realmPath+"/groups", object{"name": "ga48-registration", "attributes": object{"govai_dev_owner": []string{ownerTag}}}, 201)
		groups = a.list(ctx, realmPath+"/groups?search=ga48-registration&exact=true")
	}
	if len(groups) != 1 {
		return ErrSetup
	}
	groupID := groups[0]["id"].(string)
	group := a.get(ctx, realmPath+"/groups/"+groupID)
	if !arrayField(group["attributes"], "govai_dev_owner", ownerTag) {
		fmt.Fprintln(a.out, "DEV_AUTH_EXISTING_GROUP_NOT_OWNED BLOCKED")
		return ErrSetup
	}
	clientID, secret := a.ensureClient(ctx, registrationClient, object{"protocol": "openid-connect", "publicClient": false, "serviceAccountsEnabled": true, "standardFlowEnabled": false, "directAccessGrantsEnabled": false, "implicitFlowEnabled": false, "fullScopeAllowed": true, "secret": randomSecret()})
	principal := a.get(ctx, realmPath+"/clients/"+clientID+"/service-account-user")["id"].(string)
	a.ensureClient(ctx, verificationClient, object{"enabled": true, "protocol": "openid-connect", "publicClient": true, "standardFlowEnabled": true, "directAccessGrantsEnabled": false, "implicitFlowEnabled": false, "redirectUris": []string{callback}, "webOrigins": []string{}, "attributes": object{"pkce.code.challenge.method": "S256"}})
	permissionBase := realmPath + "/clients/" + a.clientID(ctx, "admin-permissions") + "/authz/resource-server"
	policies := a.list(ctx, permissionBase+"/policy?name=govai-dev-registration-principal")
	if len(policies) == 0 {
		policy := mustJSON[object](a.must(ctx, "POST", permissionBase+"/policy/user", object{"name": "govai-dev-registration-principal", "description": ownerTag, "type": "user", "logic": "POSITIVE", "users": []string{principal}}, 201))
		policies = []object{policy}
	}
	if len(policies) != 1 || policies[0]["description"] != ownerTag {
		return ErrSetup
	}
	policyID := policies[0]["id"].(string)
	permissions := a.list(ctx, permissionBase+"/permission?name=govai-dev-registration-group")
	permission := object{"name": "govai-dev-registration-group", "description": ownerTag, "type": "scope", "resourceType": "Groups", "resources": []string{groupID}, "scopes": []string{"view", "view-members", "manage-members", "manage-membership"}, "policies": []string{policyID}, "decisionStrategy": "UNANIMOUS"}
	if len(permissions) == 0 {
		a.must(ctx, "POST", permissionBase+"/permission/scope", permission, 201)
	} else if len(permissions) != 1 || permissions[0]["description"] != ownerTag {
		return ErrSetup
	} else {
		permission["id"] = permissions[0]["id"]
		a.must(ctx, "PUT", permissionBase+"/permission/scope/"+permissions[0]["id"].(string), permission, 201)
	}
	rm := a.clientID(ctx, "realm-management")
	queryRole := a.get(ctx, realmPath+"/clients/"+rm+"/roles/query-users")
	a.must(ctx, "POST", realmPath+"/users/"+principal+"/role-mappings/clients/"+rm, []object{queryRole}, 204)
	serviceRoot := filepath.Clean(filepath.Join(root, "..", "..", "backend", "services", "auth-service"))
	runtimeFile := filepath.Join(serviceRoot, ".env.registration")
	env, e := godotenv.Read(runtimeFile)
	if os.IsNotExist(e) {
		env = map[string]string{"REGISTRATION_FINGERPRINT_KEY": base64Key(), "REGISTRATION_RATE_KEY": base64Key()}
	} else if e != nil {
		return ErrSetup
	}
	values := map[string]string{"REGISTRATION_ENABLED": "true", "PORT": "5001", "REGISTRATION_KEYCLOAK_URL": base, "REGISTRATION_REALM": "GovAI", "REGISTRATION_CLIENT_ID": registrationClient, "REGISTRATION_CLIENT_SECRET": secret, "REGISTRATION_GROUP_PATH": "/ga48-registration", "REGISTRATION_VERIFICATION_CLIENT_ID": verificationClient, "REGISTRATION_VERIFICATION_REDIRECT": callback, "REGISTRATION_CA_FILE": filepath.ToSlash(filepath.Join(root, "tls", "ca.pem")), "API_TLS_CERT_FILE": filepath.ToSlash(filepath.Join(root, "tls", "api-cert.pem")), "API_TLS_KEY_FILE": filepath.ToSlash(filepath.Join(root, "tls", "api-key.pem")), "OIDC_ISSUER_URL": base + "/realms/GovAI"}
	for key, value := range values {
		env[key] = value
	}
	env["REGISTRATION_ENABLED"] = "false" // Activate only after the final checks pass.
	text, e := godotenv.Marshal(env)
	if e != nil {
		return ErrSetup
	}
	if os.WriteFile(runtimeFile, []byte(text+"\n"), 0600) != nil {
		return ErrSetup
	}
	fmt.Fprintln(a.out, "DEV_AUTH_PRIVATE_RUNTIME_SETTINGS PASS")
	return nil
}

func arrayField(raw any, key, want string) bool {
	m, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	a, ok := m[key].([]any)
	return ok && len(a) == 1 && a[0] == want
}
func protectedMarker(a map[string]any) bool {
	return a["multivalued"] != true && arrayField(a["permissions"], "view", "admin") && arrayField(a["permissions"], "edit", "admin")
}

func (a *localAdmin) check(ctx context.Context) error {
	realm := a.get(ctx, realmPath)
	if realm["registrationAllowed"] != false || realm["verifyEmail"] != true || realm["adminPermissionsEnabled"] != true {
		return ErrSetup
	}
	client := a.get(ctx, realmPath+"/clients/"+a.clientID(ctx, registrationClient))
	if client["description"] != ownerTag || client["directAccessGrantsEnabled"] == true || client["standardFlowEnabled"] == true || client["serviceAccountsEnabled"] != true {
		return ErrSetup
	}
	principal := a.get(ctx, realmPath+"/clients/"+client["id"].(string)+"/service-account-user")["id"].(string)
	rm := a.clientID(ctx, "realm-management")
	roles := a.list(ctx, realmPath+"/users/"+principal+"/role-mappings/clients/"+rm+"/composite")
	if len(roles) != 1 || roles[0]["name"] != "query-users" {
		fmt.Fprintln(a.out, "DEV_AUTH_ADMIN_ROLES FAIL")
		return ErrSetup
	}
	profile := a.get(ctx, realmPath+"/users/profile")
	found := false
	for _, raw := range profile["attributes"].([]any) {
		attr := raw.(map[string]any)
		if attr["name"] == "govai_registration_operation" {
			found = protectedMarker(attr)
		}
	}
	if !found {
		return ErrSetup
	}
	fmt.Fprintln(a.out, "DEV_AUTH_LOCAL_HTTPS_CONFIGURATION PASS; group-wide prototype authority remains accepted")
	return nil
}
