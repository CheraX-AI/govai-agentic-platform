package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
)

type testStore struct {
	records       map[string]Notification
	calls         int
	fail          error
	invalidResult bool
	afterCommit   func()
}

func (s *testStore) Save(ctx context.Context, notification Notification) (SaveResult, error) {
	s.calls++
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if s.fail != nil {
		return 0, s.fail
	}
	if s.invalidResult {
		return 0, nil
	}
	if original, exists := s.records[notification.Event.ID]; exists {
		old, next := original.Event, notification.Event
		if old.Issuer != next.Issuer || old.Subject != next.Subject ||
			old.Type != next.Type || !old.OccurredAt.Equal(next.OccurredAt) {
			return 0, ErrConflict
		}
		return Duplicate, nil
	}
	s.records[notification.Event.ID] = notification
	if s.afterCommit != nil {
		s.afterCommit()
	}
	return Saved, nil
}

type testRenderer struct {
	calls   int
	content Content
	fail    error
}

func (r *testRenderer) Render(contract.Event) (Content, error) {
	r.calls++
	return r.content, r.fail
}

func fixture(t *testing.T) (*Processor, *testStore, *testRenderer, Delivery) {
	t.Helper()
	store := &testStore{records: map[string]Notification{}}
	renderer := &testRenderer{content: Content{
		Title: "Synthetic alert", Body: "Synthetic test content.", Locale: "en",
	}}
	processor, err := New("https://identity.example/realms/govai", store, renderer)
	if err != nil {
		t.Fatal("processor configuration failed")
	}
	event := contract.Event{
		ID:         "123e4567-e89b-42d3-a456-426614174000",
		Type:       contract.LoginSucceeded,
		Version:    contract.Version,
		Issuer:     "https://identity.example/realms/govai",
		Subject:    "synthetic-subject",
		OccurredAt: time.Date(2026, 11, 22, 5, 0, 0, 0, time.UTC),
	}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal("synthetic encoding failed")
	}
	delivery := Delivery{
		Body: body, RoutingKey: event.Type, Queue: "notifications.security.v1",
		Acknowledge: func() error { return nil },
	}
	return processor, store, renderer, delivery
}

// TestSaveBeforeAck verifies the ordering required of the future persistent adapter.

func TestSaveBeforeAck(t *testing.T) {
	processor, store, _, delivery := fixture(t)
	acknowledgments := 0
	delivery.Acknowledge = func() error {
		if len(store.records) != 1 {
			t.Fatal("delivery acknowledged before storage committed")
		}
		acknowledgments++
		return nil
	}
	result, err := processor.Handle(context.Background(), delivery)
	if err != nil || result != Saved || acknowledgments != 1 {
		t.Fatal("successful delivery did not save then acknowledge once")
	}
	for _, notification := range store.records {
		if notification.Priority != contract.High || notification.Content.Locale != "en" {
			t.Fatal("incorrect notification priority or locale")
		}
	}
}

// TestLostAckRedelivery models commit-before-crash without a second inbox insert.

func TestLostAckRedelivery(t *testing.T) {
	processor, store, _, delivery := fixture(t)
	delivery.Acknowledge = func() error { return errors.New("synthetic lost ACK") }
	result, err := processor.Handle(context.Background(), delivery)
	if result != Saved || !errors.Is(err, ErrAcknowledgment) || len(store.records) != 1 {
		t.Fatal("lost acknowledgment did not preserve saved notification")
	}
	delivery.Acknowledge = func() error { return nil }
	result, err = processor.Handle(context.Background(), delivery)
	if err != nil || result != Duplicate || len(store.records) != 1 {
		t.Fatal("redelivery created a second notification or was not acknowledged")
	}
}

// TestFailuresNeverAck covers storage, templates, cancellation and malformed events.

