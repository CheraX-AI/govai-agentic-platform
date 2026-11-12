package recovery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/registration"
)

var (
	ErrNotFound    = errors.New("recovery account unavailable")
	ErrUnavailable = errors.New("recovery unavailable")
	ErrInvalid     = errors.New("invalid or expired recovery request")
)

type private struct{}

// Format hides the value when it is printed with fmt (Println, Printf, Sprintf).
func (private) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED]")
}

// MarshalJSON hides the value when it is converted to JSON.
func (private) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// LogValue hides the value when it is written to a slog log.
func (private) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

type Binding struct {
	private
	Issuer, Subject, OperationID string
}

type Challenge struct {
	private
	Account  Binding
	EmailTag [32]byte
}

type Delivery struct {
	private
	Recipient, Link string
	Lifetime        time.Duration
}

// Ports must map failures to safe errors; implementations must not log inputs.
type Accounts interface {
	Lookup(context.Context, string, registration.NIC) (Binding, error)
}

type Provider interface {
	Recipient(context.Context, Binding) (string, error)
	Reset(context.Context, Binding, Password) error
	Logout(context.Context, Binding) error
}

type Challenges interface {
	Issue(context.Context, Challenge, time.Duration) (Token, error)
	Claim(context.Context, Token) (Challenge, error)
	Finish(context.Context, Token, bool) error
}

type Mailer interface {
	Send(context.Context, Delivery) error
}

// LocalSessions must durably revoke browser sessions for this validated identity.
// It must also serialize safely with in-flight refresh/login before returning.
type LocalSessions interface {
	Revoke(context.Context, Binding) error
}

type Config struct {
	private
	Issuer, ResetURL  string
	BindingKey        []byte
	Lifetime, Timeout time.Duration
}

type Service struct {
	private
	cfg      Config
	accounts Accounts
	provider Provider
	tokens   Challenges
	mail     Mailer
	sessions LocalSessions
}

type Receipt struct {
	Status      string `json:"status"`
	MaskedEmail string `json:"masked_email,omitempty"`
}

var bindingSegment = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func validBinding(b Binding, issuer string) bool {
	return b.Issuer == issuer &&
		bindingSegment.MatchString(b.Subject) &&
		bindingSegment.MatchString(b.OperationID)
}

// NewService connects account lookup, reset storage, identity provider and mail.
func NewService(cfg Config, a Accounts, p Provider, t Challenges, m Mailer, l LocalSessions) (*Service, error) {
	for _, value := range []string{cfg.Issuer, cfg.ResetURL} {
		u, err := url.Parse(value)
		if err != nil ||
			u.Scheme != "https" ||
			u.Hostname() == "" ||
			u.User != nil ||
			u.RawQuery != "" ||
			u.ForceQuery ||
			u.Fragment != "" ||
			strings.ContainsAny(value, "\\\r\n \t") ||
			len(value) > 2048 {
			return nil, ErrUnavailable
		}
	}

	if len(cfg.BindingKey) != 32 ||
		cfg.Lifetime < time.Minute ||
		cfg.Lifetime > 30*time.Minute ||
		cfg.Timeout <= 0 ||
		cfg.Timeout > 30*time.Second ||
		a == nil ||
		p == nil ||
		t == nil ||
		m == nil ||
		l == nil {
		return nil, ErrUnavailable
	}

	cfg.BindingKey = append([]byte(nil), cfg.BindingKey...)
	return &Service{cfg: cfg, accounts: a, provider: p, tokens: t, mail: m, sessions: l}, nil
}

func (s *Service) tag(account Binding, email string) [32]byte {
	mac := hmac.New(sha256.New, s.cfg.BindingKey)
	data, _ := json.Marshal(
		[]string{"govai:recovery:email:v1", account.Issuer, account.Subject, account.OperationID, email},
	)
	_, _ = mac.Write(data)
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

func (s *Service) finish(token Token, success bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if s.tokens.Finish(ctx, token, success) != nil {
		return ErrUnavailable
	}

	return nil
}

// Request is internal orchestration, NOT a rate-limited public HTTP endpoint.
// Unknown accounts get a generic receipt; matching accounts intentionally reveal
// masked email under the agreed prototype design. Raw email/token never return.
func (s *Service) Request(ctx context.Context, nic string) (Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	if ctx.Err() != nil {
		return Receipt{}, ErrUnavailable
	}

	parsed, err := registration.ParseNIC(nic)
	if err != nil {
		return Receipt{}, ErrInvalid
	}

	account, err := s.accounts.Lookup(ctx, s.cfg.Issuer, parsed)
	if errors.Is(err, ErrNotFound) {
		return Receipt{Status: "received"}, nil
	}

	if err != nil || !validBinding(account, s.cfg.Issuer) {
		return Receipt{}, ErrUnavailable
	}

	email, err := s.provider.Recipient(ctx, account)
	if err != nil {
		return Receipt{}, ErrUnavailable
	}

	masked, err := MaskEmail(email)
	if err != nil {
		return Receipt{}, ErrUnavailable
	}

	token, err := s.tokens.Issue(ctx, Challenge{Account: account, EmailTag: s.tag(account, email)}, s.cfg.Lifetime)
	if err != nil {
		return Receipt{}, ErrUnavailable
	}

	u, _ := url.Parse(s.cfg.ResetURL)
	u.Fragment = "token=" + token.Reveal()
	if s.mail.Send(ctx, Delivery{Recipient: email, Link: u.String(), Lifetime: s.cfg.Lifetime}) != nil {
		// Failure may mean delivery happened. Spend the challenge rather than retrying
		// the mail or leaving an uncertain link authorized. Claim errors fail closed.
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if _, err := s.tokens.Claim(cleanup, token); err == nil {
			_ = s.finish(token, false)
		}

		return Receipt{}, ErrUnavailable
	}

	return Receipt{Status: "received", MaskedEmail: masked}, nil
}

// Reset consumes a reset challenge and updates the account password.
func (s *Service) Reset(ctx context.Context, rawToken, newPassword string) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	if ctx.Err() != nil {
		return ErrUnavailable
	}

	token, err := ParseToken(rawToken)
	if err != nil {
		return ErrInvalid
	}

	password, err := ParsePassword(newPassword)
	if err != nil {
		return ErrInvalid
	}

	challenge, err := s.tokens.Claim(ctx, token)
	if err != nil {
		return ErrInvalid
	}

	failed := func() error {

		_ = s.finish(token, false)
		return ErrUnavailable
	}
	if ctx.Err() != nil || !validBinding(challenge.Account, s.cfg.Issuer) {
		return failed()
	}

	email, err := s.provider.Recipient(ctx, challenge.Account)
	if err != nil {
		return failed()
	}

	tag := s.tag(challenge.Account, email)
	if !hmac.Equal(tag[:], challenge.EmailTag[:]) {
		return failed()
	}

	if s.sessions.Revoke(ctx, challenge.Account) != nil {
		return failed()
	}

	if s.provider.Reset(ctx, challenge.Account, password) != nil {
		return failed()
	}

	if s.provider.Logout(ctx, challenge.Account) != nil {
		return failed()
	}
	// Fence again after provider reset/logout: logins started during the reset
	// window must not leave usable local cookies, even if they finish later.
	if s.sessions.Revoke(ctx, challenge.Account) != nil {
		return failed()
	}

	return s.finish(token, true)
}
