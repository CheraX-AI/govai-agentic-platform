// Package recovery contains password recovery primitives. No routes are enabled.
package recovery

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
)

var ErrToken = errors.New("invalid recovery token")

// Token is a bearer credential. Explicit Reveal is only for delivery, never logs.
// Only Digest may cross the persistence boundary; it is not an identity token.
type Token struct{ value string }

func NewToken() (Token, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Token{}, errors.New("recovery token generation failed")
	}

	return Token{base64.RawURLEncoding.EncodeToString(raw[:])}, nil
}

func ParseToken(value string) (Token, error) {
	if len(value) != 43 {
		return Token{}, ErrToken
	}

	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return Token{}, ErrToken
	}

	return Token{value}, nil
}

func (t Token) Reveal() string { return t.value }

func (t Token) Digest() ([32]byte, error) {
	if _, err := ParseToken(t.value); err != nil {
		return [32]byte{}, err
	}

	return sha256.Sum256([]byte("govai:password-recovery:v1:" + t.value)), nil
}

func (Token) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[REDACTED]") }

func (Token) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }

func (Token) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }
