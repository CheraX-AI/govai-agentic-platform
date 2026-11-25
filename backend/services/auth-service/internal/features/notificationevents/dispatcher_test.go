package notificationevents

import (
	"context"
	"errors"
	"testing"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
)

type dispatchFixture struct {
	job          Job
	found        bool
	publishError error
	finishError  error
	finished     []Outcome
	published    int
}

func (fixture *dispatchFixture) Claim(context.Context, contract.Priority) (Job, bool, error) {
	return fixture.job, fixture.found, nil
}

func (fixture *dispatchFixture) Finish(_ context.Context, _ Job, outcome Outcome) error {
	fixture.finished = append(fixture.finished, outcome)
	return fixture.finishError
}

func (fixture *dispatchFixture) Publish(context.Context, contract.Event) error {
	fixture.published++
	return fixture.publishError
}

func TestDispatchOutcomes(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		outcome Outcome
	}{
		{"confirmed", nil, Confirmed},
		{"unavailable", ErrUnavailable, Unavailable},
		{"unroutable", ErrUnroutable, Unroutable},
		{"lost confirm", ErrUnknown, Unknown},
		{"unexpected failure", errors.New("synthetic failure"), Unknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := &dispatchFixture{found: true, publishError: test.err}
			dispatcher, _ := New(fixture, fixture)
			worked, err := dispatcher.DispatchOne(context.Background(), contract.High)
			if !worked || err != test.err || fixture.published != 1 ||
				len(fixture.finished) != 1 || fixture.finished[0] != test.outcome {
				t.Fatal("incorrect publication outcome")
			}
		})
	}
}

func TestDispatchBoundaries(t *testing.T) {
	fixture := &dispatchFixture{}
	dispatcher, _ := New(fixture, fixture)
	worked, err := dispatcher.DispatchOne(context.Background(), contract.Normal)
	if worked || err != nil || fixture.published != 0 {
		t.Fatal("empty lane published an event")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := dispatcher.DispatchOne(ctx, contract.High); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, err := dispatcher.DispatchOne(context.Background(), "unexpected"); err == nil {
		t.Fatal("invalid lane accepted")
	}
	fixture.found = true
	fixture.finishError = errors.New("synthetic database failure")
	_, err = dispatcher.DispatchOne(context.Background(), contract.High)
	if err != fixture.finishError || fixture.published != 1 {
		t.Fatal("failed database completion was hidden")
	}
	if _, err := New(nil, fixture); err == nil {
		t.Fatal("missing storage accepted")
	}
	if err := dispatcher.Run(ctx, contract.High); !errors.Is(err, context.Canceled) {
		t.Fatal("worker did not stop")
	}
}
