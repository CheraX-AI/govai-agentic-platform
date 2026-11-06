package recovery

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// MaskEmail intentionally reveals account existence, domain and at most two
// local-part characters for the agreed prototype UI. It is not anonymization.
// Never use this value for mail delivery, authorization or identity lookup.
func MaskEmail(email string) (string, error) {
	invalid := errors.New("invalid recovery email")
	if len(email) > 254 || !utf8.ValidString(email) || strings.ContainsAny(email, "\r\n") {
		return "", invalid
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return "", invalid
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return "", invalid
	}
	local := []rune(email[:at])
	prefix := ""
	if len(local) > 2 {
		prefix = string(local[:2])
	}
	return prefix + "*****@" + email[at+1:], nil
}
