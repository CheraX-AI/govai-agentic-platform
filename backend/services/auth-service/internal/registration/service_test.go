package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// All stateful implementations in this file are TEST DOUBLES, not runtime stores
// or evidence of durable uniqueness/session locking under database failover.
type testDB struct {
	mu                                                                           sync.Mutex
	ops                                                                          map[string]Operation
	claims                                                                       map[string]string
	gates                                                                        map[string]chan struct{}
	profiles                                                                     map[string]ProfileRecord
	users                                                                        map[string]Identity
	owners                                                                       map[string]string
	calls                                                                        map[string]int
	failSave                                                                     Phase
	afterCommit                                                                  bool
	failReserve, failProfile, failClose                                          bool
	createError, errorAfterCreate, readError, findError, enableError, emailError error
	createStarted, createRelease                                                 chan struct{}
	cancelCreate                                                                 context.CancelFunc
	now                                                                          time.Time
}

func newDB() *testDB {
	return &testDB{ops: map[string]Operation{}, claims: map[string]string{}, gates: map[string]chan struct{}{}, profiles: map[string]ProfileRecord{}, users: map[string]Identity{}, owners: map[string]string{}, calls: map[string]int{}, now: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)}
}
func (d *testDB) Reserve(ctx context.Context, r Reservation) (OperationSession, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	d.mu.Lock()
	if d.failReserve {
		d.mu.Unlock()
		return nil, errors.New("private database error")
	}
	if op, ok := d.ops[r.Key]; ok {
		if op.Issuer != r.Issuer || op.Fingerprint != r.Fingerprint {
			d.mu.Unlock()
			return nil, ErrRequestConflict
		}
	} else {
		n := r.Pending.NIC.NormalizedNumber()
		if _, ok := d.claims[n]; ok {
			d.mu.Unlock()
			return nil, ErrNICConflict
		}
		d.claims[n] = r.Key
		d.ops[r.Key] = Operation{Reservation: r, Phase: Reserved}
		d.gates[r.Key] = make(chan struct{}, 1)
	}
	d.mu.Unlock()
	return d.Open(ctx, r.Issuer, r.Key)
}
func (d *testDB) Open(ctx context.Context, issuer, key string) (OperationSession, error) {
	d.mu.Lock()
	op, ok := d.ops[key]
	gate := d.gates[key]
	d.mu.Unlock()
	if !ok || op.Issuer != issuer {
		return nil, ErrOperationMissing
	}
	select {
	case gate <- struct{}{}:
		if ctx.Err() != nil {
			<-gate
			return nil, ctx.Err()
		}
		return &testSession{db: d, key: key, gate: gate, ctx: ctx}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type testSession struct {
	db   *testDB
	key  string
	gate chan struct{}
	ctx  context.Context
}

func (s *testSession) Context() context.Context { return s.ctx }

func (s *testSession) Snapshot() Operation {
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	return s.db.ops[s.key]
}
func (s *testSession) Save(ctx context.Context, next Operation) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d := s.db
	d.mu.Lock()
	defer d.mu.Unlock()
	op := d.ops[s.key]
	if op.Reservation != next.Reservation || next.Revision != op.Revision+1 || (op.Subject != "" && next.Subject != op.Subject) {
		return errors.New("immutable state changed")
	}
	if d.failSave == next.Phase {
		d.failSave = ""
		if d.afterCommit {
			d.ops[s.key] = next
		}
		return errors.New("private commit failure")
	}
	d.ops[s.key] = next
	return nil
}
func (s *testSession) Close(context.Context) error {
	<-s.gate
	if s.db.failClose {
		return errors.New("private unlock failure")
	}
	return nil
}

type testProfiles struct{ db *testDB }

func (p testProfiles) Persist(ctx context.Context, r ProfileRecord) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d := p.db
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls["profile"]++
	if d.failProfile {
		return errors.New("private profile failure")
	}
	key, ok := d.claims[r.Data.NIC.NormalizedNumber()]
	op := d.ops[key]
	if !ok || op.OperationID != r.OperationID || op.Subject != r.Subject || op.Issuer != r.Issuer {
		return ErrProfileConflict
	}
	pk := r.Issuer + "|" + r.Subject
	if existing, ok := d.profiles[pk]; ok && existing != r {
		return ErrProfileConflict
	}
	d.profiles[pk] = r
	return nil
}

