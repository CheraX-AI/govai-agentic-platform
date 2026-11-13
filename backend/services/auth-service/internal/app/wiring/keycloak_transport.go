package wiring

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"os"
)

// newKeycloakTransport adds the optional CA to system trust for this client.
func newKeycloakTransport(caFile string) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile == "" {
		return transport, nil
	}

	file, err := os.Open(caFile)
	if err != nil {
		return nil, err
	}

	const maxCAFileBytes = 64 << 10
	pem, err := io.ReadAll(io.LimitReader(file, maxCAFileBytes+1))
	_ = file.Close()
	if err != nil {
		return nil, err
	}
	if len(pem) > maxCAFileBytes {
		return nil, errors.New("Keycloak CA file exceeds size limit")
	}

	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("Keycloak CA file has no usable certificates")
	}

	transport.TLSClientConfig.RootCAs = roots
	return transport, nil
}
