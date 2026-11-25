package profile

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

const MaxUpdateBytes = 16 << 10
const MaxClientRevision int64 = 1<<53 - 2

type UpdateRequest struct {
	privateData
	Revision int64
	Fields   EditableDetails
}

type RequestError struct{ Code string }

func (e *RequestError) Error() string { return e.Code }

// exactObject rejects duplicate, unknown and case-variant keys, including
// caller-supplied identities, read-only registration data and photo references.
func exactObject(body []byte, allowed ...string) (map[string]json.RawMessage, bool) {
	d := json.NewDecoder(bytes.NewReader(body))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, false
	}
	result := make(map[string]json.RawMessage, len(allowed))
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, false
		}
		name, ok := key.(string)
		if !ok {
			return nil, false
		}
		known := false
		for _, a := range allowed {
			if name == a {
				known = true
				break
			}
		}
		if _, exists := result[name]; !known || exists {
			return nil, false
		}
		var raw json.RawMessage
		if d.Decode(&raw) != nil {
			return nil, false
		}
		result[name] = raw
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return nil, false
	}
	var extra any
	return result, d.Decode(&extra) == io.EOF && len(result) == len(allowed)
}

func stringField(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || raw[0] != '"' {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}

// PUT is a complete replacement of the six permitted fields. Require every
// field so omitting an address/name by mistake cannot silently erase it.
func ParseUpdate(reader io.Reader) (UpdateRequest, *RequestError) {
	bad := func(code string) (UpdateRequest, *RequestError) { return UpdateRequest{}, &RequestError{Code: code} }
	if reader == nil {
		return bad("invalid_request")
	}
	body, err := io.ReadAll(io.LimitReader(reader, MaxUpdateBytes+1))
	if len(body) > MaxUpdateBytes {
		return bad("request_too_large")
	}
	if err != nil || !registration.ValidJSONEncoding(body) {
		return bad("invalid_request")
	}
	values, ok := exactObject(body, "revision", "name_si", "name_ta", "date_of_birth", "gender", "address")
	if !ok {
		return bad("invalid_request")
	}
	raw := string(bytes.TrimSpace(values["revision"]))
	if raw == "" {
		return bad("invalid_revision")
	}
	for _, c := range raw {
		if c < '0' || c > '9' {
			return bad("invalid_revision")
		}
	}
	revision, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || revision > MaxClientRevision {
		return bad("invalid_revision")
	}
	var fields EditableDetails
	for _, pair := range []struct {
		name   string
		target *string
	}{
		{"name_si", &fields.NameSI}, {"name_ta", &fields.NameTA}, {"date_of_birth", &fields.DateOfBirth}, {"gender", &fields.Gender},
	} {
		value, valid := stringField(values[pair.name])
		if !valid {
			return bad("invalid_request")
		}
		*pair.target = value
	}
	address, ok := exactObject(values["address"], "line1", "line2", "city")
	if !ok {
		return bad("invalid_request")
	}
	for _, pair := range []struct {
		name   string
		target *string
	}{
		{"line1", &fields.Address.Line1}, {"line2", &fields.Address.Line2}, {"city", &fields.Address.City},
	} {
		value, valid := stringField(address[pair.name])
		if !valid {
			return bad("invalid_request")
		}
		*pair.target = value
	}
	if fields.Validate(time.Now()) != nil {
		return bad("validation_failed")
	}
	return UpdateRequest{Revision: revision, Fields: fields}, nil
}
