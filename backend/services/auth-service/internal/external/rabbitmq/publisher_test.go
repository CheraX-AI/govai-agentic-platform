package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/notificationevents"
	amqp "github.com/rabbitmq/amqp091-go"
)

func publisherTestConfig() Config {
	return Config{
		Host: "127.0.0.1", Port: 5672, VHost: "govai", Scheme: "amqp",
		Username: "synthetic-publisher", Password: "synthetic-private-password",
		Issuer: "https://identity.test/realms/publisher", Timeout: time.Second,
		Development: true,
	}
}

func TestPublisherConfiguration(t *testing.T) {
	for name, mutate := range map[string]func(*Config){
		"remote plaintext":     func(c *Config) { c.Host = "remote.example" },
		"production plaintext": func(c *Config) { c.Development = false },
		"unbounded timeout":    func(c *Config) { c.Timeout = time.Minute },
		"missing credentials":  func(c *Config) { c.Password = "" },
		"host injection":       func(c *Config) { c.Host = "host/user?query" },
		"other vhost":          func(c *Config) { c.VHost = "/" },
		"missing CA":           func(c *Config) { c.CAFile = "/nonexistent/publisher-ca.pem" },
	} {
		t.Run(name, func(t *testing.T) {
			settings := publisherTestConfig()
			mutate(&settings)
			if _, err := New(settings); err == nil {
				t.Fatal("unsafe publisher settings accepted")
			}
		})
	}
	settings := publisherTestConfig()
	settings.Scheme = "amqps"
	publisher, err := New(settings)
	if err != nil || publisher.tls.InsecureSkipVerify ||
		publisher.tls.ServerName != settings.Host {
		t.Fatal("verified TLS not configured")
	}
	encoded, _ := json.Marshal(settings)
	for _, output := range []string{
		fmt.Sprintf("%+v", settings), fmt.Sprintf("%#v", publisher),
		string(encoded), settings.LogValue().String(),
	} {
		if strings.Contains(output, settings.Password) {
			t.Fatal("publisher credentials leaked")
		}
	}
}

func TestPublisherConfirmation(t *testing.T) {
	for name, prepare := range map[string]func(
		chan amqp.Return, chan amqp.Confirmation, chan *amqp.Error,
	){
		"positive": func(_ chan amqp.Return, confirms chan amqp.Confirmation,
			_ chan *amqp.Error) {
			confirms <- amqp.Confirmation{DeliveryTag: 1, Ack: true}
		},
		"returned before confirm": func(returns chan amqp.Return,
			confirms chan amqp.Confirmation, _ chan *amqp.Error) {
			returns <- amqp.Return{ReplyCode: 312}
			confirms <- amqp.Confirmation{DeliveryTag: 1, Ack: true}
		},
		"negative": func(_ chan amqp.Return, confirms chan amqp.Confirmation,
			_ chan *amqp.Error) {
			confirms <- amqp.Confirmation{DeliveryTag: 1, Ack: false}
		},
		"wrong sequence": func(_ chan amqp.Return, confirms chan amqp.Confirmation,
			_ chan *amqp.Error) {
			confirms <- amqp.Confirmation{DeliveryTag: 2, Ack: true}
		},
		"closed": func(_ chan amqp.Return, _ chan amqp.Confirmation,
			closed chan *amqp.Error) {
			close(closed)
		},
	} {
		t.Run(name, func(t *testing.T) {
			returns := make(chan amqp.Return, 1)
			confirms := make(chan amqp.Confirmation, 1)
			closed := make(chan *amqp.Error, 1)
			prepare(returns, confirms, closed)
			err := awaitConfirm(context.Background(), returns, confirms, closed)
			if name == "positive" && err != nil || name != "positive" && err == nil {
				t.Fatal("incorrect confirm result")
			}
			if name == "returned before confirm" &&
				!errors.Is(err, notificationevents.ErrUnroutable) {
				t.Fatal("returned event was not recognized")
			}
		})
	}
}

func TestPublisherHandshakeTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("test listener unavailable")
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			accepted <- connection
		}
	}()
	settings := publisherTestConfig()
	settings.Port = listener.Addr().(*net.TCPAddr).Port
	settings.Timeout = 150 * time.Millisecond
	publisher, _ := New(settings)
	event := contract.Event{
		ID:   "123e4567-e89b-42d3-a456-426614174000",
		Type: contract.LoginSucceeded, Version: 1,
		Issuer: settings.Issuer, Subject: "synthetic", OccurredAt: time.Now(),
	}
	started := time.Now()
	err = publisher.Publish(context.Background(), event)
	if !errors.Is(err, notificationevents.ErrUnavailable) ||
		time.Since(started) > time.Second {
		t.Fatal("stalled handshake was not bounded")
	}
	select {
	case connection := <-accepted:
		_ = connection.Close()
	case <-time.After(time.Second):
		t.Fatal("test connection was not accepted")
	}
}
