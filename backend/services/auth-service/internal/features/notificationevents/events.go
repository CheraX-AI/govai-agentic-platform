package notificationevents

import (
	"crypto/rand"
	"fmt"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
)

// NewID supplies opaque identifiers without implying that an auth action succeeded.

func NewID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", ErrInvalid
	}
	random[6] = random[6]&0x0f | 0x40
	random[8] = random[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", random[:4], random[4:6],
		random[6:8], random[8:10], random[10:]), nil
}

// NewEvent creates one stable envelope for a confirmed, trusted auth checkpoint.

func NewEvent(eventType, issuer, subject string) (contract.Event, error) {
	id, err := NewID()
	if err != nil {
		return contract.Event{}, err
	}
	event := contract.Event{
		ID: id, Type: eventType, Version: contract.Version, Issuer: issuer, Subject: subject,
		OccurredAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	if event.Validate(issuer) != nil {
		return contract.Event{}, ErrInvalid
	}
	return event, nil
}