type testAdmin struct{ db *testDB }

func (a testAdmin) CreateDisabled(ctx context.Context, r Request, op string) (Identity, error) {
	d := a.db
	d.mu.Lock()
	d.calls["create"]++
	valid := false
	for _, o := range d.ops {
		if o.OperationID == op && o.Phase == Creating {
			valid = true
		}
	}
	if !valid {
		d.mu.Unlock()
		panic("creation before intent commit")
	}
	err := d.createError
	started, release, cancel := d.createStarted, d.createRelease, d.cancelCreate
	d.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return Identity{}, &AdminError{Code: AdminAmbiguous, UnknownOutcome: true}
		}
	}
	if err != nil {
		return Identity{}, err
	}
	d.mu.Lock()
	u := Identity{Subject: "subject-" + op}
	d.users[u.Subject] = u
	d.owners[u.Subject] = op
	after := d.errorAfterCreate
	d.mu.Unlock()
	if cancel != nil {
		cancel()
		<-ctx.Done()
	}
	if after != nil {
		return Identity{}, after
	}
	return u, nil
}
func (a testAdmin) FindOwned(ctx context.Context, op string) (Identity, error) {
	if ctx.Err() != nil {
		return Identity{}, ctx.Err()
	}
	d := a.db
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls["find"]++
	if d.findError != nil {
		return Identity{}, d.findError
	}
	for subject, owner := range d.owners {
		if owner == op {
			return d.users[subject], nil
		}
	}
	return Identity{}, &AdminError{Code: AdminNotFound}
}
func (a testAdmin) ReadOwned(ctx context.Context, subject, op string) (Identity, error) {
	if ctx.Err() != nil {
		return Identity{}, ctx.Err()
	}
	d := a.db
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls["read"]++
	if d.readError != nil {
		return Identity{}, d.readError
	}
	if d.owners[subject] != op {
		return Identity{}, &AdminError{Code: AdminOwnership}
	}
	return d.users[subject], nil
}
func (a testAdmin) EnableOwned(ctx context.Context, subject, op string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d := a.db
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls["enable"]++
	bound := false
	for _, r := range d.profiles {
		if r.Subject == subject && r.OperationID == op {
			bound = true
		}
	}
	if !bound {
		panic("enable before committed profile")
	}
	if d.enableError != nil {
		return d.enableError
	}
	u := d.users[subject]
	u.Enabled = true
	d.users[subject] = u
	return nil
}
func (a testAdmin) SendVerification(ctx context.Context, subject, op string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d := a.db
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls["email"]++
	if !d.users[subject].Enabled || d.owners[subject] != op {
		panic("email before owned enabling")
	}
	return d.emailError
}

