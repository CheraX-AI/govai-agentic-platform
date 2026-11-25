package delivery

import (
	"errors"
	"testing"
	"time"
)

func TestRetryBudget(t *testing.T) {
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.FixedZone("local", 19800))
	result := Result{Status: "failed", Code: "smtp_4xx", Retryable: true}
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		decision, err := Decide(attempt, now, result, ErrSubmission)
		if err != nil {
			t.Fatal(err)
		}
		if attempt == MaxAttempts {
			if decision.Action != Dead || !decision.NextAttemptAt.IsZero() {
				t.Fatal("exhausted attempt must stop sending")
			}
		} else if decision.Action != Retry ||
			!decision.NextAttemptAt.Equal(now.Add(RetryDelay)) ||
			decision.NextAttemptAt.Location() != time.UTC {
			t.Fatal("temporary failure must schedule a delayed UTC retry")
		}
	}
}

func TestTerminalOutcomes(t *testing.T) {
	cases := []struct {
		name   string
		result Result
		err    error
		action Action
	}{
		{"accepted", Result{"submitted", "smtp_accepted", false}, nil, Submitted},
		{"permanent", Result{"failed", "smtp_5xx", false}, ErrSubmission, Dead},
		{"bad_auth", Result{"failed", "authentication_failed", false}, ErrSubmission, Dead},
		{"unsafe_tls", Result{"failed", "tls_failed", false}, ErrSubmission, Dead},
		{"bad_message", Result{"failed", "invalid_message", false}, ErrMessage, Dead},
		{"lost_acceptance", Result{"unknown", "submission_unknown", false}, ErrUnknown, Review},
		{"cancellation", Result{"failed", "canceled", false}, ErrSubmission, Review},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			for attempt := 1; attempt <= MaxAttempts; attempt++ {
				decision, err := Decide(attempt, time.Now(), test.result, test.err)
				if err != nil || decision.Action != test.action ||
					!decision.NextAttemptAt.IsZero() {
					t.Fatal("terminal outcome must not schedule another submission")
				}
			}
		})
	}
}

func TestInvalidOutcomeCannotAuthorizeSend(t *testing.T) {
	cases := []struct {
		result Result
		err    error
	}{
		{Result{}, nil},
		{Result{"submitted", "smtp_accepted", false}, ErrSubmission},
		{Result{"submitted", "smtp_accepted", true}, nil},
		{Result{"unknown", "submission_unknown", true}, ErrUnknown},
		{Result{"unknown", "submission_unknown", false}, nil},
		{Result{"failed", "smtp_4xx", true}, nil},
		{Result{"failed", "smtp_5xx", true}, ErrSubmission},
		{Result{"failed", "smtp_4xx", false}, ErrSubmission},
		{Result{"failed", "private provider reply", true}, ErrSubmission},
		{Result{"failed", "invalid_message", false}, ErrSubmission},
	}
	for _, test := range cases {
		decision, err := Decide(1, time.Now(), test.result, test.err)
		if !errors.Is(err, ErrOutcome) || decision != (Decision{}) {
			t.Fatal("invalid or contradictory evidence must fail closed")
		}
	}
	for _, attempt := range []int{-1, 0, MaxAttempts + 1} {
		_, err := Decide(attempt, time.Now(), Result{}, nil)
		if !errors.Is(err, ErrOutcome) {
			t.Fatal("invalid attempt count accepted")
		}
	}
	if _, err := Decide(1, time.Time{}, Result{}, nil); !errors.Is(err, ErrOutcome) {
		t.Fatal("missing attempt timestamp accepted")
	}
}
