package delivery

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type SMTP struct {
	private
	config Config
}

// New validates transport policy; production never falls back to plaintext.

func New(config Config) (*SMTP, error) {
	if config.Host == "" || strings.ContainsAny(config.Host, "/\\?#@ \r\n\t") ||
		config.Port < 1 || config.Port > 65535 || !plainAddress(config.From) ||
		config.Timeout < 100*time.Millisecond || config.Timeout > 30*time.Second ||
		(config.Username == "") != (config.Password == "") {
		return nil, ErrConfiguration
	}
	switch config.TLSMode {
	case "starttls", "tls":
	case "plain":
		ip := net.ParseIP(config.Host)
		if !config.Development || ip == nil || !ip.IsLoopback() ||
			config.Username != "" || config.Password != "" {
			return nil, ErrConfiguration
		}
	default:
		return nil, ErrConfiguration
	}
	if config.RootCAs != nil {
		config.RootCAs = config.RootCAs.Clone()
	}
	return &SMTP{config: config}, nil
}

// Send submits one message without retries; the durable worker owns attempt policy.

func (s *SMTP) Send(ctx context.Context, message Message) (result Result, sendErr error) {
	encoded, err := encodeMessage(s.config.From, message)
	if err != nil {
		return Result{Status: "failed", Code: "invalid_message"}, ErrMessage
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	defer func() {
		if result.Status == "failed" && ctx.Err() != nil {
			result.Code = "timeout"
			result.Retryable = true
			if errors.Is(ctx.Err(), context.Canceled) {
				result.Code = "canceled"
				result.Retryable = false
			}
			sendErr = ErrSubmission
		}
	}()
	address := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return failure(err, "connection_failed", false)
	}
	defer connection.Close()
	deadline, _ := ctx.Deadline()
	if connection.SetDeadline(deadline) != nil {
		return failure(nil, "connection_failed", false)
	}
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	secure := &tls.Config{
		ServerName: s.config.Host,
		RootCAs:    s.config.RootCAs,
		MinVersion: tls.VersionTLS12,
	}
	transport := connection
	if s.config.TLSMode == "tls" {
		protected := tls.Client(connection, secure)
		if protected.HandshakeContext(ctx) != nil {
			return Result{Status: "failed", Code: "tls_failed"}, ErrSubmission
		}
		transport = protected
	}
	client, err := smtp.NewClient(transport, s.config.Host)
	if err != nil {
		return failure(err, "greeting_failed", false)
	}
	defer client.Close()
	if s.config.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return Result{Status: "failed", Code: "tls_required"}, ErrSubmission
		}
		if client.StartTLS(secure) != nil {
			return Result{Status: "failed", Code: "tls_failed"}, ErrSubmission
		}
	}
	if s.config.Username != "" {
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		if err := client.Auth(auth); err != nil {
			return failure(err, "authentication_failed", false)
		}
	}
	return s.submit(client, message.Recipient, encoded)
}

func (s *SMTP) submit(client *smtp.Client, recipient string, body []byte) (Result, error) {
	if err := client.Mail(s.config.From); err != nil {
		return failure(err, "sender_rejected", false)
	}
	if err := client.Rcpt(recipient); err != nil {
		return failure(err, "recipient_rejected", false)
	}
	writer, err := client.Data()
	if err != nil {
		return failure(err, "data_rejected", false)
	}

	// After DATA starts, a lost connection may hide acceptance; never blindly resend.

	if _, err := writer.Write(body); err != nil {
		return failure(err, "submission_unknown", true)
	}
	if err := writer.Close(); err != nil {
		return failure(err, "submission_unknown", true)
	}

	// A failed QUIT cannot undo the server's final positive DATA acknowledgment.

	_ = client.Quit()
	return Result{Status: "submitted", Code: "smtp_accepted"}, nil
}

func failure(err error, code string, afterData bool) (Result, error) {
	var reply *textproto.Error
	if errors.As(err, &reply) && reply.Code >= 400 && reply.Code < 600 {
		retryable := reply.Code < 500
		safeCode := "smtp_5xx"
		if retryable {
			safeCode = "smtp_4xx"
		}
		return Result{Status: "failed", Code: safeCode, Retryable: retryable}, ErrSubmission
	}
	if afterData {
		return Result{Status: "unknown", Code: "submission_unknown"}, ErrUnknown
	}
	return Result{Status: "failed", Code: code,
		Retryable: code != "authentication_failed"}, ErrSubmission
}
