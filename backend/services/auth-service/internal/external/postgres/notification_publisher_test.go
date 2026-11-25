package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/rabbitmq"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

func publisherFixtureSettings() rabbitmq.Config {
	return rabbitmq.Config{
		Host: "127.0.0.1", Port: 5672, Scheme: "amqp", VHost: "govai",
		Username: "outbox_publisher", Password: os.Getenv("GOVAI_OUTBOX_TEST_PASSWORD"),
		Issuer: outboxTestIssuer, Timeout: 2 * time.Second, Development: true,
	}
}

func publisherFixtureBroker(t *testing.T) *amqp.Channel {
	t.Helper()
	password := os.Getenv("GOVAI_OUTBOX_TEST_PASSWORD")
	for _, setup := range []struct {
		path string
		body any
	}{
		{"users/outbox_publisher", map[string]string{"password": password, "tags": ""}},
		{"permissions/govai/outbox_publisher", map[string]string{
			"configure": "^$", "write": "^notifications$", "read": "^$",
		}},
	} {
		encoded, _ := json.Marshal(setup.body)
		request, err := http.NewRequest(http.MethodPut,
			"http://127.0.0.1:15672/api/"+setup.path, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal("fixture request failed")
		}
		request.SetBasicAuth("outbox_admin", password)
		request.Header.Set("Content-Type", "application/json")
		response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
		if err != nil {
			t.Fatal("fixture management endpoint unavailable")
		}
		_ = response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			t.Fatal("fixture permissions setup failed")
		}
	}
	endpoint := &url.URL{
		Scheme: "amqp", Host: "127.0.0.1:5672", Path: "/govai",
		User: url.UserPassword("outbox_admin", password),
	}
	connection, err := amqp.Dial(endpoint.String())
	if err != nil {
		t.Fatal("fixture AMQP connection unavailable")
	}
	t.Cleanup(func() { _ = connection.Close() })
	channel, err := connection.Channel()
	if err != nil || channel.ExchangeDeclare("notifications", "topic", true,
		false, false, false, nil) != nil {
		t.Fatal("fixture exchange setup failed")
	}
	for _, queue := range []string{"notifications.security.v1", "notifications.account.v1"} {
		if _, err := channel.QueueDeclare(queue, true, false, false, false,
			amqp.Table{"x-queue-type": "quorum"}); err != nil {
			t.Fatal("fixture queue setup failed")
		}
	}
	for eventType, queue := range map[string]string{
		contract.LoginSucceeded:  "notifications.security.v1",
		contract.PasswordChanged: "notifications.security.v1",
		contract.EmailVerified:   "notifications.account.v1",
	} {
		if channel.QueueBind(queue, eventType, "notifications", false, nil) != nil {
			t.Fatal("fixture routing setup failed")
		}
	}
	return channel
}

func enqueuePublisherFixture(
	t *testing.T, pool *pgxpool.Pool, id, eventType string,
) contract.Event {
	t.Helper()
	event := outboxTestEvent()
	event.ID, event.Type = id, eventType
	writer, _ := NewNotificationOutbox(outboxTestIssuer)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal("fixture transaction unavailable")
	}
	defer tx.Rollback(ctx)
	if _, err := writer.EnqueueTx(ctx, tx, event.ID, event); err != nil ||
		tx.Commit(ctx) != nil {
		t.Fatal("fixture event enqueue failed")
	}
	return event
}

func forcePublicationDue(t *testing.T, owner *pgxpool.Pool, id string) {
	t.Helper()
	if _, err := owner.Exec(context.Background(), `
UPDATE auth.notification_outbox SET next_attempt_at=clock_timestamp()-interval '1 second'
WHERE event_id=$1`, id); err != nil {
		t.Fatal("fixture due-time adjustment failed")
	}
}

func assertPublicationStatus(t *testing.T, pool *pgxpool.Pool, id, code string, published bool) {
	t.Helper()
	var correct bool
	err := pool.QueryRow(context.Background(), `
SELECT (published_at IS NOT NULL)=$2 AND last_error_code=$3 AND lease_token IS NULL
FROM auth.notification_outbox WHERE event_id=$1`, id, published, code).Scan(&correct)
	if err != nil || !correct {
		t.Fatal("publication status was not persisted correctly")
	}
}

