package registration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
)

var (
	ErrRequestConflict  = errors.New("registration request conflict")
	ErrProfileConflict  = errors.New("registration profile conflict")
	ErrOperationMissing = errors.New("registration operation missing")
)

// privateValue prevents accidental logging/serialization of persistence DTOs.
// Adapters must explicitly map allowed fields; it is not an encryption mechanism.
type privateValue struct{}

func (privateValue) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[REDACTED]") }

func (privateValue) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }

func (privateValue) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

// ProfileData has no identity credential, token or email fields. Pending copies
// must be access-restricted/encrypted so recovery needs no original HTTP body.
type ProfileData struct {
	privateValue
	FullName     string
	NIC          NIC
	MobileNumber string
}

type ProfileRecord struct {
	privateValue
	OperationID, Issuer, Subject string
	Data                         ProfileData
}

// ProfileStore.Persist commits the profile AND the NIC-to-issuer/subject binding
// atomically, verifying the existing journal/NIC reservation owner. Same exact
// operation+identity+data is idempotent. Conflicts must never upsert/overwrite a
// different owner. Success means durable commit, not buffered work. Journal phase
// advancement follows separately; recovery may repeat this idempotent write.
type ProfileStore interface {
	Persist(context.Context, ProfileRecord) error
}

type Phase string

const (
	Reserved        Phase = "reserved"
	Creating        Phase = "creating"
	SubjectRecorded Phase = "subject_recorded"
	ProfileReady    Phase = "profile_ready"
	Enabling        Phase = "enabling"
	Enabled         Phase = "enabled"
	EmailSending    Phase = "email_sending"
	Completed       Phase = "completed"
	Duplicate       Phase = "duplicate"
	Rejected        Phase = "rejected"
	Attention       Phase = "attention"
)

type Reservation struct {
	privateValue
	Issuer, Key, OperationID string
	Fingerprint              [32]byte // keyed digest of canonical non-password fields only
	Pending                  ProfileData
}

type Operation struct {
	Reservation
	Revision         uint64
	Phase            Phase
	Subject          string
	EmailAttempts    int
	NextEmailAttempt time.Time
	EmailState       string // empty, request_accepted, or already_verified
}

// RegistrationJournal combines the earlier NICReservations.Reserve obligation
// with journal reservation in ONE transaction. Key uniqueness is (issuer,key);
// normalized NIC uniqueness covers the entire GovAI citizen namespace, including
// pending/uncertain operations, independently of email or issuer. Same-key input
// mismatch -> ErrRequestConflict; other-key NIC claim -> ErrNICConflict.
// Reserve returns the original immutable operation on replay, not the proposed
// replacement OperationID/Pending. Open is for trusted internal recovery only.
type RegistrationJournal interface {
	Reserve(context.Context, Reservation) (OperationSession, error)
	Open(context.Context, string, string) (OperationSession, error)
}

// OperationSession holds exclusive operation ownership across network calls.
// Implement with a dedicated DB/session lock, NOT a transaction left open during
// HTTP. Every Save is an independent committed revision CAS; it updates Snapshot
// only after commit. Save must reject changed reservation fields/subject rebinding
// and invalid transitions. Close releases ownership even after cancellation.
// Lost ownership must cancel the callback context and reject further commits.
// A lease must not simply expire and allow overlapping mutations. Creating and
// EmailSending checkpoints additionally prevent external POST replay on takeover.
type OperationSession interface {
	// Context derives from acquisition and is also canceled on ownership loss.
	Context() context.Context
	Snapshot() Operation
	Save(context.Context, Operation) error
	Close(context.Context) error
}
