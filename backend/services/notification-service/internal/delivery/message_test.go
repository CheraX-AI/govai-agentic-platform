package delivery

import (
	"context"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"
)

// TestMessageEncoding checks Unicode, stable identifiers and SMTP dot transparency.

func TestMessageEncoding(t *testing.T) {
	config, messages := smtpFixture(t, scenario{mode: "plain"})
	sender, err := New(config)
	if err != nil {
		t.Fatal("fixture setup failed")
	}
	input := message()
	input.Subject = "Synthetic café alert"
	input.Body = "Synthetic café content.\n.A dot-prefixed line.\n"
	if _, err := sender.Send(context.Background(), input); err != nil {
		t.Fatal("test submission failed")
	}
	select {
	case raw := <-messages:
		parsed, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			t.Fatal("invalid MIME message")
		}
		if _, err := parsed.Header.Date(); err != nil {
			t.Fatal("missing or invalid required Date header")
		}
		subject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
		if err != nil || subject != input.Subject {
			t.Fatal("subject encoding changed text")
		}
		body, err := io.ReadAll(quotedprintable.NewReader(parsed.Body))
		normalized := strings.ReplaceAll(string(body), "\r\n", "\n")
		if err != nil || normalized != input.Body {
			t.Fatal("body encoding changed text or dot-prefixed lines")
		}
		if parsed.Header.Get("Message-ID") != "<"+input.ID+"@govai.test>" {
			t.Fatal("delivery identifier is not stable")
		}
	case <-time.After(time.Second):
		t.Fatal("synthetic message was not captured")
	}
}

// TestInvalidMessage rejects injection and unbounded content before connecting.

func TestInvalidMessage(t *testing.T) {
	config := Config{
		Host: "127.0.0.1", Port: 1, From: "noreply@govai.test",
		TLSMode: "plain", Development: true, Timeout: time.Second,
	}
	sender, err := New(config)
	if err != nil {
		t.Fatal("test configuration failed")
	}
	for _, name := range []string{
		"recipient injection", "subject injection", "subject control", "empty body",
		"oversized body", "invalid utf8", "bad identifier",
	} {
		t.Run(name, func(t *testing.T) {
			input := message()
			switch name {
			case "recipient injection":
				input.Recipient += "\r\nBcc: synthetic@govai.test"
			case "subject injection":
				input.Subject += "\r\nBcc: synthetic@govai.test"
			case "subject control":
				input.Subject += "\t"
			case "empty body":
				input.Body = " "
			case "oversized body":
				input.Body = strings.Repeat("x", (64<<10)+1)
			case "invalid utf8":
				input.Body = string([]byte{0xff})
			case "bad identifier":
				input.ID = "invalid"
			}
			result, err := sender.Send(context.Background(), input)
			if err != ErrMessage || result.Code != "invalid_message" || result.Retryable {
				t.Fatal("invalid input was accepted or reached the connection stage")
			}
		})
	}
}

// TestTransportConfiguration rejects insecure or incomplete settings.

func TestTransportConfiguration(t *testing.T) {
	for _, name := range []string{
		"remote plaintext", "production plaintext", "plaintext credentials",
		"password without username", "bad port", "sender injection", "bad host", "bad TLS mode",
	} {
		t.Run(name, func(t *testing.T) {
			config := Config{
				Host: "127.0.0.1", Port: 2525, From: "noreply@govai.test",
				TLSMode: "plain", Development: true, Timeout: time.Second,
			}
			switch name {
			case "remote plaintext":
				config.Host = "smtp.example"
			case "production plaintext":
				config.Development = false
			case "plaintext credentials":
				config.Username, config.Password = "synthetic", "synthetic"
			case "password without username":
				config.Password = "synthetic"
			case "bad port":
				config.Port = 70000
			case "sender injection":
				config.From += "\r\nBcc: synthetic@govai.test"
			case "bad host":
				config.Host = "smtp.example/path"
			case "bad TLS mode":
				config.TLSMode = "insecure"
			}
			if _, err := New(config); err != ErrConfiguration {
				t.Fatal("invalid transport settings accepted")
			}
		})
	}
}
