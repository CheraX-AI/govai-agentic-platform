// Package keycloak implements isolated registration transport, not public APIs.
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

const marker = "govai_registration_operation"

var segment = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// RegistrationConfig is exclusively for the administrative service account.
// It must never be populated from future password-login client configuration.
// BaseURL may include a fixed deployment prefix, but no trailing slash.
type RegistrationConfig struct {
	// Required server-only absolute top-level path, e.g. /ga48-registration.
	RegistrationGroupPath                      string
	BaseURL, Realm, ClientID, ClientSecret     string
	VerificationClientID, VerificationRedirect string
	VerificationLifespan                       time.Duration
	CallTimeout                                time.Duration
	MaxResponseBytes                           int64
	TokenSkew                                  time.Duration
}

func (RegistrationConfig) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (RegistrationConfig) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (RegistrationConfig) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

type Admin struct {
	cfg             RegistrationConfig
	client          *http.Client
	users, tokenURL string
	gate            chan struct{}
	mu              sync.Mutex
	token           string
	expires         time.Time
	now             func() time.Time
}

var _ registration.Admin = (*Admin)(nil)

func (*Admin) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (*Admin) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (*Admin) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

// NewRegistrationAdmin clones the client/transport, preserving trusted roots but
// rejecting opaque transports and TLS bypasses. No network call occurs here.
// Caller must not instrument the supplied transport to capture credentials.
func NewRegistrationAdmin(cfg RegistrationConfig, client *http.Client) (*Admin, error) {
	bad := func() (*Admin, error) { return nil, failure(registration.AdminInvalid, false) }
	if !strings.HasPrefix(cfg.RegistrationGroupPath, "/") || !segment.MatchString(strings.TrimPrefix(cfg.RegistrationGroupPath, "/")) {
		return bad()
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || !secureURL(u) || u.RawQuery != "" || u.ForceQuery || u.RawPath != "" || strings.HasSuffix(cfg.BaseURL, "/") || strings.Contains(cfg.BaseURL, "%") || !cleanPath(u.Path) || !segment.MatchString(cfg.Realm) || cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.VerificationClientID == "" {
		return bad()
	}
	r, err := url.Parse(cfg.VerificationRedirect)
	if err != nil || !secureURL(r) || r.RawQuery != "" || r.ForceQuery {
		return bad()
	}
	if cfg.CallTimeout <= 0 || cfg.CallTimeout > 5*time.Second || cfg.MaxResponseBytes < 256 || cfg.MaxResponseBytes > 1<<20 || cfg.TokenSkew < 0 || cfg.TokenSkew > time.Minute || cfg.VerificationLifespan < time.Second || cfg.VerificationLifespan > 24*time.Hour || cfg.VerificationLifespan%time.Second != 0 || client == nil {
		return bad()
	}
	t := client.Transport
	if t == nil {
		t = http.DefaultTransport
	}
	transport, ok := t.(*http.Transport)
	if !ok || transport == nil {
		return bad()
	}
	transport = transport.Clone()
	if transport.DialTLS != nil || transport.DialTLSContext != nil {
		return bad()
	}
	if tls := transport.TLSClientConfig; tls != nil {
		if tls.InsecureSkipVerify || tls.ServerName != "" || tls.VerifyConnection != nil || tls.VerifyPeerCertificate != nil {
			return bad()
		}
	}
	transport.MaxResponseHeaderBytes = 16 << 10
	transport.ResponseHeaderTimeout = cfg.CallTimeout
	c := *client
	c.Transport, c.Jar = transport, nil
	c.Timeout = cfg.CallTimeout
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Admin{cfg: cfg, client: &c, users: cfg.BaseURL + "/admin/realms/" + cfg.Realm + "/users", tokenURL: cfg.BaseURL + "/realms/" + cfg.Realm + "/protocol/openid-connect/token", gate: make(chan struct{}, 1), now: time.Now}, nil
}

func secureURL(u *url.URL) bool {
	return u != nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == "" && !strings.ContainsAny(u.String(), "\\ \t\r\n#")
}

func cleanPath(p string) bool {
	if p == "" {
		return true
	}
	for _, s := range strings.Split(strings.TrimPrefix(p, "/"), "/") {
		if !segment.MatchString(s) {
			return false
		}
	}
	return true
}

func failure(code registration.AdminCode, unknown bool) error {
	return &registration.AdminError{Code: code, UnknownOutcome: unknown}
}

// Close releases idle connections. No background refresh goroutine is started.
func (a *Admin) Close() { a.client.CloseIdleConnections() }

func (a *Admin) serviceToken(ctx context.Context) (string, error) {
	select {
	case a.gate <- struct{}{}:
		defer func() { <-a.gate }()
	case <-ctx.Done():
		return "", failure(registration.AdminCanceled, false)
	}
	if ctx.Err() != nil {
		return "", failure(registration.AdminCanceled, false)
	}
	a.mu.Lock()
	if a.token != "" && a.now().Before(a.expires) {
		token := a.token
		a.mu.Unlock()
		return token, nil
	}
	a.token = ""
	a.mu.Unlock()
	started := a.now()
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {a.cfg.ClientID}, "client_secret": {a.cfg.ClientSecret}}
	status, _, body, err := a.exchange(ctx, http.MethodPost, a.tokenURL, []byte(form.Encode()), "application/x-www-form-urlencoded", "", false)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		if status == 400 || status == 401 || status == 403 {
			return "", failure(registration.AdminPermission, false)
		}
		return "", failure(registration.AdminUpstream, false)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if json.Unmarshal(body, &token) != nil || token.AccessToken == "" || len(token.AccessToken) > 16<<10 || strings.ContainsAny(token.AccessToken, " \t\r\n") || !strings.EqualFold(token.TokenType, "Bearer") || token.ExpiresIn <= 0 || token.ExpiresIn > 86400 {
		return "", failure(registration.AdminUpstream, false)
	}
	expires := started.Add(time.Duration(token.ExpiresIn) * time.Second).Add(-a.cfg.TokenSkew)
	if !a.now().Before(expires) {
		return "", failure(registration.AdminUpstream, false)
	}
	a.mu.Lock()
	a.token, a.expires = token.AccessToken, expires
	a.mu.Unlock()
	return token.AccessToken, nil
}

