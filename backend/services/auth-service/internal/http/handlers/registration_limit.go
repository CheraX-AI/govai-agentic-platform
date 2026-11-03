package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

var ErrRegistrationLimited = errors.New("registration rate limited")
var errRegistrationLimit = errors.New("registration limiter unavailable")

// Acquire charges the source attempt and reserves bounded in-flight capacity.
// Every acquired permit must be released, including on invalid input/cancellation.
type RegistrationLimiter interface {
	Acquire(context.Context, string) (RegistrationPermit, time.Duration, error)
}
type RegistrationPermit interface {
	CheckEmail(context.Context, string) (time.Duration, error)
	Release()
}

type RegistrationLimitConfig struct {
	Key                       []byte // server-only HMAC key; not a credential, never derived from input
	IPAttempts, EmailAttempts int
	IPWindow, EmailWindow     time.Duration
	MaxKeys, MaxConcurrent    int
	Now                       func() time.Time
}

func (RegistrationLimitConfig) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (RegistrationLimitConfig) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (RegistrationLimitConfig) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

type limitBucket struct {
	attempts int
	expires  time.Time
}

// LocalRegistrationLimiter stores ONLY bounded expiring abuse counters, never
// registration/profile/journal data. It supplies no distributed enforcement and
// is created only by explicit registration runtime composition or tests.
type LocalRegistrationLimiter struct {
	mu          sync.Mutex
	cfg         RegistrationLimitConfig
	ips, emails map[[32]byte]limitBucket
	inflight    int
}

func (*LocalRegistrationLimiter) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[REDACTED]") }
func (*LocalRegistrationLimiter) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (*LocalRegistrationLimiter) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }

func NewLocalRegistrationLimiter(cfg RegistrationLimitConfig) (*LocalRegistrationLimiter, error) {
	if len(cfg.Key) < 32 || cfg.IPAttempts < 1 || cfg.IPAttempts > 1000 || cfg.EmailAttempts < 1 || cfg.EmailAttempts > 1000 || cfg.IPWindow <= 0 || cfg.IPWindow > 24*time.Hour || cfg.EmailWindow <= 0 || cfg.EmailWindow > 24*time.Hour || cfg.MaxKeys < 1 || cfg.MaxKeys > 10000 || cfg.MaxConcurrent < 1 || cfg.MaxConcurrent > 1000 {
		return nil, errRegistrationLimit
	}
	cfg.Key = append([]byte(nil), cfg.Key...)
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &LocalRegistrationLimiter{cfg: cfg, ips: map[[32]byte]limitBucket{}, emails: map[[32]byte]limitBucket{}}, nil
}
func (l *LocalRegistrationLimiter) digest(kind, value string) [32]byte {
	h := hmac.New(sha256.New, l.cfg.Key)
	_, _ = io.WriteString(h, kind+"\x00"+value)
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}
func pruneBuckets(m map[[32]byte]limitBucket, now time.Time) {
	for k, v := range m {
		if !now.Before(v.expires) {
			delete(m, k)
		}
	}
}
func chargeBucket(m map[[32]byte]limitBucket, key [32]byte, now time.Time, window time.Duration, attempts, capacity int) (time.Duration, error) {
	if b, ok := m[key]; ok {
		if b.attempts >= attempts {
			return b.expires.Sub(now), ErrRegistrationLimited
		}
		b.attempts++
		m[key] = b
		return 0, nil
	}
	if len(m) >= capacity {
		retry := window
		for _, b := range m {
			if d := b.expires.Sub(now); d < retry {
				retry = d
			}
		}
		return retry, ErrRegistrationLimited // do not evict active buckets to admit new identities
	}
	m[key] = limitBucket{attempts: 1, expires: now.Add(window)}
	return 0, nil
}
func (l *LocalRegistrationLimiter) Acquire(ctx context.Context, source string) (RegistrationPermit, time.Duration, error) {
	if ctx.Err() != nil {
		return nil, 0, ctx.Err()
	}
	if source == "" || len(source) > 64 {
		return nil, 0, errRegistrationLimit
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.cfg.Now()
	pruneBuckets(l.ips, now)
	pruneBuckets(l.emails, now)
	if retry, err := chargeBucket(l.ips, l.digest("ip", source), now, l.cfg.IPWindow, l.cfg.IPAttempts, l.cfg.MaxKeys); err != nil {
		return nil, retry, err
	}
	if l.inflight >= l.cfg.MaxConcurrent {
		return nil, time.Second, ErrRegistrationLimited
	}
	l.inflight++
	return &localRegistrationPermit{limiter: l, active: true}, 0, nil
}

type localRegistrationPermit struct {
	mu              sync.Mutex
	limiter         *LocalRegistrationLimiter
	active, checked bool
	email           [32]byte
	retry           time.Duration
	err             error
}

func (p *localRegistrationPermit) CheckEmail(ctx context.Context, email string) (time.Duration, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if email == "" || len(email) > 254 {
		return 0, errRegistrationLimit
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.active {
		return 0, errRegistrationLimit
	}
	l := p.limiter
	key := l.digest("email", email)
	if p.checked {
		if p.email != key {
			return 0, errRegistrationLimit
		}
		return p.retry, p.err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.cfg.Now()
	pruneBuckets(l.emails, now)
	p.checked = true
	p.email = key
	p.retry, p.err = chargeBucket(l.emails, key, now, l.cfg.EmailWindow, l.cfg.EmailAttempts, l.cfg.MaxKeys)
	return p.retry, p.err
}
func (p *localRegistrationPermit) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.active {
		return
	}
	p.active = false
	p.limiter.mu.Lock()
	p.limiter.inflight--
	p.limiter.mu.Unlock()
}
