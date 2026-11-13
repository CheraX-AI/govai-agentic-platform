package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/config"
)

func TestNewWithFeaturesDisabledServesHealth(t *testing.T) {
	a, err := New(context.Background(), Config{Base: config.Config{Port: "5000"}})
	if err != nil {
		t.Fatal(err)
	}

	defer a.Close()
	if a.tls {
		t.Fatal("TLS enabled although no credential feature is on")
	}

	for path, want := range map[string]int{"/api/health": 200, "/api/ready": 404} {
		w := httptest.NewRecorder()
		a.server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Fatalf("%s = %d, want %d", path, w.Code, want)
		}
	}
}

func TestNewFailsWithoutTLSCertificate(t *testing.T) {
	t.Setenv("API_TLS_CERT_FILE", "missing-cert.pem")
	t.Setenv("API_TLS_KEY_FILE", "missing-key.pem")
	cfg := Config{Base: config.Config{Port: "5001"}}
	cfg.Login.Enabled = true

	a, err := New(context.Background(), cfg)
	if !errors.Is(err, config.ErrRegistrationConfig) || a != nil {
		t.Fatalf("got app=%v err=%v, want startup failure", a != nil, err)
	}
}

func TestCloseRunsNewestFirstAndOnlyOnce(t *testing.T) {
	var order []int
	a := &App{}
	a.onClose(func() { order = append(order, 1) })
	a.onClose(func() { order = append(order, 2) })

	a.Close()
	a.Close()
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("close order = %v, want [2 1]", order)
	}
}

func TestRunStopsWhenContextIsCanceled(t *testing.T) {
	a, err := New(context.Background(), Config{Base: config.Config{Port: "0"}})
	if err != nil {
		t.Fatal(err)
	}

	defer a.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v after cancel", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}
