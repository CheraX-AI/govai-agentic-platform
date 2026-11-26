// Package preferences defines channel choices independently of inbox read state.

package preferences

import (
	"context"
	"errors"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/inbox"
)

var ErrUnavailable = errors.New("notification preferences unavailable")

type Channels struct {
	EmailEnabled bool `json:"email_enabled"`
	InAppEnabled bool `json:"in_app_enabled"`
}

// Effective keeps security-critical inbox alerts enabled even when ordinary
// in-app notifications are disabled. Email opt-out still applies to every event.

func (channels Channels) Effective(eventType string) (Channels, error) {
	priority, err := contract.PriorityFor(eventType)
	if err != nil {
		return Channels{}, err
	}
	if priority == contract.High {
		channels.InAppEnabled = true
	}
	return channels, nil
}

// Store owns persistence and approved first-use defaults. Replace commits the two
// channel flags atomically; it must not change locale, inbox history or read state.

type Store interface {
	Get(context.Context, inbox.Owner) (Channels, error)
	Replace(context.Context, inbox.Owner, Channels) error
}
