package postgres

import (
	"crypto/rand"
	"encoding/json"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
)

// Deliberate mapping avoids the profile model's redacted JSON representation.
type addressJSON struct {
	Line1 string `json:"line1"`
	Line2 string `json:"line2"`
	City  string `json:"city"`
}

type detailsJSON struct {
	NameSI      string      `json:"name_si"`
	NameTA      string      `json:"name_ta"`
	DateOfBirth string      `json:"date_of_birth"`
	Gender      string      `json:"gender"`
	Address     addressJSON `json:"address"`
}

func detailsAAD(issuer, subject string) []byte {
	b, _ := json.Marshal([]string{"govai-citizen-details-v1", issuer, subject})
	return b
}

func (c cryptor) sealDetails(d profile.EditableDetails, issuer, subject string) ([]byte, error) {
	if issuer == "" || subject == "" {
		return nil, ErrStore
	}
	if d.Validate(time.Now()) != nil {
		return nil, profile.ErrInvalid
	}
	if d.NameSI == "" && d.NameTA == "" && d.DateOfBirth == "" && d.Gender == "" && d.Address.Empty() {
		return nil, nil
	}
	b, err := json.Marshal(detailsJSON{d.NameSI, d.NameTA, d.DateOfBirth, d.Gender, addressJSON{d.Address.Line1, d.Address.Line2, d.Address.City}})
	if err != nil {
		return nil, ErrStore
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, ErrStore
	}
	return c.aead.Seal(nonce, nonce, b, detailsAAD(issuer, subject)), nil
}

func (c cryptor) openDetails(b []byte, issuer, subject string) (profile.EditableDetails, error) {
	if issuer == "" || subject == "" {
		return profile.EditableDetails{}, ErrStore
	}
	if b == nil {
		return profile.EditableDetails{}, nil
	}
	n := c.aead.NonceSize()
	if len(b) < n+c.aead.Overhead() || len(b) > 8192 {
		return profile.EditableDetails{}, ErrStore
	}
	plain, err := c.aead.Open(nil, b[:n], b[n:], detailsAAD(issuer, subject))
	if err != nil {
		return profile.EditableDetails{}, ErrStore
	}
	var value detailsJSON
	if json.Unmarshal(plain, &value) != nil {
		return profile.EditableDetails{}, ErrStore
	}
	d := profile.EditableDetails{NameSI: value.NameSI, NameTA: value.NameTA, DateOfBirth: value.DateOfBirth, Gender: value.Gender, Address: profile.Address{Line1: value.Address.Line1, Line2: value.Address.Line2, City: value.Address.City}}
	if d.Validate(time.Now()) != nil {
		return profile.EditableDetails{}, ErrStore
	}
	return d, nil
}
