package registration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nyaruka/phonenumbers/v2"
)

const MaxRequestBytes = 16 * 1024

// Request is transient sensitive data, not a persistence DTO. Never log accessor
// results or serialize this to a queue/store. Password has no profile counterpart.
type Request struct {
	FullName     string
	NIC          NIC
	MobileNumber string
	Email        string
	password     string
}

// TransientPassword is solely for the future Keycloak credential adapter.
func (r Request) TransientPassword() string  { return r.password }
func (Request) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (Request) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (Request) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

// RequestError exposes only fixed codes and allowlisted field names.
// A future HTTP adapter maps invalid_request to 400, too_large to 413 and
// validation_failed to 422. Raw decoder/library errors are never propagated.
type RequestError struct {
	Code   string            `json:"code"`
	Fields map[string]string `json:"fields,omitempty"`
}

func (e *RequestError) Error() string { return e.Code }

var fieldNames = []string{"full_name", "nic", "mobile_number", "email", "password"}

// ParseRequest reads at most MaxRequestBytes+1 and accepts exactly one JSON object.
// JSON key matching is exact and case-sensitive, unlike struct decoding.
func ParseRequest(reader io.Reader) (Request, *RequestError) {
	bad := func(code string) (Request, *RequestError) { return Request{}, &RequestError{Code: code} }
	if reader == nil {
		return bad("invalid_request")
	}
	body, err := io.ReadAll(io.LimitReader(reader, MaxRequestBytes+1))
	if len(body) > MaxRequestBytes {
		return bad("too_large")
	}
	if err != nil || !utf8.Valid(body) || !json.Valid(body) || !validSurrogates(body) {
		return bad("invalid_request")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return bad("invalid_request")
	}
	values := make(map[string]string, 5)
	seen := make(map[string]bool, 5)
	fields := make(map[string]string)
	for d.More() {
		tok, err = d.Token()
		if err != nil {
			return bad("invalid_request")
		}
		key, ok := tok.(string)
		if !ok || seen[key] {
			return bad("invalid_request")
		}
		known := false
		for _, name := range fieldNames {
			if key == name {
				known = true
				break
			}
		}
		if !known {
			return bad("invalid_request")
		}
		seen[key] = true
		var raw json.RawMessage
		if d.Decode(&raw) != nil {
			return bad("invalid_request")
		}
		if len(raw) == 0 || raw[0] != '"' {
			fields[key] = "invalid_type"
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return bad("invalid_request")
		}
		values[key] = value
	}
	if _, err = d.Token(); err != nil {
		return bad("invalid_request")
	}
	if _, err = d.Token(); err != io.EOF {
		return bad("invalid_request")
	}
	for _, name := range fieldNames {
		if !seen[name] || (fields[name] == "" && values[name] == "") {
			fields[name] = "required"
		}
	}
	r := Request{}
	if fields["full_name"] == "" {
		name := values["full_name"]
		switch {
		case utf8.RuneCountInString(name) > 200:
			fields["full_name"] = "too_long"
		case strings.TrimSpace(name) == "":
			fields["full_name"] = "required"
		default:
			for _, c := range name {
				if unicode.IsControl(c) || c == '\u2028' || c == '\u2029' {
					fields["full_name"] = "invalid_format"
					break
				}
			}
		}
		r.FullName = name
	}
	if fields["nic"] == "" {
		r.NIC, err = ParseNIC(values["nic"])
		if err != nil {
			fields["nic"] = "invalid_format"
		}
	}
	if fields["mobile_number"] == "" {
		value := values["mobile_number"]
		if len(value) > 64 {
			fields["mobile_number"] = "too_long"
		} else {
			r.MobileNumber = normalizePhone(value)
			if r.MobileNumber == "" {
				fields["mobile_number"] = "invalid_format"
			}
		}
	}
	if fields["email"] == "" {
		value := values["email"]
		if len(value) > 254 {
			fields["email"] = "too_long"
		} else {
			r.Email = normalizeEmail(value)
			if r.Email == "" {
				fields["email"] = "invalid_format"
			}
		}
	}
	if fields["password"] == "" {
		if len(values["password"]) > 1024 {
			fields["password"] = "too_long"
		} else {
			r.password = values["password"]
		}
	}
	if len(fields) > 0 {
		return Request{}, &RequestError{Code: "validation_failed", Fields: fields}
	}
	return r, nil
}

func normalizePhone(value string) string {
	if !strings.HasPrefix(value, "+") {
		return ""
	}
	for _, c := range value[1:] {
		if !(c >= '0' && c <= '9') && c != ' ' && c != '-' && c != '(' && c != ')' && c != '.' {
			return ""
		}
	}
	number, err := phonenumbers.Parse(value, "")
	if err != nil || !phonenumbers.IsValidNumber(number) || number.GetExtension() != "" {
		return ""
	}
	return phonenumbers.Format(number, phonenumbers.E164)
}

var emailLocal = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\\.[A-Za-z0-9!#$%&'*+/=?^_`{|}~-]+)*$")
var domainLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)

func normalizeEmail(value string) string {
	parts := strings.Split(value, "@")
	if len(parts) != 2 || len(parts[0]) > 64 || !emailLocal.MatchString(parts[0]) {
		return ""
	}
	labels := strings.Split(parts[1], ".")
	if len(labels) < 2 {
		return ""
	}
	for _, label := range labels {
		if len(label) > 63 || !domainLabel.MatchString(label) {
			return ""
		}
	}
	// New-account policy only. No rewriting of existing identities or provider aliases.
	return strings.ToLower(value)
}

// encoding/json replaces lone UTF-16 surrogate escapes. Reject them instead so
// names and passwords cannot be silently changed. json.Valid runs before this.
func validSurrogates(body []byte) bool {
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' {
			continue
		}
		i++
		if body[i] != 'u' {
			continue
		}
		n, _ := strconv.ParseUint(string(body[i+1:i+5]), 16, 16)
		i += 4
		if n >= 0xDC00 && n <= 0xDFFF {
			return false
		}
		if n >= 0xD800 && n <= 0xDBFF {
			if i+6 >= len(body) || body[i+1] != '\\' || body[i+2] != 'u' {
				return false
			}
			low, _ := strconv.ParseUint(string(body[i+3:i+7]), 16, 16)
			if low < 0xDC00 || low > 0xDFFF {
				return false
			}
			i += 6
		}
	}
	return true
}
