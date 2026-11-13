package login

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestStrictLoginInput(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`null`,
		`[]`,
		`{"email":"a@example.test","password":null}`,
		`{"email":"a@example.test","password":"x","password":"y"}`,
		`{"email":"a@example.test","password":"x","client_id":"admin"}`,
		`{"email":"a@example.test","password":"\ud800"}`,
		`{"email":"a@example.test","password":"x"} {}`,
		`{"email":"not-email","password":"x"}`,
	} {
		if _, e := Parse(strings.NewReader(body)); e == nil {
			t.Fatal("invalid body accepted")
		}
	}

	request, e := Parse(
		strings.NewReader(`{"email":"Citizen@Example.Test","password":"  தமிழ்🔒  "}`),
	)
	if e != nil || request.email != "citizen@example.test" || request.password != "  தமிழ்🔒  " {
		t.Fatal("credential normalization corrupted data")
	}

	if strings.Contains(fmt.Sprintf("%+v", request), "தமிழ்") {
		t.Fatal("request redaction failed")
	}

	if _, e = Parse(strings.NewReader(strings.Repeat(" ", MaxRequestBytes+1))); !errors.Is(e, ErrTooLarge) {
		t.Fatal("size limit failed")
	}
}