func (a *Admin) invalidate(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token == token {
		a.token = ""
		a.expires = time.Time{}
	}
}

// exchange makes exactly one attempt. Disable GetBody so net/http cannot replay
// credential/mutation POSTs. No idempotency header is set on Keycloak requests.
func (a *Admin) exchange(ctx context.Context, method, endpoint string, body []byte, contentType, token string, mutation bool) (int, http.Header, []byte, error) {
	if ctx.Err() != nil {
		return 0, nil, nil, failure(registration.AdminCanceled, false)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, failure(registration.AdminInvalid, false)
	}
	req.GetBody = nil
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		if mutation {
			return 0, nil, nil, failure(registration.AdminAmbiguous, true)
		}
		if ctx.Err() != nil {
			return 0, nil, nil, failure(registration.AdminCanceled, false)
		}
		return 0, nil, nil, failure(registration.AdminUpstream, false)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		a.invalidate(token)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, a.cfg.MaxResponseBytes+1))
	if err != nil || int64(len(data)) > a.cfg.MaxResponseBytes {
		if mutation {
			return 0, nil, nil, failure(registration.AdminAmbiguous, true)
		}
		return 0, nil, nil, failure(registration.AdminUpstream, false)
	}
	return resp.StatusCode, resp.Header, data, nil
}

func statusError(status int, create, mutation bool) error {
	switch status {
	case 401, 403:
		return failure(registration.AdminPermission, false)
	case 409:
		if create {
			return failure(registration.AdminDuplicate, false)
		}
	case 400:
		return failure(registration.AdminPolicyRejected, create)
	case 404:
		return failure(registration.AdminNotFound, false)
	}
	if mutation {
		return failure(registration.AdminAmbiguous, true)
	}
	return failure(registration.AdminUpstream, false)
}

func (a *Admin) call(ctx context.Context, method, endpoint string, body []byte, expected int, create bool) ([]byte, http.Header, error) {
	token, err := a.serviceToken(ctx)
	if err != nil {
		return nil, nil, err
	}
	ct := ""
	if body != nil {
		ct = "application/json"
	}
	status, header, data, err := a.exchange(ctx, method, endpoint, body, ct, token, method != http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	if status != expected {
		return nil, nil, statusError(status, create, method != http.MethodGet)
	}
	return data, header, nil
}

func (a *Admin) CreateDisabled(ctx context.Context, request registration.Request, operationID string) (registration.Identity, error) {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.CallTimeout)
	defer cancel()
	if !segment.MatchString(operationID) || request.Email == "" || len(request.Email) > 254 || request.TransientPassword() == "" || len(request.TransientPassword()) > 1024 {
		return registration.Identity{}, failure(registration.AdminInvalid, false)
	}
	// The only group comes from validated server config. No profile fields,
	// role mappings or caller-selected attributes/groups are forwarded.
	payload := struct {
		Username        string              `json:"username"`
		Email           string              `json:"email"`
		Enabled         bool                `json:"enabled"`
		EmailVerified   bool                `json:"emailVerified"`
		RequiredActions []string            `json:"requiredActions"`
		Attributes      map[string][]string `json:"attributes"`
		Credentials     []map[string]any    `json:"credentials"`
		Groups          []string            `json:"groups"`
	}{request.Email, request.Email, false, false, []string{"VERIFY_EMAIL"}, map[string][]string{marker: {operationID}}, []map[string]any{{"type": "password", "value": request.TransientPassword(), "temporary": false}}, []string{a.cfg.RegistrationGroupPath}}
	body, _ := json.Marshal(payload)
	_, h, err := a.call(ctx, http.MethodPost, a.users, body, http.StatusCreated, true)
	if err != nil {
		return registration.Identity{}, err
	}
	if len(h.Values("Location")) != 1 {
		return registration.Identity{}, failure(registration.AdminAmbiguous, true)
	}
	subject, ok := a.locationSubject(h.Get("Location"))
	if !ok {
		return registration.Identity{}, failure(registration.AdminAmbiguous, true)
	}
	return registration.Identity{Subject: subject}, nil
}

