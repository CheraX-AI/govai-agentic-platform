package devsetup

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func (a *localAdmin) configureAuthorization(ctx context.Context, root string) error {
	client := a.get(ctx, realmPath+"/clients/"+a.clientID(ctx, mobileLoginClient))
	if client["description"] != ownerTag || client["serviceAccountsEnabled"] != false || client["directAccessGrantsEnabled"] != true {
		return ErrSetup
	}
	settings := map[string]string{"API_AUTH_ENABLED": "true", "API_AUTH_ISSUER": base + "/realms/GovAI", "API_AUTH_AUDIENCE": "govai-api", "API_AUTH_CLIENT_IDS": mobileLoginClient, "API_AUTH_CA_FILE": filepath.ToSlash(filepath.Join(root, "tls", "ca.pem"))}
	text, e := godotenv.Marshal(settings)
	if e != nil {
		return ErrSetup
	}
	path := filepath.Join(root, "..", "..", "backend", "services", "auth-service", ".env.authorization")
	if os.WriteFile(path, []byte(text+"\n"), 0600) != nil {
		return ErrSetup
	}
	fmt.Fprintln(a.out, "DEV_AUTH_API_AUTHORIZATION_CONFIG PASS; no realm, role or user mutation")
	return nil
}

func (a *localAdmin) authorizationCheck(ctx context.Context) error {
	for _, test := range []struct{ method, path, token string }{{"GET", "/api/v1/me", ""}, {"GET", "/api/v1/me", "invalid-token"}, {"POST", "/api/v1/ai/chat", ""}} {
		req, e := http.NewRequestWithContext(ctx, test.method, "https://localhost:5001"+test.path, nil)
		if e != nil {
			return ErrSetup
		}
		if test.token != "" {
			req.Header.Set("Authorization", "Bearer "+test.token)
		}
		r, e := a.client.Do(req)
		if e != nil {
			return ErrSetup
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 4096))
		_ = r.Body.Close()
		if r.StatusCode != 401 || r.Header.Get("Cache-Control") != "no-store" || r.Header.Get("WWW-Authenticate") != "Bearer" {
			fmt.Fprintf(a.out, "DEV_AUTH_PROTECTED_ROUTE status=%d BLOCKED\n", r.StatusCode)
			return ErrSetup
		}
	}
	fmt.Fprintln(a.out, "DEV_AUTH_LOCAL_PROTECTED_401 PASS; valid-token/RBAC evidence is in the isolated live test")
	return nil
}
