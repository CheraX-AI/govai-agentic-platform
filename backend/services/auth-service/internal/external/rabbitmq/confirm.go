package rabbitmq

import (
	"context"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	amqp "github.com/rabbitmq/amqp091-go"
)

func awaitConfirm(
	ctx context.Context, returns <-chan amqp.Return,
	confirms <-chan amqp.Confirmation, closed <-chan *amqp.Error,
) error {
	select {
	case _, open := <-returns:
		if open {
			return notificationevents.ErrUnroutable
		}
	case confirmation, open := <-confirms:
		if !open || !confirmation.Ack || confirmation.DeliveryTag != 1 {
			return notificationevents.ErrUnknown
		}

		// RabbitMQ sends basic.return before its confirm for an unroutable message.

		select {
		case _, open := <-returns:
			if open {
				return notificationevents.ErrUnroutable
			}
		default:
		}
		return nil
	case <-closed:
	case <-ctx.Done():
	}
	return notificationevents.ErrUnknown
}
