// Package login implements the approved prototype password-login flow. Passwords
// and returned access tokens are transient, never application persistence data.
package login

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

const MaxRequestBytes = 4096

var ErrInvalid = errors.New("invalid_login_request")

var ErrTooLarge = errors.New("login_request_too_large")

var ErrFailed = errors.New("login_failed")

var ErrUnavailable = errors.New("login_unavailable")

type privateValue struct{}

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (privateValue) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (privateValue) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (privateValue) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

type Request struct {
	privateValue
	email, password string
	nic             registration.NIC
	isNIC           bool
}

// Email returns only an explicitly supplied email, never the NIC identifier.
func (r Request) Email() string { return r.email }

func (r Request) LimitKey() string {
	if r.isNIC {
		return "nic:" + r.nic.NormalizedNumber()
	}

	return r.email
}

func Parse(reader io.Reader) (Request, error) {
	if reader == nil {
		return Request{}, ErrInvalid
	}

	b, e := io.ReadAll(io.LimitReader(reader, MaxRequestBytes+1))
	if e != nil {
		return Request{}, ErrInvalid
	}

	if len(b) > MaxRequestBytes {
		return Request{}, ErrTooLarge
	}

	if !registration.ValidJSONEncoding(b) {
		return Request{}, ErrInvalid
	}

	decoder := json.NewDecoder(bytes.NewReader(b))
	start, e := decoder.Token()
	if e != nil || start != json.Delim('{') {
		return Request{}, ErrInvalid
	}

	fields := map[string]string{}
	for decoder.More() {
		raw, e := decoder.Token()
		key, ok := raw.(string)
		if e != nil || !ok || (key != "email" && key != "identifier" && key != "password") {
			return Request{}, ErrInvalid
		}

		if _, seen := fields[key]; seen {
			return Request{}, ErrInvalid
		}

		var value json.RawMessage
		if decoder.Decode(&value) != nil || len(value) == 0 || value[0] != '"' {
			return Request{}, ErrInvalid
		}

		var text string
		if json.Unmarshal(value, &text) != nil {
			return Request{}, ErrInvalid
		}

		fields[key] = text
	}

	if end, e := decoder.Token(); e != nil || end != json.Delim('}') {
		return Request{}, ErrInvalid
	}

	if _, e = decoder.Token(); e != io.EOF {
		return Request{}, ErrInvalid
	}

	password := fields["password"]
	if len(fields) != 2 || len(password) == 0 || len(password) > 1024 {
		return Request{}, ErrInvalid
	}

	if value, ok := fields["email"]; ok {
		email := registration.CanonicalEmail(value)
		if email == "" {
			return Request{}, ErrInvalid
		}

		return Request{email: email, password: password}, nil
	}

	value, ok := fields["identifier"]
	if !ok {
		return Request{}, ErrInvalid
	}

	if email := registration.CanonicalEmail(value); email != "" {
		return Request{email: email, password: password}, nil
	}

	nic, err := registration.ParseNIC(value)
	if err != nil {
		return Request{}, ErrInvalid
	}

	return Request{nic: nic, isNIC: true, password: password}, nil
}
