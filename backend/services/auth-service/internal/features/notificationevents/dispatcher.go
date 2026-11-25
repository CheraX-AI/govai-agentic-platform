package notificationevents

import (
	"context"
	"errors"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
)

var (
	ErrConfiguration = errors.New("notification event dispatcher is not configured")
	ErrUnavailable   = errors.New("notification event broker unavailable")
	ErrUnknown       = errors.New("notification event publication unconfirmed")
	ErrUnroutable    = errors.New("notification event has no broker route")
	ErrInvalid       = errors.New("invalid notification publication")
)

type Outcome string

const (
	Confirmed   Outcome = "confirmed"
	Unavailable Outcome = "broker_unavailable"
	Unknown     Outcome = "publish_unknown"
	Unroutable  Outcome = "unroutable"
)

type Job struct {
	Event      contract.Event
	LeaseToken string
}

// Store leases one due event and conditionally records its publication outcome.

type Store interface {
	Claim(context.Context, contract.Priority) (Job, bool, error)
	Finish(context.Context, Job, Outcome) error
}

// Publisher returns nil only for a positive broker confirm without a return.

type Publisher interface {
	Publish(context.Context, contract.Event) error
}

type Dispatcher struct {
	store     Store
	publisher Publisher
}

func New(store Store, publisher Publisher) (*Dispatcher, error) {
	if store == nil || publisher == nil {
		return nil, ErrConfiguration
	}
	return &Dispatcher{store: store, publisher: publisher}, nil
}

// DispatchOne never deletes an event. Failure keeps it pending with backoff.

func (dispatcher *Dispatcher) DispatchOne(
	ctx context.Context, priority contract.Priority,
) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if priority != contract.High && priority != contract.Normal {
		return false, ErrConfiguration
	}
	job, found, err := dispatcher.store.Claim(ctx, priority)
	if err != nil || !found {
		return found, err
	}
	publicationError := dispatcher.publisher.Publish(ctx, job.Event)
	outcome := Confirmed
	if publicationError != nil {
		outcome = Unknown
		if errors.Is(publicationError, ErrUnavailable) {
			outcome = Unavailable
		} else if errors.Is(publicationError, ErrUnroutable) {
			outcome = Unroutable
		}
	}
	if err := dispatcher.store.Finish(ctx, job, outcome); err != nil {
		return true, err
	}
	return true, publicationError
}

// Run serves one priority lane. Run separate lanes to reserve security capacity.
// The application must join these workers during shutdown before closing its pool.

func (dispatcher *Dispatcher) Run(ctx context.Context, priority contract.Priority) error {
	if priority != contract.High && priority != contract.Normal {
		return ErrConfiguration
	}
	for ctx.Err() == nil {
		worked, err := dispatcher.DispatchOne(ctx, priority)
		if worked && err == nil {
			continue
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return ctx.Err()
}
