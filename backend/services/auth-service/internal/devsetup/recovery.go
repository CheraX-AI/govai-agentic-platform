package devsetup

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Fixed loopback API, validated by the existing local CA client. No real NIC,
// password, reset link, token or cookie is sent, read from inbox or displayed.
func (a *localAdmin) recoveryCheck(ctx context.Context) error {
	if a.check(ctx) != nil {
		return ErrSetup
	}
	for _, test := range []struct {
		method, path, body, code string
		status                   int
	}{
		{"GET", "/api/ready", "", "", 200},
		{"POST", "/api/v1/auth/forgot-password", `{"nic":"invalid"}`, "invalid_request", 400},
		{"POST", "/api/v1/auth/reset-password", `{"token":"invalid","new_password":"synthetic-only","confirm_password":"synthetic-only"}`, "invalid_request", 400},
		{"OPTIONS", "/api/v1/auth/reset-password", "", "", 204},
	} {
		req, err := http.NewRequestWithContext(ctx, test.method, "https://localhost:5001"+test.path, strings.NewReader(test.body))
		if err != nil {
			return ErrSetup
		}
		if test.method == "POST" {
			req.Header.Set("Content-Type", "application/json")
		}
		if test.method == "OPTIONS" {
			req.Header.Set("Origin", "https://localhost:5001")
			req.Header.Set("Access-Control-Request-Method", "POST")
			req.Header.Set("Access-Control-Request-Headers", "Content-Type")
		}
		response, err := a.client.Do(req)
		if err != nil {
			return ErrSetup
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
		_ = response.Body.Close()
		if err != nil || len(body) > 4096 || response.StatusCode != test.status || response.Header.Get("Cache-Control") != "no-store" {
			return ErrSetup
		}
		if test.code != "" {
			var result struct {
				Code string `json:"code"`
			}
			if json.Unmarshal(body, &result) != nil || result.Code != test.code {
				return ErrSetup
			}
		}
		if test.method == "OPTIONS" && (response.Header.Get("Access-Control-Allow-Origin") != "https://localhost:5001" || response.Header.Get("Access-Control-Allow-Credentials") != "") {
			return ErrSetup
		}
	}
	fmt.Fprintln(a.out, "DEV_AUTH_LOCAL_RECOVERY PASS; readiness, invalid-body and CORS boundaries only; manual reset still pending")
	return nil
}

// Metadata-only inspection of the saved automatically generated test account.
// This never resets credentials, marks email verified, or reads token material.
func (a *localAdmin) recoverySampleCheck(ctx context.Context, root string, sendVerification ...bool) error {
	path := filepath.Join(root, "..", "..", "services", "auth-service", ".local", "registration-smoke.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ErrSetup
	}
	var sample smokeCase
	if json.Unmarshal(data, &sample) != nil || !strings.HasPrefix(sample.Email, "govai-local-smoke-") || !strings.HasSuffix(sample.Email, "@govai.test") {
		return ErrSetup
	}
	users := a.list(ctx, realmPath+"/users?email="+url.QueryEscape(sample.Email)+"&exact=true")
	if len(users) != 1 {
		fmt.Fprintln(a.out, "DEV_AUTH_SAVED_SAMPLE found=false")
		return nil
	}
	user := a.get(ctx, realmPath+"/users/"+users[0]["id"].(string))
	actions, _ := user["requiredActions"].([]any)
	fmt.Fprintf(a.out, "DEV_AUTH_SAVED_SAMPLE found=true enabled=%t email_verified=%t required_actions=%d\n", user["enabled"] == true, user["emailVerified"] == true, len(actions))
	if len(sendVerification) == 1 && sendVerification[0] {
		if user["enabled"] != true {
			return ErrSetup
		}
		if user["emailVerified"] == true {
			fmt.Fprintln(a.out, "DEV_AUTH_SAVED_SAMPLE already_verified=true")
			return nil
		}
		settings, err := godotenv.Read(filepath.Join(root, "..", "..", "services", "auth-service", ".env.registration"))
		if err != nil {
			return ErrSetup
		}
		callback := settings["REGISTRATION_VERIFICATION_REDIRECT"]
		target, err := url.Parse(callback)
		if err != nil || target.Scheme != "https" || target.Host != "localhost:8443" || target.User != nil || target.RawQuery != "" || target.Fragment != "" || settings["REGISTRATION_VERIFICATION_CLIENT_ID"] == "" {
			return ErrSetup
		}
		q := url.Values{"client_id": {settings["REGISTRATION_VERIFICATION_CLIENT_ID"]}, "redirect_uri": {callback}, "lifespan": {"900"}}
		// Only the exact saved synthetic account; no credential/verified flag changes.
		a.must(ctx, "PUT", realmPath+"/users/"+users[0]["id"].(string)+"/send-verify-email?"+q.Encode(), nil, 204)
		fmt.Fprintln(a.out, "DEV_AUTH_SAVED_SAMPLE_VERIFICATION_SENT PASS; open existing Keycloak verification inbox on localhost:8025")
	}
	return nil
}
