package login

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

const MaxRefreshRequestBytes = 20 << 10

// RefreshRequest is a transient credential and must not enter logs or storage.
type RefreshRequest struct {
	privateValue
	token string
}

func (r RefreshRequest) Token() string { return r.token }

func ParseRefresh(reader io.Reader) (RefreshRequest, error) {
	if reader == nil {
		return RefreshRequest{}, ErrInvalid
	}

	body, err := io.ReadAll(io.LimitReader(reader, MaxRefreshRequestBytes+1))
	if err != nil {
		return RefreshRequest{}, ErrInvalid
	}

	if len(body) > MaxRefreshRequestBytes {
		return RefreshRequest{}, ErrTooLarge
	}

	if !registration.ValidJSONEncoding(body) {
		return RefreshRequest{}, ErrInvalid
	}

	d := json.NewDecoder(bytes.NewReader(body))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return RefreshRequest{}, ErrInvalid
	}

	key, err := d.Token()
	if err != nil || key != "refresh_token" {
		return RefreshRequest{}, ErrInvalid
	}

	var value json.RawMessage
	if d.Decode(&value) != nil || len(value) == 0 || value[0] != '"' {
		return RefreshRequest{}, ErrInvalid
	}

	var raw string
	if json.Unmarshal(value, &raw) != nil || !validRefresh(raw) {
		return RefreshRequest{}, ErrInvalid
	}

	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return RefreshRequest{}, ErrInvalid
	}

	if _, err = d.Token(); err != io.EOF {
		return RefreshRequest{}, ErrInvalid
	}

	return RefreshRequest{token: raw}, nil
}
