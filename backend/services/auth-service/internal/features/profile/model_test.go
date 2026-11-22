package profile

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestEditableDetailsValidation(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	valid := EditableDetails{NameSI: "සිංහල නම", NameTA: "தமிழ் பெயர்", DateOfBirth: "2000-02-29", Address: Address{Line1: "Synthetic street", City: "Synthetic city"}}
	if valid.Validate(now) != nil || (EditableDetails{}).Validate(now) != nil {
		t.Fatal("valid optional details rejected")
	}
	for name, d := range map[string]EditableDetails{
		"future date":       {DateOfBirth: "2026-11-24"},
		"invalid leap day":  {DateOfBirth: "2001-02-29"},
		"zero year":         {DateOfBirth: "0000-01-01"},
		"date time":         {DateOfBirth: "2000-01-01T00:00:00Z"},
		"long unicode name": {NameSI: strings.Repeat("සි", 201)},
		"invalid utf8":      {NameTA: string([]byte{0xff})},
		"control":           {Gender: "value\n"},
		"long address":      {Address: Address{City: strings.Repeat("x", 101)}},
	} {
		t.Run(name, func(t *testing.T) {
			if d.Validate(now) == nil {
				t.Fatal("invalid details accepted")
			}
		})
	}
}

func TestDetailsRedaction(t *testing.T) {
	for _, value := range []any{Address{Line1: "synthetic-secret"}, EditableDetails{NameSI: "synthetic-secret"}, Details{PhotoObjectKey: "synthetic-secret"}} {
		encoded, err := json.Marshal(value)
		if err != nil || strings.Contains(string(encoded), "synthetic-secret") || strings.Contains(fmt.Sprintf("%+v", value), "synthetic-secret") {
			t.Fatal("profile data leaked through default serialization")
		}
	}
}
