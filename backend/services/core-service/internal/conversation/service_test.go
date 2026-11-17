package conversation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/chatcontract"
)

const (
	citizenA = "synthetic-citizen-a"
	citizenB = "synthetic-citizen-b"
)

// fakeStore is a test double for Store; production uses PostgresStore.
type fakeStore struct {
	owners  map[string]string
	creates int
	err     error
}

func newFakeStore() *fakeStore { return &fakeStore{owners: map[string]string{}} }

func (f *fakeStore) Create(_ context.Context, id, owner string) error {
	if f.err != nil {
		return f.err
	}

	if _, exists := f.owners[id]; exists {
		return errors.New("duplicate")
	}

	f.creates++
	f.owners[id] = owner
	return nil
}

func (f *fakeStore) Owns(_ context.Context, id, owner string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}

	return f.owners[id] == owner, nil
}

func newService(t *testing.T, store Store) *Service {
	t.Helper()
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	return service
}

func TestNewConversationGetsUUIDOwnedByCitizen(t *testing.T) {
	store := newFakeStore()
	service := newService(t, store)

	id, err := service.Resolve(context.Background(), citizenA, "")
	if err != nil || !chatcontract.ValidConversationID(id) || id[14] != '4' {
		t.Fatalf("expected a new UUID v4, got %q %v", id, err)
	}

	if store.owners[id] != citizenA {
		t.Fatal("new conversation not saved for its owner")
	}

	other, err := service.Resolve(context.Background(), citizenA, "")
	if err != nil || other == id {
		t.Fatal("each new conversation needs its own ID")
	}
}

func TestExistingConversationKeepsItsID(t *testing.T) {
	store := newFakeStore()
	service := newService(t, store)
	id, _ := service.Resolve(context.Background(), citizenA, "")

	for range 3 {
		again, err := service.Resolve(context.Background(), citizenA, id)
		if err != nil || again != id {
			t.Fatalf("conversation ID changed: %q %v", again, err)
		}
	}

	if store.creates != 1 {
		t.Fatalf("reusing an ID must not create conversations, created %d", store.creates)
	}
}

func TestCitizenCannotUseAnotherCitizensConversation(t *testing.T) {
	store := newFakeStore()
	service := newService(t, store)
	id, _ := service.Resolve(context.Background(), citizenA, "")

	if _, err := service.Resolve(context.Background(), citizenB, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("citizen B reached citizen A's conversation: %v", err)
	}
}

func TestUnknownOrMalformedConversationIsNotFound(t *testing.T) {
	service := newService(t, newFakeStore())
	for _, id := range []string{
		"3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34",
		strings.ToUpper("3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"),
		"not-a-uuid",
	} {
		if _, err := service.Resolve(context.Background(), citizenA, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%q: expected ErrNotFound, got %v", id, err)
		}
	}
}

func TestStorageFailuresAreUnavailable(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("synthetic database failure with details")
	service := newService(t, store)

	for _, requested := range []string{"", "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"} {
		if _, err := service.Resolve(context.Background(), citizenA, requested); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected ErrUnavailable, got %v", err)
		}
	}
}

func TestOwnerMustBePresent(t *testing.T) {
	store := newFakeStore()
	service := newService(t, store)

	for _, owner := range []string{"", strings.Repeat("x", maxOwnerLength+1)} {
		if _, err := service.Resolve(context.Background(), owner, ""); !errors.Is(err, ErrUnavailable) {
			t.Fatal("conversation created without a valid owner")
		}
	}

	if store.creates != 0 {
		t.Fatal("conversation saved without a valid owner")
	}
}

func TestBrokenIDGeneratorIsUnavailable(t *testing.T) {
	service := newService(t, newFakeStore())
	service.newID = func() (string, error) { return "", errors.New("no randomness") }

	if _, err := service.Resolve(context.Background(), citizenA, ""); !errors.Is(err, ErrUnavailable) {
		t.Fatal("expected ErrUnavailable")
	}
}

func TestNewServiceNeedsAStore(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("service created without a store")
	}
}
