package profile

import (
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

func TestCitizenGenderChoices(t *testing.T) {
	rules := CitizenFieldRules{}
	for _, value := range []string{"", "male", "female"} {
		if rules.ValidateGender(value) != nil {
			t.Fatal("approved gender rejected")
		}
	}
	for _, value := range []string{"Male", "Female", "other", "unknown", " male", "female ", "MALE"} {
		if rules.ValidateGender(value) == nil {
			t.Fatal("unsupported gender accepted")
		}
	}
}

func TestNICDateOfBirthCorrespondence(t *testing.T) {
	rules := CitizenFieldRules{}
	for _, tc := range []struct {
		name, nic, dob string
		valid          bool
	}{
		{"old-march", "900610001V", "1990-03-01", true},
		{"old-female-x", "905610001x", "1990-03-01", true},
		{"new-march", "199006100001", "1990-03-01", true},
		{"new-female", "199056100001", "1990-03-01", true},
		{"old-january", "900010001v", "1990-01-01", true},
		{"new-february28", "199005900001", "1990-02-28", true},
		{"old-leap-day", "000600001V", "2000-02-29", true},
		{"new-leap-day", "200006000001", "2000-02-29", true},
		{"female-leap-day", "200056000001", "2000-02-29", true},
		{"non-leap-december31", "199036600001", "1990-12-31", true},
		{"leap-december31", "200036600001", "2000-12-31", true},
		{"optional-empty", "123456789v", "", true},
		{"wrong-year", "199006100001", "1991-03-01", false},
		{"wrong-old-year", "900610001V", "1991-03-01", false},
		{"wrong-day", "199006100001", "1990-03-02", false},
		{"normal-calendar-off-by-one", "199006000001", "1990-03-01", false},
		{"impossible-leap-day", "199006000001", "1990-02-29", false},
		{"century-not-leap", "190006000001", "1900-02-29", false},
		{"zero-day", "199000000001", "1990-01-01", false},
		{"gap-day", "199050000001", "1990-01-01", false},
		{"out-of-range", "199086700001", "1990-12-31", false},
		{"invalid-format", "1990bad00001", "1990-03-01", false},
		{"future", "209006100001", "2090-03-01", false},
		{"missing-nic", "", "1990-03-01", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nic, _ := registration.ParseNIC(tc.nic)
			err := rules.ValidateDOB(nic, tc.dob)
			if (err == nil) != tc.valid {
				t.Fatal("unexpected NIC/DOB correspondence result")
			}
		})
	}
}
