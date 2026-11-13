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

func TestCertificatesCoverExtraHosts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tls")
	if _, e := Certificates(dir, "govai.example.ac.lk", "10.0.0.5"); e != nil {
		t.Fatal("certificate generation with extra hosts failed")
	}

	ca, e := os.ReadFile(filepath.Join(dir, "ca.pem"))
	if e != nil {
		t.Fatal(e)
	}

	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(ca)
	pair, e := tls.LoadX509KeyPair(filepath.Join(dir, "api-cert.pem"), filepath.Join(dir, "api-key.pem"))
	if e != nil {
		t.Fatal("invalid key pair")
	}

	cert, e := x509.ParseCertificate(pair.Certificate[0])
	if e != nil {
		t.Fatal(e)
	}

	for _, host := range []string{"localhost", "127.0.0.1", "govai.example.ac.lk", "10.0.0.5"} {
		if _, e = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: host}); e != nil {
			t.Fatalf("host %s not verified", host)
		}
	}

	if _, e = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "other.example.test"}); e == nil {
		t.Fatal("wrong hostname accepted")
	}
}

func TestCertificatesRejectBadHosts(t *testing.T) {
	for _, host := range []string{"", "*.example.test", "host:443", "-bad.test", "bad..test", "a b.test", "https://x.test"} {
		dir := filepath.Join(t.TempDir(), "tls")
		if _, e := Certificates(dir, host); e == nil {
			t.Fatalf("accepted bad host %q", host)
		}

		if _, e := os.Stat(dir); !os.IsNotExist(e) {
			t.Fatalf("created directory for bad host %q", host)
		}
	}
}

func TestValidOrigin(t *testing.T) {
	for _, origin := range []string{"https://localhost:5001", "https://govai.example.ac.lk", "https://10.0.0.5:5173"} {
		if !validOrigin(origin) {
			t.Fatalf("rejected %s", origin)
		}
	}

	for _, origin := range []string{
		"", "http://localhost:5001", "https://", "https://x.test/", "https://x.test/app",
		"https://x.test?a=1", "https://x.test#f", "https://user@x.test", "localhost:5001",
	} {
		if validOrigin(origin) {
			t.Fatalf("accepted %q", origin)
		}
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
