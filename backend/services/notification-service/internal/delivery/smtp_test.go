package delivery

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

type scenario struct {
	mode          string
	rcptCode      int
	finalCode     int
	dropAfterData bool
	badQuit       bool
	stall         bool
	requireAuth   bool
	rejectAuth    bool
}

func smtpFixture(t *testing.T, test scenario) (Config, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("fixture listener failed")
	}
	t.Cleanup(func() { _ = listener.Close() })
	certificate := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(certificate.Close)
	roots := x509.NewCertPool()
	roots.AddCert(certificate.Certificate())
	messages := make(chan string, 1)
	go func() {
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		_ = raw.SetDeadline(time.Now().Add(3 * time.Second))
		var connection net.Conn = raw
		secure := test.mode == "tls"
		if secure {
			protected := tls.Server(raw, certificate.TLS.Clone())
			if protected.Handshake() != nil {
				return
			}
			connection = protected
		}
		reader := textproto.NewReader(bufio.NewReader(connection))
		if test.stall {
			_, _ = reader.ReadLine()
			return
		}
		_, _ = io.WriteString(connection, "220 synthetic SMTP\r\n")
		authenticated := false
		for {
			line, err := reader.ReadLine()
			if err != nil {
				return
			}
			command := strings.SplitN(line, " ", 2)[0]
			switch command {
			case "EHLO", "HELO":
				response := "250-synthetic\r\n250 OK\r\n"
				if test.mode == "starttls" && !secure {
					response = "250-synthetic\r\n250 STARTTLS\r\n"
				} else if test.requireAuth {
					response = "250-synthetic\r\n250 AUTH PLAIN\r\n"
				}
				_, _ = io.WriteString(connection, response)
			case "STARTTLS":
				_, _ = io.WriteString(connection, "220 start TLS\r\n")
				protected := tls.Server(raw, certificate.TLS.Clone())
				if protected.Handshake() != nil {
					return
				}
				connection = protected
				reader = textproto.NewReader(bufio.NewReader(connection))
				secure = true
			case "AUTH":
				if !secure || test.rejectAuth {
					_, _ = io.WriteString(connection, "535 rejected synthetic auth\r\n")
				} else {
					authenticated = true
					_, _ = io.WriteString(connection, "235 authenticated\r\n")
				}
			case "MAIL":
				if test.requireAuth && !authenticated {
					_, _ = io.WriteString(connection, "530 authentication required\r\n")
				} else {
					_, _ = io.WriteString(connection, "250 sender accepted\r\n")
				}
			case "RCPT":
				code := test.rcptCode
				if code == 0 {
					code = 250
				}
				_, _ = fmt.Fprintf(connection, "%d synthetic recipient reply\r\n", code)
			case "DATA":
				_, _ = io.WriteString(connection, "354 send data\r\n")
				body, err := reader.ReadDotBytes()
				if err != nil {
					return
				}
				messages <- string(body)
				if test.dropAfterData {
					return
				}
				code := test.finalCode
				if code == 0 {
					code = 250
				}
				_, _ = fmt.Fprintf(connection, "%d synthetic data reply\r\n", code)
			case "QUIT":
				if !test.badQuit {
					_, _ = io.WriteString(connection, "221 goodbye\r\n")
				}
				return
			default:
				return
			}
		}
	}()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	number, _ := strconv.Atoi(port)
	config := Config{
		Host: "127.0.0.1", Port: number, From: "noreply@govai.test",
		TLSMode: test.mode, Timeout: 2 * time.Second, Development: true, RootCAs: roots,
	}
	if test.requireAuth {
		config.Username, config.Password = "synthetic-user", "synthetic-password"
	}
	return config, messages
}

func message() Message {
	return Message{
		ID:        "123e4567-e89b-42d3-a456-426614174000",
		Recipient: "citizen@govai.test",
		Subject:   "Synthetic notification",
		Body:      "Synthetic content.\n.A line beginning with a dot.",
	}
}