func capturePublisherFixture(t *testing.T, channel *amqp.Channel,
	queue, id string,
) []byte {
	t.Helper()
	message, found, err := channel.Get(queue, true)
	if err != nil || !found || message.MessageId != id ||
		message.DeliveryMode != amqp.Persistent || message.ContentType != "application/json" {
		t.Fatal("confirmed persistent event was not routed correctly")
	}
	event, err := contract.Decode(message.Body, outboxTestIssuer)
	if err != nil || event.ID != id || event.Type != message.RoutingKey {
		t.Fatal("published event contract changed")
	}
	return message.Body
}

func TestNotificationPublisherBroker(t *testing.T) {
	if os.Getenv("GOVAI_OUTBOX_PUBLISHER_TEST") != "1" {
		t.Skip("opt-in: isolated publisher fixture required")
	}
	owner := outboxTestPool(t, "outbox_owner")
	app := outboxTestPool(t, "govai_auth_app")
	channel := publisherFixtureBroker(t)

	store, _ := NewNotificationDispatchStore(app, outboxTestIssuer)
	publisher, _ := rabbitmq.New(publisherFixtureSettings())
	dispatcher, _ := notificationevents.New(store, publisher)
	ctx := context.Background()

	t.Run("publisher cannot configure or consume", func(t *testing.T) {
		settings := publisherFixtureSettings()
		endpoint := &url.URL{
			Scheme: "amqp", Host: "127.0.0.1:5672", Path: "/govai",
			User: url.UserPassword(settings.Username, settings.Password),
		}
		connection, err := amqp.Dial(endpoint.String())
		if err != nil {
			t.Fatal("publisher fixture login failed")
		}
		defer connection.Close()
		writeOnly, err := connection.Channel()
		if err != nil {
			t.Fatal("publisher fixture channel unavailable")
		}
		_, err = writeOnly.QueueDeclare("forbidden-publisher-queue",
			true, false, false, false, nil)
		var denied *amqp.Error
		if !errors.As(err, &denied) || denied.Code != 403 {
			t.Fatal("publisher configure permission was not denied")
		}
		writeOnly, err = connection.Channel()
		if err != nil {
			t.Fatal("publisher fixture channel unavailable")
		}
		_, _, err = writeOnly.Get("notifications.security.v1", false)
		if !errors.As(err, &denied) || denied.Code != 403 {
			t.Fatal("publisher read permission was not denied")
		}
	})
	t.Run("all confirmed account checkpoints reach the broker", func(t *testing.T) {
		if _, err := owner.Exec(ctx, `
UPDATE auth.notification_outbox
SET next_attempt_at=clock_timestamp()-interval '1 second',
 lease_token=NULL, lease_expires_at=NULL WHERE published_at IS NULL`); err != nil {
			t.Fatal("pending checkpoint fixture adjustment failed")
		}
		for _, lane := range []contract.Priority{contract.High, contract.Normal} {
			for processed := 0; processed < 20; processed++ {
				worked, err := dispatcher.DispatchOne(ctx, lane)
				if err != nil {
					t.Fatal("account checkpoint publication failed")
				}
				if !worked {
					break
				}
			}
		}
		counts := map[string]int{}
		seen := map[string]bool{}
		for _, queue := range []string{"notifications.security.v1", "notifications.account.v1"} {
			for {
				message, found, err := channel.Get(queue, true)
				if err != nil {
					t.Fatal("account checkpoint capture failed")
				}
				if !found {
					break
				}
				event, err := contract.Decode(message.Body, outboxTestIssuer)
				if err != nil || seen[event.ID] || event.ID != message.MessageId {
					t.Fatal("account checkpoint event was malformed or duplicated")
				}
				seen[event.ID] = true
				counts[event.Type]++
				assertPublicationStatus(t, app, event.ID, "", true)
			}
		}
		if counts[contract.LoginSucceeded] != 2 || counts[contract.PasswordChanged] != 2 ||
			counts[contract.EmailVerified] != 2 {
			t.Fatal("confirmed login, password and verification events did not all arrive once")
		}
	})
	if _, err := owner.Exec(ctx,
		"TRUNCATE auth.notification_outbox, public.outbox_test_checkpoint"); err != nil {
		t.Fatal("disposable fixture reset failed")
	}
	t.Run("security and normal routing with confirms", func(t *testing.T) {
		security := enqueuePublisherFixture(t, app,
			"123e4567-e89b-42d3-a456-426614174011", contract.LoginSucceeded)
		normal := enqueuePublisherFixture(t, app,
			"123e4567-e89b-42d3-a456-426614174012", contract.EmailVerified)
		for _, lane := range []contract.Priority{contract.High, contract.Normal} {
			if worked, err := dispatcher.DispatchOne(ctx, lane); !worked || err != nil {
				t.Fatal("confirmed dispatch failed")
			}
		}
		capturePublisherFixture(t, channel, "notifications.security.v1", security.ID)
		capturePublisherFixture(t, channel, "notifications.account.v1", normal.ID)
		assertPublicationStatus(t, app, security.ID, "", true)
		assertPublicationStatus(t, app, normal.ID, "", true)
	})

	t.Run("unroutable stays pending", func(t *testing.T) {
		event := enqueuePublisherFixture(t, app,
			"123e4567-e89b-42d3-a456-426614174013", contract.LoginSucceeded)
		if channel.QueueUnbind("notifications.security.v1", event.Type,
			"notifications", nil) != nil {
			t.Fatal("fixture unbind failed")
		}
		_, err := dispatcher.DispatchOne(ctx, contract.High)
		if !errors.Is(err, notificationevents.ErrUnroutable) {
			t.Fatal("returned message was treated as confirmed")
		}
		assertPublicationStatus(t, app, event.ID, "unroutable", false)
		if channel.QueueBind("notifications.security.v1", event.Type,
			"notifications", false, nil) != nil {
			t.Fatal("fixture rebind failed")
		}
		forcePublicationDue(t, owner, event.ID)
		if _, err := dispatcher.DispatchOne(ctx, contract.High); err != nil {
			t.Fatal("rebound event did not recover")
		}
		capturePublisherFixture(t, channel, "notifications.security.v1", event.ID)
		assertPublicationStatus(t, app, event.ID, "", true)
	})

	t.Run("lost confirm reuses the same event", func(t *testing.T) {
		event := enqueuePublisherFixture(t, app,
			"123e4567-e89b-42d3-a456-426614174014", contract.PasswordChanged)
		settings := publisherFixtureSettings()
		settings.Port = dropConfirmProxy(t)
		uncertain, err := rabbitmq.New(settings)
		if err != nil {
			t.Fatal("fixture publisher setup failed")
		}
		worker, _ := notificationevents.New(store, uncertain)
		_, err = worker.DispatchOne(ctx, contract.High)
		if !errors.Is(err, notificationevents.ErrUnknown) {
			t.Fatal("lost confirm was treated as success")
		}
		assertPublicationStatus(t, app, event.ID, "publish_unknown", false)
		first := capturePublisherFixture(t, channel, "notifications.security.v1", event.ID)
		forcePublicationDue(t, owner, event.ID)
		if _, err := dispatcher.DispatchOne(ctx, contract.High); err != nil {
			t.Fatal("unconfirmed event did not recover")
		}
		second := capturePublisherFixture(t, channel, "notifications.security.v1", event.ID)
		if !bytes.Equal(first, second) {
			t.Fatal("retry changed the event ID or payload")
		}
		assertPublicationStatus(t, app, event.ID, "", true)
	})
}

