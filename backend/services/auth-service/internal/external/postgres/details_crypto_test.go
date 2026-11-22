package postgres

import (
	"bytes"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
)

func TestCitizenDetailsEncryption(t *testing.T) {
	c, err := newCryptor(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}
	d := profile.EditableDetails{NameSI: "සිංහල නම", NameTA: "தமிழ் பெயர்", DateOfBirth: "2000-02-29", Gender: "synthetic",
		Address: profile.Address{Line1: "Synthetic private street", Line2: "Unit 7", City: "Synthetic city"}}
	one, err := c.sealDetails(d, "https://issuer.test", "citizen-a")
	if err != nil {
		t.Fatal(err)
	}
	two, _ := c.sealDetails(d, "https://issuer.test", "citizen-a")
	if bytes.Equal(one, two) {
		t.Fatal("nonce reused")
	}
	for _, value := range []string{d.NameSI, d.NameTA, d.DateOfBirth, d.Gender, d.Address.Line1, d.Address.Line2, d.Address.City} {
		if bytes.Contains(one, []byte(value)) {
			t.Fatal("plaintext leaked")
		}
	}
	decoded, err := c.openDetails(one, "https://issuer.test", "citizen-a")
	if err != nil || decoded.NameSI != d.NameSI || decoded.NameTA != d.NameTA || decoded.DateOfBirth != d.DateOfBirth || decoded.Gender != d.Gender || decoded.Address.Line1 != d.Address.Line1 || decoded.Address.Line2 != d.Address.Line2 || decoded.Address.City != d.Address.City {
		t.Fatal("round trip failed")
	}
	for _, owner := range [][2]string{{"https://issuer.test", "citizen-b"}, {"https://other.test", "citizen-a"}} {
		if _, err := c.openDetails(one, owner[0], owner[1]); err == nil {
			t.Fatal("identity substitution accepted")
		}
	}
	one[len(one)-1] ^= 1
	if _, err := c.openDetails(one, "https://issuer.test", "citizen-a"); err == nil {
		t.Fatal("tampering accepted")
	}
	if _, err := c.openDetails([]byte{1}, "https://issuer.test", "citizen-a"); err == nil {
		t.Fatal("truncated ciphertext accepted")
	}
	empty, err := c.sealDetails(profile.EditableDetails{}, "https://issuer.test", "citizen-a")
	if err != nil || empty != nil {
		t.Fatal("empty details should be SQL NULL")
	}
	decoded, err = c.openDetails(nil, "https://issuer.test", "citizen-a")
	if err != nil || !decoded.Address.Empty() || decoded.NameSI != "" {
		t.Fatal("empty details failed")
	}
	if _, err := c.openDetails([]byte{}, "https://issuer.test", "citizen-a"); err == nil {
		t.Fatal("empty non-NULL ciphertext accepted")
	}
}
