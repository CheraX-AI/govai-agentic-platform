// Package conversation owns each chat's conversation ID and its encrypted messages.
package conversation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/chatcontract"
)

var (
	// ErrNotFound means the ID is unknown or belongs to another citizen. The
	// two cases are deliberately the same, so IDs cannot be probed.
	ErrNotFound = errors.New("conversation not found")

	// ErrUnavailable hides storage details from callers.
	ErrUnavailable = errors.New("conversation storage unavailable")
)

// maxOwnerLength matches the citizen_subject column limit.
const maxOwnerLength = 255

// Store keeps conversation IDs with the citizen who owns them.
type Store interface {
	// Create saves a new conversation; it fails if the ID already exists.
	Create(ctx context.Context, id, owner string) error

	// Owns reports whether the conversation exists and belongs to owner.
	Owns(ctx context.Context, id, owner string) (bool, error)
}

type Service struct {
	store Store
	newID func() (string, error)
}

func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, ErrUnavailable
	}

	return &Service{store: store, newID: newUUID}, nil
}

// Resolve returns the conversation ID for one chat message. owner must be the
// verified token subject, never a client-supplied value. With no requested ID
// a new conversation is created for owner; otherwise the ID must exist and
// belong to owner, or ErrNotFound is returned.
func (s *Service) Resolve(ctx context.Context, owner, requested string) (string, error) {
	if owner == "" || len(owner) > maxOwnerLength {
		return "", ErrUnavailable
	}

	if requested == "" {
		return s.create(ctx, owner)
	}

	// Callers pass Parse output, but the check is repeated before storage.
	if !chatcontract.ValidConversationID(requested) {
		return "", ErrNotFound
	}

	owned, err := s.store.Owns(ctx, requested, owner)
	if err != nil {
		return "", ErrUnavailable
	}

	if !owned {
		return "", ErrNotFound
	}

	return requested, nil
}

func (s *Service) create(ctx context.Context, owner string) (string, error) {
	id, err := s.newID()
	if err != nil || !chatcontract.ValidConversationID(id) {
		return "", ErrUnavailable
	}

	if s.store.Create(ctx, id, owner) != nil {
		return "", ErrUnavailable
	}

	return id, nil
}

// newUUID returns a random version 4 UUID in canonical lowercase form.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 9562 variant

	h := hex.EncodeToString(b[:])
	id := h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]

	return id, nil
}
