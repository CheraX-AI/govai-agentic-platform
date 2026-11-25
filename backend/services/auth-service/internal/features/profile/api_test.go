package profile

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

const validUpdate = `{"revision":0,"name_si":"","name_ta":"","date_of_birth":"","gender":"","address":{"line1":"Synthetic road","line2":"","city":"Synthetic city"}}`

func TestParseUpdate(t *testing.T) {
	if _, err := ParseUpdate(strings.NewReader(validUpdate)); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"nic", "full_name", "mobile_number", "email", "subject", "issuer", "role", "photo_object_key", "photo_url", "Name_si"} {
		t.Run("forged-"+field, func(t *testing.T) {
			body := strings.TrimSuffix(validUpdate, "}") + `,"` + field + `":"forged"}`
			if _, err := ParseUpdate(strings.NewReader(body)); err == nil {
				t.Fatal("forged field accepted")
			}
		})
	}
	for name, body := range map[string]string{
		"duplicate":        strings.Replace(validUpdate, `"revision":0`, `"revision":0,"revision":1`, 1),
		"missing":          `{"revision":0}`,
		"null":             strings.Replace(validUpdate, `"name_si":""`, `"name_si":null`, 1),
		"nested-forgery":   strings.Replace(validUpdate, `"line2":""`, `"line2":"","subject":"other"`, 1),
		"nested-duplicate": strings.Replace(validUpdate, `"line2":""`, `"line2":"","line2":"other"`, 1),
		"bad-date":         strings.Replace(validUpdate, `"date_of_birth":""`, `"date_of_birth":"2001-02-29"`, 1),
		"future-date":      strings.Replace(validUpdate, `"date_of_birth":""`, `"date_of_birth":"9999-01-01"`, 1),
		"negative":         strings.Replace(validUpdate, `"revision":0`, `"revision":-1`, 1),
		"float":            strings.Replace(validUpdate, `"revision":0`, `"revision":0.0`, 1),
		"exponent":         strings.Replace(validUpdate, `"revision":0`, `"revision":1e0`, 1),
		"unsafe-integer":   strings.Replace(validUpdate, `"revision":0`, `"revision":9007199254740992`, 1),
		"surrogate":        strings.Replace(validUpdate, `"name_si":""`, `"name_si":"\ud800"`, 1),
		"invalid-utf8":     strings.Replace(validUpdate, `"name_si":""`, "\"name_si\":\""+string([]byte{0xff})+"\"", 1),
		"trailing":         validUpdate + `{}`,
		"array":            `[]`,
		"oversized":        strings.Repeat("x", MaxUpdateBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseUpdate(strings.NewReader(body)); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

type apiStore struct {
	registered registration.ProfileData
	details    Details
	owner      Caller
	saves      int
}

func (s *apiStore) RegisteredProfile(_ context.Context, issuer, subject string) (registration.ProfileData, error) {
	if issuer != s.owner.Issuer || subject != s.owner.Subject {
		return registration.ProfileData{}, ErrNotFound
	}
	return s.registered, nil
}
func (s *apiStore) CitizenDetails(_ context.Context, issuer, subject string) (Details, error) {
	if issuer != s.owner.Issuer || subject != s.owner.Subject {
		return Details{}, ErrNotFound
	}
	return s.details, nil
}
func (s *apiStore) SaveCitizenDetails(_ context.Context, issuer, subject string, revision int64, fields EditableDetails) (Details, error) {
	if issuer != s.owner.Issuer || subject != s.owner.Subject {
		return Details{}, ErrNotFound
	}
	if revision != s.details.Revision {
		return Details{}, ErrConflict
	}
	s.saves++
	s.details.Fields = fields
	s.details.Revision++
	return s.details, nil
}

// A synthetic policy verifies the integration seam; these are test values,
// not a claimed GovAI NIC algorithm or approved production gender list.
type syntheticRules struct{}

func (syntheticRules) ValidateDOB(_ registration.NIC, date string) error {
	if date != "2000-02-29" {
		return ErrInvalid
	}
	return nil
}
func (syntheticRules) ValidateGender(value string) error {
	if value != "synthetic-approved" {
		return ErrInvalid
	}
	return nil
}

func TestProfileServiceOwnershipAndRules(t *testing.T) {
	caller := Caller{Issuer: "https://issuer.test", Subject: "citizen-a", Email: "synthetic@example.test"}
	nic, _ := registration.ParseNIC("123456789v")
	store := &apiStore{owner: caller, registered: registration.ProfileData{FullName: "Registered A", NIC: nic, MobileNumber: "+94770000000"}}
	service, _ := NewService(store, nil)
	request, _ := ParseUpdate(strings.NewReader(validUpdate))
	for _, wrong := range []Caller{{Issuer: caller.Issuer, Subject: "citizen-b"}, {Issuer: "https://other.test", Subject: caller.Subject}, {}} {
		if _, err := service.Read(context.Background(), wrong); !errors.Is(err, ErrNotFound) {
			t.Fatal("other owner read accepted")
		}
		if _, err := service.Update(context.Background(), wrong, request); !errors.Is(err, ErrNotFound) {
			t.Fatal("other owner save accepted")
		}
	}
	saved, err := service.Update(context.Background(), caller, request)
	if err != nil || saved.Email != caller.Email || saved.Registered.FullName != "Registered A" || saved.Details.Revision != 1 {
		t.Fatal("profile update failed")
	}
	if _, err := service.Update(context.Background(), caller, request); !errors.Is(err, ErrConflict) {
		t.Fatal("stale save accepted")
	}
	request.Revision = 1
	request.Fields.DateOfBirth = "2000-02-29"
	if _, err := service.Update(context.Background(), caller, request); !errors.Is(err, ErrRulesUnavailable) {
		t.Fatal("unapproved DOB contract silently accepted")
	}
	request.Fields.DateOfBirth = ""
	request.Fields.Gender = "unknown"
	if _, err := service.Update(context.Background(), caller, request); !errors.Is(err, ErrRulesUnavailable) {
		t.Fatal("unapproved gender silently accepted")
	}
	if store.saves != 1 {
		t.Fatal("failed validation modified database")
	}
	service, _ = NewService(store, syntheticRules{})
	request.Fields.DateOfBirth = ""
	request.Fields.Gender = ""
	if _, err := service.Update(context.Background(), caller, request); !errors.Is(err, ErrInvalid) {
		t.Fatal("installed required-field rule ignored")
	}
	request.Fields.DateOfBirth = "1999-01-01"
	request.Fields.Gender = "synthetic-approved"
	if _, err := service.Update(context.Background(), caller, request); !errors.Is(err, ErrInvalid) {
		t.Fatal("DOB contract rejection ignored")
	}
	request.Fields.DateOfBirth = "2000-02-29"
	request.Fields.Gender = "unknown"
	if _, err := service.Update(context.Background(), caller, request); !errors.Is(err, ErrInvalid) {
		t.Fatal("gender rejection ignored")
	}
	request.Fields.Gender = "synthetic-approved"
	if _, err := service.Update(context.Background(), caller, request); err != nil {
		t.Fatal("approved synthetic policy failed", err)
	}
}
