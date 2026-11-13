// Package oidccheck checks provider connectivity, not tokens or authentication.
package oidccheck

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxDocument = 1 << 20

// ValidateURL permits HTTP only for explicit loopback development hosts.
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil ||
		u.Hostname() == "" ||
		u.User != nil ||
		u.RawQuery != "" ||
		u.ForceQuery ||
		u.Fragment != "" ||
		strings.ContainsAny(raw, "#\\ \t\r\n") {
		return errors.New("invalid OIDC URL")
	}

	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return errors.New("OIDC URL requires HTTPS except on loopback")
	}

	return nil
}

// Check bounds both requests with one deadline. It sends no credentials and never
// returns remote bodies, URLs, transport errors, or key material in errors.
func Check(ctx context.Context, issuer string, timeout time.Duration) error {
	if issuer == "" {
		return errors.New("OIDC_ISSUER_URL is required for the connectivity check")
	}

	if ValidateURL(issuer) != nil {
		return errors.New("invalid OIDC issuer configuration")
	}

	if timeout <= 0 || timeout > 30*time.Second {
		return errors.New("OIDC check timeout must be positive and at most 30s")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	var discovery struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := getJSON(
		ctx,
		client,
		strings.TrimSuffix(issuer, "/")+"/.well-known/openid-configuration",
		&discovery,
	); err != nil {
		return errors.New("OIDC discovery check failed: " + err.Error())
	}

	if discovery.Issuer != issuer {
		return errors.New("OIDC discovery issuer mismatch")
	}

	if ValidateURL(discovery.JWKSURI) != nil {
		return errors.New("OIDC discovery has invalid JWKS URL")
	}

	expected, _ := url.Parse(issuer)
	keysURL, _ := url.Parse(discovery.JWKSURI)
	// Local Keycloak serves keys on its own origin. Do not follow metadata to other hosts.
	if keysURL.Scheme != expected.Scheme || keysURL.Host != expected.Host {
		return errors.New("OIDC JWKS URL must share the configured issuer origin")
	}

	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
		} `json:"keys"`
	}
	if err := getJSON(ctx, client, discovery.JWKSURI, &jwks); err != nil {
		return errors.New("OIDC JWKS check failed: " + err.Error())
	}

	if len(jwks.Keys) == 0 {
		return errors.New("OIDC JWKS contains no keys")
	}

	for _, key := range jwks.Keys {
		if key.Kty == "" {
			return errors.New("OIDC JWKS has an invalid key entry")
		}
	}

	return nil
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return errors.New("invalid endpoint")
	}

	req.Header.Set("Accept", "application/json")
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return errors.New("deadline exceeded or check canceled")
		}

		return errors.New("connection or TLS failure")
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("unexpected HTTP status")
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxDocument+1))
	if err != nil {
		return errors.New("response read failed")
	}

	if len(body) > maxDocument {
		return errors.New("response exceeds size limit")
	}

	if json.Unmarshal(body, target) != nil {
		return errors.New("invalid JSON document")
	}

	return nil
}
