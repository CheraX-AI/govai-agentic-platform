package delivery

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
)

var (
	ErrConfiguration = errors.New("invalid notification SMTP configuration")
	ErrMessage       = errors.New("invalid notification email")
	ErrSubmission    = errors.New("notification email submission failed")
	ErrUnknown       = errors.New("notification email submission outcome is unknown")
)

type private struct{}

// Format hides credentials and recipient/content fields from generic formatting.

func (private) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED SMTP data]")
}

// LogValue hides SMTP input structs from structured logging.

func (private) LogValue() slog.Value {
	return slog.StringValue("[REDACTED SMTP data]")
}

// MarshalJSON prevents accidental serialization of adapter input.

func (private) MarshalJSON() ([]byte, error) {
	return []byte("\"[REDACTED SMTP data]\""), nil
}

type Config struct {
	private
	Host        string
	Port        int
	Username    string
	Password    string
	From        string
	TLSMode     string
	Timeout     time.Duration
	Development bool
	RootCAs     *x509.CertPool
}

// Message is rendered text and a trusted recipient, supplied by the delivery worker.

type Message struct {
	private
	ID        string
	Recipient string
	Subject   string
	Body      string
}

// Result records submission evidence, never a guarantee of recipient inbox delivery.

type Result struct {
	Status    string
	Code      string
	Retryable bool
}
