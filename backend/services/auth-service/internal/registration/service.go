package registration

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type ServiceConfig struct {
	privateValue
	Issuer            string
	FingerprintKey    []byte // server-only, >=32 random bytes; never password-derived
	Timeout           time.Duration
	EmailRetryBackoff time.Duration
	MaxEmailAttempts  int
	Now               func() time.Time
}

type Service struct {
	privateValue
	cfg      ServiceConfig
	admin    Admin
	profiles ProfileStore
	journal  RegistrationJournal
}

type Receipt struct {
	Status string `json:"status"`
}
type ServiceError struct{ Code string }

func (e *ServiceError) Error() string { return e.Code }
func serviceError(code string) error  { return &ServiceError{Code: code} }

var keyPattern = regexp.MustCompile(`^[A-Fa-f0-9]{8}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{12}$`)
var identityPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func NewService(cfg ServiceConfig, admin Admin, profiles ProfileStore, journal RegistrationJournal) (*Service, error) {
	u, err := url.Parse(cfg.Issuer)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || strings.ContainsAny(cfg.Issuer, "\\ \t\r\n#") || len(cfg.FingerprintKey) < 32 || cfg.Timeout <= 0 || cfg.Timeout > 30*time.Second || cfg.EmailRetryBackoff <= 0 || cfg.EmailRetryBackoff > time.Hour || cfg.MaxEmailAttempts < 1 || cfg.MaxEmailAttempts > 10 || admin == nil || profiles == nil || journal == nil {
		return nil, serviceError("registration_configuration")
	}
	cfg.FingerprintKey = append([]byte(nil), cfg.FingerprintKey...)
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Service{cfg: cfg, admin: admin, profiles: profiles, journal: journal}, nil
}

// Register accepts the existing strict, bounded request parser. No identity,
// role, group or operation marker comes from caller JSON. Password is passed
// only to Admin.CreateDisabled, never into a persistence DTO or fingerprint.
func (s *Service) Register(ctx context.Context, key string, body io.Reader) (Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	if ctx.Err() != nil {
		return Receipt{}, serviceError("registration_canceled")
	}
	if !keyPattern.MatchString(key) {
		return Receipt{}, serviceError("registration_invalid_key")
	}
	r, inputErr := ParseRequest(body)
	if inputErr != nil {
		return Receipt{}, inputErr
	}
	data := ProfileData{FullName: r.FullName, NIC: r.NIC, MobileNumber: r.MobileNumber}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return Receipt{}, serviceError("registration_unavailable")
	}
	reservation := Reservation{Issuer: s.cfg.Issuer, Key: strings.ToLower(key), OperationID: hex.EncodeToString(random[:]), Fingerprint: s.fingerprint(r), Pending: data}
	session, err := s.journal.Reserve(ctx, reservation)
	if err != nil {
		if errors.Is(err, ErrNICConflict) {
			return Receipt{Status: "received"}, nil
		}
		if errors.Is(err, ErrRequestConflict) {
			return Receipt{}, serviceError("registration_request_conflict")
		}
		return Receipt{}, s.storeError(ctx)
	}
	return s.withSession(ctx, session, &r)
}

// Resume is an INTERNAL recovery entry point. It loads only password-free state.
// Reserved operations need credential resubmission through Register. Ambiguous
// create/email checkpoints never become a blind retry merely because of time.
func (s *Service) Resume(ctx context.Context, key string) (Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	if !keyPattern.MatchString(key) {
		return Receipt{}, serviceError("registration_invalid_key")
	}
	session, err := s.journal.Open(ctx, s.cfg.Issuer, strings.ToLower(key))
	if err != nil {
		return Receipt{}, s.storeError(ctx)
	}
	return s.withSession(ctx, session, nil)
}