func (a *Admin) locationSubject(raw string) (string, bool) {
	u, err := url.Parse(raw)
	base, _ := url.Parse(a.users)
	if err != nil || raw == "" || u.RawPath != "" || strings.ContainsAny(raw, "%\\#") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", false
	}
	// Accept absolute or root-relative paths only; no relative/network-path URLs.
	if !u.IsAbs() {
		if u.Host != "" || !strings.HasPrefix(raw, "/") {
			return "", false
		}
		u.Scheme, u.Host = base.Scheme, base.Host
	}
	if u.Scheme != base.Scheme || u.Host != base.Host || !strings.HasPrefix(u.Path, base.Path+"/") {
		return "", false
	}
	id := strings.TrimPrefix(u.Path, base.Path+"/")
	return id, segment.MatchString(id)
}

type userRepresentation struct {
	ID            string              `json:"id"`
	Enabled       bool                `json:"enabled"`
	EmailVerified bool                `json:"emailVerified"`
	Attributes    map[string][]string `json:"attributes"`
}

func owned(u userRepresentation, operation string) bool {
	v := u.Attributes[marker]
	return segment.MatchString(u.ID) && len(v) == 1 && v[0] == operation
}

func identity(u userRepresentation) registration.Identity {
	return registration.Identity{Subject: u.ID, Enabled: u.Enabled, EmailVerified: u.EmailVerified}
}

func (a *Admin) FindOwned(ctx context.Context, operationID string) (registration.Identity, error) {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.CallTimeout)
	defer cancel()
	if !segment.MatchString(operationID) {
		return registration.Identity{}, failure(registration.AdminInvalid, false)
	}
	query := url.Values{"q": {marker + ":" + operationID}, "max": {"2"}, "briefRepresentation": {"false"}}
	body, _, err := a.call(ctx, http.MethodGet, a.users+"?"+query.Encode(), nil, http.StatusOK, false)
	if err != nil {
		return registration.Identity{}, err
	}
	var users []userRepresentation
	if json.Unmarshal(body, &users) != nil || bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		return registration.Identity{}, failure(registration.AdminUpstream, false)
	}
	if len(users) == 0 {
		return registration.Identity{}, failure(registration.AdminNotFound, false)
	}
	if len(users) != 1 || !owned(users[0], operationID) {
		return registration.Identity{}, failure(registration.AdminOwnership, false)
	}
	return identity(users[0]), nil
}

func (a *Admin) ReadOwned(ctx context.Context, subject, operationID string) (registration.Identity, error) {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.CallTimeout)
	defer cancel()
	if !segment.MatchString(subject) || !segment.MatchString(operationID) {
		return registration.Identity{}, failure(registration.AdminInvalid, false)
	}
	body, _, err := a.call(ctx, http.MethodGet, a.users+"/"+subject, nil, http.StatusOK, false)
	if err != nil {
		return registration.Identity{}, err
	}
	var user userRepresentation
	if json.Unmarshal(body, &user) != nil {
		return registration.Identity{}, failure(registration.AdminUpstream, false)
	}
	if user.ID != subject || !owned(user, operationID) {
		return registration.Identity{}, failure(registration.AdminOwnership, false)
	}
	return identity(user), nil
}

func (a *Admin) EnableOwned(ctx context.Context, subject, operationID string) error {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.CallTimeout)
	defer cancel()
	user, err := a.ReadOwned(ctx, subject, operationID)
	if err != nil {
		return err
	}
	if user.Enabled {
		return nil
	}
	// Minimal update preserves required actions/attributes; never clear verification.
	_, _, err = a.call(ctx, http.MethodPut, a.users+"/"+subject, []byte(`{"enabled":true}`), http.StatusNoContent, false)
	return err
}

func (a *Admin) SendVerification(ctx context.Context, subject, operationID string) error {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.CallTimeout)
	defer cancel()
	user, err := a.ReadOwned(ctx, subject, operationID)
	if err != nil {
		return err
	}
	if !user.Enabled {
		return failure(registration.AdminInvalid, false)
	}
	if user.EmailVerified {
		return nil
	}
	q := url.Values{"client_id": {a.cfg.VerificationClientID}, "redirect_uri": {a.cfg.VerificationRedirect}, "lifespan": {strconv.FormatInt(int64(a.cfg.VerificationLifespan/time.Second), 10)}}
	_, _, err = a.call(ctx, http.MethodPut, a.users+"/"+subject+"/send-verify-email?"+q.Encode(), nil, http.StatusNoContent, false)
	return err
}
