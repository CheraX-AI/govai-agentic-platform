package permissionlab

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

// EXPLICITLY SYNTHETIC TEST STORES. Never compiled into an application binary.
// Mutex/channel behavior does not model durable commits or distributed locking.
type combinedStores struct {
	mu            sync.Mutex
	ops           map[string]registration.Operation
	claims        map[string]string
	gates         map[string]chan struct{}
	profiles      map[string]registration.ProfileRecord
	failProfile   bool
	profileCalls  int
	beforeProfile func(context.Context, registration.ProfileRecord) error
}

func newCombinedStores() *combinedStores {
	return &combinedStores{
		ops:      map[string]registration.Operation{},
		claims:   map[string]string{},
		gates:    map[string]chan struct{}{},
		profiles: map[string]registration.ProfileRecord{},
	}
}

func (*combinedStores) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[REDACTED]") }

func storeKey(issuer, key string) string { return issuer + "\x00" + key }

func (d *combinedStores) Reserve(ctx context.Context, r registration.Reservation) (registration.OperationSession, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	d.mu.Lock()
	k := storeKey(r.Issuer, r.Key)
	if old, ok := d.ops[k]; ok {
		if old.Fingerprint != r.Fingerprint {
			d.mu.Unlock()
			return nil, registration.ErrRequestConflict
		}
	} else {
		n := r.Pending.NIC.NormalizedNumber()
		if n == "" {
			d.mu.Unlock()
			return nil, registration.ErrNICFormat
		}

		if _, ok := d.claims[n]; ok {
			d.mu.Unlock()
			return nil, registration.ErrNICConflict
		}

		d.claims[n] = k
		d.ops[k] = registration.Operation{Reservation: r, Phase: registration.Reserved}
		d.gates[k] = make(chan struct{}, 1)
	}

	d.mu.Unlock()
	return d.Open(ctx, r.Issuer, r.Key)
}

func (d *combinedStores) Open(ctx context.Context, issuer, key string) (registration.OperationSession, error) {
	d.mu.Lock()
	k := storeKey(issuer, key)
	gate, ok := d.gates[k]
	d.mu.Unlock()
	if !ok {
		return nil, registration.ErrOperationMissing
	}

	select {
	case gate <- struct{}{}:
		if ctx.Err() != nil {
			<-gate
			return nil, ctx.Err()
		}
		return &combinedSession{d: d, key: k, gate: gate, ctx: ctx}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type combinedSession struct {
	d    *combinedStores
	key  string
	gate chan struct{}
	ctx  context.Context
}

func (s *combinedSession) Context() context.Context { return s.ctx }

func (s *combinedSession) Snapshot() registration.Operation {
	s.d.mu.Lock()
	defer s.d.mu.Unlock()
	return s.d.ops[s.key]
}

func (s *combinedSession) Save(ctx context.Context, next registration.Operation) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	s.d.mu.Lock()
	defer s.d.mu.Unlock()
	old := s.d.ops[s.key]
	if old.Reservation != next.Reservation ||
		next.Revision != old.Revision+1 ||
		(old.Subject != "" &&
			old.Subject != next.Subject) {
		return errors.New("synthetic checkpoint conflict")
	}

	s.d.ops[s.key] = next
	return nil
}

func (s *combinedSession) Close(context.Context) error {

	<-s.gate
	return nil
}

func (d *combinedStores) Persist(ctx context.Context, r registration.ProfileRecord) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if d.beforeProfile != nil {
		if err := d.beforeProfile(ctx, r); err != nil {
			return err
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.profileCalls++
	if d.failProfile {
		return errors.New("injected synthetic profile failure")
	}

	k, ok := d.claims[r.Data.NIC.NormalizedNumber()]
	op := d.ops[k]
	if !ok || op.OperationID != r.OperationID || op.Subject != r.Subject || op.Issuer != r.Issuer {
		return registration.ErrProfileConflict
	}

	pk := storeKey(r.Issuer, r.Subject)
	if old, ok := d.profiles[pk]; ok && old != r {
		return registration.ErrProfileConflict
	}

	d.profiles[pk] = r
	return nil
}

func (d *combinedStores) operation(issuer, key string) registration.Operation {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ops[storeKey(issuer, key)]
}

func (d *combinedStores) hasProfile(issuer, subject, op string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	r, ok := d.profiles[storeKey(issuer, subject)]
	return ok && r.OperationID == op
}
