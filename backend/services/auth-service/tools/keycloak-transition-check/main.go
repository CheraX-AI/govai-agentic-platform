// keycloak-transition-check checks private offline exports and fixed loopback
// rehearsal/live admin endpoints. It never prints identity or credential values.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/keycloak"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/login"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
	"github.com/joho/godotenv"
)

type object = map[string]any

func require(ok bool, label string) {
	if !ok {
		panic(errors.New(label))
	}
}

func read(path string) object {
	b, e := os.ReadFile(path)
	require(e == nil, "export_read")
	var v object
	require(json.Unmarshal(b, &v) == nil, "export_json")
	return v
}

func normal(v any) any {
	switch x := v.(type) {
	case map[string]any:
		r := object{}
		for k, v := range x {
			r[k] = normal(v)
		}
		return r
	case []any:
		r := make([]any, len(x))
		for i, v := range x {
			r[i] = normal(v)
		}
		sort.Slice(r, func(i, j int) bool {
			a, _ := json.Marshal(r[i])
			b, _ := json.Marshal(r[j])
			return string(a) < string(b)
		})
		return r
	default:
		return v
	}
}

func equal(a, b any) bool {
	x, _ := json.Marshal(normal(a))
	y, _ := json.Marshal(normal(b))
	return bytes.Equal(x, y)
}

func list(v any) []any {
	if v == nil {
		return nil
	}

	return v.([]any)
}

func compareUser(a, b object) {
	for _, k := range []string{
		"id",
		"username",
		"email",
		"emailVerified",
		"enabled",
		"attributes",
		"realmRoles",
		"clientRoles",
		"groups",
		"requiredActions",
	} {
		if v, ok := a[k]; ok {
			require(equal(v, b[k]), "user_"+k)
		}
	}

	for _, raw := range list(a["credentials"]) {
		c := raw.(object)
		found := false
		for _, r := range list(b["credentials"]) {
			d := r.(object)
			if equal(c["type"], d["type"]) &&
				equal(c["secretData"], d["secretData"]) &&
				equal(c["credentialData"], d["credentialData"]) {
				found = true
			}
		}

		require(found, "credential_material")
	}
}

