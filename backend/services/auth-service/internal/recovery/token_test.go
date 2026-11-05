package recovery

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestTokenRoundTripAndRedaction(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}

	b, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}

	if a.Reveal() == b.Reveal() {
		t.Fatal("duplicate random token")
	}

	parsed, err := ParseToken(a.Reveal())
	if err != nil {
		t.Fatal(err)
	}

	h, err := a.Digest()
	if err != nil {
		t.Fatal(err)
	}

	p, _ := parsed.Digest()
	other, _ := b.Digest()
	if h != p || h == other {
		t.Fatal("digest binding failed")
	}

	encoded, _ := json.Marshal(a)
	for _, output := range []string{fmt.Sprintf("%v %+v %#v %s", a, a, a, a), string(encoded), a.LogValue().String()} {
		if strings.Contains(output, a.Reveal()) || !strings.Contains(output, "REDACTED") {
			t.Fatal("token redaction failed")
		}
	}
}

func TestMalformedTokens(t *testing.T) {
	for _, value := range []string{
		"",
		strings.Repeat("a", 42),
		strings.Repeat("a", 44),
		strings.Repeat("!", 43),
		strings.Repeat("A", 42) + "B",
		" " + strings.Repeat("A", 43),
	} {
		if _, err := ParseToken(value); err == nil {
			t.Fatal("accepted malformed token")
		}
	}

	if _, err := (Token{}).Digest(); err == nil {
		t.Fatal("zero value accepted")
	}
}
