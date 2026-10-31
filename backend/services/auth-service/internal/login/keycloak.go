package login

import (
	"context"
	"encoding/json"
	"github.com/coreos/go-oidc/v3/oidc"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type KeycloakConfig struct {
	privateValue
	Issuer, ClientID, ClientSecret, Audience string
}
type Keycloak struct {
	privateValue
	cfg      KeycloakConfig
	client   *http.Client
	verifier *oidc.IDTokenVerifier
	now      func() time.Time
}
type boundedTransport struct{ base *http.Transport }

func (t boundedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response, e := t.base.RoundTrip(r)
	if e != nil {
		return nil, e
	}
	if response.ContentLength > 64<<10 {
		_ = response.Body.Close()
		return nil, ErrUnavailable
	}
	response.Body = http.MaxBytesReader(nil, response.Body, 64<<10)
	return response, nil
}
func NewKeycloak(c KeycloakConfig, client *http.Client) (*Keycloak, error) {
	u, e := url.Parse(c.Issuer)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.HasSuffix(c.Issuer, "/") || strings.ContainsAny(c.Issuer, "\\ \t\r\n#%") || c.ClientID == "" || c.ClientSecret == "" || c.Audience == "" || client == nil {
		return nil, ErrUnavailable
	}
	raw := client.Transport
	if raw == nil {
		raw = http.DefaultTransport
	}
	transport, ok := raw.(*http.Transport)
	if !ok || transport == nil {
		return nil, ErrUnavailable
	}
	transport = transport.Clone()
	if transport.DialTLS != nil || transport.DialTLSContext != nil {
		return nil, ErrUnavailable
	}
	if cfg := transport.TLSClientConfig; cfg != nil && (cfg.InsecureSkipVerify || cfg.ServerName != "" || cfg.VerifyConnection != nil || cfg.VerifyPeerCertificate != nil) {
		return nil, ErrUnavailable
	}
	transport.MaxResponseHeaderBytes = 16 << 10
	transport.ResponseHeaderTimeout = 5 * time.Second
	bounded := *client
	bounded.Timeout = 5 * time.Second
	bounded.Jar = nil
	bounded.Transport = boundedTransport{transport}
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	keys := oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), &bounded), c.Issuer+"/protocol/openid-connect/certs")
	verifier := oidc.NewVerifier(c.Issuer, keys, &oidc.Config{ClientID: c.Audience, SupportedSigningAlgs: []string{oidc.RS256}})
	return &Keycloak{cfg: c, client: &bounded, verifier: verifier, now: time.Now}, nil
}
func (k *Keycloak) Close() { k.client.Transport.(boundedTransport).base.CloseIdleConnections() }
func (k *Keycloak) Password(ctx context.Context, r Request) (Issued, error) {
	form := url.Values{"grant_type": {"password"}, "username": {r.email}, "password": {r.password}, "client_id": {k.cfg.ClientID}, "client_secret": {k.cfg.ClientSecret}, "scope": {"email"}}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, k.cfg.Issuer+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	if e != nil {
		return Issued{}, ErrUnavailable
	}
	req.GetBody = nil
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	response, e := k.client.Do(req)
	if e != nil {
		return Issued{}, ErrUnavailable
	}
	defer response.Body.Close()
	body, e := io.ReadAll(response.Body)
	if e != nil {
		return Issued{}, ErrUnavailable
	}
	if response.StatusCode != 200 {
		var problem struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &problem) == nil && (response.StatusCode == 400 || response.StatusCode == 401) && problem.Error == "invalid_grant" {
			return Issued{}, ErrFailed
		}
		return Issued{}, ErrUnavailable
	}
	var result struct {
		Token   string `json:"access_token"`
		Type    string `json:"token_type"`
		Expires int64  `json:"expires_in"`
	}
	if json.Unmarshal(body, &result) != nil || result.Token == "" || len(result.Token) > 16<<10 || strings.ContainsAny(result.Token, " \t\r\n") || !strings.EqualFold(result.Type, "Bearer") || result.Expires <= 0 || result.Expires > 3600 {
		return Issued{}, ErrUnavailable
	}
	// refresh_token/id_token and other fields are intentionally neither retained
	// in the result nor returned. No credential POST is automatically retried.
	return Issued{Token: result.Token, ExpiresIn: result.Expires}, nil
}
func (k *Keycloak) Verify(ctx context.Context, raw string) (Claims, error) {
	if len(raw) > 16<<10 || raw == "" {
		return Claims{}, ErrUnavailable
	}
	token, e := k.verifier.Verify(ctx, raw)
	if e != nil {
		return Claims{}, ErrUnavailable
	}
	var claims struct {
		Type      string `json:"typ"`
		Party     string `json:"azp"`
		Verified  bool   `json:"email_verified"`
		Issued    int64  `json:"iat"`
		NotBefore int64  `json:"nbf"`
		Realm     struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if token.Claims(&claims) != nil || token.Issuer != k.cfg.Issuer || token.Subject == "" || len(token.Subject) > 128 || claims.Type != "Bearer" || claims.Party != k.cfg.ClientID {
		return Claims{}, ErrUnavailable
	}
	now := k.now()
	if !token.Expiry.After(now) || token.Expiry.After(now.Add(time.Hour)) || claims.Issued <= 0 || claims.Issued > now.Add(30*time.Second).Unix() || claims.NotBefore > now.Add(30*time.Second).Unix() {
		return Claims{}, ErrUnavailable
	}
	citizen := false
	for _, role := range claims.Realm.Roles {
		if role == "citizen" {
			citizen = true
		}
	}
	return Claims{Issuer: token.Issuer, Subject: token.Subject, Expires: token.Expiry, Verified: claims.Verified, Citizen: citizen}, nil
}