func compare(dir, target string) {
	for _, name := range []string{"GovAI", "master"} {
		a := read(filepath.Join(dir, "export", name+"-realm.json"))
		b := read(filepath.Join(dir, target, name+"-realm.json"))
		for _, k := range []string{
			"id",
			"realm",
			"enabled",
			"verifyEmail",
			"registrationAllowed",
			"passwordPolicy",
			"revokeRefreshToken",
			"refreshTokenMaxReuse",
			"ssoSessionIdleTimeout",
			"ssoSessionMaxLifespan",
			"smtpServer",
			"roles",
			"groups",
			"components",
			"authenticationFlows",
			"authenticatorConfig",
			"requiredActions",
			"browserFlow",
			"registrationFlow",
			"directGrantFlow",
			"resetCredentialsFlow",
		} {
			if v, ok := a[k]; ok {
				if k == "roles" && name == "GovAI" {
					sourceRoles := v.(object)["client"].(object)
					targetRoles := b[k].(object)["client"].(object)
					if len(list(sourceRoles["admin-permissions"])) == 0 {
						generated := list(targetRoles["admin-permissions"])
						require(
							len(generated) == 1 &&
								generated[0].(object)["name"] == "uma_protection" &&
								generated[0].(object)["clientRole"] == true,
							"internal_authz_role_materialization",
						)
						targetRoles["admin-permissions"] = []any{}
						fmt.Println("TRANSITION_INTERNAL_AUTHZ_UMA_ROLE_MATERIALIZATION VERIFIED")
					}
				}

				require(equal(v, b[k]), "realm_"+k)
			}
		}

		users := map[string]object{}
		for _, r := range list(b["users"]) {
			u := r.(object)
			users[u["id"].(string)] = u
		}

		for _, r := range list(a["users"]) {
			u := r.(object)
			v, ok := users[u["id"].(string)]
			require(ok, "user_id")
			compareUser(u, v)
		}

		originalIDs := map[string]bool{}
		for _, raw := range list(a["users"]) {
			originalIDs[raw.(object)["id"].(string)] = true
		}

		for id, user := range users {
			if originalIDs[id] {
				continue
			}

			require(
				name == "GovAI" &&
					user["username"] == "service-account-admin-permissions" &&
					user["serviceAccountClientId"] == "admin-permissions",
				"unexpected_imported_user",
			)
		}

		clients := map[string]object{}
		for _, r := range list(b["clients"]) {
			c := r.(object)
			clients[c["clientId"].(string)] = c
		}

		for _, r := range list(a["clients"]) {
			c := r.(object)
			d, ok := clients[c["clientId"].(string)]
			require(ok, "client_present")
			for _, k := range []string{
				"id",
				"secret",
				"enabled",
				"publicClient",
				"bearerOnly",
				"serviceAccountsEnabled",
				"standardFlowEnabled",
				"directAccessGrantsEnabled",
				"implicitFlowEnabled",
				"fullScopeAllowed",
				"redirectUris",
				"webOrigins",
				"attributes",
				"protocolMappers",
				"defaultClientScopes",
				"optionalClientScopes",
				"authorizationSettings",
			} {
				if v, ok := c[k]; ok {
					if k == "protocolMappers" {
						for _, raw := range list(v) {
							sourceMapper := raw.(object)
							for _, item := range list(d[k]) {
								targetMapper := item.(object)
								if sourceMapper["id"] != targetMapper["id"] {
									continue
								}

								sc, _ := sourceMapper["config"].(object)
								tc, _ := targetMapper["config"].(object)
								if _, exists := sc["userinfo.token.claim"]; !exists &&
									tc["userinfo.token.claim"] == "false" {
									delete(tc, "userinfo.token.claim")
								}
							}
						}
					}

					if k == "attributes" {
						sourceAttributes := v.(object)
						targetAttributes := d[k].(object)
						if _, present := sourceAttributes["post.logout.redirect.uris"]; !present {
							if value, exists := targetAttributes["post.logout.redirect.uris"]; exists {
								require(
									value == "" ||
										(value == "+" &&
											(c["clientId"] == "realm-management" ||
												c["clientId"] == "GovAI-realm" ||
												c["clientId"] == "master-realm") &&
											len(list(c["redirectUris"])) == 0 &&
											len(list(d["redirectUris"])) == 0),
									"unexpected_logout_redirect",
								)
								delete(targetAttributes, "post.logout.redirect.uris")
							}
						}

						if _, present := sourceAttributes["client.secret.creation.time"]; !present {
							delete(targetAttributes, "client.secret.creation.time")
						}
					}

					if k == "authorizationSettings" && !equal(v, d[k]) {
						aAuth := v.(object)
						// Keycloak serializes scope sets inside policy config JSON strings.
						// Compare their parsed arrays; every scope value remains checked.
						for _, settings := range []object{aAuth, d[k].(object)} {
							for _, raw := range list(settings["policies"]) {
								policy := raw.(object)
								if policy["type"] != "scope" {
									continue
								}

								cfg, ok := policy["config"].(object)
								if !ok {
									continue
								}

								value, ok := cfg["scopes"].(string)
								if !ok {
									continue
								}

								var scopes []string
								require(json.Unmarshal([]byte(value), &scopes) == nil, "policy_scope_json")
								sort.Strings(scopes)
								encoded, err := json.Marshal(scopes)
								require(err == nil, "policy_scope_json")
								cfg["scopes"] = string(encoded)
							}
						}

						bAuth := d[k].(object)
						for _, collection := range []string{"resources", "policies", "scopes"} {
							for _, raw := range list(aAuth[collection]) {
								item := raw.(object)
								for _, candidate := range list(bAuth[collection]) {
									other := candidate.(object)
									if item["name"] == other["name"] {
										for field, value := range item {
											if !equal(value, other[field]) {
												fmt.Println("TRANSITION_AUTHZ_DIFFERENCE", collection, field)
											}
										}
									}
								}
							}
						}
					}

					require(equal(v, d[k]), "client_"+k)
				}
			}
		}

		fmt.Printf(
			"TRANSITION_EXPORT_COMPARE realm=%s original_users=%d original_clients=%d PASS\n",
			name,
			len(list(a["users"])),
			len(list(a["clients"])),
		)
	}
}

