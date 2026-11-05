package keycloak

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

const usersPath = "/admin/realms/GovAI/users"

func config(base string) RegistrationConfig {
	return RegistrationConfig{
		RegistrationGroupPath: "/ga48-registration",
		BaseURL:               base,
		Realm:                 "GovAI",
		ClientID:              "registration-only",
		ClientSecret:          "synthetic-secret",
		VerificationClientID:  "action-client",
		VerificationRedirect:  "https://app.example.test/verified",
		VerificationLifespan:  15 * time.Minute,
		CallTimeout:           time.Second,
		MaxResponseBytes:      4096,
		TokenSkew:             5 * time.Second,
	}
}

func adapter(t *testing.T, handler http.HandlerFunc) (*Admin, *httptest.Server) {
	t.Helper()
	s := httptest.NewTLSServer(handler)
	t.Cleanup(s.Close)
	a, err := NewRegistrationAdmin(config(s.URL), s.Client())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(a.Close)
	return a, s
}

func request(t *testing.T) registration.Request {
	t.Helper()
	r, err := registration.ParseRequest(
		strings.NewReader(
			`{"full_name":"Synthetic Citizen","nic":"000000001V","mobile_number":"+14155552671","email":"synthetic@example.test","password":" password &+= "}`,
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	return r
}

func assertError(t *testing.T, err error, code registration.AdminCode, unknown bool) {
	t.Helper()
	var e *registration.AdminError
	if !errors.As(err, &e) || e.Code != code || e.UnknownOutcome != unknown {
		t.Fatalf("outcome = %v, want %s unknown=%v", err, code, unknown)
	}

	if strings.Contains(err.Error(), "synthetic") {
		t.Fatal("unsafe error")
	}
}

func issueToken(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"access_token":"synthetic-token","token_type":"Bearer","expires_in":60}`)
}

func TestCreateConstructionAndCache(t *testing.T) {
	var tokens, creates atomic.Int32
	var base string
	a, s := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/realms/GovAI/protocol/openid-connect/token" {
			tokens.Add(1)
			if r.Method != "POST" ||
				r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
				r.Header.Get("Authorization") != "" {
				t.Error("incorrect token request")
			}

			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}

			if len(r.PostForm) != 3 ||
				r.PostForm.Get("grant_type") != "client_credentials" ||
				r.PostForm.Get("client_id") != "registration-only" ||
				r.PostForm.Get("client_secret") != "synthetic-secret" {
				t.Error("incorrect administrative credentials form")
			}

			issueToken(w)
			return
		}

		creates.Add(1)
		if r.Method != "POST" ||
			r.URL.Path != usersPath ||
			r.Header.Get("Authorization") != "Bearer synthetic-token" ||
			r.Header.Get("Content-Type") != "application/json" {
			t.Error("incorrect create request")
		}

		var body map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 8 {
			t.Error("unexpected create shape")
		}

		for _, key := range []string{
			"username",
			"email",
			"enabled",
			"emailVerified",
			"requiredActions",
			"attributes",
			"credentials",
			"groups",
		} {
			if body[key] == nil {
				t.Error("missing allowlisted field")
			}
		}

		if string(body["enabled"]) != "false" ||
			string(body["emailVerified"]) != "false" ||
			string(body["requiredActions"]) != `["VERIFY_EMAIL"]` ||
			string(body["attributes"]) != `{"govai_registration_operation":["operation-1"]}` {
			t.Error("unsafe initial state")
		}

		if string(body["groups"]) != `["/ga48-registration"]` {
			t.Error("incorrect server group representation")
		}

		var credentials []struct {
			Type, Value string
			Temporary   bool
		}
		if json.Unmarshal(body["credentials"], &credentials) != nil ||
			len(credentials) != 1 ||
			credentials[0].Type != "password" ||
			credentials[0].Value != " password &+= " ||
			credentials[0].Temporary {
			t.Error("credential changed")
		}

		w.Header().Set("Location", base+usersPath+"/subject-1")
		w.WriteHeader(201)
	})
	base = s.URL
	for i := 0; i < 2; i++ {
		u, err := a.CreateDisabled(context.Background(), request(t), "operation-1")
		if err != nil || u.Subject != "subject-1" || u.Enabled || u.EmailVerified {
			t.Fatalf("incorrect create result: %v", err)
		}
	}

	if tokens.Load() != 1 || creates.Load() != 2 {
		t.Fatal("token not cached")
	}
}

func TestTokenExpiryAndConcurrentRetrieval(t *testing.T) {
	var count atomic.Int32
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {

		count.Add(1)
		issueToken(w)
	})
	now := time.Now()
	a.now = func() time.Time { return now }
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.serviceToken(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}

	wg.Wait()
	if count.Load() != 1 {
		t.Fatal("concurrent cache misses were not serialized")
	}

	now = now.Add(55 * time.Second) // expires_in less configured skew, boundary is expired
	if _, err := a.serviceToken(context.Background()); err != nil {
		t.Fatal(err)
	}

	if count.Load() != 2 {
		t.Fatal("expired token reused")
	}
}

func TestTokenFailuresNotCached(t *testing.T) {
	for _, body := range []string{
		`{`,
		`{"access_token":"x","token_type":"Bearer","expires_in":0}`,
		`{"access_token":"x","token_type":"Bearer","expires_in":5}`,
		`{"access_token":"x","token_type":"Bearer","expires_in":86401}`,
		`{"access_token":"x","token_type":"Other","expires_in":60}`,
		`{"token_type":"Bearer","expires_in":60}`,
		`{"access_token":"x\r\ny","token_type":"Bearer","expires_in":60}`,
		strings.Repeat("x", 4097),
	} {
		t.Run(fmt.Sprint(len(body), body[:1]), func(t *testing.T) {
			var count atomic.Int32
			a, _ := adapter(
				t,
				func(w http.ResponseWriter, r *http.Request) {

					count.Add(1)
					_, _ = io.WriteString(w, body)
				},
			)
			for i := 0; i < 2; i++ {
				_, err := a.serviceToken(context.Background())
				assertError(t, err, registration.AdminUpstream, false)
			}

			if count.Load() != 2 {
				t.Fatal("failed token cached")
			}
		})
	}
}

func TestStatusMappingAndNoRetry(t *testing.T) {
	for _, tc := range []struct {
		status  int
		code    registration.AdminCode
		unknown bool
	}{
		{400, registration.AdminPolicyRejected, true},
		{401, registration.AdminPermission, false},
		{403, registration.AdminPermission, false},
		{409, registration.AdminDuplicate, false},
		{500, registration.AdminAmbiguous, true},
		{429, registration.AdminAmbiguous, true},
		{302, registration.AdminAmbiguous, true},
		{200, registration.AdminAmbiguous, true},
	} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			var creates atomic.Int32
			a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					issueToken(w)
					return
				}

				creates.Add(1)
				w.Header().Set("Location", "https://elsewhere.example.test/")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, `{"errorMessage":"synthetic secret personal data"}`)
			})
			_, err := a.CreateDisabled(context.Background(), request(t), "operation-1")
			assertError(t, err, tc.code, tc.unknown)
			if creates.Load() != 1 {
				t.Fatal("mutation retried")
			}
		})
	}
}

func TestTokenHTTPFailures(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   registration.AdminCode
	}{
		{400, registration.AdminPermission},
		{401, registration.AdminPermission},
		{403, registration.AdminPermission},
		{500, registration.AdminUpstream},
		{302, registration.AdminUpstream},
	} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			var calls atomic.Int32
			a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/must-not-follow")
				w.WriteHeader(tc.status)
			})
			_, err := a.CreateDisabled(context.Background(), request(t), "operation-1")
			assertError(t, err, tc.code, false)
			if calls.Load() != 1 {
				t.Fatal("token error retried or redirect followed")
			}
		})
	}
}

func TestUnauthorizedInvalidatesWithoutReplay(t *testing.T) {
	var tokens, creates atomic.Int32
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			tokens.Add(1)
			issueToken(w)
			return
		}

		creates.Add(1)
		w.WriteHeader(401)
	})
	for i := 0; i < 2; i++ {
		_, err := a.CreateDisabled(context.Background(), request(t), "op")
		assertError(t, err, registration.AdminPermission, false)
	}

	if tokens.Load() != 2 || creates.Load() != 2 {
		t.Fatal("401 replayed or stale token retained")
	}
}

func TestLocationValidation(t *testing.T) {
	for _, location := range []string{
		"",
		"https://evil.example.test" + usersPath + "/id",
		"http://HOST" + usersPath + "/id",
		"//HOST" + usersPath + "/id",
		usersPath + "/../id",
		usersPath + "/id/extra",
		usersPath + "/id?x=1",
		usersPath + "/id#",
		usersPath + "/%2e%2e",
		usersPath + "/id%2Fextra",
		"/admin/realms/Other/users/id",
		"id",
		usersPath + "/id/",
		"https://user:pass@HOST" + usersPath + "/id",
		usersPath + "/id,other",
	} {
		t.Run(location, func(t *testing.T) {
			a, s := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					issueToken(w)
					return
				}

				w.Header().Set("Location", location)
				w.WriteHeader(201)
			})
			location = strings.ReplaceAll(location, "HOST", strings.TrimPrefix(s.URL, "https://"))
			_, err := a.CreateDisabled(context.Background(), request(t), "op")
			assertError(t, err, registration.AdminAmbiguous, true)
		})
	}

	for _, absolute := range []bool{false, true} {
		t.Run(fmt.Sprint("valid", absolute), func(t *testing.T) {
			var base string
			a, s := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					issueToken(w)
					return
				}

				location := usersPath + "/id"
				if absolute {
					location = base + location
				}

				w.Header().Set("Location", location)
				w.WriteHeader(201)
			})
			base = s.URL
			u, err := a.CreateDisabled(context.Background(), request(t), "op")
			if err != nil || u.Subject != "id" {
				t.Fatal("valid location rejected", err)
			}
		})
	}

	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			issueToken(w)
			return
		}

		w.Header().Add("Location", usersPath+"/id")
		w.Header().Add("Location", usersPath+"/other")
		w.WriteHeader(201)
	})
	_, err := a.CreateDisabled(context.Background(), request(t), "op")
	assertError(t, err, registration.AdminAmbiguous, true)
}

func TestOwnedOperations(t *testing.T) {
	var enabled atomic.Bool
	var mail atomic.Int32
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			issueToken(w)
			return
		}

		if r.Header.Get("Authorization") != "Bearer synthetic-token" {
			t.Error("missing admin authorization")
		}

		switch {
		case r.Method == "GET":
			user := fmt.Sprintf(
				`{"id":"id","enabled":%t,"emailVerified":false,"attributes":{"govai_registration_operation":["op"]}}`,
				enabled.Load(),
			)
			if r.URL.Path == usersPath {
				if r.URL.Query().Get("q") != marker+":op" ||
					r.URL.Query().Get("max") != "2" ||
					r.URL.Query().Get("briefRepresentation") != "false" {
					t.Error("unsafe search")
				}

				user = "[" + user + "]"
			}
			_, _ = io.WriteString(w, user)
		case r.Method == "PUT" && r.URL.Path == usersPath+"/id":
			body, _ := io.ReadAll(r.Body)
			if string(body) != `{"enabled":true}` {
				t.Error("update overwrites other state")
			}
			enabled.Store(true)
			w.WriteHeader(204)
		case r.Method == "PUT" && r.URL.Path == usersPath+"/id/send-verify-email":
			q := r.URL.Query()
			if len(q) != 3 ||
				q.Get("client_id") != "action-client" ||
				q.Get("redirect_uri") != "https://app.example.test/verified" ||
				q.Get("lifespan") != "900" {
				t.Error("unsafe action configuration")
			}
			mail.Add(1)
			w.WriteHeader(204)
		default:
			t.Error("unexpected operation")
			w.WriteHeader(500)
		}
	})
	u, err := a.FindOwned(context.Background(), "op")
	if err != nil || u.Subject != "id" {
		t.Fatal(err)
	}

	if err = a.SendVerification(context.Background(), "id", "op"); err == nil {
		t.Fatal("email for disabled user")
	}

	if err = a.EnableOwned(context.Background(), "id", "op"); err != nil {
		t.Fatal(err)
	}

	if err = a.SendVerification(context.Background(), "id", "op"); err != nil {
		t.Fatal(err)
	}

	if mail.Load() != 1 {
		t.Fatal("email not requested")
	}
}

func TestOwnershipAndReconciliationFailures(t *testing.T) {
	for _, tc := range []struct {
		body string
		code registration.AdminCode
	}{
		{`[]`, registration.AdminNotFound},
		{`null`, registration.AdminUpstream},
		{`{`, registration.AdminUpstream},
		{
			`[{"id":"id","attributes":{"govai_registration_operation":["other"]}}]`,
			registration.AdminOwnership,
		},
		{`[{"id":"id"},{"id":"other"}]`, registration.AdminOwnership},
		{
			`[{"id":"id","attributes":{"govai_registration_operation":["op","op"]}}]`,
			registration.AdminOwnership,
		},
	} {
		t.Run(tc.body, func(t *testing.T) {
			a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					issueToken(w)
					return
				}

				_, _ = io.WriteString(w, tc.body)
			})
			_, err := a.FindOwned(context.Background(), "op")
			assertError(t, err, tc.code, false)
		})
	}

	var mutations atomic.Int32
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			issueToken(w)
			return
		}

		if r.Method != "GET" {
			mutations.Add(1)
		}

		_, _ = io.WriteString(w, `{"id":"id","attributes":{"govai_registration_operation":["other"]}}`)
	})
	assertError(t, a.EnableOwned(context.Background(), "id", "op"), registration.AdminOwnership, false)
	assertError(
		t,
		a.SendVerification(context.Background(), "id", "op"),
		registration.AdminOwnership,
		false,
	)
	if mutations.Load() != 0 {
		t.Fatal("mutated unowned account")
	}
}

func TestCancellationAndTimeouts(t *testing.T) {
	for _, create := range []bool{false, true} {
		t.Run(fmt.Sprint(create), func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			var calls atomic.Int32
			a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				if create && strings.HasSuffix(r.URL.Path, "/token") {
					issueToken(w)
					return
				}

				calls.Add(1)
				close(started)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			})
			defer close(release)
			a.cfg.CallTimeout = 100 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			r := request(t)
			go func() {

				_, err := a.CreateDisabled(ctx, r, "op")
				result <- err
			}()
			<-started
			cancel()
			select {
			case err := <-result:
				if create {
					assertError(t, err, registration.AdminAmbiguous, true)
				} else {
					assertError(t, err, registration.AdminCanceled, false)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation not propagated")
			}

			if calls.Load() != 1 {
				t.Fatal("request retried")
			}
		})
	}
	// Total operation deadline applies even if the supplied client had no timeout.
	release := make(chan struct{})
	defer close(release)
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			issueToken(w)
			return
		}

		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	a.cfg.CallTimeout = 30 * time.Millisecond
	_, err := a.CreateDisabled(context.Background(), request(t), "op")
	assertError(t, err, registration.AdminAmbiguous, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = a.CreateDisabled(ctx, request(t), "op")
	assertError(t, err, registration.AdminCanceled, false)
	// A waiter must be cancellable while another request owns the token gate.
	a.gate <- struct{}{}
	_, err = a.serviceToken(ctx)
	assertError(t, err, registration.AdminCanceled, false)
	<-a.gate
}

func TestResponseBoundsAndReadFailures(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		t.Run(fmt.Sprint(oversized), func(t *testing.T) {
			a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					issueToken(w)
					return
				}

				w.Header().Set("Location", usersPath+"/id")
				if !oversized {
					w.Header().Set("Content-Length", "100")
				}

				w.WriteHeader(201)
				if oversized {
					_, _ = io.WriteString(w, strings.Repeat("x", 4097))
				} else {
					_, _ = io.WriteString(w, "x")
				}
			})
			_, err := a.CreateDisabled(context.Background(), request(t), "op")
			assertError(t, err, registration.AdminAmbiguous, true)
		})
	}
}

func TestTLSAndConfiguration(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { issueToken(w) }))
	defer s.Close()
	a, err := NewRegistrationAdmin(config(s.URL), &http.Client{})
	if err != nil {
		t.Fatal(err)
	}

	defer a.Close()
	_, err = a.serviceToken(context.Background())
	assertError(t, err, registration.AdminUpstream, false) // unknown CA
	for _, change := range []func(*RegistrationConfig){
		func(c *RegistrationConfig) { c.BaseURL = "http://localhost:8180" },
		func(c *RegistrationConfig) { c.BaseURL = s.URL + "/../escape" },
		func(c *RegistrationConfig) { c.BaseURL = s.URL + "?q=secret" },
		func(c *RegistrationConfig) { c.Realm = "../master" },
		func(c *RegistrationConfig) { c.ClientSecret = "" },
		func(c *RegistrationConfig) { c.CallTimeout = 6 * time.Second },
		func(c *RegistrationConfig) { c.MaxResponseBytes = 2 << 20 },
		func(c *RegistrationConfig) { c.VerificationRedirect = "http://app.example.test" },
		func(c *RegistrationConfig) { c.VerificationLifespan = 0 },
		func(c *RegistrationConfig) { c.TokenSkew = -time.Second },
	} {
		cfg := config(s.URL)
		change(&cfg)
		_, err = NewRegistrationAdmin(cfg, s.Client())
		assertError(t, err, registration.AdminInvalid, false)
	}

	for _, tlsConfig := range []*tls.Config{
		{InsecureSkipVerify: true},
		{ServerName: "other.test"},
		{VerifyConnection: func(tls.ConnectionState) error { return nil }},
	} {
		transport := s.Client().Transport.(*http.Transport).Clone()
		transport.TLSClientConfig = tlsConfig
		_, err = NewRegistrationAdmin(config(s.URL), &http.Client{Transport: transport})
		assertError(t, err, registration.AdminInvalid, false)
	}

	_, err = NewRegistrationAdmin(config(s.URL), nil)
	assertError(t, err, registration.AdminInvalid, false)
}

func TestSensitiveFormattingAndNoLogs(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(old)
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) { issueToken(w) })
	if _, err := a.serviceToken(context.Background()); err != nil {
		t.Fatal(err)
	}

	if logs.Len() != 0 {
		t.Fatal("adapter emitted logs")
	}

	for _, v := range []any{a, a.cfg, registration.Identity{Subject: "synthetic-person"}, request(t)} {
		if fmt.Sprintf("%+v", v) != "[REDACTED]" || fmt.Sprintf("%#v", v) != "[REDACTED]" {
			t.Fatal("unsafe formatting")
		}

		body, err := json.Marshal(v)
		if err != nil || string(body) != `"[REDACTED]"` {
			t.Fatal("unsafe serialization")
		}

		slog.Info("redaction test", "value", v)
	}

	for _, secret := range []string{
		"synthetic-secret",
		"synthetic-token",
		"synthetic-person",
		"synthetic@example.test",
		"/ga48-registration",
	} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("unsafe structured logging")
		}
	}
}

func TestRegistrationGroupConfigRequiredBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer s.Close()
	for _, group := range []string{
		"",
		"registration",
		"/",
		"//registration",
		"/registration/",
		"/parent/child",
		"/.",
		"/..",
		"/a/../b",
		"/%72egistration",
		"/registration?x=1",
		"/registration#x",
		"/registration name",
		"/registration\\other",
		"/citizen\n",
		"/සිංහල",
		"/" + strings.Repeat("a", 129),
	} {
		cfg := config(s.URL)
		cfg.RegistrationGroupPath = group
		a, err := NewRegistrationAdmin(cfg, s.Client())
		assertError(t, err, registration.AdminInvalid, false)
		if a != nil {
			t.Fatal("invalid group constructed adapter")
		}
	}

	if calls.Load() != 0 {
		t.Fatal("configuration validation contacted upstream")
	}
}

func TestServerGroupIsCopiedAndCallerCannotAssignRoles(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			issueToken(w)
			return
		}

		var body map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid body")
		}

		if string(body["groups"]) != `["/server-registration"]` || len(body) != 8 {
			t.Error("server group or payload allowlist changed")
		}

		for _, field := range []string{"roles", "realmRoles", "clientRoles", "group", "full_name", "nic", "mobile_number"} {
			if _, ok := body[field]; ok {
				t.Error("caller field escaped allowlist")
			}
		}

		w.Header().Set("Location", usersPath+"/id")
		w.WriteHeader(201)
	}))
	defer s.Close()
	cfg := config(s.URL)
	cfg.RegistrationGroupPath = "/server-registration"
	a, err := NewRegistrationAdmin(cfg, s.Client())
	if err != nil {
		t.Fatal(err)
	}

	defer a.Close()
	cfg.RegistrationGroupPath = "/attacker-controlled"
	r := request(t)
	r.FullName = `{"groups":["/administrators"],"realmRoles":["admin"]}`
	if _, err = a.CreateDisabled(context.Background(), r, "op"); err != nil {
		t.Fatal(err)
	}
}

func TestPublicRegistrationRejectsGroupAndRoleOverrides(t *testing.T) {
	base := map[string]any{
		"full_name":     "Synthetic Citizen",
		"nic":           "000000001V",
		"mobile_number": "+14155552671",
		"email":         "synthetic@example.test",
		"password":      "synthetic-password",
	}
	for _, field := range []string{
		"group",
		"groups",
		"registration_group_path",
		"RegistrationGroupPath",
		"role",
		"roles",
		"realmRoles",
		"clientRoles",
	} {
		base[field] = []string{"administrators"}
		body, _ := json.Marshal(base)
		_, err := registration.ParseRequest(strings.NewReader(string(body)))
		if err == nil || err.Code != "invalid_request" {
			t.Fatal("caller privilege field was accepted")
		}

		delete(base, field)
	}
}

func TestTrustedRootDoesNotBypassHostname(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("credential request reached a server with the wrong hostname")
	}))
	defer s.Close()
	client := s.Client()
	transport := client.Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, s.Listener.Addr().String())
	}
	client.Transport = transport
	a, err := NewRegistrationAdmin(config("https://wrong-host.invalid"), client)
	if err != nil {
		t.Fatal(err)
	}

	defer a.Close()
	_, err = a.serviceToken(context.Background())
	assertError(t, err, registration.AdminUpstream, false)
}

func TestTokenDeadlinePreventsCreation(t *testing.T) {
	var creates atomic.Int32
	release := make(chan struct{})
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/token") {
			creates.Add(1)
			return
		}

		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	defer close(release)
	a.cfg.CallTimeout = 30 * time.Millisecond
	_, err := a.CreateDisabled(context.Background(), request(t), "op")
	assertError(t, err, registration.AdminCanceled, false)
	if creates.Load() != 0 {
		t.Fatal("creation dispatched after token deadline")
	}
}

func TestInvalidInputsDoNotDispatch(t *testing.T) {
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid input dispatched") })
	_, err := a.CreateDisabled(context.Background(), request(t), "bad:operation")
	assertError(t, err, registration.AdminInvalid, false)
	_, err = a.CreateDisabled(context.Background(), registration.Request{}, "op")
	assertError(t, err, registration.AdminInvalid, false)
	_, err = a.FindOwned(context.Background(), "bad:operation")
	assertError(t, err, registration.AdminInvalid, false)
	assertError(
		t,
		a.EnableOwned(context.Background(), "../legacy", "op"),
		registration.AdminInvalid,
		false,
	)
	assertError(t, a.SendVerification(context.Background(), "id", ""), registration.AdminInvalid, false)
}
