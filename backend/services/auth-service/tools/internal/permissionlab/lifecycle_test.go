package permissionlab

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLifecycleActionOriginAndRealm(t *testing.T) {
	l := &lab{base: "https://localhost:8543", realm: testRealm}
	path := "/realms/" + testRealm + "/login-actions/action-token?key=synthetic"
	if _, ok := l.safeAction(l.base + path); !ok {
		t.Fatal("configured action rejected")
	}

	for _, raw := range []string{
		"http://localhost:8543" + path,
		"https://other.test" + path,
		"https://user:secret@localhost:8543" + path,
		l.base + strings.Replace(path, testRealm, "GovAI", 1),
		l.base + path + "#fragment",
		l.base + "/realms/" + testRealm + "/login-actions/authenticate?key=synthetic",
	} {
		if _, ok := l.safeAction(raw); ok {
			t.Fatal("unsafe action accepted")
		}
	}
}

func TestTamperOnlyChangesSignature(t *testing.T) {
	raw := "https://localhost/action?key=a.b.ccc&client_id=test"
	changed, err := tamperLink(raw)
	if err != nil ||
		changed == raw ||
		!strings.Contains(changed, "key=a.b.A") ||
		!strings.Contains(changed, "client_id=test") {
		t.Fatal("incorrect tampering")
	}

	if _, err = tamperLink("https://localhost/action?key=malformed"); err == nil {
		t.Fatal("invalid input accepted")
	}
}

func TestVerificationOnlyFollowsActionConfirmation(t *testing.T) {
	var base string
	calls := 0
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "" {
			t.Error("admin token forwarded")
		}

		if r.URL.Query().Get("key") == "first" {
			http.SetCookie(w, &http.Cookie{Name: "synthetic-session", Value: "bound", Secure: true, HttpOnly: true})
			_, _ = w.Write(
				[]byte(
					`<a href="https://evil.test/">external</a><a href="` + base + `/realms/` + testRealm + `/login-actions/action-token?key=second&amp;client_id=test">confirm</a>`,
				),
			)
			return
		}

		if c, err := r.Cookie("synthetic-session"); err != nil || c.Value != "bound" {
			t.Error("confirmation session not bound")
		}

		_, _ = w.Write(
			[]byte(`Email address has been verified. <a href="https://localhost/ga48-verified">continue</a>`),
		)
	}))
	defer s.Close()
	base = s.URL
	l := &lab{base: base, realm: testRealm, client: s.Client()}
	status, page, err := l.visitVerification(context.Background(), base+"/realms/"+testRealm+"/login-actions/action-token?key=first")
	if err != nil || status != 200 || page != "VERIFIED" || calls != 2 {
		t.Fatal("incorrect bounded verification navigation")
	}
}

func TestLifecyclePreservesExistingDirectoryAndFailures(t *testing.T) {
	var out bytes.Buffer
	if RunLifecycle(context.Background(), t.TempDir(), &out) != 1 ||
		out.String() != "LAB_EXECUTION BLOCKED\n" {
		t.Fatal("existing state overwritten")
	}

	out.Reset()
	l := &lab{out: &out}
	l.lifeCheck("LIFE_CASE", false)
	if !l.unsafe || out.String() != "LIFE_CASE FAIL\n" {
		t.Fatal("failure hidden")
	}
}

func TestVerificationClassifierIgnoresEmbeddedTranslations(t *testing.T) {
	if classifyVerification(
		`<script>translations={already:"already verified",expired:"expired"}</script><p>Invalid token.</p>`,
	) != "INVALID" {
		t.Fatal("script classified as visible outcome")
	}

	if classifyVerification(`<p>Your email address has been verified already.</p>`) != "ALREADY_VERIFIED" {
		t.Fatal("already-verified outcome lost")
	}
}

func TestVerificationErrorPageCannotBecomeSuccessObservation(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("This link may have already been verified."))
	}))
	defer s.Close()
	l := &lab{base: s.URL, realm: testRealm, client: s.Client()}
	status, page, err := l.visitVerification(
		context.Background(),
		s.URL+"/realms/"+testRealm+"/login-actions/action-token?key=synthetic",
	)
	if err != nil || status != 400 || page != "ERROR_PAGE" {
		t.Fatal("generic error mistaken for verification success")
	}
}
