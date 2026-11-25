package postgres

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// Probe network readiness without starting an Erlang CLI during broker bootstrap.

func TestNotificationPublisherReady(t *testing.T) {
	if os.Getenv("GOVAI_OUTBOX_PUBLISHER_TEST") != "1" {
		t.Skip("opt-in: isolated publisher fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := &http.Client{Timeout: time.Second}
	for ctx.Err() == nil {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"http://127.0.0.1:15672/api/overview", nil)
		if err != nil {
			t.Fatal("readiness request unavailable")
		}
		request.SetBasicAuth("outbox_admin", os.Getenv("GOVAI_OUTBOX_TEST_PASSWORD"))
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				socket, err := net.DialTimeout("tcp", "127.0.0.1:5672", time.Second)
				if err == nil {
					_ = socket.Close()
					return
				}
			}
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	t.Fatal("disposable broker HTTP/AMQP listeners did not become ready")
}
