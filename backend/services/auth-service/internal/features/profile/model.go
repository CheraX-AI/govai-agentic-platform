// Package profile defines application profile data, not identity credentials.
package profile

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("invalid citizen profile details")
	ErrNotFound = errors.New("citizen profile not found")
	ErrConflict = errors.New("citizen profile revision conflict")
)

// Explicit HTTP DTOs must map approved fields rather than serialize these values.
type privateData struct{}

func (privateData) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (privateData) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (privateData) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

type Address struct {
	privateData
	Line1 string
	Line2 string
	City  string
}

func (a Address) Empty() bool { return a.Line1 == "" && a.Line2 == "" && a.City == "" }

// EditableDetails is a full replacement of permitted fields. Empty strings clear
// optional fields. It cannot carry NIC, registered name/mobile, email or a photo
// key. Only the separate, server-owned photo operation may change a photo key.
type EditableDetails struct {
	privateData
	NameSI      string
	NameTA      string
	DateOfBirth string // empty or an ISO calendar date YYYY-MM-DD
	Gender      string
	Address     Address
}

type Details struct {
	privateData
	Fields         EditableDetails
	PhotoObjectKey string
	Revision       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Validate enforces storage bounds, UTF-8 and calendar validity. Product-specific
// gender values, required fields and NIC/DOB consistency belong to the agreed
// GA-253 contract; this package does not invent those rules.
func (d EditableDetails) Validate(now time.Time) error {
	for _, field := range []struct {
		value string
		max   int
	}{
		{d.NameSI, 200}, {d.NameTA, 200}, {d.Gender, 64},
		{d.Address.Line1, 200}, {d.Address.Line2, 200}, {d.Address.City, 100},
	} {
		if !utf8.ValidString(field.value) || utf8.RuneCountInString(field.value) > field.max {
			return ErrInvalid
		}
		for _, r := range field.value {
			if unicode.IsControl(r) {
				return ErrInvalid
			}
		}
	}
	if d.DateOfBirth != "" {
		date, err := time.Parse("2006-01-02", d.DateOfBirth)
		if err != nil || date.Year() < 1 || d.DateOfBirth > now.UTC().Format("2006-01-02") {
			return ErrInvalid
		}
	}
	return nil
}
