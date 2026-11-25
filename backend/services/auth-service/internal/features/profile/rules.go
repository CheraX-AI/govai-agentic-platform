package profile

import (
	"fmt"
	"strconv"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

// CitizenFieldRules implements the approved male/female choices and NIC/DOB
// correspondence. This is not evidence that an NIC was issued or is authentic.
type CitizenFieldRules struct{}

var ErrDOBMismatch = fmt.Errorf("%w: date of birth does not match NIC", ErrInvalid)
var ErrGenderChoice = fmt.Errorf("%w: unsupported gender choice", ErrInvalid)

func (CitizenFieldRules) ValidateGender(value string) error {
	// Existing optional values remain clearable; the UI labels are Male/Female.
	if value == "" || value == "male" || value == "female" {
		return nil
	}
	return ErrGenderChoice
}

func (CitizenFieldRules) ValidateDOB(nic registration.NIC, value string) error {
	if value == "" {
		return nil
	}
	if (EditableDetails{DateOfBirth: value}).Validate(time.Now()) != nil {
		return ErrInvalid
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return ErrInvalid
	}
	number := nic.NormalizedNumber()
	if _, err = registration.ParseNIC(number); err != nil {
		return ErrDOBMismatch
	}
	yearDigits := 4
	if len(number) == 10 {
		yearDigits = 2
	}
	year, err := strconv.Atoi(number[:yearDigits])
	if err != nil {
		return ErrInvalid
	}
	expectedYear := date.Year()
	if yearDigits == 2 {
		// Old NICs omit the century. Compare the supplied year's last two
		// digits; do not invent a 19xx/20xx identity mapping or conversion.
		expectedYear %= 100
	}
	if year != expectedYear {
		return ErrDOBMismatch
	}
	day, err := strconv.Atoi(number[yearDigits : yearDigits+3])
	if err != nil {
		return ErrInvalid
	}
	if day >= 501 && day <= 866 {
		day -= 500
	}
	if day < 1 || day > 366 {
		return ErrDOBMismatch
	}
	// NIC day codes reserve a February 29 slot even in non-leap years.
	// Parse above still rejects a real DOB of February 29 in a non-leap year.
	expectedDay := time.Date(2000, date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).YearDay()
	if day != expectedDay {
		return ErrDOBMismatch
	}
	return nil
}
