package chat

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// HTTPConfig is server-owned. URL is the exact POST endpoint, not client input.
// Authentication is provisional pending the Python owner's contract agreement.
type HTTPConfig struct {
	URL, AuthMode, BearerToken string
	Timeout                    time.Duration
	AllowLoopbackHTTP          bool
}

func (HTTPConfig) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[REDACTED]") }

func (HTTPConfig) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }

func (HTTPConfig) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

type HTTPAgent struct {
	client    *http.Client
	cfg       HTTPConfig
	transport *http.Transport
}

var _ Agent = (*HTTPAgent)(nil)

func ValidateHTTPConfig(c HTTPConfig) error {
	u, err := url.Parse(c.URL)
	if err != nil ||
		u.Hostname() == "" ||
		u.User != nil ||
		u.Opaque != "" ||
		u.RawQuery != "" ||
		u.ForceQuery ||
		u.Fragment != "" ||
		u.RawPath != "" ||
		len(c.URL) > 2048 ||
		strings.ContainsAny(c.URL, "\\\r\n\t %") {
		return ErrUnavailable
	}

	if u.Path != "" &&
		u.Path != "/" &&
		path.Clean(strings.TrimSuffix(u.Path, "/")) != strings.TrimSuffix(u.Path, "/") {
		return ErrUnavailable
	}

	ip := net.ParseIP(u.Hostname())
	loopback := ip != nil && ip.IsLoopback()
	switch u.Scheme {
	case "https":
	case "http":
		if !c.AllowLoopbackHTTP || !loopback || c.AuthMode != "none" {
			return ErrUnavailable
		}
	default:
		return ErrUnavailable
	}

	switch c.AuthMode {
	case "none":
		if !loopback || c.BearerToken != "" {
			return ErrUnavailable
		}
	case "bearer":
		if u.Scheme != "https" || c.BearerToken == "" || len(c.BearerToken) > 8192 {
			return ErrUnavailable
		}
		for _, r := range c.BearerToken {
			if r <= 32 || r >= 127 {
				return ErrUnavailable
			}
		}
	default:
		return ErrUnavailable
	}

	if c.Timeout < 10*time.Millisecond || c.Timeout > time.Minute {
		return ErrUnavailable
	}

	return nil
}

func NewHTTP(c HTTPConfig, client *http.Client) (*HTTPAgent, error) {
	if ValidateHTTPConfig(c) != nil || client == nil {
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

	if c := transport.TLSClientConfig; c != nil &&
		(c.InsecureSkipVerify ||
			c.ServerName != "" ||
			c.VerifyConnection != nil ||
			c.VerifyPeerCertificate != nil) {
		return nil, ErrUnavailable
	}

	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		transport.TLSClientConfig.MinVersion = tls.VersionTLS12
	}

	transport.Proxy = nil
	transport.DisableCompression = true
	transport.MaxResponseHeaderBytes = 16 << 10
	transport.ResponseHeaderTimeout = c.Timeout
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.MaxConnsPerHost = 8
	transport.MaxIdleConnsPerHost = 8
	transport.MaxIdleConns = 8
	bounded := *client
	bounded.Transport = transport
	bounded.Jar = nil
	bounded.Timeout = c.Timeout
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &HTTPAgent{cfg: c, client: &bounded, transport: transport}, nil
}

func (*HTTPAgent) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[REDACTED]") }

func (*HTTPAgent) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }

func (*HTTPAgent) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

func (a *HTTPAgent) Close() { a.transport.CloseIdleConnections() }

func (a *HTTPAgent) Ask(ctx context.Context, q Question) (Reply, error) {
	// The injected port can be called independently of HTTP validation.
	encoded, err := json.Marshal(map[string]string{"message": q.Text()})
	if err != nil {
		return Reply{}, ErrInvalid
	}

	canonical, err := Parse(bytes.NewReader(encoded))
	if err != nil || canonical.Text() != q.Text() {
		return Reply{}, ErrInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.URL, bytes.NewReader(encoded))
	if err != nil {
		return Reply{}, ErrUnavailable
	}

	req.GetBody = nil
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if a.cfg.AuthMode == "bearer" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.BearerToken)
	}

	if id := requestIDFrom(ctx); id != "" {
		req.Header.Set("X-Request-ID", id)
	}

	response, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Reply{}, ctx.Err()
		}

		return Reply{}, ErrUnavailable
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == 429 || response.StatusCode >= 500 {
			return Reply{}, ErrUnavailable
		}

		return Reply{}, ErrResponse
	}

	media, params, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil ||
		media != "application/json" ||
		len(response.Header.Values("Content-Type")) != 1 ||
		response.Header.Get("Content-Encoding") != "" ||
		response.ContentLength > MaxResponseBytes {
		return Reply{}, ErrResponse
	}

	for key, value := range params {
		if key != "charset" || !strings.EqualFold(value, "utf-8") {
			return Reply{}, ErrResponse
		}
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return Reply{}, ctx.Err()
		}

		return Reply{}, ErrUnavailable
	}

	if len(body) > MaxResponseBytes {
		return Reply{}, ErrResponse
	}

	return DecodeReply(body)
}
