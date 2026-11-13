package wiring

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKeycloakTransportTrust(t *testing.T) {
	defaultTransport, err := newKeycloakTransport("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(defaultTransport.CloseIdleConnections)
	if defaultTransport.TLSClientConfig.MinVersion != tls.VersionTLS12 ||
		defaultTransport.TLSClientConfig.RootCAs != nil ||
		defaultTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("default transport must use system trust and verify TLS 1.2 or newer")
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caPath, certificate, 0600); err != nil {
		t.Fatal(err)
	}

	transport, err := newKeycloakTransport(caPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected response status %d", response.StatusCode)
	}

	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "missing"},
		{name: "invalid", data: []byte("not a certificate")},
		{name: "oversized", data: append(bytes.Clone(certificate), bytes.Repeat([]byte{' '}, 64<<10)...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".pem")
			if tc.data != nil {
				if err := os.WriteFile(path, tc.data, 0600); err != nil {
					t.Fatal(err)
				}
			}

			rejected, err := newKeycloakTransport(path)
			if rejected != nil {
				rejected.CloseIdleConnections()
			}
			if err == nil || rejected != nil {
				t.Fatal("invalid CA file accepted")
			}
		})
	}
}
