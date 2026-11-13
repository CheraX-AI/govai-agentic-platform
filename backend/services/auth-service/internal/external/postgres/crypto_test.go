package postgres

import (
	"bytes"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

func TestEncryptedProfiles(t *testing.T) {
	c, e := newCryptor(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	if e != nil {
		t.Fatal(e)
	}

	nic, _ := registration.ParseNIC("123456789v")
	p := registration.ProfileData{FullName: "Synthetic Citizen", NIC: nic, MobileNumber: "+94770000000"}
	b, e := c.seal(p, "https://identity.test/realm", "op")
	if e != nil {
		t.Fatal(e)
	}

	for _, s := range []string{p.FullName, nic.RestrictedOriginal(), p.MobileNumber} {
		if bytes.Contains(b, []byte(s)) {
			t.Fatal("plaintext leaked")
		}
	}

	q, e := c.open(b, "https://identity.test/realm", "op")
	if e != nil || !sameProfile(p, q) {
		t.Fatal("round trip failed")
	}

	if _, e = c.open(b, "https://identity.test/realm", "other"); e == nil {
		t.Fatal("operation substitution accepted")
	}

	b[len(b)-1] ^= 1
	if _, e = c.open(b, "https://identity.test/realm", "op"); e == nil {
		t.Fatal("tamper accepted")
	}

	upper, _ := registration.ParseNIC("123456789V")
	if !bytes.Equal(c.digest(nic), c.digest(upper)) {
		t.Fatal("NIC case mismatch")
	}

	if _, e = newCryptor(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{1}, 32)); e == nil {
		t.Fatal("shared keys accepted")
	}
}
