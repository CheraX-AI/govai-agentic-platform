package app

import (
	"context"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/rabbitmq"
)

type notificationWorkerDouble struct {
	started chan contract.Priority
	stopped chan contract.Priority
}

func (worker *notificationWorkerDouble) Run(ctx context.Context, lane contract.Priority) error {
	worker.started <- lane
	<-ctx.Done()
	worker.stopped <- lane
	return ctx.Err()
}

func TestNotificationWorkerLifecycle(t *testing.T) {
	worker := &notificationWorkerDouble{
		started: make(chan contract.Priority, 2), stopped: make(chan contract.Priority, 2),
	}
	app := &App{notifications: worker}
	stop := app.startNotificationWorkers(context.Background())
	defer stop()
	lanes := map[contract.Priority]bool{}
	for i := 0; i < 2; i++ {
		select {
		case lane := <-worker.started:
			lanes[lane] = true
		case <-time.After(time.Second):
			t.Fatal("notification workers did not start")
		}
	}
	stop()
	stop()
	if !lanes[contract.High] || !lanes[contract.Normal] || len(worker.stopped) != 2 {
		t.Fatal("independent lanes were not started and joined")
	}
}

func TestNotificationIssuerBinding(t *testing.T) {
	cfg := Config{Database: config.Database{URL: "synthetic-config-only"}}
	cfg.NotificationEvents = config.NotificationEvents{
		Enabled: true,
		Publisher: rabbitmq.Config{
			Host: "127.0.0.1", Port: 5672, Scheme: "amqp", VHost: "govai",
			Username: "synthetic", Password: "synthetic-password", Development: true,
			Issuer: "https://identity.test/realms/outbox-test", Timeout: time.Second,
		},
	}
	cfg.Login.Enabled = true
	cfg.Login.Provider.Issuer = "https://other.test/realms/other"
	if _, err := cfg.prepareNotificationEvents(); err == nil {
		t.Fatal("inconsistent notification issuer accepted")
	}
	cfg.Login.Provider.Issuer = cfg.NotificationEvents.Publisher.Issuer
	bound, err := cfg.prepareNotificationEvents()
	if err != nil || bound.Login.NotificationIssuer != cfg.Login.Provider.Issuer {
		t.Fatal("native login hook was not enabled")
	}
	cfg.NotificationEvents.Enabled = false
	bound, err = cfg.prepareNotificationEvents()
	if err != nil || bound.Login.NotificationIssuer != "" || bound.Web.NotificationIssuer != "" {
		t.Fatal("disabled feature left notification hooks enabled")
	}
}

type emailObserverWorkerDouble struct {
	started chan struct{}
	stopped chan struct{}
}

func (observer *emailObserverWorkerDouble) Run(ctx context.Context) error {
	close(observer.started)
	<-ctx.Done()
	close(observer.stopped)
	return ctx.Err()
}

func TestVerificationObserverJoinsPublisherShutdown(t *testing.T) {
	publisher := &notificationWorkerDouble{
		started: make(chan contract.Priority, 2), stopped: make(chan contract.Priority, 2),
	}
	observer := &emailObserverWorkerDouble{
		started: make(chan struct{}), stopped: make(chan struct{}),
	}
	app := &App{notifications: publisher, emailVerifications: observer}
	stop := app.startNotificationWorkers(context.Background())
	defer stop()
	select {
	case <-observer.started:
	case <-time.After(time.Second):
		t.Fatal("verification observer did not start")
	}
	stop()
	select {
	case <-observer.stopped:
	default:
		t.Fatal("verification observer was not joined")
	}
	if len(publisher.stopped) != 2 {
		t.Fatal("publisher lanes were not joined with the observer")
	}
}
