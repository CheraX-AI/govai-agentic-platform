// Package chatgateway transports requests to the fixed Go core API, not Python.
package chatgateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("chat_gateway_unavailable")
var ErrTimeout = errors.New("chat_gateway_timeout")
var ErrDenied = errors.New("chat_gateway_denied")
var ErrResponse = errors.New("chat_gateway_invalid_response")

type Client struct {
	endpoint  string
	http      *http.Client
	transport *http.Transport
}

// Payload is untrusted upstream JSON. The HTTP adapter must validate its schema
// before returning it to the browser. No route uses this transport yet.
type Payload struct{ body []byte }

func (Payload) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (Payload) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }
func (Payload) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (p Payload) Bytes() []byte              { return bytes.Clone(p.body) }

func New(endpoint string, roots *x509.CertPool, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "/api/chat/message" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(endpoint, "\\ \r\n\t#") || timeout < time.Second || timeout > time.Minute {
		return nil, ErrUnavailable
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	tr.DisableCompression = true
	tr.MaxResponseHeaderBytes = 16 << 10
	tr.MaxConnsPerHost = 8
	tr.ResponseHeaderTimeout = timeout
	tr.TLSHandshakeTimeout = 5 * time.Second
	return &Client{endpoint: endpoint, transport: tr, http: &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Close() { c.transport.CloseIdleConnections() }

// Ask never forwards cookies, caller headers or refresh tokens. Request parsing
// and session authorization belong to the gateway's HTTP/session adapters.
func (c *Client) Ask(ctx context.Context, access string, body []byte) (Payload, error) {
	if access == "" || len(access) > 16384 || strings.ContainsAny(access, " \t\r\n") || len(body) > 16<<10 || !json.Valid(body) {
		return Payload{}, ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return Payload{}, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Payload{}, ErrTimeout
		}
		return Payload{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return Payload{}, ErrDenied
	}
	if response.StatusCode == 504 {
		return Payload{}, ErrTimeout
	}
	if response.StatusCode != 200 {
		return Payload{}, ErrUnavailable
	}
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || response.Header.Get("Content-Encoding") != "" {
		return Payload{}, ErrResponse
	}
	b, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		return Payload{}, ErrTimeout
	}
	if err != nil || len(b) > 64<<10 || !json.Valid(b) {
		return Payload{}, ErrResponse
	}
	return Payload{body: b}, nil
}
