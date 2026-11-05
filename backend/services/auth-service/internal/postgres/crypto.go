// Package postgres implements the registration persistence contracts. Database
// errors are deliberately not returned: they may contain citizen data or DSNs.
package postgres

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

var ErrStore = errors.New("authentication database unavailable or invalid state")

type cryptor struct {
	aead   cipher.AEAD
	nicKey []byte
}

type profileJSON struct {
	Name   string `json:"full_name"`
	NIC    string `json:"nic"`
	Mobile string `json:"mobile_number"`
}

func newCryptor(dataKey, nicKey []byte) (cryptor, error) {
	if len(dataKey) != 32 || len(nicKey) != 32 || hmac.Equal(dataKey, nicKey) {
		return cryptor{}, ErrStore
	}

	b, err := aes.NewCipher(dataKey)
	if err != nil {
		return cryptor{}, ErrStore
	}

	a, err := cipher.NewGCM(b)
	return cryptor{a, append([]byte(nil), nicKey...)}, err
}

func (c cryptor) digest(n registration.NIC) []byte {
	h := hmac.New(sha256.New, c.nicKey)
	h.Write([]byte("govai-nic-v1\x00" + n.NormalizedNumber()))
	return h.Sum(nil)
}

func aad(issuer, op string) []byte {
	b, _ := json.Marshal([]string{"govai-profile-v1", issuer, op})
	return b
}

func (c cryptor) seal(p registration.ProfileData, issuer, op string) ([]byte, error) {
	if p.NIC.NormalizedNumber() == "" || p.FullName == "" || p.MobileNumber == "" {
		return nil, ErrStore
	}

	b, err := json.Marshal(profileJSON{p.FullName, p.NIC.RestrictedOriginal(), p.MobileNumber})
	if err != nil {
		return nil, ErrStore
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, ErrStore
	}

	return c.aead.Seal(nonce, nonce, b, aad(issuer, op)), nil
}

func (c cryptor) open(b []byte, issuer, op string) (registration.ProfileData, error) {
	n := c.aead.NonceSize()
	if len(b) < n {
		return registration.ProfileData{}, ErrStore
	}

	plain, err := c.aead.Open(nil, b[:n], b[n:], aad(issuer, op))
	if err != nil {
		return registration.ProfileData{}, ErrStore
	}

	var p profileJSON
	if json.Unmarshal(plain, &p) != nil {
		return registration.ProfileData{}, ErrStore
	}

	nic, err := registration.ParseNIC(p.NIC)
	if err != nil {
		return registration.ProfileData{}, ErrStore
	}

	return registration.ProfileData{FullName: p.Name, NIC: nic, MobileNumber: p.Mobile}, nil
}

func sameProfile(a, b registration.ProfileData) bool {
	return a.FullName == b.FullName &&
		a.NIC.RestrictedOriginal() == b.NIC.RestrictedOriginal() &&
		a.MobileNumber == b.MobileNumber
}