func request(ctx context.Context, c *http.Client, base, token, method, path string, body any, want int) object {
	var r io.Reader
	content := "application/json"
	switch b := body.(type) {
	case url.Values:
		r = strings.NewReader(b.Encode())
		content = "application/x-www-form-urlencoded"
	case nil:
	default:
		x, _ := json.Marshal(b)
		r = bytes.NewReader(x)
	}

	req, e := http.NewRequestWithContext(ctx, method, base+path, r)
	require(e == nil, "http_request")
	req.Header.Set("Content-Type", content)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	response, e := c.Do(req)
	require(e == nil, "tls_or_connection")
	defer response.Body.Close()
	data, e := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	require(
		e == nil &&
			response.StatusCode == want,
		fmt.Sprintf("http_status_%d_expected_%d", response.StatusCode, want),
	)
	if len(data) == 0 {
		return object{}
	}

	var v object
	require(json.Unmarshal(data, &v) == nil, "http_json")
	return v
}

func fresh() string {
	var b [16]byte
	_, e := rand.Read(b[:])
	require(e == nil, "random")
	return hex.EncodeToString(b[:])
}

func online(ctx context.Context, base, dir string, probe bool, align bool) {
	require(
		base == "https://localhost:8443" || base == "https://localhost:8444",
		"fixed_loopback_only",
	)
	require(!probe || base == "https://localhost:8444", "probe_rehearsal_only")
	root, e := filepath.Abs("../../../infrastructure/keycloak")
	require(e == nil, "root")
	pem, e := os.ReadFile(filepath.Join(root, "tls", "ca.pem"))
	require(e == nil, "ca")
	roots := x509.NewCertPool()
	require(roots.AppendCertsFromPEM(pem), "ca_parse")
	client := &http.Client{
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}},
		Timeout:       8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	env, e := godotenv.Read(filepath.Join(root, ".env"))
	require(e == nil, "admin_configuration")
	token := request(
		ctx,
		client,
		base,
		"",
		"POST",
		"/realms/master/protocol/openid-connect/token",
		url.Values{
			"client_id":  {"admin-cli"},
			"grant_type": {"password"},
			"username":   {env["KC_BOOTSTRAP_ADMIN_USERNAME"]},
			"password":   {env["KC_BOOTSTRAP_ADMIN_PASSWORD"]},
		},
		200,
	)["access_token"].(string)
	discovery := request(ctx, client, base, "", "GET", "/realms/GovAI/.well-known/openid-configuration", nil, 200)
	require(discovery["issuer"] == base+"/realms/GovAI", "issuer")
	realm := request(ctx, client, base, token, "GET", "/admin/realms/GovAI", nil, 200)
	source := read(filepath.Join(dir, "export", "GovAI-realm.json"))
	require(realm["id"] == source["id"], "online_realm_id")
	require(realm["verifyEmail"] == true && realm["revokeRefreshToken"] == true, "online_policy")
	if align {
		alignDefaults(ctx, client, base, token, dir)
	}

	fmt.Println("TRANSITION_ADMIN_CREDENTIAL_ISSUER_REALM_POLICY PASS")
	if !probe {
		return
	}

	registrationEnv, e := godotenv.Read(".env")
	require(e == nil, "registration_configuration")
	cfg := keycloak.RegistrationConfig{
		BaseURL:               base,
		Realm:                 "GovAI",
		ClientID:              registrationEnv["REGISTRATION_CLIENT_ID"],
		ClientSecret:          registrationEnv["REGISTRATION_CLIENT_SECRET"],
		RegistrationGroupPath: registrationEnv["REGISTRATION_GROUP_PATH"],
		VerificationClientID:  registrationEnv["REGISTRATION_VERIFICATION_CLIENT_ID"],
		VerificationRedirect:  registrationEnv["REGISTRATION_VERIFICATION_REDIRECT"],
		VerificationLifespan:  15 * time.Minute,
		CallTimeout:           5 * time.Second,
		MaxResponseBytes:      65536,
		TokenSkew:             5 * time.Second,
	}
	admin, e := keycloak.NewRegistrationAdmin(cfg, client)
	require(e == nil, "registration_adapter")
	defer admin.Close()
	password := fresh() + fresh()
	email := "transition-" + fresh() + "@govai.test"
	payload, _ := json.Marshal(
		map[string]string{
			"full_name":     "Synthetic Transition Probe",
			"nic":           "000000001V",
			"mobile_number": "+14155552671",
			"email":         email,
			"password":      password,
		},
	)
	parsed, invalid := registration.ParseRequest(bytes.NewReader(payload))
	require(invalid == nil, "synthetic_request")
	operation := fresh()
	identity, e := admin.CreateDisabled(ctx, parsed, operation)
	require(e == nil && !identity.Enabled, "scoped_create")
	require(admin.EnableOwned(ctx, identity.Subject, operation) == nil, "scoped_enable")
	require(
		admin.SendVerification(ctx, identity.Subject, operation) == nil,
		"smtp_verification_request",
	)
	// Only this new rehearsal-only synthetic account is marked verified. Existing
	// imported credentials, profiles and email-verification flags are never edited.
	request(
		ctx,
		client,
		base,
		token,
		"PUT",
		"/admin/realms/GovAI/users/"+identity.Subject,
		object{"emailVerified": true, "requiredActions": []string{}},
		204,
	)
	settings, e := godotenv.Read(".env")
	require(e == nil, "login_configuration")
	for _, prefix := range []string{"LOGIN_", "WEB_"} {

		provider, e := login.NewKeycloak(
			login.KeycloakConfig{
				Issuer:        base + "/realms/GovAI",
				ClientID:      settings[prefix+"CLIENT_ID"],
				ClientSecret:  settings[prefix+"CLIENT_SECRET"],
				Audience:      "govai-api",
				EnableRefresh: true,
			},
			client,
		)
		require(e == nil, "login_adapter")
		r, e := login.Parse(strings.NewReader(fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)))
		require(e == nil, "login_parse")
		issued, e := provider.Password(ctx, r)
		require(e == nil, "synthetic_login")
		claims, e := provider.Verify(ctx, issued.Token)
		require(
			e == nil &&
				claims.Subject == identity.Subject &&
				claims.Verified &&
				claims.Citizen,
			"token_claims",
		)
		rotated, e := provider.Refresh(ctx, issued.RefreshToken)
		require(e == nil, "refresh")
		require(provider.RevokeRefresh(ctx, rotated.RefreshToken) == nil, "revoke")
		provider.Close()
	}

	request(
		ctx,
		client,
		base,
		token,
		"DELETE",
		"/admin/realms/GovAI/users/"+identity.Subject,
		nil,
		204,
	)
	fmt.Println("TRANSITION_REHEARSAL_SCOPED_REGISTRATION_SMTP_AND_BOTH_LOGIN_CLIENTS PASS")
}

