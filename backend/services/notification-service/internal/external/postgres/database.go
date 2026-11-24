package postgres

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrConfiguration = errors.New("invalid notification database configuration")
	ErrUnavailable   = errors.New("notification database unavailable")
)

type Config struct {
	Host           string
	Port           int
	Username       string
	Password       string
	Database       string
	TLSMode        string
	CAFile         string
	Development    bool
	MaxConnections int32
	Timeout        time.Duration
}

// Format prevents passwords from appearing in ordinary diagnostic formatting.

func (Config) Format(output fmt.State, _ rune) {
	_, _ = io.WriteString(output, "[REDACTED database settings]")
}

func (Config) LogValue() slog.Value {
	return slog.StringValue("[REDACTED database settings]")
}

func (Config) MarshalJSON() ([]byte, error) {
	return []byte("\"[REDACTED database settings]\""), nil
}

type Database struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// Validate checks settings without opening a connection or creating any tables.

func Validate(settings Config) error {
	_, err := poolConfig(settings)
	return err
}

func poolConfig(settings Config) (*pgxpool.Config, error) {
	if settings.Host == "" || strings.ContainsAny(settings.Host, "/\\?#@ \t\r\n") ||
		settings.Port < 1 || settings.Port > 65535 ||
		strings.TrimSpace(settings.Username) == "" || settings.Password == "" ||
		settings.Database != "govai_notifications" ||
		settings.MaxConnections < 1 || settings.MaxConnections > 32 ||
		settings.Timeout < 100*time.Millisecond || settings.Timeout > 30*time.Second {
		return nil, ErrConfiguration
	}
	local := settings.Host == "localhost" || settings.Host == "127.0.0.1" ||
		settings.Host == "::1" || settings.Host == "notification-postgres"
	if settings.TLSMode != "verify-full" &&
		!(settings.TLSMode == "disable" && settings.Development && local) {
		return nil, ErrConfiguration
	}
	endpoint := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(settings.Host, strconv.Itoa(settings.Port)),
		User:   url.UserPassword(settings.Username, settings.Password),
		Path:   "/" + settings.Database,
	}
	query := url.Values{"sslmode": {settings.TLSMode}}
	if settings.CAFile != "" {
		query.Set("sslrootcert", settings.CAFile)
	}
	endpoint.RawQuery = query.Encode()
	parsed, err := pgxpool.ParseConfig(endpoint.String())
	if err != nil {
		return nil, ErrConfiguration
	}

	// Explicit limits and session settings keep service behavior predictable.

	parsed.MaxConns = settings.MaxConnections
	parsed.MinConns = 0
	parsed.MaxConnLifetime = 30 * time.Minute
	parsed.MaxConnIdleTime = 5 * time.Minute
	parsed.ConnConfig.ConnectTimeout = settings.Timeout
	if parsed.ConnConfig.TLSConfig != nil {
		parsed.ConnConfig.TLSConfig.MinVersion = tls.VersionTLS12
	}
	parsed.ConnConfig.RuntimeParams = map[string]string{
		"application_name":  "govai-notification-service",
		"timezone":          "UTC",
		"statement_timeout": strconv.FormatInt(settings.Timeout.Milliseconds(), 10),
		"search_path":       "public",
	}
	return parsed, nil
}

// Open confirms connectivity; a failed probe closes the newly created pool.

func Open(ctx context.Context, settings Config) (*Database, error) {
	parsed, err := poolConfig(settings)
	if err != nil {
		return nil, err
	}
	probe, cancel := context.WithTimeout(ctx, settings.Timeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(probe, parsed)
	if err != nil {
		return nil, ErrUnavailable
	}
	if err := pool.Ping(probe); err != nil {
		pool.Close()
		return nil, ErrUnavailable
	}
	return &Database{pool: pool, timeout: settings.Timeout}, nil
}

// Ping reports connectivity only; it does not certify migrations or inbox readiness.

func (database *Database) Ping(ctx context.Context) error {
	probe, cancel := context.WithTimeout(ctx, database.timeout)
	defer cancel()
	if database.pool.Ping(probe) != nil {
		return ErrUnavailable
	}
	return nil
}

func (database *Database) Close() {
	database.pool.Close()
}
