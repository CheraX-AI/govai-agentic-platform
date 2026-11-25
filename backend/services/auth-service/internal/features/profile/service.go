package profile

import (
	"context"
	"errors"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/registration"
)

var ErrUnavailable = errors.New("profile temporarily unavailable")
var ErrRulesUnavailable = errors.New("profile field validation contract unavailable")

type Caller struct {
	privateData
	Issuer, Subject, Email string
}

type Snapshot struct {
	privateData
	Registered registration.ProfileData
	Details    Details
	Email      string // verified Keycloak claim only; never application persistence
}

type Persistence interface {
	RegisteredProfile(context.Context, string, string) (registration.ProfileData, error)
	CitizenDetails(context.Context, string, string) (Details, error)
	SaveCitizenDetails(context.Context, string, string, int64, EditableDetails) (Details, error)
}

// FieldRules supplies NIC/DOB correspondence and approved gender choices.
// Production uses CitizenFieldRules. An unconfigured validator fails closed
// for new nonempty DOB/gender values rather than treating syntax as validation.
type FieldRules interface {
	ValidateDOB(registration.NIC, string) error
	ValidateGender(string) error
}

type Service struct {
	store Persistence
	rules FieldRules
}

func NewService(store Persistence, rules FieldRules) (*Service, error) {
	if store == nil {
		return nil, ErrUnavailable
	}
	return &Service{store: store, rules: rules}, nil
}

func (s *Service) Read(ctx context.Context, caller Caller) (Snapshot, error) {
	if caller.Issuer == "" || caller.Subject == "" {
		return Snapshot{}, ErrNotFound
	}
	registered, err := s.store.RegisteredProfile(ctx, caller.Issuer, caller.Subject)
	if err != nil {
		return Snapshot{}, err
	}
	details, err := s.store.CitizenDetails(ctx, caller.Issuer, caller.Subject)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Registered: registered, Details: details, Email: caller.Email}, nil
}

func (s *Service) Update(ctx context.Context, caller Caller, request UpdateRequest) (Snapshot, error) {
	if request.Revision < 0 || request.Revision > MaxClientRevision || request.Fields.Validate(time.Now()) != nil {
		return Snapshot{}, ErrInvalid
	}
	current, err := s.Read(ctx, caller)
	if err != nil {
		return Snapshot{}, err
	}
	if current.Details.Revision != request.Revision {
		return Snapshot{}, ErrConflict
	}
	if s.rules != nil {
		// Include empty values so an approved policy can enforce required fields.
		if err := s.rules.ValidateDOB(current.Registered.NIC, request.Fields.DateOfBirth); err != nil {
			return Snapshot{}, err
		}
		if err := s.rules.ValidateGender(request.Fields.Gender); err != nil {
			return Snapshot{}, err
		}
	} else if (request.Fields.DateOfBirth != "" && request.Fields.DateOfBirth != current.Details.Fields.DateOfBirth) ||
		(request.Fields.Gender != "" && request.Fields.Gender != current.Details.Fields.Gender) {
		return Snapshot{}, ErrRulesUnavailable
	}
	saved, err := s.store.SaveCitizenDetails(ctx, caller.Issuer, caller.Subject, request.Revision, request.Fields)
	if err != nil {
		return Snapshot{}, err
	}
	current.Details = saved
	return current, nil
}