func TestFailuresNeverAck(t *testing.T) {
	cases := []string{
		"storage", "invalid store result", "conflict", "template", "empty content",
		"oversized content", "invalid locale", "invalid utf8", "invalid payload",
		"wrong queue", "cancel before", "cancel after commit", "missing acknowledgment",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			processor, store, renderer, delivery := fixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			acknowledgments := 0
			delivery.Acknowledge = func() error {
				acknowledgments++
				return nil
			}
			switch name {
			case "storage":
				store.fail = errors.New("synthetic secret database error")
			case "invalid store result":
				store.invalidResult = true
			case "conflict":
				store.fail = ErrConflict
			case "template":
				renderer.fail = errors.New("synthetic secret template error")
			case "empty content":
				renderer.content.Title = ""
			case "oversized content":
				renderer.content.Body = strings.Repeat("x", 8193)
			case "invalid locale":
				renderer.content.Locale = "si"
			case "invalid utf8":
				renderer.content.Body = string([]byte{0xff})
			case "invalid payload":
				delivery.Body = []byte("invalid")
			case "wrong queue":
				delivery.Queue = "notifications.account.v1"
			case "cancel before":
				cancel()
			case "cancel after commit":
				store.afterCommit = cancel
			case "missing acknowledgment":
				delivery.Acknowledge = nil
			}
			_, err := processor.Handle(ctx, delivery)
			if err == nil || acknowledgments != 0 {
				t.Fatal("failure was acknowledged or reported as successful")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("internal failure detail escaped")
			}
			if name != "cancel after commit" && len(store.records) != 0 {
				t.Fatal("failed processing inserted a record")
			}
			if name == "cancel after commit" && len(store.records) != 1 {
				t.Fatal("cancellation removed a committed record")
			}
			if (name == "invalid payload" || name == "wrong queue" ||
				name == "cancel before" || name == "missing acknowledgment") &&
				(store.calls != 0 || renderer.calls != 0) {
				t.Fatal("invalid delivery reached downstream dependencies")
			}
		})
	}
}

// TestConflictingRedelivery rejects event-ID reuse for a different owner.

func TestConflictingRedelivery(t *testing.T) {
	processor, store, _, delivery := fixture(t)
	if _, err := processor.Handle(context.Background(), delivery); err != nil {
		t.Fatal("initial save failed")
	}
	delivery.Body = []byte(strings.Replace(
		string(delivery.Body), "synthetic-subject", "another-subject", 1,
	))
	delivery.Acknowledge = func() error {
		t.Fatal("conflicting event was acknowledged")
		return nil
	}
	_, err := processor.Handle(context.Background(), delivery)
	if !errors.Is(err, ErrConflict) || len(store.records) != 1 {
		t.Fatal("conflicting event overwrote the original notification")
	}
}

// TestNormalEvent checks that confirmed verification uses the normal lane.

func TestNormalEvent(t *testing.T) {
	processor, store, _, delivery := fixture(t)
	delivery.Body = []byte(strings.ReplaceAll(
		string(delivery.Body), contract.LoginSucceeded, contract.EmailVerified,
	))
	delivery.RoutingKey = contract.EmailVerified
	delivery.Queue = "notifications.account.v1"
	if _, err := processor.Handle(context.Background(), delivery); err != nil {
		t.Fatal("normal event processing failed")
	}
	for _, notification := range store.records {
		if notification.Priority != contract.Normal {
			t.Fatal("normal event was assigned security priority")
		}
	}
}

// TestConfiguration rejects missing dependencies before processing begins.

func TestConfiguration(t *testing.T) {
	_, store, renderer, _ := fixture(t)
	if _, err := New("http://identity.example", store, renderer); err == nil {
		t.Fatal("insecure issuer accepted")
	}
	if _, err := New("https://identity.example", nil, renderer); err == nil {
		t.Fatal("missing store accepted")
	}
	if _, err := New("https://identity.example", store, nil); err == nil {
		t.Fatal("missing renderer accepted")
	}
}