func alignDefaults(ctx context.Context, client *http.Client, base, token, dir string) {
	for _, realmName := range []string{"GovAI", "master"} {
		source := read(filepath.Join(dir, "export", realmName+"-realm.json"))
		for _, raw := range list(source["clients"]) {
			original := raw.(object)
			name := original["clientId"]
			if name == "realm-management" || name == "GovAI-realm" || name == "master-realm" {
				continue
			}

			path := "/admin/realms/" + realmName + "/clients/" + original["id"].(string)
			current := request(ctx, client, base, token, "GET", path, nil, 200)
			changed := false
			if name == "admin-permissions" &&
				!equal(current["fullScopeAllowed"], original["fullScopeAllowed"]) {
				require(
					original["fullScopeAllowed"] == false &&
						current["fullScopeAllowed"] == true,
					"unexpected_internal_scope_change",
				)
				current["fullScopeAllowed"] = false
				changed = true
			}

			oldAttrs, _ := original["attributes"].(object)
			if _, present := oldAttrs["post.logout.redirect.uris"]; !present {
				attrs, _ := current["attributes"].(object)
				if attrs == nil {
					attrs = object{}
				}

				if value, exists := attrs["post.logout.redirect.uris"]; exists {
					require(value == "+" || value == "", "unexpected_import_logout_default")
				}

				attrs["post.logout.redirect.uris"] = ""
				current["attributes"] = attrs
				changed = true
			}

			if changed {
				request(ctx, client, base, token, "PUT", path, current, 204)
			}

			if realmName == "GovAI" && name == "admin-permissions" {
				all := listRequest(ctx, client, base, token, "/admin/realms/GovAI/client-scopes")
				scopeIDs := map[string]string{}
				for _, scope := range all {
					scopeIDs[scope["name"].(string)] = scope["id"].(string)
				}

				for _, kind := range []string{"default", "optional"} {
					expected := map[string]bool{}
					for _, v := range list(original[kind+"ClientScopes"]) {
						expected[v.(string)] = true
					}

					currentScopes := listRequest(ctx, client, base, token, path+"/"+kind+"-client-scopes")
					present := map[string]bool{}
					for _, scope := range currentScopes {
						scopeName := scope["name"].(string)
						present[scopeName] = true
						if !expected[scopeName] {
							request(
								ctx,
								client,
								base,
								token,
								"DELETE",
								path+"/"+kind+"-client-scopes/"+scope["id"].(string),
								nil,
								204,
							)
						}
					}

					for scopeName := range expected {
						if !present[scopeName] {
							id := scopeIDs[scopeName]
							require(id != "", "source_scope_missing")
							request(ctx, client, base, token, "PUT", path+"/"+kind+"-client-scopes/"+id, nil, 204)
						}
					}
				}
			}
		}
	}

	fmt.Println("TRANSITION_SOURCE_CLIENT_SCOPE_AND_UNCONFIGURED_CALLBACK_POLICY_RESTORED PASS")
}

