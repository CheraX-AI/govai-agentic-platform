package delivery

import (
	"errors"
	"time"
)

const (
	MaxAttempts = 3
	RetryDelay  = 30 * time.Second
)

var ErrOutcome = errors.New("invalid email delivery outcome")

type Action string

const (
	Submitted Action = "submitted"
	Retry     Action = "retry"
	Dead      Action = "dead"
	Review    Action = "review"
)

// Decision contains only safe metadata for the durable delivery worker.
// Dead requires a persisted dead-letter intent, not deletion of the inbox row.

type Decision struct {
	Action        Action
	Code          string
	NextAttemptAt time.Time
}

// Decide evaluates an attempt already reserved in PostgreSQL before SMTP starts.
// Attempts includes this submission: three total attempts, not three extra retries.
// Invalid evidence never authorizes another send or a successful completion.

func Decide(attempts int, now time.Time, result Result, sendErr error) (Decision, error) {
	if attempts < 1 || attempts > MaxAttempts || now.IsZero() {
		return Decision{}, ErrOutcome
	}
	if result.Status == "submitted" && result.Code == "smtp_accepted" &&
		!result.Retryable && sendErr == nil {
		return Decision{Action: Submitted, Code: result.Code}, nil
	}
	if result.Status == "unknown" && result.Code == "submission_unknown" &&
		!result.Retryable && errors.Is(sendErr, ErrUnknown) {
		return Decision{Action: Review, Code: result.Code}, nil
	}
	if result.Status != "failed" || !validFailure(result, sendErr) {
		return Decision{}, ErrOutcome
	}
	if result.Code == "canceled" {
		return Decision{Action: Review, Code: result.Code}, nil
	}
	if !result.Retryable || attempts == MaxAttempts {
		return Decision{Action: Dead, Code: result.Code}, nil
	}
	return Decision{
		Action:        Retry,
		Code:          result.Code,
		NextAttemptAt: now.UTC().Add(RetryDelay),
	}, nil
}

func validFailure(result Result, err error) bool {
	if result.Code == "invalid_message" {
		return !result.Retryable && errors.Is(err, ErrMessage)
	}
	if !errors.Is(err, ErrSubmission) {
		return false
	}
	switch result.Code {
	case "smtp_4xx", "connection_failed", "greeting_failed", "sender_rejected",
		"recipient_rejected", "data_rejected", "timeout":
		return result.Retryable
	case "smtp_5xx", "authentication_failed", "tls_failed", "tls_required", "canceled":
		return !result.Retryable
	default:
		return false
	}
}
