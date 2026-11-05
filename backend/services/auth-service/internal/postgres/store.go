package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store must be shared per pool. At most four journal sessions hold pool
// connections, leaving capacity for independent short profile transactions.
type Store struct {
	pool  *pgxpool.Pool
	crypt cryptor
	slots chan struct{}
}

// NewStore prepares encrypted profiles and registration journal access.
func NewStore(p *pgxpool.Pool, dataKey, nicKey []byte) (*Store, error) {
	if p == nil || p.Config().MaxConns < 8 {
		return nil, ErrStore
	}

	c, e := newCryptor(dataKey, nicKey)
	if e != nil {
		return nil, e
	}

	return &Store{p, c, make(chan struct{}, 4)}, nil
}

var _ registration.RegistrationJournal = (*Store)(nil)

var _ registration.ProfileStore = (*Store)(nil)

var journalKey = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

type session struct {
	mu       sync.Mutex
	store    *Store
	conn     *pgxpool.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	stop     chan struct{}
	done     chan struct{}
	snapshot registration.Operation
	lock     int64
	closed   bool
	lost     bool
}

func (s *Store) acquire(ctx context.Context, issuer, key string) (*session, error) {
	if issuer == "" || !journalKey.MatchString(key) {
		return nil, ErrStore
	}

	select {
	case s.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ErrStore
	}

	c, e := s.pool.Acquire(ctx)
	if e != nil {
		<-s.slots
		return nil, ErrStore
	}

	hash := sha256.Sum256(aad(issuer, strings.ToLower(key)))
	lock := int64(binary.BigEndian.Uint64(hash[:8]))
	// Poll rather than block an SQL statement indefinitely. Cancellation is bounded.
	for {
		var ok bool
		e = c.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lock).Scan(&ok)
		if e != nil {
			c.Release()
			<-s.slots
			return nil, ErrStore
		}

		if ok {
			break
		}

		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			c.Release()
			<-s.slots
			return nil, ErrStore
		case <-timer.C:
		}
	}

	owned, cancel := context.WithCancel(ctx)
	ss := &session{
		store:  s,
		conn:   c,
		ctx:    owned,
		cancel: cancel,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		lock:   lock,
	}
	go ss.monitor()
	return ss, nil
}

// A failed heartbeat cancels ownership. No lease expiry permits another owner
// while this DB session remains live. Network partitions cannot provide remote
// fencing in Keycloak; intent checkpoints still prevent duplicate create/send.
func (s *session) monitor() {
	defer close(s.done)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-s.ctx.Done():
			return
		case <-tick.C:
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			ctx, cancel := context.WithTimeout(s.ctx, 500*time.Millisecond)
			e := s.conn.Ping(ctx)
			cancel()
			if e != nil {
				s.lost = true
				s.cancel()
			}
			s.mu.Unlock()
			if e != nil {
				return
			}
		}
	}
}

func (s *session) Context() context.Context { return s.ctx }

func (s *session) Snapshot() registration.Operation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot
}

func (s *session) Close(ctx context.Context) error {
	ctx, cancelClose := context.WithTimeout(ctx, time.Second)
	defer cancelClose()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}

	s.closed = true
	close(s.stop)
	s.cancel()
	var ok bool
	e := s.conn.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", s.lock).Scan(&ok)
	if e != nil || !ok || s.lost {
		raw := s.conn.Hijack()
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = raw.Close(cleanup)
		cancel()
	} else {
		s.conn.Release()
	}

	<-s.store.slots
	s.mu.Unlock()
	<-s.done
	if e != nil || !ok || s.lost {
		return ErrStore
	}

	return nil
}

const columns = `operation_id,issuer,idempotency_key::text,fingerprint,pending_ciphertext,revision,phase,
 COALESCE(subject,''),email_attempts,next_email_attempt,email_state,nic_digest`

func (s *Store) read(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, issuer, key string) (registration.Operation, error) {
	var o registration.Operation
	var fp, encrypted, digest []byte
	var rev int64
	var next *time.Time
	e := q.QueryRow(
		ctx,
		"SELECT "+columns+" FROM auth.registration_operations WHERE issuer=$1 AND idempotency_key=$2::uuid",
		issuer,
		key,
	).Scan(
		&o.OperationID,
		&o.Issuer,
		&o.Key,
		&fp,
		&encrypted,
		&rev,
		&o.Phase,
		&o.Subject,
		&o.EmailAttempts,
		&next,
		&o.EmailState,
		&digest)
	if errors.Is(e, pgx.ErrNoRows) {
		return o, registration.ErrOperationMissing
	}

	if e != nil || len(fp) != 32 || rev < 0 {
		return o, ErrStore
	}

	o.Pending, e = s.crypt.open(encrypted, o.Issuer, o.OperationID)
	if e != nil || !hmac.Equal(digest, s.crypt.digest(o.Pending.NIC)) {
		return o, ErrStore
	}

	copy(o.Fingerprint[:], fp)
	o.Revision = uint64(rev)
	if next != nil {
		o.NextEmailAttempt = *next
	}

	return o, nil
}

func (s *Store) Open(ctx context.Context, issuer, key string) (registration.OperationSession, error) {
	ss, e := s.acquire(ctx, issuer, key)
	if e != nil {
		return nil, e
	}

	ss.mu.Lock()
	o, e := s.read(ss.ctx, ss.conn, issuer, key)
	ss.snapshot = o
	ss.mu.Unlock()
	if e != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = ss.Close(cleanup)
		cancel()
		return nil, e
	}

	return ss, nil
}

