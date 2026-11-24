package messaging

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var ErrConnection = errors.New("notification broker connection unavailable")

type Settings struct {
	URL              string
	CAFile           string
	SecurityPrefetch int
	NormalPrefetch   int
}

type Client struct {
	settings   Settings
	tls        *tls.Config
	logger     *slog.Logger
	mu         sync.RWMutex
	connection *amqp.Connection
	security   *amqp.Channel
	normal     *amqp.Channel
}

// New checks TLS settings before starting the connection supervisor.

func New(settings Settings, logger *slog.Logger) (*Client, error) {
	secure := &tls.Config{MinVersion: tls.VersionTLS12}
	if settings.CAFile != "" {
		data, err := os.ReadFile(settings.CAFile)
		if err != nil {
			return nil, ErrConnection
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(data) {
			return nil, ErrConnection
		}
		secure.RootCAs = roots
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{settings: settings, tls: secure, logger: logger}, nil
}

// Connected checks the connection and both lanes, without consuming messages.

func (c *Client) Connected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connection != nil && !c.connection.IsClosed() &&
		c.security != nil && !c.security.IsClosed() &&
		c.normal != nil && !c.normal.IsClosed()
}

// Run reconnects with bounded backoff until shutdown; broker errors stay private.

func (c *Client) Run(ctx context.Context) {
	delay := time.Second
	for ctx.Err() == nil {
		connection, err := c.connect(ctx)
		if err != nil {
			c.logger.Warn("notification broker unavailable; retrying")
			if !pause(ctx, delay) {
				return
			}
			delay = min(delay*2, 10*time.Second)
			continue
		}

		delay = time.Second
		c.logger.Info("notification broker connected", "processing_enabled", false)
		closed := connection.NotifyClose(make(chan *amqp.Error, 1))
		c.mu.RLock()
		securityClosed := c.security.NotifyClose(make(chan *amqp.Error, 1))
		normalClosed := c.normal.NotifyClose(make(chan *amqp.Error, 1))
		c.mu.RUnlock()
		select {
		case <-ctx.Done():
		case <-closed:
		case <-securityClosed:
		case <-normalClosed:
		}
		c.disconnect()
		if ctx.Err() == nil {
			c.logger.Warn("notification broker disconnected")
			if !pause(ctx, delay) {
				return
			}
		}
	}
}

func (c *Client) connect(ctx context.Context) (*amqp.Connection, error) {
	dial := func(network, address string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: 5 * time.Second}
		connection, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, ErrConnection
		}
		if err = connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			_ = connection.Close()
			return nil, ErrConnection
		}
		return connection, nil
	}
	connection, err := amqp.DialConfig(c.settings.URL, amqp.Config{
		Dial:            dial,
		Heartbeat:       5 * time.Second,
		TLSClientConfig: c.tls,
		Properties:      amqp.Table{"connection_name": "govai-notification-foundation"},
	})
	if err != nil {
		return nil, ErrConnection
	}
	security, normal, err := c.channels(connection)
	if err != nil {
		_ = connection.Close()
		return nil, ErrConnection
	}
	c.mu.Lock()
	c.connection, c.security, c.normal = connection, security, normal
	c.mu.Unlock()
	return connection, nil
}

func (c *Client) channels(connection *amqp.Connection) (
	*amqp.Channel, *amqp.Channel, error,
) {
	security, err := connection.Channel()
	if err != nil {
		return nil, nil, ErrConnection
	}
	normal, err := connection.Channel()
	if err != nil {
		_ = security.Close()
		return nil, nil, ErrConnection
	}

	// Each lane has its own prefetch budget for the future durable consumers.

	if security.Qos(c.settings.SecurityPrefetch, 0, false) != nil ||
		normal.Qos(c.settings.NormalPrefetch, 0, false) != nil {
		_ = security.Close()
		_ = normal.Close()
		return nil, nil, ErrConnection
	}
	return security, normal, nil
}

func (c *Client) disconnect() {
	c.mu.Lock()
	connection := c.connection
	c.connection, c.security, c.normal = nil, nil, nil
	c.mu.Unlock()
	if connection != nil {
		_ = connection.Close()
	}
}

func pause(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
