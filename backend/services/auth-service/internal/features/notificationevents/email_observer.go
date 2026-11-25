package notificationevents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

var ErrEmailObservation = errors.New("email verification observation unavailable")

type EmailCheck struct {
	Issuer, Subject, OperationID, LeaseToken string
}

func (EmailCheck) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED verification check]")
}

func (EmailCheck) LogValue() slog.Value {
	return slog.StringValue("[REDACTED verification check]")
}

func (EmailCheck) MarshalJSON() ([]byte, error) {
	return []byte("\"[REDACTED verification check]\""), nil
}

type EmailObservationStore interface {
	ClaimEmailCheck(context.Context) (EmailCheck, bool, error)
	FinishEmailCheck(context.Context, EmailCheck, bool) error
}

type OwnedEmailState interface {
	ReadOwned(context.Context, string, string) (registration.Identity, error)
}

type EmailObserver struct {
	store  EmailObservationStore
	source OwnedEmailState
}

func NewEmailObserver(store EmailObservationStore, source OwnedEmailState) (*EmailObserver, error) {
	if store == nil || source == nil {
		return nil, ErrConfiguration
	}
	return &EmailObserver{store: store, source: source}, nil
}

// CheckOne observes owned provider state, never a request to send verification mail.

func (observer *EmailObserver) CheckOne(ctx context.Context) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	check, found, err := observer.store.ClaimEmailCheck(ctx)
	if err != nil || !found {
		return found, err
	}
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	identity, providerError := observer.source.ReadOwned(probe, check.Subject, check.OperationID)
	verified := providerError == nil && identity.Subject == check.Subject &&
		identity.Enabled && identity.EmailVerified
	if err := observer.store.FinishEmailCheck(ctx, check, verified); err != nil {
		return true, err
	}
	if providerError != nil || identity.Subject != check.Subject {
		return true, ErrEmailObservation
	}
	return true, nil
}

// Limit polling to one check per second per instance, with persistent per-user delay.

func (observer *EmailObserver) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		if _, err := observer.CheckOne(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("email verification observation unavailable")
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return ctx.Err()
}