func TestNotificationPublisherOutage(t *testing.T) {
	phase := os.Getenv("GOVAI_OUTBOX_BROKER_PHASE")
	if phase != "down" && phase != "up" {
		t.Skip("opt-in: isolated broker outage fixture required")
	}
	owner := outboxTestPool(t, "outbox_owner")
	app := outboxTestPool(t, "govai_auth_app")
	store, _ := NewNotificationDispatchStore(app, outboxTestIssuer)
	publisher, _ := rabbitmq.New(publisherFixtureSettings())
	dispatcher, _ := notificationevents.New(store, publisher)
	id := "123e4567-e89b-42d3-a456-426614174020"
	ctx := context.Background()
	if phase == "down" {
		enqueuePublisherFixture(t, app, id, contract.LoginSucceeded)
		_, err := dispatcher.DispatchOne(ctx, contract.High)
		if !errors.Is(err, notificationevents.ErrUnavailable) {
			t.Fatal("offline broker was not detected")
		}
		assertPublicationStatus(t, app, id, "broker_unavailable", false)
		return
	}
	channel := publisherFixtureBroker(t)
	forcePublicationDue(t, owner, id)
	if _, err := dispatcher.DispatchOne(ctx, contract.High); err != nil {
		t.Fatal("publication did not recover after broker restart")
	}
	assertPublicationStatus(t, app, id, "", true)
	capturePublisherFixture(t, channel, "notifications.security.v1", id)
}