// Reserve atomically claims the registration operation and NIC.
func (s *Store) Reserve(ctx context.Context, r registration.Reservation) (registration.OperationSession, error) {
	if r.OperationID == "" || r.Pending.NIC.NormalizedNumber() == "" {
		return nil, ErrStore
	}

	ss, e := s.acquire(ctx, r.Issuer, r.Key)
	if e != nil {
		return nil, e
	}

	ss.mu.Lock()
	e = s.reserveLocked(ss, r)
	ss.mu.Unlock()
	if e != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = ss.Close(cleanup)
		cancel()
		return nil, e
	}

	return ss, nil
}

func (s *Store) reserveLocked(ss *session, r registration.Reservation) error {
	o, e := s.read(ss.ctx, ss.conn, r.Issuer, r.Key)
	if e == nil {
		if o.Fingerprint != r.Fingerprint {
			return registration.ErrRequestConflict
		}

		ss.snapshot = o
		return nil
	}

	if !errors.Is(e, registration.ErrOperationMissing) {
		return e
	}

	encrypted, e := s.crypt.seal(r.Pending, r.Issuer, r.OperationID)
	if e != nil {
		return e
	}
	// A single row contains journal reservation AND globally unique NIC claim.
	_, e = ss.conn.Exec(ss.ctx, `INSERT INTO auth.registration_operations
 (operation_id,issuer,idempotency_key,fingerprint,nic_digest,pending_ciphertext)
 VALUES($1,$2::text,$3::uuid,$4,$5,$6)`,
		r.OperationID,
		r.Issuer,
		r.Key,
		r.Fingerprint[:],
		s.crypt.digest(r.Pending.NIC),
		encrypted)
	if e != nil {
		var pe *pgconn.PgError
		if errors.As(e, &pe) &&
			pe.Code == "23505" &&
			pe.ConstraintName == "registration_operations_nic_digest_key" {
			return registration.ErrNICConflict
		}

		return ErrStore
	}

	o, e = s.read(ss.ctx, ss.conn, r.Issuer, r.Key)
	if e != nil {
		return e
	}

	ss.snapshot = o
	return nil
}

func (s *session) Save(ctx context.Context, next registration.Operation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.snapshot
	if s.closed ||
		s.lost ||
		s.ctx.Err() != nil ||
		ctx.Err() != nil ||
		next.Revision != old.Revision+1 ||
		next.Revision > 1<<63-1 ||

		next.OperationID != old.OperationID ||
		next.Issuer != old.Issuer ||
		next.Key != old.Key ||
		next.Fingerprint != old.Fingerprint ||
		!sameProfile(next.Pending, old.Pending) {
		return ErrStore
	}

	work, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	defer cancel()
	var subject any
	if next.Subject != "" {
		subject = next.Subject
	}

	var nextTime any
	if !next.NextEmailAttempt.IsZero() {
		nextTime = next.NextEmailAttempt
	}

	tag, e := s.conn.Exec(work, `UPDATE auth.registration_operations SET revision=$1,phase=$2,subject=$3,
 email_attempts=$4,next_email_attempt=$5,email_state=$6 WHERE operation_id=$7 AND revision=$8`,
		int64(next.Revision),
		next.Phase,
		subject,
		next.EmailAttempts,
		nextTime,
		next.EmailState,
		old.OperationID,
		int64(old.Revision))
	if e != nil || tag.RowsAffected() != 1 {
		// Uncertain commits must be reloaded under a new session, never retried here.
		s.lost = true
		s.cancel()
		return ErrStore
	}

	s.snapshot = next
	return nil
}

// Persist writes the profile and its identity binding atomically.
func (s *Store) Persist(ctx context.Context, r registration.ProfileRecord) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return ErrStore
	}

	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var key string
	if e = tx.QueryRow(
		ctx,
		"SELECT idempotency_key::text FROM auth.registration_operations WHERE operation_id=$1 FOR UPDATE",
		r.OperationID,
	).Scan(&key); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return registration.ErrProfileConflict
		}

		return ErrStore
	}

	o, e := s.read(ctx, tx, r.Issuer, key)
	if e != nil {
		return e
	}

	if o.OperationID != r.OperationID ||
		o.Subject == "" ||
		o.Subject != r.Subject ||
		!sameProfile(o.Pending, r.Data) {
		return registration.ErrProfileConflict
	}

	var existing []byte
	var issuer, subject string
	e = tx.QueryRow(
		ctx,
		"SELECT issuer,subject,profile_ciphertext FROM auth.citizen_profiles WHERE operation_id=$1",
		r.OperationID,
	).Scan(&issuer, &subject, &existing)
	if e == nil {
		p, err := s.crypt.open(existing, issuer, r.OperationID)
		if err != nil || issuer != r.Issuer || subject != r.Subject || !sameProfile(p, r.Data) {
			return registration.ErrProfileConflict
		}
	} else if errors.Is(e, pgx.ErrNoRows) {
		if o.Phase != registration.SubjectRecorded {
			return registration.ErrProfileConflict
		}

		encrypted, err := s.crypt.seal(r.Data, r.Issuer, r.OperationID)
		if err != nil {
			return err
		}

		_, e = tx.Exec(
			ctx,
			`INSERT INTO auth.citizen_profiles(operation_id,issuer,subject,nic_digest,profile_ciphertext) VALUES($1,$2,$3,$4,$5)`,
			r.OperationID,
			r.Issuer,
			r.Subject,
			s.crypt.digest(r.Data.NIC),
			encrypted,
		)
		if e != nil {
			var pe *pgconn.PgError
			if errors.As(e, &pe) && pe.Code == "23505" {
				return registration.ErrProfileConflict
			}

			return ErrStore
		}
	} else {
		return ErrStore
	}

	if tx.Commit(ctx) != nil {
		return ErrStore
	}

	return nil
}
