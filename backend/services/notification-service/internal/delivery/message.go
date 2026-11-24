package delivery

import (
	"bytes"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var messageID = regexp.MustCompile(
	"^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
)

func plainAddress(value string) bool {
	if len(value) > 254 || value == "" {
		return false
	}
	for _, character := range value {
		if character < 33 || character > 126 {
			return false
		}
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

func validSubject(value string) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if character < 32 || character == 127 {
			return false
		}
	}
	return true
}

func encodeMessage(from string, message Message) ([]byte, error) {
	if !messageID.MatchString(message.ID) || !plainAddress(message.Recipient) ||
		!validSubject(message.Subject) || !utf8.ValidString(message.Body) ||
		strings.TrimSpace(message.Body) == "" || len(message.Body) > 64<<10 ||
		strings.ContainsRune(message.Body, 0) {
		return nil, ErrMessage
	}
	domain := from[strings.LastIndex(from, "@")+1:]
	var buffer bytes.Buffer
	headers := []string{
		"From: " + from,
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"To: " + message.Recipient,
		"Subject: " + mime.QEncoding.Encode("UTF-8", message.Subject),
		"Message-ID: <" + message.ID + "@" + domain + ">",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: quoted-printable",
	}
	for _, header := range headers {
		_, _ = fmt.Fprint(&buffer, header, "\r\n")
	}
	_, _ = fmt.Fprint(&buffer, "\r\n")
	writer := quotedprintable.NewWriter(&buffer)
	if _, err := writer.Write([]byte(message.Body)); err != nil {
		return nil, ErrMessage
	}
	if writer.Close() != nil {
		return nil, ErrMessage
	}
	return buffer.Bytes(), nil
}
