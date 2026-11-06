package recovery

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DevelopmentPlaintext is allowed only to a literal loopback SMTP address, with
// no authentication. Remote SMTP always requires verified STARTTLS. Mail payloads
// contain bearer links and must never be traced or persisted by this adapter.
type SMTPConfig struct {
	private
	Host, From, Username, Password string
	Port                           int
	Timeout                        time.Duration
	DevelopmentPlaintext           bool
	RootCAs                        *x509.CertPool
}
type SMTPMailer struct {
	private
	cfg SMTPConfig
}

var _ Mailer = (*SMTPMailer)(nil)

func plainAddress(value string) bool {
	if len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	for _, r := range value {
		if r > 127 {
			return false
		}
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}
func NewSMTPMailer(cfg SMTPConfig) (*SMTPMailer, error) {
	if cfg.Host == "" || strings.ContainsAny(cfg.Host, "/\\ \r\n\t") || cfg.Port < 1 || cfg.Port > 65535 || cfg.Timeout <= 0 || cfg.Timeout > 10*time.Second || !plainAddress(cfg.From) {
		return nil, ErrUnavailable
	}
	if cfg.DevelopmentPlaintext {
		ip := net.ParseIP(cfg.Host)
		if ip == nil || !ip.IsLoopback() || cfg.Username != "" || cfg.Password != "" {
			return nil, ErrUnavailable
		}
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		return nil, ErrUnavailable
	}
	if cfg.RootCAs != nil {
		cfg.RootCAs = cfg.RootCAs.Clone()
	}
	return &SMTPMailer{cfg: cfg}, nil
}
func (m *SMTPMailer) Send(ctx context.Context, d Delivery) error {
	if !plainAddress(d.Recipient) || d.Lifetime < time.Minute || d.Lifetime > 30*time.Minute {
		return ErrUnavailable
	}
	u, err := url.Parse(d.Link)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.ContainsAny(d.Link, "\r\n") {
		return ErrUnavailable
	}
	if !strings.HasPrefix(u.Fragment, "token=") {
		return ErrUnavailable
	}
	if _, err = ParseToken(strings.TrimPrefix(u.Fragment, "token=")); err != nil {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port)))
	if err != nil {
		return ErrUnavailable
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return ErrUnavailable
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return ErrUnavailable
	}
	defer client.Close()
	if !m.cfg.DevelopmentPlaintext {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return ErrUnavailable
		}
		if client.StartTLS(&tls.Config{ServerName: m.cfg.Host, RootCAs: m.cfg.RootCAs, MinVersion: tls.VersionTLS12}) != nil {
			return ErrUnavailable
		}
	}
	if m.cfg.Username != "" && client.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)) != nil {
		return ErrUnavailable
	}
	if client.Mail(m.cfg.From) != nil || client.Rcpt(d.Recipient) != nil {
		return ErrUnavailable
	}
	writer, err := client.Data()
	if err != nil {
		return ErrUnavailable
	}
	var message bytes.Buffer
	_, _ = fmt.Fprintf(&message, "From: %s\r\nTo: %s\r\nSubject: Reset your GovAI password\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", m.cfg.From, d.Recipient)
	body := quotedprintable.NewWriter(&message)
	_, err = fmt.Fprintf(body, "A password reset was requested for your GovAI account.\n\nOpen this link to choose a new password:\n%s\n\nThis link expires in %d minutes and can only be used once.\nIf you did not request this, ignore this email.\n", d.Link, int(d.Lifetime/time.Minute))
	if err != nil || body.Close() != nil {
		return ErrUnavailable
	}
	if _, err = writer.Write(message.Bytes()); err != nil {
		return ErrUnavailable
	}
	if writer.Close() != nil {
		return ErrUnavailable
	}
	// DATA acceptance is delivery submission evidence. A failed QUIT must not
	// trigger a second email after the SMTP server already accepted the message.
	_ = client.Quit()
	return nil
}
