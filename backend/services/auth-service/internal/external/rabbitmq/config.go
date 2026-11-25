package rabbitmq

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"regexp"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
)

type private struct{}

func (private) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED publisher settings]")
}

func (private) LogValue() slog.Value {
	return slog.StringValue("[REDACTED publisher settings]")
}

func (private) MarshalJSON() ([]byte, error) {
	return []byte("\"[REDACTED publisher settings]\""), nil
}

type Config struct {
	private
	Host        string
	Port        int
	Username    string
	Password    string
	Scheme      string
	VHost       string
	CAFile      string
	Issuer      string
	Timeout     time.Duration
	Development bool
}

type Publisher struct {
	private
	settings Config
	tls      *tls.Config
}

var hostname = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

// New validates transport policy without connecting or declaring broker topology.

func New(settings Config) (*Publisher, error) {
	if settings.Host == "" ||
		(net.ParseIP(settings.Host) == nil && !hostname.MatchString(settings.Host)) ||
		settings.Port < 1 || settings.Port > 65535 || settings.Username == "" ||
		settings.Password == "" || settings.VHost != "govai" ||
		settings.Timeout < 100*time.Millisecond || settings.Timeout > 10*time.Second ||
		contract.ValidateIssuer(settings.Issuer) != nil {
		return nil, notificationevents.ErrConfiguration
	}
	local := settings.Host == "127.0.0.1" || settings.Host == "localhost" ||
		settings.Host == "::1" || settings.Host == "rabbitmq"
	if settings.Scheme != "amqps" &&
		!(settings.Scheme == "amqp" && settings.Development && local) {
		return nil, notificationevents.ErrConfiguration
	}
	secure := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: settings.Host}
	if settings.CAFile != "" {
		data, err := os.ReadFile(settings.CAFile)
		if err != nil {
			return nil, notificationevents.ErrConfiguration
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(data) {
			return nil, notificationevents.ErrConfiguration
		}
		secure.RootCAs = roots
	}
	return &Publisher{settings: settings, tls: secure}, nil
}