func (s *Service) fingerprint(r Request) [32]byte {
	// Explicit non-secret allowlist: no password or password-derived material.
	body, _ := json.Marshal([]string{s.cfg.Issuer, r.FullName, r.NIC.NormalizedNumber(), r.MobileNumber, r.Email})
	h := hmac.New(sha256.New, s.cfg.FingerprintKey)
	_, _ = h.Write(body)
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func (s *Service) storeError(ctx context.Context) error {
	if ctx.Err() != nil {
		return serviceError("registration_canceled")
	}
	return serviceError("registration_unavailable")
}

func (s *Service) withSession(ctx context.Context, session OperationSession, r *Request) (receipt Receipt, err error) {
	if session == nil {
		return Receipt{}, serviceError("registration_unavailable")
	}
	defer func() {
		release, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if e := session.Close(release); e != nil && err == nil {
			receipt = Receipt{}
			err = serviceError("registration_unavailable")
		}
	}()
	ownership := session.Context()
	if ownership == nil {
		return Receipt{}, serviceError("registration_unavailable")
	}
	work, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(ownership, cancel)
	defer stop()
	defer cancel()
	ctx = work
	if ownership.Err() != nil {
		return Receipt{}, serviceError("registration_canceled")
	}
	// A bounded state machine; every forward checkpoint is committed before its
	// dependent external effect. It never loops to retry a failed network call.
	for step := 0; step < 16; step++ {
		if ctx.Err() != nil {
			return Receipt{}, s.storeError(ctx)
		}
		op := session.Snapshot()
		if op.Issuer != s.cfg.Issuer || !identityPattern.MatchString(op.OperationID) || op.Pending.NIC.NormalizedNumber() == "" {
			return Receipt{}, serviceError("registration_reconciliation_required")
		}
		save := func(next Operation) error {
			next.Revision = op.Revision + 1
			if e := session.Save(ctx, next); e != nil {
				return s.storeError(ctx)
			}
			return nil
		}
		switch op.Phase {
		case Completed, Duplicate:
			return Receipt{Status: "received"}, nil
		case Rejected:
			return Receipt{}, serviceError("registration_policy_rejected")
		case Attention:
			return Receipt{}, serviceError("registration_attention_required")
		case Reserved:
			if r == nil {
				return Receipt{}, serviceError("registration_credentials_required")
			}
			if op.Fingerprint != s.fingerprint(*r) {
				return Receipt{}, serviceError("registration_request_conflict")
			}
			op.Phase = Creating
			if err = save(op); err != nil {
				return Receipt{}, err
			}
			// From this point, a crash/canceled journal write leaves Creating.
			// Another process must reconcile, never call CreateDisabled again.
			u, e := s.admin.CreateDisabled(ctx, *r, op.OperationID)
			if e != nil {
				var ae *AdminError
				if !errors.As(e, &ae) || ae.UnknownOutcome {
					return Receipt{}, serviceError("registration_reconciliation_required")
				}
				op = session.Snapshot()
				switch ae.Code {
				case AdminDuplicate:
					op.Phase = Duplicate
				case AdminPolicyRejected:
					op.Phase = Rejected
				case AdminCanceled, AdminPermission, AdminInvalid, AdminUpstream:
					op.Phase = Reserved
				default:
					return Receipt{}, serviceError("registration_reconciliation_required")
				}
				op.Revision++
				if err = session.Save(ctx, op); err != nil {
					return Receipt{}, s.storeError(ctx)
				}
				if op.Phase == Duplicate {
					return Receipt{Status: "received"}, nil
				}
				if op.Phase == Rejected {
					return Receipt{}, serviceError("registration_policy_rejected")
				}
				return Receipt{}, s.storeError(ctx)
			}
			if !identityPattern.MatchString(u.Subject) || u.Enabled || u.EmailVerified {
				return Receipt{}, serviceError("registration_reconciliation_required")
			}
			op = session.Snapshot()
			op.Phase = SubjectRecorded
			op.Subject = u.Subject
			op.Revision++
			if err = session.Save(ctx, op); err != nil {
				return Receipt{}, s.storeError(ctx)
			}
		case Creating:
			u, e := s.admin.FindOwned(ctx, op.OperationID)
			if e != nil || !identityPattern.MatchString(u.Subject) || u.Enabled || u.EmailVerified {
				return Receipt{}, serviceError("registration_reconciliation_required")
			}
			op.Phase = SubjectRecorded
			op.Subject = u.Subject
			if err = save(op); err != nil {
				return Receipt{}, err
			}
		case SubjectRecorded:
			u, e := s.admin.ReadOwned(ctx, op.Subject, op.OperationID)
			if e != nil || u.Subject != op.Subject || u.Enabled {
				return Receipt{}, serviceError("registration_reconciliation_required")
			}
			e = s.profiles.Persist(ctx, ProfileRecord{OperationID: op.OperationID, Issuer: op.Issuer, Subject: op.Subject, Data: op.Pending})
			if e != nil {
				if errors.Is(e, ErrProfileConflict) {
					return Receipt{}, serviceError("registration_reconciliation_required")
				}
				return Receipt{}, s.storeError(ctx)
			}
			op.Phase = ProfileReady
			if err = save(op); err != nil {
				return Receipt{}, err
			}
		case ProfileReady:
			op.Phase = Enabling
			if err = save(op); err != nil {
				return Receipt{}, err
			}
		case Enabling:
			u, e := s.admin.ReadOwned(ctx, op.Subject, op.OperationID)
			if e != nil || u.Subject != op.Subject {
				return Receipt{}, serviceError("registration_reconciliation_required")
			}
			if !u.Enabled {
				if e = s.admin.EnableOwned(ctx, op.Subject, op.OperationID); e != nil {
					return Receipt{}, s.storeError(ctx)
				}
			}
			op.Phase = Enabled
			if err = save(op); err != nil {
				return Receipt{}, err
			}
		case Enabled:
			u, e := s.admin.ReadOwned(ctx, op.Subject, op.OperationID)
			if e != nil || u.Subject != op.Subject || !u.Enabled {
				return Receipt{}, serviceError("registration_reconciliation_required")
			}
			if u.EmailVerified {
				op.Phase = Completed
				op.EmailState = "already_verified"
				if err = save(op); err != nil {
					return Receipt{}, err
				}
				continue
			}
			if op.EmailAttempts >= s.cfg.MaxEmailAttempts {
				op.Phase = Attention
				if err = save(op); err != nil {
					return Receipt{}, err
				}
				return Receipt{}, serviceError("registration_attention_required")
			}
			if s.cfg.Now().Before(op.NextEmailAttempt) {
				return Receipt{}, serviceError("registration_verification_pending")
			}
			op.Phase = EmailSending
			op.EmailAttempts++
			if err = save(op); err != nil {
				return Receipt{}, err
			}
			e = s.admin.SendVerification(ctx, op.Subject, op.OperationID)
			op = session.Snapshot()
			if e != nil {
				var ae *AdminError
				if !errors.As(e, &ae) || ae.UnknownOutcome {
					return Receipt{}, serviceError("registration_reconciliation_required")
				}
				switch ae.Code {
				case AdminPermission, AdminInvalid, AdminCanceled, AdminUpstream, AdminPolicyRejected:
					// Explicit adapter outcomes guaranteeing no email acceptance.
				default:
					return Receipt{}, serviceError("registration_reconciliation_required")
				}
				op.Phase = Enabled
				op.NextEmailAttempt = s.cfg.Now().Add(s.cfg.EmailRetryBackoff * time.Duration(op.EmailAttempts))
				op.Revision++
				if err = session.Save(ctx, op); err != nil {
					return Receipt{}, s.storeError(ctx)
				}
				return Receipt{}, s.storeError(ctx)
			}
			op.Phase = Completed
			op.EmailState = "request_accepted"
			op.Revision++
			if err = session.Save(ctx, op); err != nil {
				return Receipt{}, s.storeError(ctx)
			}
		case EmailSending:
			// SMTP has no readback/idempotency guarantee. A prior accepted request
			// may already have sent email. Verification can resolve the need, but
			// an unverified account must not trigger an automatic resend here.
			u, e := s.admin.ReadOwned(ctx, op.Subject, op.OperationID)
			if e != nil || u.Subject != op.Subject || !u.EmailVerified {
				return Receipt{}, serviceError("registration_reconciliation_required")
			}
			op.Phase = Completed
			op.EmailState = "already_verified"
			if err = save(op); err != nil {
				return Receipt{}, err
			}
		default:
			return Receipt{}, serviceError("registration_reconciliation_required")
		}
	}
	return Receipt{}, serviceError("registration_reconciliation_required")
}
