// Package notificationcontract defines account events shared by Go services.

package notificationcontract

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const MaxEventBytes = 16 << 10

const Version = 1

const (
	LoginSucceeded  = "citizen.login.succeeded"
	PasswordChanged = "citizen.password.changed"
	EmailVerified   = "citizen.email.verified"
)

type Priority string

const (
	High   Priority = "high"
	Normal Priority = "normal"
)

var ErrInvalid = errors.New("invalid notification event")

var ErrTooLarge = errors.New("notification event exceeds size limit")

var eventID = regexp.MustCompile(
	"^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
)

var subjectID = regexp.MustCompile("^[A-Za-z0-9_-]{1,128}$")

// Event contains identity references only; message wording comes from templates.

type Event struct {
	ID         string    `json:"event_id"`
	Type       string    `json:"event_type"`
	Version    int       `json:"version"`
	Issuer     string    `json:"issuer"`
	Subject    string    `json:"subject"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       struct{}  `json:"data"`
}

// PriorityFor derives scheduling priority from the trusted event-type allowlist.

func PriorityFor(eventType string) (Priority, error) {
	switch eventType {
	case LoginSucceeded, PasswordChanged:
		return High, nil
	case EmailVerified:
		return Normal, nil
	default:
		return "", ErrInvalid
	}
}

// ValidateIssuer checks the configured identity-provider address.

func ValidateIssuer(trustedIssuer string) error {

	issuer, err := url.Parse(trustedIssuer)
	if err != nil || len(trustedIssuer) > 2048 || issuer.Scheme != "https" ||
		issuer.Hostname() == "" || issuer.User != nil || issuer.RawQuery != "" ||
		issuer.ForceQuery || issuer.Fragment != "" ||
		strings.ContainsAny(trustedIssuer, "\\ \t\r\n") {
		return ErrInvalid
	}
	return nil
}

// Validate checks shape and issuer matching, not transport authenticity.

func (e Event) Validate(trustedIssuer string) error {
	if err := ValidateIssuer(trustedIssuer); err != nil {
		return err
	}
	if e.Issuer != trustedIssuer || !eventID.MatchString(e.ID) ||
		!subjectID.MatchString(e.Subject) || e.Version != Version ||
		e.OccurredAt.IsZero() || e.OccurredAt.Year() < 1970 {
		return ErrInvalid
	}
	_, err := PriorityFor(e.Type)
	return err
}

// Format prevents accidental identity disclosure through generic formatting.

func (Event) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED notification event]")
}

// LogValue keeps entire events out of structured logs.

func (Event) LogValue() slog.Value {
	return slog.StringValue("[REDACTED notification event]")
}
