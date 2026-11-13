package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// All values are synthetic syntax fixtures, not issued identities.
func TestNICValidationAndRedaction(t *testing.T) {
	for _, input := range []string{"123456789v", "123456789X", "201234567890"} {
		n, err := ParseNIC(input)
		if err != nil {
			t.Fatal("valid syntax rejected")
		}

		if n.RestrictedOriginal() != input {
			t.Fatal("original changed")
		}

		encoded, _ := json.Marshal(n)
		var logs bytes.Buffer
		slog.New(slog.NewJSONHandler(&logs, nil)).Info("test", "nic", n)
		for _, rendered := range []string{fmt.Sprintf("%v %+v %#v", n, n, n), string(encoded), logs.String()} {
			if strings.Contains(rendered, input) || !strings.Contains(rendered, "[REDACTED]") {
				t.Fatal("sensitive formatting exposed")
			}
		}
	}

	for _, input := range []string{
		"",
		" 123456789V",
		"123456789V ",
		"123456789Z",
		"123456789",
		"2012345678900",
		"２０１２３４５６７８９０",
		"12345-6789V",
		"123456789V\n",
	} {
		if _, err := ParseNIC(input); !errors.Is(err, ErrNICFormat) {
			t.Fatal("invalid syntax accepted")
		}
	}

	n, _ := ParseNIC("123456789v")
	if n.NormalizedNumber() != "123456789V" {
		t.Fatal("suffix normalization failed")
	}
}

// This mutex fake illustrates required semantics only. It does NOT prove atomic
// database uniqueness, durability, cross-process safety or actual NIC equivalence.
type reservationFake struct {
	mu       sync.Mutex
	owners   map[string]string
	bindings map[string]string
}

var _ NICReservations = (*reservationFake)(nil)

func (f *reservationFake) Reserve(ctx context.Context, nic NIC, op string) error {
	key := nic.NormalizedNumber()
	if key == "" {
		return ErrNICFormat
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	if owner, exists := f.owners[key]; exists && owner != op {
		return ErrNICConflict
	}

	for other, owner := range f.owners {
		if other != key && owner == op {
			return ErrNICConflict
		}
	}

	f.owners[key] = op
	return nil
}

func (f *reservationFake) Bind(ctx context.Context, nic NIC, op, issuer, subject string) error {
	key := nic.NormalizedNumber()
	if key == "" {
		return ErrNICFormat
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	if f.owners[key] != op {
		return ErrNICConflict
	}

	identity := issuer + "\x00" + subject
	for other, binding := range f.bindings {
		if other != key && binding == identity {
			return ErrNICConflict
		}
	}

	if binding, exists := f.bindings[key]; exists && binding != identity {
		return ErrNICConflict
	}

	f.bindings[key] = identity
	return nil
}

func syntheticNIC(t *testing.T, value string) NIC {
	t.Helper()
	n, err := ParseNIC(value)
	if err != nil {
		t.Fatal("synthetic syntax rejected")
	}

	return n
}

func TestSyntheticNumberReservationConcurrency(t *testing.T) {
	store := &reservationFake{owners: map[string]string{}, bindings: map[string]string{}}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			input := "123456789v"
			if i%2 == 0 {
				input = "123456789V"
			}

			n := syntheticNIC(t, input)
			err := store.Reserve(context.Background(), n, fmt.Sprintf("operation-%d", i))
			if err == nil {
				wins.Add(1)
			} else if err != ErrNICConflict {
				t.Error("unexpected reservation error")
			}
		}(i)
	}

	wg.Wait()
	if wins.Load() != 1 {
		t.Fatal("normalized number got separate reservations")
	}

	n := syntheticNIC(t, "123456789V")
	op := store.owners[n.NormalizedNumber()]
	if store.Reserve(context.Background(), n, op) != nil {
		t.Fatal("retry not idempotent")
	}

	other := syntheticNIC(t, "201234567890")
	if store.Reserve(context.Background(), other, op) != ErrNICConflict {
		t.Fatal("operation reused across numbers")
	}

	if err := store.Bind(context.Background(), n, op, "synthetic-issuer", "subject-A"); err != nil {
		t.Fatal("initial bind failed")
	}

	if err := store.Bind(context.Background(), n, op, "synthetic-issuer", "subject-A"); err != nil {
		t.Fatal("repeated bind not idempotent")
	}

	if store.Bind(context.Background(), n, op, "synthetic-issuer", "subject-B") != ErrNICConflict {
		t.Fatal("binding overwritten")
	}

	if store.Reserve(context.Background(), other, "other-op") != nil {
		t.Fatal("distinct number rejected")
	}

	if store.Bind(context.Background(), other, "other-op", "synthetic-issuer", "subject-A") != ErrNICConflict {
		t.Fatal("subject reused")
	}

	if store.Bind(context.Background(), other, "wrong-op", "synthetic-issuer", "subject-B") != ErrNICConflict {
		t.Fatal("ownership ignored")
	}
}

func TestNumbersRemainDistinctWithoutMapping(t *testing.T) {
	// Even a digit-conversion-shaped pair remains distinct. No person relationship
	// is asserted for these synthetic values. Leading zeros and V/X are preserved.
	inputs := []string{"123456789V", "191234506789", "123456789X", "001234567890", "201234567890"}
	store := &reservationFake{owners: map[string]string{}, bindings: map[string]string{}}
	for i, input := range inputs {
		n := syntheticNIC(t, input)
		if n.NormalizedNumber() != input {
			t.Fatal("digits or suffix changed")
		}

		if store.Reserve(context.Background(), n, fmt.Sprintf("op-%d", i)) != nil {
			t.Fatal("distinct numbers merged")
		}
	}

	lower := syntheticNIC(t, "123456789x")
	if store.Reserve(context.Background(), lower, "another-op") != ErrNICConflict {
		t.Fatal("lowercase X bypassed number uniqueness")
	}
}

func TestReservationRejectsZeroAndCancellation(t *testing.T) {
	store := &reservationFake{owners: map[string]string{}, bindings: map[string]string{}}
	if store.Reserve(context.Background(), NIC{}, "op") != ErrNICFormat ||
		store.Bind(context.Background(), NIC{}, "op", "issuer", "subject") != ErrNICFormat {
		t.Fatal("zero NIC accepted")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n := syntheticNIC(t, "201234567890")
	if !errors.Is(store.Reserve(ctx, n, "op"), context.Canceled) ||
		!errors.Is(store.Bind(ctx, n, "op", "issuer", "subject"), context.Canceled) {
		t.Fatal("cancellation ignored")
	}

	if len(store.owners) != 0 || len(store.bindings) != 0 {
		t.Fatal("canceled operation mutated state")
	}

	if strings.Contains(ErrNICConflict.Error(), n.NormalizedNumber()) {
		t.Fatal("conflict exposed number")
	}
}
