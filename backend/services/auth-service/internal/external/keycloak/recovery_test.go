package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/recovery"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

func recoveryUser() map[string]any {
	return map[string]any{
		"id":            "subject-1",
		"email":         "synthetic@example.test",
		"enabled":       true,
		"emailVerified": true,
		"attributes":    map[string][]string{marker: {"operation-1"}},
	}
}

func TestRecoveryTransport(t *testing.T) {
	var resets, logouts atomic.Int32
	password, _ := recovery.ParsePassword(" password &+= ")
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			issueToken(w)
			return
		}

		if r.Header.Get("Authorization") != "Bearer synthetic-token" {
			t.Error("missing admin bearer")
		}

		switch {
		case r.Method == "GET" && r.URL.Path == usersPath+"/subject-1":
			_ = json.NewEncoder(w).Encode(recoveryUser())
		case r.Method == "PUT" && r.URL.Path == usersPath+"/subject-1/reset-password":
			resets.Add(1)
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil ||
				len(body) != 3 ||
				body["value"] != password.Reveal() ||
				body["type"] != "password" ||
				body["temporary"] != false {
				t.Error("incorrect password payload")
			}
			w.WriteHeader(204)
		case r.Method == "POST" && r.URL.Path == usersPath+"/subject-1/logout":
			logouts.Add(1)
			w.WriteHeader(204)
		default:
			t.Error("unexpected endpoint")
			w.WriteHeader(500)
		}
	})
	recipient, err := a.RecoveryRecipient(context.Background(), "subject-1", "operation-1")
	if err != nil || recipient.Email() != "synthetic@example.test" {
		t.Fatal("recipient read failed")
	}

	b, _ := json.Marshal(recipient)
	if strings.Contains(string(b)+fmt.Sprintf("%+v", recipient), recipient.Email()) {
		t.Fatal("recipient redaction failed")
	}

	if err = a.ResetOwnedPassword(context.Background(), "subject-1", "operation-1", password); err != nil {
		t.Fatal(err)
	}

	if logouts.Load() != 0 {
		t.Fatal("logout silently combined with password mutation")
	}

	if err = a.LogoutOwnedSessions(context.Background(), "subject-1", "operation-1"); err != nil {
		t.Fatal(err)
	}

	if resets.Load() != 1 || logouts.Load() != 1 {
		t.Fatal("unexpected mutation count")
	}
}

func TestRecoveryRejectsUnownedOrUnsafeRecipient(t *testing.T) {
	for _, kind := range []string{"marker", "subject", "disabled", "unverified", "email", "display_name"} {
		t.Run(kind, func(t *testing.T) {
			var mutations atomic.Int32
			a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					issueToken(w)
					return
				}

				if r.Method != "GET" {
					mutations.Add(1)
					w.WriteHeader(204)
					return
				}

				u := recoveryUser()
				switch kind {
				case "marker":
					u["attributes"] = map[string][]string{marker: {"other-operation"}}
				case "subject":
					u["id"] = "other-subject"
				case "disabled":
					u["enabled"] = false
				case "unverified":
					u["emailVerified"] = false
				case "email":
					u["email"] = "invalid\r\nemail"
				case "display_name":
					u["email"] = "Name <synthetic@example.test>"
				}

				_ = json.NewEncoder(w).Encode(u)
			})
			password, _ := recovery.ParsePassword("synthetic-password")
			if a.ResetOwnedPassword(context.Background(), "subject-1", "operation-1", password) == nil {
				t.Fatal("unsafe reset allowed")
			}

			if a.LogoutOwnedSessions(context.Background(), "subject-1", "operation-1") == nil {
				t.Fatal("unsafe logout allowed")
			}

			if mutations.Load() != 0 {
				t.Fatal("mutated before ownership validation")
			}
		})
	}
}

func TestRecoveryMutationErrorsNotRetried(t *testing.T) {
	for _, status := range []int{400, 403, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					issueToken(w)
					return
				}

				if r.Method == "GET" {
					_ = json.NewEncoder(w).Encode(recoveryUser())
					return
				}

				calls.Add(1)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "private upstream error")
			})
			password, _ := recovery.ParsePassword("synthetic-password")
			err := a.ResetOwnedPassword(context.Background(), "subject-1", "operation-1", password)
			code := registration.AdminAmbiguous
			unknown := true
			if status == 400 {
				code = registration.AdminPolicyRejected
				unknown = false
			}

			if status == 403 {
				code = registration.AdminPermission
				unknown = false
			}

			assertError(t, err, code, unknown)
			if calls.Load() != 1 {
				t.Fatal("mutation retried")
			}
		})
	}
}

func TestRecoveryInvalidInputBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {

		calls.Add(1)
		w.WriteHeader(500)
	})
	valid, _ := recovery.ParsePassword("synthetic-password")
	if a.ResetOwnedPassword(context.Background(), "../subject", "operation-1", valid) == nil {
		t.Fatal("path accepted")
	}

	if a.ResetOwnedPassword(context.Background(), "subject-1", "operation-1", recovery.Password{}) == nil {
		t.Fatal("empty password accepted")
	}

	if calls.Load() != 0 {
		t.Fatal("invalid input reached network")
	}
}

func TestRecoveryProviderIssuerBoundary(t *testing.T) {
	var calls atomic.Int32
	a, _ := adapter(t, func(w http.ResponseWriter, r *http.Request) {

		calls.Add(1)
		w.WriteHeader(500)
	})
	provider, err := NewRecoveryProvider(a)
	if err != nil {
		t.Fatal(err)
	}

	binding := recovery.Binding{
		Issuer:      "https://other.example.test/realms/GovAI",
		Subject:     "subject-1",
		OperationID: "operation-1",
	}
	password, _ := recovery.ParsePassword("synthetic-password")
	if _, err = provider.Recipient(context.Background(), binding); err == nil {
		t.Fatal("wrong issuer read accepted")
	}

	if provider.Reset(context.Background(), binding, password) == nil ||
		provider.Logout(context.Background(), binding) == nil {
		t.Fatal("wrong issuer mutation accepted")
	}

	if calls.Load() != 0 {
		t.Fatal("wrong issuer reached network")
	}
}
