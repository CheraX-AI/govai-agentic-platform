package httpapi

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRegistrationLimiterBoundsAndExpiry(t *testing.T) {
	now := time.Unix(100, 0)
	cfg := limitConfig()
	cfg.Now = func() time.Time { return now }
	cfg.MaxKeys = 1
	cfg.MaxConcurrent = 1
	cfg.IPAttempts = 2
	cfg.EmailAttempts = 1
	l, err := NewLocalRegistrationLimiter(cfg)
	if err != nil {
		t.Fatal(err)
	}

	p, _, err := l.Acquire(context.Background(), "one")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = p.CheckEmail(context.Background(), "one@example.test"); err != nil {
		t.Fatal(err)
	}

	if _, err = p.CheckEmail(context.Background(), "one@example.test"); err != nil {
		t.Fatal("same permit charged twice")
	}

	if _, _, err = l.Acquire(context.Background(), "two"); !errors.Is(err, ErrRegistrationLimited) {
		t.Fatal("capacity bypass")
	}

	if _, _, err = l.Acquire(context.Background(), "one"); !errors.Is(err, ErrRegistrationLimited) {
		t.Fatal("inflight bypass")
	}

	p.Release()
	p.Release()
	if l.inflight != 0 || len(l.ips) != 1 || len(l.emails) != 1 {
		t.Fatal("unbounded state or bad release")
	}

	now = now.Add(time.Hour)
	p, _, err = l.Acquire(context.Background(), "two")
	if err != nil {
		t.Fatal("expired source not reclaimed")
	}

	defer p.Release()
	if _, err = p.CheckEmail(context.Background(), "two@example.test"); err != nil {
		t.Fatal("expired email not reclaimed")
	}
}

func TestRegistrationLimiterConcurrent(t *testing.T) {
	cfg := limitConfig()
	cfg.IPAttempts = 7
	cfg.MaxConcurrent = 100
	l, _ := NewLocalRegistrationLimiter(cfg)
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, _, err := l.Acquire(context.Background(), "same")
			if err == nil {
				mu.Lock()
				admitted++
				mu.Unlock()
				p.Release()
			} else if !errors.Is(err, ErrRegistrationLimited) {
				t.Error("unexpected error")
			}
		}()
	}

	wg.Wait()
	if admitted != 7 {
		t.Fatalf("admitted %d", admitted)
	}
}

func TestRegistrationLimiterCancellationAndConfig(t *testing.T) {
	cfg := limitConfig()
	cfg.Key = nil
	if _, err := NewLocalRegistrationLimiter(cfg); err == nil {
		t.Fatal("unsafe config accepted")
	}

	cfg = limitConfig()
	l, _ := NewLocalRegistrationLimiter(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := l.Acquire(ctx, "source"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}

	if fmt.Sprintf("%+v", cfg) != "[REDACTED]" || fmt.Sprintf("%+v", l) != "[REDACTED]" {
		t.Fatal("unsafe formatting")
	}
}
