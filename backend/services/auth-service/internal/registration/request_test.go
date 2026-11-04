package registration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func requestFixture() map[string]any {
	return map[string]any{"full_name": "Synthetic  Citizen", "nic": "123456789v", "mobile_number": "+94 77 123 4567", "email": "Citizen+Test@EXAMPLE.TEST", "password": " synthetic secret "}
}
func fixtureJSON(t *testing.T, fields map[string]any) string {
	t.Helper()
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal("fixture encoding failed")
	}
	return string(b)
}

func TestRequestNormalizationAndPreservation(t *testing.T) {
	for _, tc := range []struct{ name, fullName, phone, wantPhone, nic, wantNIC, password string }{
		{"sinhala", "  නිමල්  පෙරේරා ", "+94 77 123 4567", "+94771234567", "123456789v", "123456789V", "  abc  "},
		{"tamil", "அருள்  குமார்", "+44 7911 123456", "+447911123456", "123456789x", "123456789X", "\tsecret\n"},
		{"combining", "Jose\u0301  O’Neil", "+1 (202) 555-0123", "+12025550123", "001234567890", "001234567890", "e\u0301🔐\\uD800"},
		{"short_password", "A", "+94771234567", "+94771234567", "201234567890", "201234567890", "x"},
		{"spaces_password", "A", "+94771234567", "+94771234567", "201234567890", "201234567890", "   "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := requestFixture()
			f["full_name"] = tc.fullName
			f["mobile_number"] = tc.phone
			f["nic"] = tc.nic
			f["password"] = tc.password
			r, e := ParseRequest(strings.NewReader(fixtureJSON(t, f)))
			if e != nil {
				t.Fatalf("unexpected error: %s", e.Code)
			}
			if r.FullName != tc.fullName || r.MobileNumber != tc.wantPhone || r.NIC.NormalizedNumber() != tc.wantNIC || r.NIC.RestrictedOriginal() != tc.nic || r.Email != "citizen+test@example.test" || r.TransientPassword() != tc.password {
				t.Fatal("normalization/preservation mismatch")
			}
		})
	}
}

func TestRequestFieldErrors(t *testing.T) {
	for _, field := range fieldNames {
		t.Run(field, func(t *testing.T) {
			for _, tc := range []struct {
				name  string
				value any
				code  string
			}{
				{"empty", "", "required"}, {"null", nil, "invalid_type"}, {"number", 12, "invalid_type"}, {"bool", true, "invalid_type"}, {"array", []string{}, "invalid_type"}, {"object", map[string]string{}, "invalid_type"},
			} {
				t.Run(tc.name, func(t *testing.T) { f := requestFixture(); f[field] = tc.value; assertFieldError(t, f, field, tc.code) })
			}
			f := requestFixture()
			delete(f, field)
			assertFieldError(t, f, field, "required")
		})
	}
	for _, tc := range []struct{ name, field, value, code string }{
		{"blank_name", "full_name", " \u2003 ", "required"},
		{"name_control", "full_name", "A\nB", "invalid_format"},
		{"name_limit", "full_name", strings.Repeat("அ", 201), "too_long"},
		{"nic_whitespace", "nic", " 123456789V", "invalid_format"},
		{"nic_suffix", "nic", "123456789Z", "invalid_format"},
		{"phone_local", "mobile_number", "0771234567", "invalid_format"},
		{"phone_00", "mobile_number", "0094771234567", "invalid_format"},
		{"phone_extension", "mobile_number", "+12025550123 ext 1", "invalid_format"},
		{"phone_letters", "mobile_number", "+1CALLNOW", "invalid_format"},
		{"phone_short", "mobile_number", "+941", "invalid_format"},
		{"phone_country", "mobile_number", "+999123456789", "invalid_format"},
		{"phone_limit", "mobile_number", "+" + strings.Repeat("1", 64), "too_long"},
		{"email_display", "email", "Citizen <a@example.test>", "invalid_format"},
		{"email_space", "email", " a@example.test", "invalid_format"},
		{"email_double_dot", "email", "a..b@example.test", "invalid_format"},
		{"email_unicode", "email", "அ@example.test", "invalid_format"},
		{"email_localhost", "email", "a@localhost", "invalid_format"},
		{"email_label", "email", "a@-example.test", "invalid_format"},
		{"email_local_limit", "email", strings.Repeat("a", 65) + "@example.test", "invalid_format"},
		{"email_limit", "email", strings.Repeat("a", 255), "too_long"},
		{"password_limit", "password", strings.Repeat("a", 1025), "too_long"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := requestFixture()
			f[tc.field] = tc.value
			assertFieldError(t, f, tc.field, tc.code)
		})
	}
}