func service(t *testing.T, d *testDB) *Service {
	t.Helper()
	s, err := NewService(ServiceConfig{Issuer: "https://issuer.example.test/realms/GovAI", FingerprintKey: bytes.Repeat([]byte{1}, 32), Timeout: time.Second, EmailRetryBackoff: time.Minute, MaxEmailAttempts: 3, Now: func() time.Time { return d.now }}, testAdmin{d}, testProfiles{d}, d)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func requestJSON(nic, email, password string) *strings.Reader {
	body, _ := json.Marshal(map[string]string{"full_name": "Synthetic Citizen", "nic": nic, "mobile_number": "+14155552671", "email": email, "password": password})
	return strings.NewReader(string(body))
}
func key(i int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", i) }
func register(s *Service, k string) (Receipt, error) {
	return s.Register(context.Background(), k, requestJSON("000000001V", "synthetic@example.test", "synthetic-secret-password"))
}
func code(t *testing.T, err error, want string) {
	t.Helper()
	var e *ServiceError
	if !errors.As(err, &e) || e.Code != want {
		t.Fatalf("got %v want %s", err, want)
	}
}

func TestServiceHappyPathReplayAndPasswordExclusion(t *testing.T) {
	d := newDB()
	s := service(t, d)
	receipt, err := register(s, key(1))
	if err != nil || receipt.Status != "received" {
		t.Fatal(err)
	}
	if d.ops[key(1)].Phase != Completed || d.ops[key(1)].EmailState != "request_accepted" || len(d.profiles) != 1 {
		t.Fatal("incomplete state")
	}
	_, err = s.Register(context.Background(), key(1), requestJSON("000000001v", "SYNTHETIC@example.test", "replacement-password"))
	if err != nil {
		t.Fatal(err)
	}
	if d.calls["create"] != 1 || d.calls["profile"] != 1 || d.calls["enable"] != 1 || d.calls["email"] != 1 {
		t.Fatal("replay repeated effects")
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	for _, v := range []any{d.ops[key(1)], d.ops[key(1)].Reservation, d.ops[key(1)].Pending, s, s.cfg} {
		b, e := json.Marshal(v)
		if e != nil || string(b) != `"[REDACTED]"` || fmt.Sprintf("%#v", v) != "[REDACTED]" {
			t.Fatal("unsafe persistence formatting")
		}
		logger.Info("safe", "value", v)
	}
	if strings.Contains(logs.String(), "synthetic") || strings.Contains(logs.String(), "password") {
		t.Fatal("sensitive log")
	}
	// Persistence ports accept only explicit credential-free DTOs.
	for _, typ := range []reflect.Type{reflect.TypeOf(ProfileData{}), reflect.TypeOf(ProfileRecord{}), reflect.TypeOf(Reservation{}), reflect.TypeOf(Operation{})} {
		for i := 0; i < typ.NumField(); i++ {
			n := strings.ToLower(typ.Field(i).Name)
			if strings.Contains(n, "password") || strings.Contains(n, "token") || typ.Field(i).Type == reflect.TypeOf(Request{}) {
				t.Fatal("credential-bearing storage DTO")
			}
		}
	}
	r1, _ := ParseRequest(requestJSON("000000001V", "synthetic@example.test", "first-password"))
	r2, _ := ParseRequest(requestJSON("000000001V", "synthetic@example.test", "different-password"))
	if s.fingerprint(r1) != s.fingerprint(r2) {
		t.Fatal("password fingerprinted")
	}
}

func TestServiceRejectsOverridesAndConflictsBeforeMutation(t *testing.T) {
	d := newDB()
	s := service(t, d)
	_, err := s.Register(context.Background(), key(1), strings.NewReader(`{"role":"admin"}`))
	var input *RequestError
	if !errors.As(err, &input) || len(d.ops) != 0 {
		t.Fatal("validation not reused")
	}
	_, err = register(s, "invalid-key")
	code(t, err, "registration_invalid_key")
	if _, err = register(s, key(1)); err != nil {
		t.Fatal(err)
	}
	_, err = s.Register(context.Background(), key(1), requestJSON("000000002V", "other@example.test", "password"))
	code(t, err, "registration_request_conflict")
	_, err = s.Register(context.Background(), key(2), requestJSON("000000001v", "other@example.test", "password"))
	if err != nil {
		t.Fatal(err)
	}
	if d.calls["create"] != 1 || len(d.ops) != 1 {
		t.Fatal("conflicting NIC caused identity mutation")
	}
}

func TestServiceEveryCheckpointFailureRecoversWithoutCreateReplay(t *testing.T) {
	for _, phase := range []Phase{Creating, SubjectRecorded, ProfileReady, Enabling, Enabled, EmailSending, Completed} {
		for _, after := range []bool{false, true} {
			t.Run(string(phase)+fmt.Sprint(after), func(t *testing.T) {
				d := newDB()
				d.failSave = phase
				d.afterCommit = after
				s := service(t, d)
				_, err := register(s, key(1))
				if err == nil {
					t.Fatal("injected commit error ignored")
				}
				_, err = register(s, key(1))
				if phase == Creating && after {
					code(t, err, "registration_reconciliation_required")
					if d.calls["create"] != 0 {
						t.Fatal("intent replayed despite uncertain checkpoint")
					}
					return
				}
				if (phase == EmailSending && after) || (phase == Completed && !after) {
					code(t, err, "registration_reconciliation_required")
					if d.calls["email"] > 1 {
						t.Fatal("uncertain email resent")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if d.calls["create"] != 1 || d.calls["email"] != 1 || d.ops[key(1)].Phase != Completed {
					t.Fatal("non-idempotent recovery")
				}
			})
		}
	}
}

func TestServiceAmbiguousCreateReconcilesExactOwnedMarker(t *testing.T) {
	d := newDB()
	d.errorAfterCreate = &AdminError{Code: AdminAmbiguous, UnknownOutcome: true}
	s := service(t, d)
	_, err := register(s, key(1))
	code(t, err, "registration_reconciliation_required")
	if d.ops[key(1)].Phase != Creating || d.calls["enable"] != 0 {
		t.Fatal("unsafe ambiguous handling")
	}
	d.errorAfterCreate = nil
	if _, err = s.Resume(context.Background(), key(1)); err != nil {
		t.Fatal(err)
	}
	if d.calls["create"] != 1 || d.calls["find"] != 1 {
		t.Fatal("blind create retry")
	}
	for _, findErr := range []error{&AdminError{Code: AdminNotFound}, &AdminError{Code: AdminOwnership}, errors.New("private search failure")} {
		t.Run(fmt.Sprint(findErr), func(t *testing.T) {
			d := newDB()
			d.createError = &AdminError{Code: AdminAmbiguous, UnknownOutcome: true}
			d.findError = findErr
			s := service(t, d)
			_, _ = register(s, key(1))
			_, err := register(s, key(1))
			code(t, err, "registration_reconciliation_required")
			if d.calls["create"] != 1 || d.calls["profile"] != 0 || d.calls["enable"] != 0 {
				t.Fatal("unowned account adopted")
			}
		})
	}
}

func TestServiceDuplicateDoesNotAdoptOrOverwrite(t *testing.T) {
	d := newDB()
	d.createError = &AdminError{Code: AdminDuplicate}
	d.users["legacy"] = Identity{Subject: "legacy", Enabled: true}
	d.owners["legacy"] = "someone-else"
	s := service(t, d)
	if _, err := register(s, key(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := register(s, key(1)); err != nil {
		t.Fatal(err)
	}
	if d.calls["find"] != 0 || d.calls["profile"] != 0 || d.calls["enable"] != 0 || d.calls["email"] != 0 || len(d.users) != 1 || d.ops[key(1)].Phase != Duplicate {
		t.Fatal("duplicate adopted/changed an account")
	}
}

func TestServiceStoreAndProfileOutages(t *testing.T) {
	d := newDB()
	d.failReserve = true
	s := service(t, d)
	_, err := register(s, key(1))
	code(t, err, "registration_unavailable")
	if d.calls["create"] != 0 {
		t.Fatal("created without reservation")
	}
	d.failReserve = false
	d.failProfile = true
	_, err = register(s, key(1))
	code(t, err, "registration_unavailable")
	if d.calls["enable"] != 0 || d.ops[key(1)].Phase != SubjectRecorded {
		t.Fatal("enabled before profile")
	}
	d.failProfile = false
	if _, err = s.Resume(context.Background(), key(1)); err != nil {
		t.Fatal(err)
	}
	if d.calls["create"] != 1 {
		t.Fatal("profile failure recreated identity")
	}
}

func TestServiceCancellationAndConcurrentRetries(t *testing.T) {
	d := newDB()
	d.createStarted = make(chan struct{}, 1)
	d.createRelease = make(chan struct{})
	s := service(t, d)
	first := make(chan error, 1)
	go func() { _, err := register(s, key(1)); first <- err }()
	<-d.createStarted
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Register(ctx, key(1), requestJSON("000000001V", "synthetic@example.test", "password"))
	code(t, err, "registration_canceled")
	second := make(chan error, 1)
	go func() { _, err := register(s, key(1)); second <- err }()
	// Another key with the reserved normalized number must not wait/create.
	if _, err := register(s, key(2)); err != nil {
		t.Fatal(err)
	}
	close(d.createRelease)
	if err = <-first; err != nil {
		t.Fatal(err)
	}
	if err = <-second; err != nil {
		t.Fatal(err)
	}
	if d.calls["create"] != 1 || d.calls["email"] != 1 {
		t.Fatal("concurrent replay")
	}
	// Cancellation after identity effect but before subject save leaves intent.
	d2 := newDB()
	ctx, cancel = context.WithCancel(context.Background())
	d2.cancelCreate = cancel
	s2 := service(t, d2)
	_, err = s2.Register(ctx, key(1), requestJSON("000000001V", "synthetic@example.test", "password"))
	code(t, err, "registration_canceled")
	d2.cancelCreate = nil
	if _, err = s2.Resume(context.Background(), key(1)); err != nil {
		t.Fatal(err)
	}
	if d2.calls["create"] != 1 {
		t.Fatal("canceled create replayed")
	}
}

func TestServiceEmailRetryBackoffCapAndUncertainSend(t *testing.T) {
	d := newDB()
	d.emailError = &AdminError{Code: AdminPermission}
	s := service(t, d)
	for i := 0; i < 3; i++ {
		_, err := register(s, key(1))
		code(t, err, "registration_unavailable")
		_, err = register(s, key(1))
		if i == 2 {
			code(t, err, "registration_attention_required")
		} else {
			code(t, err, "registration_verification_pending")
		}
		d.now = d.now.Add(10 * time.Minute)
	}
	_, err := register(s, key(1))
	code(t, err, "registration_attention_required")
	if d.calls["email"] != 3 || d.calls["create"] != 1 {
		t.Fatal("retry bound exceeded")
	}
	d2 := newDB()
	d2.emailError = &AdminError{Code: AdminAmbiguous, UnknownOutcome: true}
	s2 := service(t, d2)
	_, err = register(s2, key(1))
	code(t, err, "registration_reconciliation_required")
	_, err = s2.Resume(context.Background(), key(1))
	code(t, err, "registration_reconciliation_required")
	if d2.calls["email"] != 1 {
		t.Fatal("ambiguous send replayed")
	}
	d2.mu.Lock()
	for k, u := range d2.users {
		u.EmailVerified = true
		d2.users[k] = u
	}
	d2.mu.Unlock()
	if _, err = s2.Resume(context.Background(), key(1)); err != nil {
		t.Fatal(err)
	}
	if d2.ops[key(1)].EmailState != "already_verified" {
		t.Fatal("verified recovery state lost")
	}
}

func TestServiceDependencyFailuresAndDefiniteCreateRetry(t *testing.T) {
	for _, kind := range []string{"create", "read", "enable", "close"} {
		t.Run(kind, func(t *testing.T) {
			d := newDB()
			s := service(t, d)
			switch kind {
			case "create":
				d.createError = &AdminError{Code: AdminPermission}
			case "read":
				d.readError = &AdminError{Code: AdminOwnership}
			case "enable":
				d.enableError = &AdminError{Code: AdminAmbiguous, UnknownOutcome: true}
			case "close":
				d.failClose = true
			}
			_, err := register(s, key(1))
			if err == nil {
				t.Fatal("dependency failure ignored")
			}
			if strings.Contains(err.Error(), "private") {
				t.Fatal("raw dependency error escaped")
			}
			if kind == "read" && d.calls["profile"] != 0 {
				t.Fatal("ownership failure persisted profile")
			}
			if kind == "enable" && d.calls["email"] != 0 {
				t.Fatal("email before confirmed enabling")
			}
			d.createError = nil
			d.readError = nil
			d.enableError = nil
			d.failClose = false
			if _, err = register(s, key(1)); err != nil {
				t.Fatal(err)
			}
			want := 1
			if kind == "create" {
				want = 2
			}
			if d.calls["create"] != want || len(d.users) != 1 {
				t.Fatal("unexpected identity replay")
			}
		})
	}
	// A policy failure whose atomicity is unknown must be reconciled, even if a
	// particular live test previously found no residual account for that case.
	d := newDB()
	d.createError = &AdminError{Code: AdminPolicyRejected, UnknownOutcome: true}
	s := service(t, d)
	_, err := register(s, key(1))
	code(t, err, "registration_reconciliation_required")
	_, err = register(s, key(1))
	code(t, err, "registration_reconciliation_required")
	if d.calls["create"] != 1 {
		t.Fatal("uncertain policy rejection retried")
	}
	// A known pre-create rejection remains terminal; no implicit password reset.
	d2 := newDB()
	d2.createError = &AdminError{Code: AdminPolicyRejected}
	s2 := service(t, d2)
	_, err = register(s2, key(1))
	code(t, err, "registration_policy_rejected")
	_, err = register(s2, key(1))
	code(t, err, "registration_policy_rejected")
	if d2.calls["create"] != 1 {
		t.Fatal("terminal rejection replayed")
	}
}

func TestServiceReservedRecoveryAndProfileOwnershipConflict(t *testing.T) {
	d := newDB()
	d.failSave = Creating
	s := service(t, d)
	_, _ = register(s, key(1))
	_, err := s.Resume(context.Background(), key(1))
	code(t, err, "registration_credentials_required")
	if d.calls["create"] != 0 {
		t.Fatal("worker manufactured credentials")
	}
	d.failProfile = true
	_, _ = register(s, key(1))
	d.failProfile = false
	op := d.ops[key(1)]
	d.profiles[op.Issuer+"|"+op.Subject] = ProfileRecord{OperationID: "unrelated-owner", Issuer: op.Issuer, Subject: op.Subject, Data: op.Pending}
	_, err = s.Resume(context.Background(), key(1))
	code(t, err, "registration_reconciliation_required")
	if d.calls["enable"] != 0 {
		t.Fatal("profile owner overwritten")
	}
}

func TestServiceWaitingSessionHonorsCancellation(t *testing.T) {
	d := newDB()
	d.createStarted = make(chan struct{}, 1)
	d.createRelease = make(chan struct{})
	s := service(t, d)
	finished := make(chan error, 1)
	go func() { _, err := register(s, key(1)); finished <- err }()
	<-d.createStarted
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := s.Register(ctx, key(1), requestJSON("000000001V", "synthetic@example.test", "password"))
	code(t, err, "registration_canceled")
	close(d.createRelease)
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
	if d.calls["create"] != 1 {
		t.Fatal("session waiter created identity")
	}
}

func TestServiceEnableAmbiguityReadbackAndVerifiedEmailNoop(t *testing.T) {
	d := newDB()
	d.failSave = Enabled
	s := service(t, d)
	_, err := register(s, key(1))
	code(t, err, "registration_unavailable")
	if d.calls["enable"] != 1 {
		t.Fatal("enable not invoked")
	}
	d.mu.Lock()
	for k, u := range d.users {
		u.EmailVerified = true
		d.users[k] = u
	}
	d.mu.Unlock()
	if _, err = s.Resume(context.Background(), key(1)); err != nil {
		t.Fatal(err)
	}
	if d.calls["enable"] != 1 || d.calls["email"] != 0 || d.ops[key(1)].EmailState != "already_verified" {
		t.Fatal("enabled readback or verified-email skip failed")
	}
}

type ownershipSession struct {
	OperationSession
	ownership context.Context
}

func (s ownershipSession) Context() context.Context { return s.ownership }

type ownershipJournal struct {
	RegistrationJournal
	ownership context.Context
}

func (j ownershipJournal) Reserve(ctx context.Context, r Reservation) (OperationSession, error) {
	s, e := j.RegistrationJournal.Reserve(ctx, r)
	if e != nil {
		return nil, e
	}
	return ownershipSession{s, j.ownership}, nil
}

func TestServiceOwnershipLossCancelsExternalWork(t *testing.T) {
	d := newDB()
	s := service(t, d)
	owned, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.journal = ownershipJournal{d, owned}
	d.cancelCreate = cancel
	_, err := register(s, key(1))
	code(t, err, "registration_canceled")
	if d.ops[key(1)].Phase != Creating || d.calls["profile"] != 0 || d.calls["enable"] != 0 {
		t.Fatal("continued after operation ownership loss")
	}
	// A new owner reconciles the exact marker rather than replaying creation.
	d.cancelCreate = nil
	s.journal = d
	if _, err = s.Resume(context.Background(), key(1)); err != nil {
		t.Fatal(err)
	}
	if d.calls["create"] != 1 {
		t.Fatal("ownership loss replayed creation")
	}
}

func TestServiceUnknownSendOutcomesFailClosed(t *testing.T) {
	for _, emailErr := range []error{errors.New("private SMTP failure"), &AdminError{Code: AdminAmbiguous}, &AdminError{Code: AdminCode("unrecognized")}} {
		d := newDB()
		d.emailError = emailErr
		s := service(t, d)
		_, err := register(s, key(1))
		code(t, err, "registration_reconciliation_required")
		_, err = s.Resume(context.Background(), key(1))
		code(t, err, "registration_reconciliation_required")
		if d.calls["email"] != 1 || d.ops[key(1)].Phase != EmailSending {
			t.Fatal("unclassified failure permitted email replay")
		}
	}
}
