package recovery

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

func smtpFixture(t *testing.T, secure bool) (SMTPConfig, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	certServer := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(certServer.Close)
	roots := x509.NewCertPool()
	roots.AddCert(certServer.Certificate())
	messages := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		_, _ = io.WriteString(conn, "220 synthetic SMTP\r\n")
		reader := textproto.NewReader(bufio.NewReader(conn))
		for {
			line, e := reader.ReadLine()
			if e != nil {
				return
			}
			command := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
			switch command {
			case "EHLO", "HELO":
				if secure {
					_, _ = io.WriteString(conn, "250-synthetic\r\n250 STARTTLS\r\n")
				} else {
					_, _ = io.WriteString(conn, "250 synthetic\r\n")
				}
			case "STARTTLS":
				_, _ = io.WriteString(conn, "220 start TLS\r\n")
				tlsConn := tls.Server(conn, certServer.TLS.Clone())
				if tlsConn.Handshake() != nil {
					return
				}
				conn = tlsConn
				reader = textproto.NewReader(bufio.NewReader(conn))
			case "MAIL", "RCPT":
				_, _ = io.WriteString(conn, "250 accepted\r\n")
			case "DATA":
				_, _ = io.WriteString(conn, "354 send data\r\n")
				message, e := reader.ReadDotBytes()
				if e != nil {
					return
				}
				messages <- string(message)
				_, _ = io.WriteString(conn, "250 queued\r\n")
			case "QUIT":
				_, _ = io.WriteString(conn, "221 goodbye\r\n")
				return
			default:
				return
			}
		}
	}()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	number, _ := strconv.Atoi(port)
	return SMTPConfig{Host: "127.0.0.1", Port: number, From: "noreply@govai.test", Timeout: 2 * time.Second, DevelopmentPlaintext: !secure, RootCAs: roots}, messages
}
func TestSMTPDelivery(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(strconv.FormatBool(secure), func(t *testing.T) {
			cfg, messages := smtpFixture(t, secure)
			sender, err := NewSMTPMailer(cfg)
			if err != nil {
				t.Fatal(err)
			}
			token, _ := NewToken()
			link := "https://app.example.test/reset#token=" + token.Reveal()
			if err = sender.Send(context.Background(), Delivery{Recipient: "citizen@govai.test", Link: link, Lifetime: 15 * time.Minute}); err != nil {
				t.Fatal("SMTP submission failed")
			}
			select {
			case message := <-messages:
				parts := strings.SplitN(message, "\n\n", 2)
				if len(parts) != 2 {
					t.Fatal("missing MIME body")
				}
				decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(parts[1])))
				if err != nil || !strings.Contains(string(decoded), link) || !strings.Contains(string(decoded), "15 minutes") || !strings.Contains(message, "To: citizen@govai.test") {
					t.Fatal("incorrect mail content")
				}
			case <-time.After(time.Second):
				t.Fatal("mail missing")
			}
		})
	}
}
func TestSMTPRejectsUntrustedTLS(t *testing.T) {
	cfg, messages := smtpFixture(t, true)
	cfg.RootCAs = nil
	sender, err := NewSMTPMailer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := NewToken()
	if sender.Send(context.Background(), Delivery{Recipient: "citizen@govai.test", Link: "https://app.test/reset#token=" + token.Reveal(), Lifetime: time.Minute}) == nil {
		t.Fatal("untrusted SMTP certificate accepted")
	}
	select {
	case <-messages:
		t.Fatal("sent before TLS verification")
	default:
	}
}
func TestSMTPRejectsUnsafeInputs(t *testing.T) {
	for _, cfg := range []SMTPConfig{
		{Host: "smtp.example.test", Port: 25, From: "noreply@govai.test", Timeout: time.Second, DevelopmentPlaintext: true},
		{Host: "127.0.0.1", Port: 25, From: "noreply@govai.test", Timeout: time.Second, DevelopmentPlaintext: true, Username: "user", Password: "secret"},
		{Host: "127.0.0.1", Port: 25, From: "bad\r\nBcc: other@govai.test", Timeout: time.Second},
	} {
		if _, err := NewSMTPMailer(cfg); err == nil {
			t.Fatal("unsafe SMTP configuration")
		}
	}
	sender, err := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: 1, From: "noreply@govai.test", Timeout: time.Second, DevelopmentPlaintext: true})
	if err != nil {
		t.Fatal(err)
	}
	if sender.Send(context.Background(), Delivery{Recipient: "bad\r\naddress", Link: "https://app.test/", Lifetime: time.Minute}) == nil {
		t.Fatal("header injection accepted")
	}
}

func TestSMTPRequiresSTARTTLS(t *testing.T) {
	cfg, messages := smtpFixture(t, false)
	cfg.DevelopmentPlaintext = false
	sender, err := NewSMTPMailer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := NewToken()
	if sender.Send(context.Background(), Delivery{Recipient: "citizen@govai.test", Link: "https://app.test/reset#token=" + token.Reveal(), Lifetime: time.Minute}) == nil {
		t.Fatal("STARTTLS downgrade accepted")
	}
	select {
	case <-messages:
		t.Fatal("sent to plaintext server")
	default:
	}
}
func TestSMTPCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		var b [1]byte
		_, _ = conn.Read(b[:])
	}()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	number, _ := strconv.Atoi(port)
	sender, err := NewSMTPMailer(SMTPConfig{Host: "127.0.0.1", Port: number, From: "noreply@govai.test", Timeout: 2 * time.Second, DevelopmentPlaintext: true})
	if err != nil {
		t.Fatal(err)
	}
	token, _ := NewToken()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- sender.Send(ctx, Delivery{Recipient: "citizen@govai.test", Link: "https://app.test/reset#token=" + token.Reveal(), Lifetime: time.Minute})
	}()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("connection not accepted")
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("canceled mail accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt SMTP")
	}
	<-done
}
