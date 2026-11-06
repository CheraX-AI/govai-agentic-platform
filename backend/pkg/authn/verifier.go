// Package authn validates Keycloak API access tokens and supplies Gin
// authentication/authorization middleware reusable by independent Go services.
package authn

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ErrConfig = errors.New("invalid access-token verifier configuration")
var ErrToken = errors.New("invalid access token")

type privateValue struct{}

func (privateValue) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (privateValue) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (privateValue) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

type Config struct {
	Issuer, Audience string
	Clients          []string
	RefreshInterval  time.Duration
	Now              func() time.Time
}

// Identity never contains the original token, email, NIC or password. Explicit
// JSON allowlists are required when a service deliberately returns its fields.
type Identity struct {
	privateValue
	Issuer, Subject, Client string
	Roles                   []string
	EmailVerified           bool
	Expires                 time.Time
}
type Verifier struct {
	privateValue
	cfg       Config
	oidc      *oidc.IDTokenVerifier
	transport *keysTransport
	gate      chan struct{}
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

type keysTransport struct {
	base     *http.Transport
	endpoint string
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
	now      func() time.Time
}

func (t *keysTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != "GET" || r.URL.String() != t.endpoint {
		return nil, ErrToken
	}
	t.mu.Lock()
	now := t.now()
	if now.Before(t.next) {
		t.mu.Unlock()
		return nil, ErrToken
	}
	t.next = now.Add(t.interval)
	t.mu.Unlock()
	response, e := t.base.RoundTrip(r)
	if e != nil {
		return nil, ErrToken
	}
	if response.ContentLength > 64<<10 {
		_ = response.Body.Close()
		return nil, ErrToken
	}
	response.Body = http.MaxBytesReader(nil, response.Body, 64<<10)
	return response, nil
}
func New(c Config, client *http.Client) (*Verifier, error) {
	u, e := url.Parse(c.Issuer)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(c.Issuer, "\\ \t\r\n#%") || strings.HasSuffix(c.Issuer, "/") || len(c.Issuer) > 2048 || c.Audience == "" || len(c.Audience) > 256 || len(c.Clients) == 0 || len(c.Clients) > 16 || client == nil {
		return nil, ErrConfig
	}
	for _, part := range strings.Split(u.Path, "/") {
		if part == "." || part == ".." {
			return nil, ErrConfig
		}
	}
	for _, id := range c.Clients {
		if !idPattern.MatchString(id) {
			return nil, ErrConfig
		}
	}
	c.Clients = append([]string(nil), c.Clients...)
	if c.RefreshInterval == 0 {
		c.RefreshInterval = 5 * time.Second
	}
	if c.RefreshInterval < time.Second || c.RefreshInterval > time.Minute {
		return nil, ErrConfig
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	raw := client.Transport
	if raw == nil {
		raw = http.DefaultTransport
	}
	t, ok := raw.(*http.Transport)
	if !ok || t == nil {
		return nil, ErrConfig
	}
	t = t.Clone()
	if t.DialTLS != nil || t.DialTLSContext != nil {
		return nil, ErrConfig
	}
	if cfg := t.TLSClientConfig; cfg != nil && (cfg.InsecureSkipVerify || cfg.ServerName != "" || cfg.VerifyConnection != nil || cfg.VerifyPeerCertificate != nil) {
		return nil, ErrConfig
	}
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else if t.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.TLSClientConfig.MinVersion = tls.VersionTLS12
	}
	t.MaxResponseHeaderBytes = 16 << 10
	t.ResponseHeaderTimeout = 5 * time.Second
	transport := &keysTransport{base: t, endpoint: c.Issuer + "/protocol/openid-connect/certs", interval: c.RefreshInterval, now: c.Now}
	bounded := *client
	bounded.Transport = transport
	bounded.Timeout = 5 * time.Second
	bounded.Jar = nil
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	keys := oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), &bounded), transport.endpoint)
	verifier := oidc.NewVerifier(c.Issuer, keys, &oidc.Config{ClientID: c.Audience, SupportedSigningAlgs: []string{oidc.RS256}, Now: c.Now})
	return &Verifier{cfg: c, oidc: verifier, transport: transport, gate: make(chan struct{}, 32)}, nil
}
func (v *Verifier) Close() { v.transport.base.CloseIdleConnections() }
func (v *Verifier) Verify(ctx context.Context, raw string) (Identity, error) {
	if raw == "" || len(raw) > 16<<10 || strings.ContainsAny(raw, " \t\r\n") {
		return Identity{}, ErrToken
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	token, e := v.oidc.Verify(ctx, raw)
	if e != nil {
		return Identity{}, ErrToken
	}
	var claims struct {
		Type      string `json:"typ"`
		Client    string `json:"azp"`
		Verified  bool   `json:"email_verified"`
		Issued    int64  `json:"iat"`
		NotBefore int64  `json:"nbf"`
		Realm     struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if token.Claims(&claims) != nil || token.Issuer != v.cfg.Issuer || !idPattern.MatchString(token.Subject) || claims.Type != "Bearer" {
		return Identity{}, ErrToken
	}
	allowed := false
	for _, client := range v.cfg.Clients {
		if claims.Client == client {
			allowed = true
		}
	}
	if !allowed {
		return Identity{}, ErrToken
	}
	now := v.cfg.Now()
	if !token.Expiry.After(now) || token.Expiry.After(now.Add(time.Hour)) || claims.Issued <= 0 || claims.Issued > now.Add(30*time.Second).Unix() || claims.NotBefore > now.Add(30*time.Second).Unix() || len(claims.Realm.Roles) > 64 {
		return Identity{}, ErrToken
	}
	for _, role := range claims.Realm.Roles {
		if len(role) == 0 || len(role) > 128 {
			return Identity{}, ErrToken
		}
	}
	if ctx.Err() != nil {
		return Identity{}, ErrToken
	}
	return Identity{Issuer: token.Issuer, Subject: token.Subject, Client: claims.Client, Roles: append([]string(nil), claims.Realm.Roles...), EmailVerified: claims.Verified, Expires: token.Expiry}, nil
}
