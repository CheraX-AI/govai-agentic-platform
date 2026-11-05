package devsetup

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalCertificates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tls")
	fingerprint, e := Certificates(dir)
	if e != nil || len(fingerprint) != 64 {
		t.Fatal("certificate generation failed")
	}

	ca, e := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if e != nil {
		t.Fatal(e)
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		t.Fatal("invalid CA")
	}

	for _, name := range []string{"api", "keycloak"} {
		pair, e := tls.LoadX509KeyPair(filepath.Join(dir, name+"-cert.pem"), filepath.Join(dir, name+"-key.pem"))
		if e != nil {
			t.Fatal("invalid key pair")
		}

		cert, e := x509.ParseCertificate(pair.Certificate[0])
		if e != nil {
			t.Fatal(e)
		}

		for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
			if _, e = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: host}); e != nil {
				t.Fatal("local hostname not verified")
			}
		}

		if _, e = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "other.example.test"}); e == nil {
			t.Fatal("wrong hostname accepted")
		}
	}

	if _, e = Certificates(dir); e == nil {
		t.Fatal("overwrote existing certificate directory")
	}

	if _, e = os.Stat(filepath.Join(dir, "ca-key.pem")); !os.IsNotExist(e) {
		t.Fatal("CA signing key persisted")
	}
}

func TestMarkerPermissions(t *testing.T) {
	good := object{
		"permissions": object{"view": []any{"admin"}, "edit": []any{"admin"}},
		"multivalued": false,
	}
	if !protectedMarker(good) {
		t.Fatal("valid marker rejected")
	}

	good["permissions"] = object{"view": []any{"admin"}, "edit": []any{"user"}}
	if protectedMarker(good) {
		t.Fatal("user-writable marker accepted")
	}
}