func listRequest(ctx context.Context, c *http.Client, base, token, path string) []object {
	req, e := http.NewRequestWithContext(ctx, "GET", base+path, nil)
	require(e == nil, "scope_request")
	req.Header.Set("Authorization", "Bearer "+token)
	response, e := c.Do(req)
	require(e == nil, "scope_tls")
	defer response.Body.Close()
	body, e := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	require(e == nil && response.StatusCode == 200, "scope_status")
	var result []object
	require(json.Unmarshal(body, &result) == nil, "scope_json")
	return result
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				fmt.Fprintln(os.Stderr, "TRANSITION_CHECK_FAILED", e.Error())
			} else {
				fmt.Fprintln(os.Stderr, "TRANSITION_CHECK_FAILED")
			}

			os.Exit(1)
		}
	}()
	dir := flag.String("dir", "", "private transition directory")
	target := flag.String("compare", "", "rehearsal-export or final-export")
	base := flag.String("url", "", "fixed local endpoint")
	probe := flag.Bool("probe", false, "rehearsal-only synthetic probe")
	align := flag.Bool(
		"align-import-defaults",
		false,
		"restore source absent logout defaults in approved local target",
	)
	flag.Parse()
	root, e := filepath.Abs("../../../infrastructure/postgres/keycloak/.local")
	require(e == nil, "state_root")
	absolute, e := filepath.Abs(*dir)
	require(
		e == nil && strings.HasPrefix(absolute, root+string(os.PathSeparator)),
		"private_state_path",
	)
	if *target != "" {
		require(*target == "rehearsal-export" || *target == "final-export", "comparison_path")
		compare(absolute, *target)
	}

	if *base != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		online(ctx, *base, absolute, *probe, *align)
	}

}
