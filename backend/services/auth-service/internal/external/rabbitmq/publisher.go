package rabbitmq

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publish uses one confirmed channel per call, so failures reconnect on the next job.
// A mandatory return or missing confirm must never mark the outbox row published.

func (publisher *Publisher) Publish(ctx context.Context, event contract.Event) error {
	if event.Validate(publisher.settings.Issuer) != nil {
		return notificationevents.ErrInvalid
	}
	body, err := json.Marshal(event)
	if err != nil || len(body) > contract.MaxEventBytes {
		return notificationevents.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, publisher.settings.Timeout)
	defer cancel()
	connection, cleanup, err := publisher.connect(ctx)
	if err != nil {
		return notificationevents.ErrUnavailable
	}
	defer cleanup()
	channel, err := connection.Channel()
	if err != nil || channel.Confirm(false) != nil {
		return notificationevents.ErrUnavailable
	}
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	closed := channel.NotifyClose(make(chan *amqp.Error, 1))
	if ctx.Err() != nil {
		return notificationevents.ErrUnavailable
	}
	err = channel.PublishWithContext(ctx, "notifications", event.Type, true, false,
		amqp.Publishing{
			ContentType: "application/json", DeliveryMode: amqp.Persistent,
			MessageId: event.ID, Type: event.Type, Timestamp: event.OccurredAt,
			Body: body,
		})
	if err != nil {
		return notificationevents.ErrUnknown
	}
	return awaitConfirm(ctx, returns, confirms, closed)
}

func (publisher *Publisher) connect(ctx context.Context) (
	*amqp.Connection, func(), error,
) {
	endpoint := &url.URL{
		Scheme: publisher.settings.Scheme,
		Host:   net.JoinHostPort(publisher.settings.Host, strconv.Itoa(publisher.settings.Port)),
		Path:   "/" + publisher.settings.VHost,
		User:   url.UserPassword(publisher.settings.Username, publisher.settings.Password),
	}
	var socket net.Conn
	var stop func() bool
	dial := func(network, address string) (net.Conn, error) {
		var err error
		socket, err = (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, notificationevents.ErrUnavailable
		}

		// The client library may clear handshake deadlines; cancellation closes I/O.

		stop = context.AfterFunc(ctx, func() { _ = socket.Close() })
		return socket, nil
	}
	connection, err := amqp.DialConfig(endpoint.String(), amqp.Config{
		Dial: dial, TLSClientConfig: publisher.tls, Heartbeat: 5 * time.Second,
		Properties: amqp.Table{"connection_name": "govai-auth-notification-publisher"},
	})
	cleanup := func() {
		if stop != nil {
			stop()
		}
		if socket != nil {
			_ = socket.Close()
		}
		if connection != nil {
			_ = connection.Close()
		}
	}
	if err != nil {
		cleanup()
		return nil, nil, notificationevents.ErrUnavailable
	}
	return connection, cleanup, nil
}
