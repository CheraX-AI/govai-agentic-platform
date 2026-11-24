package notifications

import (
	"context"
	"errors"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/validation"
)

type Processor struct {
	issuer   string
	store    Store
	renderer Renderer
}

// New validates dependencies without starting any consumer or database connection.

func New(issuer string, store Store, renderer Renderer) (*Processor, error) {
	if store == nil || renderer == nil || contract.ValidateIssuer(issuer) != nil {
		return nil, ErrConfiguration
	}
	return &Processor{issuer: issuer, store: store, renderer: renderer}, nil
}

// Handle validates and saves one event before acknowledging that delivery.
// On any error it leaves broker disposition to the future bounded-retry consumer.

func (p *Processor) Handle(ctx context.Context, delivery Delivery) (SaveResult, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if delivery.Acknowledge == nil {
		return 0, ErrConfiguration
	}
	event, err := validation.DecodeEvent(
		delivery.Body, p.issuer, delivery.RoutingKey, delivery.Queue,
	)
	if err != nil {
		return 0, err
	}
	priority, err := contract.PriorityFor(event.Type)
	if err != nil {
		return 0, err
	}
	content, err := p.renderer.Render(event)
	if err != nil || !validContent(content) {
		return 0, ErrTemplate
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	result, err := p.store.Save(ctx, Notification{
		Event: event, Priority: priority, Content: content,
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return 0, ErrConflict
		}
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, ErrStorage
	}
	if result != Saved && result != Duplicate {
		return 0, ErrStorage
	}

	// A lost ACK can cause redelivery; the store must recognize the same event ID.

	if err := ctx.Err(); err != nil {
		return result, err
	}
	if delivery.Acknowledge() != nil {
		return result, ErrAcknowledgment
	}
	return result, nil
}
