package notificationcontract

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

// Decode accepts one bounded JSON object with exact, unique field names.

func Decode(body []byte, trustedIssuer string) (Event, error) {
	if len(body) > MaxEventBytes {
		return Event{}, ErrTooLarge
	}
	if !utf8.Valid(body) || !json.Valid(body) {
		return Event{}, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return Event{}, ErrInvalid
	}
	allowed := map[string]bool{
		"event_id": true, "event_type": true, "version": true,
		"issuer": true, "subject": true, "occurred_at": true, "data": true,
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return Event{}, ErrInvalid
		}
		name, ok := token.(string)
		if _, seen := fields[name]; !ok || !allowed[name] || seen {
			return Event{}, ErrInvalid
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return Event{}, ErrInvalid
		}
		fields[name] = value
	}
	if last, err := decoder.Token(); err != nil || last != json.Delim('}') {
		return Event{}, ErrInvalid
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return Event{}, ErrInvalid
	}
	return decodeFields(fields, trustedIssuer)
}

func decodeFields(fields map[string]json.RawMessage, issuer string) (Event, error) {
	var event Event
	stringsToRead := map[string]*string{
		"event_id":   &event.ID,
		"event_type": &event.Type,
		"issuer":     &event.Issuer,
		"subject":    &event.Subject,
	}
	for name, target := range stringsToRead {
		raw := bytes.TrimSpace(fields[name])
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, target) != nil {
			return Event{}, ErrInvalid
		}
	}
	if string(fields["version"]) != "1" {
		return Event{}, ErrInvalid
	}
	event.Version = Version
	timestamp := bytes.TrimSpace(fields["occurred_at"])
	if len(timestamp) == 0 || timestamp[0] != '"' ||
		json.Unmarshal(timestamp, &event.OccurredAt) != nil {
		return Event{}, ErrInvalid
	}

	// Version one needs no personal metadata; only an absent or empty object is allowed.

	if raw, present := fields["data"]; present {
		var empty map[string]json.RawMessage
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || raw[0] != '{' ||
			json.Unmarshal(raw, &empty) != nil || len(empty) != 0 {
			return Event{}, ErrInvalid
		}
	}
	if err := event.Validate(issuer); err != nil {
		return Event{}, err
	}
	return event, nil
}