// TestSubmissionOutcomes checks transport, acceptance and safe retry classification.

func TestSubmissionOutcomes(t *testing.T) {
	cases := []struct {
		name   string
		server scenario
		status string
		retry  bool
	}{
		{"loopback test", scenario{mode: "plain"}, "submitted", false},
		{"STARTTLS", scenario{mode: "starttls", requireAuth: true}, "submitted", false},
		{"implicit TLS", scenario{mode: "tls", requireAuth: true}, "submitted", false},
		{"QUIT lost after acceptance", scenario{mode: "plain", badQuit: true}, "submitted", false},
		{"transient recipient", scenario{mode: "plain", rcptCode: 450}, "failed", true},
		{"permanent recipient", scenario{mode: "plain", rcptCode: 550}, "failed", false},
		{"transient DATA", scenario{mode: "plain", finalCode: 451}, "failed", true},
		{"permanent DATA", scenario{mode: "plain", finalCode: 554}, "failed", false},
		{"ambiguous acceptance", scenario{mode: "plain", dropAfterData: true}, "unknown", false},
		{"invalid auth", scenario{
			mode: "starttls", requireAuth: true, rejectAuth: true,
		}, "failed", false},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			config, _ := smtpFixture(t, item.server)
			sender, err := New(config)
			if err != nil {
				t.Fatal("fixture SMTP configuration rejected")
			}
			result, err := sender.Send(context.Background(), message())
			if result.Status != item.status || result.Retryable != item.retry {
				t.Fatalf("unexpected classification: %+v", result)
			}
			if (err == nil) != (item.status == "submitted") {
				t.Fatal("incorrect submission error")
			}
			if err != nil && strings.Contains(err.Error(), "synthetic") {
				t.Fatal("raw provider response escaped")
			}
		})
	}
}

// TestTLSAndCancellation rejects untrusted servers and bounds blocked connections.

func TestTLSAndCancellation(t *testing.T) {
	t.Run("untrusted certificate", func(t *testing.T) {
		config, _ := smtpFixture(t, scenario{mode: "starttls"})
		config.RootCAs = x509.NewCertPool()
		sender, _ := New(config)
		result, err := sender.Send(context.Background(), message())
		if err == nil || result.Code != "tls_failed" || result.Retryable {
			t.Fatal("untrusted TLS was accepted or marked retryable")
		}
	})
	t.Run("no TLS downgrade", func(t *testing.T) {
		config, _ := smtpFixture(t, scenario{mode: "plain"})
		config.TLSMode = "starttls"
		sender, _ := New(config)
		result, err := sender.Send(context.Background(), message())
		if err == nil || result.Code != "tls_required" {
			t.Fatal("plaintext fallback was allowed")
		}
	})
	for _, canceled := range []bool{false, true} {
		t.Run(strconv.FormatBool(canceled), func(t *testing.T) {
			config, _ := smtpFixture(t, scenario{mode: "plain", stall: true})
			config.Timeout = 150 * time.Millisecond
			sender, _ := New(config)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceled {
				time.AfterFunc(30*time.Millisecond, cancel)
			}
			start := time.Now()
			result, err := sender.Send(ctx, message())
			expected := "timeout"
			if canceled {
				expected = "canceled"
			}
			if err == nil || result.Code != expected || time.Since(start) > time.Second {
				t.Fatal("blocked SMTP was not bounded or classified correctly")
			}
		})
	}
}

// TestRedaction keeps credentials, recipients and message text out of generic outputs.

func TestRedaction(t *testing.T) {
	input := message()
	for _, value := range []any{
		input, Config{Password: "synthetic-password"}, &SMTP{},
	} {
		encoded, err := json.Marshal(value)
		if err != nil || strings.Contains(string(encoded), "synthetic-password") ||
			strings.Contains(fmt.Sprintf("%+v", value), "citizen@govai.test") {
			t.Fatal("SMTP input was not redacted")
		}
	}
}