func assertFieldError(t *testing.T, f map[string]any, field, code string) {
	t.Helper()
	r, e := ParseRequest(strings.NewReader(fixtureJSON(t, f)))
	if e == nil || e.Code != "validation_failed" || e.Fields[field] != code {
		t.Fatal("incorrect safe field error")
	}
	if r.TransientPassword() != "" || r.FullName != "" {
		t.Fatal("partial request escaped on error")
	}
}

func TestRequestStrictJSON(t *testing.T) {
	good := fixtureJSON(t, requestFixture())
	for _, input := range []string{"", "null", "[]", "42", "{", good + " {}", good + " null", good + " secret", strings.TrimSuffix(good, "}") + `,"role":"admin"}`, strings.TrimSuffix(good, "}") + `,"email":"other@example.test"}`, strings.TrimSuffix(good, "}") + `,"em\u0061il":"other@example.test"}`, `{"password":"\uD800"}`, `{"full_name":"\uDC00"}`, "{\"full_name\":\"" + string([]byte{255}) + "\"}"} {
		_, e := ParseRequest(strings.NewReader(input))
		if e == nil || e.Code != "invalid_request" {
			t.Fatal("invalid JSON accepted")
		}
	}
	// Valid surrogate pairs and escaped backslashes must not be rejected/rewritten.
	f := requestFixture()
	f["password"] = "🔐"
	body := strings.Replace(fixtureJSON(t, f), "🔐", `\ud83d\udd10`, 1)
	r, e := ParseRequest(strings.NewReader(body))
	if e != nil || r.TransientPassword() != "🔐" {
		t.Fatal("valid surrogate pair rejected")
	}
	_, e = ParseRequest(strings.NewReader(good + strings.Repeat(" ", MaxRequestBytes)))
	if e == nil || e.Code != "too_large" {
		t.Fatal("body cap ignored")
	}
	_, e = ParseRequest(strings.NewReader(good + strings.Repeat(" ", MaxRequestBytes-len(good))))
	if e != nil {
		t.Fatal("body boundary rejected")
	}
	if _, e = ParseRequest(nil); e == nil || e.Code != "invalid_request" {
		t.Fatal("nil reader accepted")
	}
	if _, e = ParseRequest(failingReader{}); e == nil || e.Error() != "invalid_request" {
		t.Fatal("reader error not sanitized")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("sensitive transport detail") }

var _ io.Reader = failingReader{}

func TestRequestLimitsAndRedaction(t *testing.T) {
	f := requestFixture()
	f["full_name"] = strings.Repeat("அ", 200)
	f["password"] = strings.Repeat("p", 1024)
	r, e := ParseRequest(strings.NewReader(fixtureJSON(t, f)))
	if e != nil {
		t.Fatal("field boundary rejected")
	}
	var logs bytes.Buffer
	slog.New(slog.NewJSONHandler(&logs, nil)).Info("test", "request", r)
	b, _ := json.Marshal(r)
	for _, value := range []string{fmt.Sprintf("%v %+v %#v", r, r, r), string(b), logs.String()} {
		if !strings.Contains(value, "[REDACTED]") || strings.Contains(value, r.TransientPassword()) || strings.Contains(value, r.Email) {
			t.Fatal("sensitive request exposed")
		}
	}
	f["email"] = "PRIVATE-SUBMITTED-VALUE"
	_, e = ParseRequest(strings.NewReader(fixtureJSON(t, f)))
	b, _ = json.Marshal(e)
	if strings.Contains(string(b), "PRIVATE") || strings.Contains(e.Error(), "PRIVATE") {
		t.Fatal("error echoed input")
	}
}

func TestNewAccountEmailPolicy(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"A.B+Tag@Example.TEST", "a.b+tag@example.test"},
		{"AB@Example.TEST", "ab@example.test"},
		{"a@xn--bcher-kva.example", "a@xn--bcher-kva.example"},
		{"a\r\n@example.test", ""},
		{"a@ex ample.test", ""},
		{"\"quoted\"@example.test", ""},
		{"a@example.test.", ""},
		{"a@[127.0.0.1]", ""},
	} {
		f := requestFixture()
		f["email"] = tc.input
		r, e := ParseRequest(strings.NewReader(fixtureJSON(t, f)))
		if tc.want == "" {
			if e == nil || e.Fields["email"] != "invalid_format" {
				t.Fatal("unsupported email accepted")
			}
		} else if e != nil || r.Email != tc.want {
			t.Fatal("email normalization mismatch")
		}
	}
}
