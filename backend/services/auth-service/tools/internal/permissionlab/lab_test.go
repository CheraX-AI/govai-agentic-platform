package permissionlab

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

const testRealm = "ga48-permission-00000000000000000000000000000000"

const otherRealm = "ga48-other-00000000000000000000000000000000"

func TestPathBoundary(t *testing.T) {
	for _, path := range []string{
		"/admin/realms/GovAI/users",
		"/admin/realms/master/users",
		"/admin/realms/" + testRealm + "extra/users",
		"https://evil.test/admin/realms",
		"/admin/realms/" + testRealm + "/../GovAI/users",
	} {
		if allowedPath(path, testRealm, otherRealm) {
			t.Fatal("unsafe path accepted")
		}
	}

	for _, path := range []string{
		"/admin/realms",
		"/realms/master/protocol/openid-connect/token",
		"/admin/realms/" + testRealm + "/users?q=x",
		"/admin/realms/" + otherRealm + "/users",
	} {
		if !allowedPath(path, testRealm, otherRealm) {
			t.Fatal("test path rejected")
		}
	}

	if allowedPath("/admin/realms", "GovAI", otherRealm) {
		t.Fatal("protected realm accepted")
	}
}

func TestHistoricalBaselineCreateRemainsGroupFree(t *testing.T) {
	r, err := registration.ParseRequest(
		strings.NewReader(
			`{"full_name":"Synthetic","nic":"000000001V","mobile_number":"+14155552671","email":"synthetic@example.test","password":"synthetic"}`,
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	body := baselinePayload(r, "operation")
	if len(body) != 7 {
		t.Fatal("historical baseline shape changed")
	}

	if _, ok := body["groups"]; ok {
		t.Fatal("group-aware adapter changed historical baseline")
	}
}

func TestCertificates(t *testing.T) {
	dir := t.TempDir()
	caPEM, err := certificates(dir)
	if err != nil {
		t.Fatal(err)
	}

	certPEM, err := os.ReadFile(filepath.Join(dir, "server.pem"))
	if err != nil {
		t.Fatal(err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("invalid leaf")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(caPEM)
	if _, err = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "localhost"}); err != nil {
		t.Fatal("certificate not trusted")
	}

	if _, err = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "wrong.test"}); err == nil {
		t.Fatal("hostname bypass")
	}

	if _, err = cert.Verify(
		x509.VerifyOptions{Roots: roots, DNSName: "localhost", CurrentTime: time.Now().Add(48 * time.Hour)},
	); err == nil {
		t.Fatal("expiry bypass")
	}

	if _, err = os.Stat(filepath.Join(dir, "ca-key.pem")); !os.IsNotExist(err) {
		t.Fatal("CA signing key persisted")
	}
}

func TestNewStateDirectoryRequired(t *testing.T) {
	var out bytes.Buffer
	if Run(context.Background(), t.TempDir(), &out) != 1 || out.String() != "LAB_EXECUTION BLOCKED\n" {
		t.Fatal("existing directory not refused safely")
	}

	dir := t.TempDir()
	if os.Mkdir(filepath.Join(dir, ".git"), 0700) != nil {
		t.Fatal("fixture failed")
	}

	if !insideGit(filepath.Join(dir, "new")) {
		t.Fatal("Git boundary not enforced")
	}
}

func TestRequestBoundaries(t *testing.T) {
	var calls int
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Location", "/do-not-follow")
		w.WriteHeader(302)
		_, _ = io.WriteString(w, "synthetic-private-body")
	}))
	defer s.Close()
	c := s.Client()
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	l := &lab{base: s.URL, realm: testRealm, other: otherRealm, client: c}
	status, _, err := l.request(
		context.Background(),
		"synthetic-token",
		"POST",
		"/admin/realms/"+testRealm+"/users",
		object{"password": "synthetic-secret"},
	)
	if err != nil || status != 302 || calls != 1 {
		t.Fatal("redirect followed/retried")
	}

	_, _, err = l.request(context.Background(), "", "GET", "/admin/realms/GovAI/users", nil)
	if err == nil || calls != 1 {
		t.Fatal("protected realm reached")
	}
}

func TestTLSAndBodyLimits(t *testing.T) {
	s := httptest.NewTLSServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, strings.Repeat("x", (1<<20)+1)) },
		),
	)
	defer s.Close()
	l := &lab{base: s.URL, realm: testRealm, other: otherRealm, client: s.Client()}
	_, data, err := l.request(context.Background(), "", "GET", "/admin/realms/"+testRealm+"/users", nil)
	if err == nil || data != nil {
		t.Fatal("response cap missing")
	}

	l.client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}
	defer l.client.CloseIdleConnections()
	_, _, err = l.request(context.Background(), "", "GET", "/admin/realms/"+testRealm+"/users", nil)
	if err == nil || err.Error() != errSafe.Error() {
		t.Fatal("TLS error not safely rejected")
	}
}

func TestSecurityAssertionsRemainFailures(t *testing.T) {
	var out bytes.Buffer
	l := &lab{out: &out}
	l.result("CASE", "FAIL_UNSAFE")
	if !l.unsafe || out.String() != "CASE FAIL_UNSAFE\n" {
		t.Fatal("unsafe capability treated as pass")
	}
}
