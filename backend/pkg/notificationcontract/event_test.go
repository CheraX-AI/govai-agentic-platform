package notificationcontract

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const trustedIssuer = "https://identity.example/realms/govai"

func validFields() map[string]any {
	return map[string]any{
		"event_id":    "123e4567-e89b-42d3-a456-426614174000",
		"event_type":  LoginSucceeded,
		"version":     1,
		"issuer":      trustedIssuer,
		"subject":     "synthetic-subject",
		"occurred_at": "2026-11-21T05:00:00Z",
		"data":        map[string]any{},
	}
}

func encode(t *testing.T, fields map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(fields)
	if err != nil {
		t.Fatal("synthetic fixture encoding failed")
	}
	return body
}

// TestInvalidFields verifies required fields, allowlists and sensitive-data rejection.

func TestInvalidFields(t *testing.T) {
	cases := []struct {
		name  string
		field string
		value any
	}{
		{"invalid UUID", "event_id", "bad"},
		{"uppercase UUID", "event_id", "123E4567-e89b-42d3-a456-426614174000"},
		{"wrong version", "version", 2},
		{"string version", "version", "1"},
		{"unknown event", "event_type", "citizen.login.failed"},
		{"wrong issuer", "issuer", "https://attacker.example"},
		{"invalid subject", "subject", "synthetic subject"},
		{"long subject", "subject", strings.Repeat("a", 129)},
		{"null subject", "subject", nil},
		{"invalid time", "occurred_at", "not-a-time"},
		{"zero time", "occurred_at", "0001-01-01T00:00:00Z"},
		{"null time", "occurred_at", nil},
		{"null data", "data", nil},
		{"personal metadata", "data", map[string]any{"nic": "synthetic"}},
		{"untrusted priority", "priority", "high"},
		{"arbitrary message", "message", "untrusted content"},
		{"wrong field case", "Event_id", "synthetic"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			fields := validFields()
			fields[item.field] = item.value
			_, err := Decode(encode(t, fields), trustedIssuer)
			if !errors.Is(err, ErrInvalid) {
				t.Fatal("unsafe field accepted or unexpected error")
			}
		})
	}
	for _, name := range []string{
		"event_id", "event_type", "version", "issuer", "subject", "occurred_at",
	} {
		fields := validFields()
		delete(fields, name)
		if _, err := Decode(encode(t, fields), trustedIssuer); err == nil {
			t.Fatalf("missing field %s was accepted", name)
		}
	}
}

// TestInvalidJSON prevents duplicate fields, trailing values and oversized input.

func TestInvalidJSON(t *testing.T) {
	valid := string(encode(t, validFields()))
	cases := []string{
		"null", "[]", "[" + valid + "]", valid + "{}",
		"{\"event_id\":\"duplicate\"," + valid[1:],
		"{\"\\u0065vent_id\":\"duplicate\"," + valid[1:],
		valid + string([]byte{0xff}),
		strings.Replace(valid, "\"version\":1", "\"version\":1e0", 1),
	}
	for _, body := range cases {
		if _, err := Decode([]byte(body), trustedIssuer); !errors.Is(err, ErrInvalid) {
			t.Fatal("malformed or ambiguous JSON accepted")
		}
	}
	tooLarge := []byte(strings.Repeat(" ", MaxEventBytes+1))
	if _, err := Decode(tooLarge, trustedIssuer); !errors.Is(err, ErrTooLarge) {
		t.Fatal("oversized payload accepted")
	}
}

// TestValidEvents verifies priorities, round trips and safe generic logging.

func TestValidEvents(t *testing.T) {
	for eventType, expected := range map[string]Priority{
		LoginSucceeded: High, PasswordChanged: High, EmailVerified: Normal,
	} {
		fields := validFields()
		fields["event_type"] = eventType
		event, err := Decode(encode(t, fields), trustedIssuer)
		if err != nil {
			t.Fatal("valid event rejected")
		}
		priority, err := PriorityFor(event.Type)
		if err != nil || priority != expected {
			t.Fatal("incorrect backend-derived priority")
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal("wire encoding failed")
		}
		if _, err := Decode(encoded, trustedIssuer); err != nil {
			t.Fatal("wire round trip failed")
		}
		if strings.Contains(fmt.Sprintf("%+v", event), event.Subject) ||
			strings.Contains(event.LogValue().String(), event.Subject) {
			t.Fatal("generic logging exposed identity")
		}
	}
	fields := validFields()
	delete(fields, "data")
	fields["occurred_at"] = "2020-01-01T00:00:00Z"
	if _, err := Decode(encode(t, fields), trustedIssuer); err != nil {
		t.Fatal("delayed event or absent optional data rejected")
	}
	if _, err := Decode(encode(t, fields), "http://identity.example"); err == nil {
		t.Fatal("insecure trusted issuer accepted")
	}
}
