package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/postgres"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/external/rabbitmq"
)

func TestNotificationRuntimeDatabase(t *testing.T) {
	password := os.Getenv("GOVAI_OUTBOX_TEST_PASSWORD")
	if password == "" {
		t.Skip("opt-in: isolated outbox helper required")
	}
	endpoint := &url.URL{
		Scheme: "postgres", Host: "127.0.0.1:5432", Path: "/govai_auth_outbox_test",
		User: url.UserPassword("govai_auth_app", password), RawQuery: "sslmode=disable",
	}
	cfg := Config{
		Base: config.Config{Port: "0"},
		Database: config.Database{
			URL: endpoint.String(), DataKey: bytes.Repeat([]byte{1}, 32),
			NICKey: bytes.Repeat([]byte{2}, 32),
		},
		NotificationEvents: config.NotificationEvents{
			Enabled: true,
			Publisher: rabbitmq.Config{
				Host: "127.0.0.1", Port: 1, Scheme: "amqp", VHost: "govai",
				Username: "synthetic", Password: "synthetic-password", Development: true,
				Issuer: "https://identity.test/realms/outbox-test", Timeout: 150 * time.Millisecond,
			},
		},
	}
	app, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal("broker outage incorrectly prevented auth startup")
	}
	defer app.Close()
	response := httptest.NewRecorder()
	app.server.Handler.ServeHTTP(response,
		httptest.NewRequest(http.MethodGet, "/api/ready", nil))
	if response.Code != http.StatusOK {
		t.Fatal("broker outage incorrectly made auth storage unready")
	}
	pool, err := postgres.Open(context.Background(), endpoint.String())
	if err != nil {
		t.Fatal("runtime test probe unavailable")
	}
	defer pool.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- app.Run(ctx) }()
	deadline := time.Now().Add(3 * time.Second)
	observed := false
	for time.Now().Before(deadline) {
		var count int
		err := pool.QueryRow(context.Background(), `
SELECT count(*) FROM auth.notification_outbox
WHERE published_at IS NULL AND last_error_code='broker_unavailable'`).Scan(&count)
		if err == nil && count > 0 {
			observed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal("notification-enabled runtime shutdown failed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("notification workers were not joined during shutdown")
	}
	if !observed {
		t.Fatal("startup did not run notification workers")
	}
}
