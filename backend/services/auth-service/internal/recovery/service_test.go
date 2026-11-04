package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

type fixture struct {
	events    []string
	fail      string
	email     string
	token     Token
	challenge Challenge
	spent     bool
	finished  string
	delivery  Delivery
	password  string
}

func (f *fixture) event(name string) error {
	f.events = append(f.events, name)
	if f.fail == name {
		return errors.New("synthetic private provider body")
	}
	return nil
}
func (f *fixture) Lookup(_ context.Context, issuer string, nic registration.NIC) (Binding, error) {
	if err := f.event("lookup"); err != nil {
		return Binding{}, err
	}
	if nic.NormalizedNumber() != "000000001V" {
		return Binding{}, ErrNotFound
	}
	return Binding{Issuer: issuer, Subject: "subject-1", OperationID: "operation-1"}, nil
}
func (f *fixture) Recipient(context.Context, Binding) (string, error) {
	if err := f.event("recipient"); err != nil {
		return "", err
	}
	return f.email, nil
}
func (f *fixture) Issue(_ context.Context, c Challenge, _ time.Duration) (Token, error) {
	if err := f.event("issue"); err != nil {
		return Token{}, err
	}
	f.challenge = c
	f.spent = false
	f.token, _ = NewToken()
	return f.token, nil
}
func (f *fixture) Claim(_ context.Context, token Token) (Challenge, error) {
	if err := f.event("claim"); err != nil {
		return Challenge{}, err
	}
	if f.spent || token.Reveal() != f.token.Reveal() {
		return Challenge{}, ErrInvalid
	}
	f.spent = true
	return f.challenge, nil
}
func (f *fixture) Finish(_ context.Context, _ Token, success bool) error {
	if err := f.event("finish"); err != nil {
		return err
	}
	f.finished = "attention"
	if success {
		f.finished = "completed"
	}
	return nil
}
func (f *fixture) Send(_ context.Context, d Delivery) error { f.delivery = d; return f.event("send") }
func (f *fixture) Revoke(context.Context, Binding) error    { return f.event("local_revoke") }
func (f *fixture) Reset(_ context.Context, _ Binding, p Password) error {
	f.password = p.Reveal()
	return f.event("reset")
}
func (f *fixture) Logout(context.Context, Binding) error { return f.event("logout") }
func setupService(t *testing.T) (*Service, *fixture) {
	t.Helper()
	f := &fixture{email: "citizen@example.test"}
	cfg := Config{Issuer: "https://identity.test/realms/GovAI", ResetURL: "https://app.example.test/reset", BindingKey: []byte(strings.Repeat("k", 32)), Lifetime: 15 * time.Minute, Timeout: 5 * time.Second}
	s, err := NewService(cfg, f, f, f, f, f)
	if err != nil {
		t.Fatal(err)
	}
	return s, f
}
func TestRecoveryServiceLifecycle(t *testing.T) {
	s, f := setupService(t)
	receipt, err := s.Request(context.Background(), "000000001v")
	if err != nil || receipt.MaskedEmail != "ci*****@example.test" {
		t.Fatal("request failed")
	}
	if !reflect.DeepEqual(f.events, []string{"lookup", "recipient", "issue", "send"}) {
		t.Fatal("incorrect request order")
	}
	u, err := url.Parse(f.delivery.Link)
	if err != nil || u.Scheme != "https" || u.Host != "app.example.test" || u.RawQuery != "" || u.Fragment != "token="+f.token.Reveal() {
		t.Fatal("unsafe reset link")
	}
	body, _ := json.Marshal(receipt)
	if strings.Contains(string(body), f.email) || strings.Contains(string(body), f.token.Reveal()) {
		t.Fatal("public secret disclosure")
	}
	if strings.Contains(fmt.Sprintf("%+v", f.delivery), f.token.Reveal()) {
		t.Fatal("delivery redaction failed")
	}
	f.events = nil
	if err = s.Reset(context.Background(), f.token.Reveal(), " new password "); err != nil {
		t.Fatal(err)
	}
	want := []string{"claim", "recipient", "local_revoke", "reset", "logout", "local_revoke", "finish"}
	if !reflect.DeepEqual(f.events, want) || f.finished != "completed" || f.password != " new password " {
		t.Fatal("incorrect reset lifecycle")
	}
	f.events = nil
	if s.Reset(context.Background(), f.token.Reveal(), "another-password") == nil || !reflect.DeepEqual(f.events, []string{"claim"}) {
		t.Fatal("token replayed")
	}
}
func TestRecoveryAddressChangeAndFailures(t *testing.T) {
	for _, step := range []string{"changed_email", "recipient", "local_revoke", "reset", "logout", "local_revoke", "finish"} {
		t.Run(step, func(t *testing.T) {
			s, f := setupService(t)
			if _, err := s.Request(context.Background(), "000000001V"); err != nil {
				t.Fatal(err)
			}
			if step == "changed_email" {
				f.email = "changed@example.test"
			} else {
				f.fail = step
			}
			f.events = nil
			if err := s.Reset(context.Background(), f.token.Reveal(), "synthetic-password"); err == nil || strings.Contains(err.Error(), "synthetic") {
				t.Fatal("failure lost or private error exposed")
			}
			if !f.spent {
				t.Fatal("failed challenge reactivated")
			}
			if step == "changed_email" || step == "recipient" || step == "local_revoke" {
				if f.password != "" {
					t.Fatal("password changed before validation/revocation")
				}
			}
			if step == "reset" {
				for _, event := range f.events {
					if event == "logout" {
						t.Fatal("continued after uncertain reset")
					}
				}
			}
			if step != "finish" && f.finished != "attention" {
				t.Fatal("uncertainty not recorded")
			}
			f.events = nil
			if s.Reset(context.Background(), f.token.Reveal(), "synthetic-password") == nil || !reflect.DeepEqual(f.events, []string{"claim"}) {
				t.Fatal("failure retried mutation")
			}
		})
	}
}
func TestRecoveryRequestFailureAndInvalidInput(t *testing.T) {
	s, f := setupService(t)
	receipt, err := s.Request(context.Background(), "000000009V")
	if err != nil || receipt.Status != "received" || receipt.MaskedEmail != "" || !reflect.DeepEqual(f.events, []string{"lookup"}) {
		t.Fatal("unknown account response")
	}
	for _, step := range []string{"lookup", "recipient", "issue", "send"} {
		s, f = setupService(t)
		f.fail = step
		if _, err = s.Request(context.Background(), "000000001V"); err == nil {
			t.Fatal("request failure ignored")
		}
		if step == "send" && (!f.spent || f.finished != "attention") {
			t.Fatal("failed delivery token not spent")
		}
	}
	s, f = setupService(t)
	if s.Reset(context.Background(), "invalid", "12345678") == nil || len(f.events) != 0 {
		t.Fatal("invalid token reached dependencies")
	}
	if _, err = s.Request(context.Background(), "bad-nic"); err == nil || len(f.events) != 0 {
		t.Fatal("invalid NIC reached dependencies")
	}
}
func TestRecoveryRejectsUnsafeConfiguration(t *testing.T) {
	s, f := setupService(t)
	for _, resetURL := range []string{"http://app.test/reset", "https://user@app.test/reset", "https://app.test/reset?token=caller", "https://app.test/reset#caller"} {
		cfg := s.cfg
		cfg.ResetURL = resetURL
		if _, err := NewService(cfg, f, f, f, f, f); err == nil {
			t.Fatal("unsafe callback accepted")
		}
	}
}
