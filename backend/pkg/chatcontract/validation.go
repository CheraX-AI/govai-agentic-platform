package chatcontract

import (
	"encoding/json"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func Parse(reader io.Reader) (Question, error) {
	if reader == nil {
		return Question{}, ErrInvalid
	}

	body, err := io.ReadAll(io.LimitReader(reader, MaxRequestBytes+1))
	if len(body) > MaxRequestBytes {
		return Question{}, ErrTooLarge
	}

	if err != nil || !utf8.Valid(body) || !json.Valid(body) || !validSurrogates(body) {
		return Question{}, ErrInvalid
	}

	// One object with "message" and an optional "conversation_id", each once,
	// exact case, in any order; any other field or trailing JSON is rejected.
	object, err := fields(body, "message", conversationIDField)
	if err != nil {
		return Question{}, ErrInvalid
	}

	rawMessage, ok := object["message"]
	if !ok {
		return Question{}, ErrInvalid
	}

	message, err := strictString(rawMessage)
	if err != nil {
		return Question{}, ErrInvalid
	}

	conversationID, ok := optionalConversationID(object)
	if !ok {
		return Question{}, ErrInvalidConversationID
	}

	message = strings.TrimSpace(message)
	if message == "" || utf8.RuneCountInString(message) > MaxMessageRunes {
		return Question{}, ErrInvalid
	}

	for _, r := range message {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return Question{}, ErrInvalid
		}
	}

	return Question{message: message, conversationID: conversationID}, nil
}

// optionalConversationID reads "conversation_id": absent is valid (""), but a
// present value must be a canonical UUID string; null and "" are rejected.
func optionalConversationID(object map[string]json.RawMessage) (string, bool) {
	raw, ok := object[conversationIDField]
	if !ok {
		return "", true
	}

	id, err := strictString(raw)
	if err != nil || !ValidConversationID(id) {
		return "", false
	}

	return id, true
}

// ValidConversationID accepts only the canonical lowercase UUID text form
// (RFC 9562, version 1-8, RFC variant), so one conversation has exactly one
// spelling. The nil and max UUIDs fail the version check.
func ValidConversationID(id string) bool {
	if len(id) != conversationIDLength {
		return false
	}

	for i := 0; i < len(id); i++ {
		c := id[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}

			continue
		}

		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}

	version := id[14]
	if version < '1' || version > '8' {
		return false
	}

	variant := id[19]

	return variant == '8' || variant == '9' || variant == 'a' || variant == 'b'
}

// Run only after json.Valid. Reject lone escaped surrogates instead of silently
// replacing characters; multilingual messages and valid pairs are preserved.
func validSurrogates(body []byte) bool {
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' {
			continue
		}

		i++
		if body[i] != 'u' {
			continue
		}

		n, _ := strconv.ParseUint(string(body[i+1:i+5]), 16, 16)
		i += 4
		if n >= 0xDC00 && n <= 0xDFFF {
			return false
		}

		if n >= 0xD800 && n <= 0xDBFF {
			if i+6 >= len(body) || body[i+1] != '\\' || body[i+2] != 'u' {
				return false
			}

			low, _ := strconv.ParseUint(string(body[i+3:i+7]), 16, 16)
			if low < 0xDC00 || low > 0xDFFF {
				return false
			}

			i += 6
		}
	}

	return true
}

// Encode preserves agent-provided answers/citations, while bounding the public
// shape. It does not establish citation truth, source authority or RAG quality.
func Encode(reply Reply) ([]byte, error) {
	if !utf8.ValidString(reply.Answer) ||
		strings.TrimSpace(reply.Answer) == "" ||
		len(reply.Answer) > 32<<10 ||
		len(reply.Sources) > MaxSources {
		return nil, ErrResponse
	}

	// Optional for now; when present it must be a canonical UUID.
	if reply.ConversationID != "" && !ValidConversationID(reply.ConversationID) {
		return nil, ErrResponse
	}

	if reply.Sources == nil {
		reply.Sources = []Source{}
	}

	numbers := map[int]bool{}
	for _, source := range reply.Sources {
		if source.Number < 1 ||
			source.Number > MaxSourceNumber ||
			numbers[source.Number] ||
			source.Page < 0 ||
			source.Page > MaxSourcePage {
			return nil, ErrResponse
		}

		numbers[source.Number] = true
		if !utf8.ValidString(source.Title) ||
			strings.TrimSpace(source.Title) == "" ||
			len(source.Title) > 512 {
			return nil, ErrResponse
		}
		// URL is optional; when present it must be a plain HTTPS link.
		if source.URL == "" {
			continue
		}

		if !utf8.ValidString(source.URL) ||
			len(source.URL) > 2048 ||
			strings.ContainsAny(source.URL, "\r\n\t ") {
			return nil, ErrResponse
		}

		u, err := url.Parse(source.URL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return nil, ErrResponse
		}
	}

	if !validPresentation(reply.Presentation, reply.Sources) {
		reply.Presentation = nil
	}
	encoded, err := json.Marshal(reply)
	// Optional presentation must never make a valid legacy answer unavailable.
	if len(encoded) > MaxResponseBytes && reply.Presentation != nil {
		reply.Presentation = nil
		encoded, err = json.Marshal(reply)
	}
	if err != nil || len(encoded) > MaxResponseBytes {
		return nil, ErrResponse
	}

	return encoded, nil
}
