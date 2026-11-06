package login

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRefreshTransport(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"success", 200, `{"access_token":"synthetic-access","refresh_token":"synthetic-next","token_type":"Bearer","expires_in":300,"refresh_expires_in":1800}`, nil},
		{"invalid", 400, `{"error":"invalid_grant","error_description":"private"}`, ErrFailed},
		{"client failure", 401, `{"error":"invalid_client"}`, ErrUnavailable},
		{"missing rotation", 200, `{"access_token":"synthetic-access","token_type":"Bearer","expires_in":300}`, ErrUnavailable},
		{"oversize", 200, strings.Repeat("x", (64<<10)+1), ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/realms/test/protocol/openid-connect/token" {
					t.Error("unexpected request")
				}
				if r.ParseForm() != nil || r.PostForm.Get("grant_type") != "refresh_token" || r.PostForm.Get("refresh_token") != "synthetic-old" || r.PostForm.Get("client_id") != "fixed" || r.PostForm.Get("client_secret") != "synthetic-secret" {
					t.Error("incorrect form")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			k, err := NewKeycloak(KeycloakConfig{Issuer: server.URL + "/realms/test", ClientID: "fixed", ClientSecret: "synthetic-secret", Audience: "api"}, server.Client())
			if err != nil {
				t.Fatal("setup")
			}
			defer k.Close()
			result, err := k.Refresh(context.Background(), "synthetic-old")
			if !errors.Is(err, tc.want) {
				t.Fatalf("unexpected error class: %v", err)
			}
			if calls != 1 {
				t.Fatal("credential request retried")
			}
			if err == nil && (result.RefreshToken != "synthetic-next" || result.ExpiresIn != 300) {
				t.Fatal("incorrect result")
			}
			encoded, _ := json.Marshal(result)
			if strings.Contains(fmt.Sprintf("%+v", result), "synthetic-next") || strings.Contains(string(encoded), "synthetic-next") {
				t.Fatal("token redaction failed")
			}
		})
	}
}

func TestRevokeAndInvalidRefresh(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.ParseForm() != nil || r.URL.Path != "/realms/test/protocol/openid-connect/revoke" || r.PostForm.Get("token_type_hint") != "refresh_token" || r.PostForm.Get("token") != "synthetic-old" {
			t.Error("incorrect revocation")
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	k, err := NewKeycloak(KeycloakConfig{Issuer: server.URL + "/realms/test", ClientID: "fixed", ClientSecret: "synthetic-secret", Audience: "api"}, server.Client())
	if err != nil {
		t.Fatal("setup")
	}
	defer k.Close()
	for _, raw := range []string{"", "a b", "a\n", strings.Repeat("x", (16<<10)+1)} {
		if _, err := k.Refresh(context.Background(), raw); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid refresh accepted")
		}
		if err := k.RevokeRefresh(context.Background(), raw); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid revocation accepted")
		}
	}
	if calls != 0 {
		t.Fatal("invalid input reached provider")
	}
	if err := k.RevokeRefresh(context.Background(), "synthetic-old"); err != nil {
		t.Fatal("revocation failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := k.Refresh(ctx, "synthetic-old"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("cancelled request accepted")
	}
	if calls != 1 {
		t.Fatal("unexpected network request")
	}
}

func TestPasswordRefreshOptIn(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"synthetic-access","refresh_token":"synthetic-refresh","token_type":"Bearer","expires_in":300,"refresh_expires_in":1800}`))
	}))
	defer server.Close()
	for _, enabled := range []bool{false, true} {
		k, err := NewKeycloak(KeycloakConfig{Issuer: server.URL + "/realms/test", ClientID: "fixed", ClientSecret: "synthetic-secret", Audience: "api", EnableRefresh: enabled}, server.Client())
		if err != nil {
			t.Fatal("setup")
		}
		result, err := k.Password(context.Background(), Request{email: "synthetic@example.test", password: "synthetic-password"})
		k.Close()
		if err != nil || ((result.RefreshToken != "") != enabled) {
			t.Fatal("refresh opt-in ignored")
		}
		if !enabled && result.RefreshExpiresIn != 0 {
			t.Fatal("access-only mode retained refresh metadata")
		}
	}
}
