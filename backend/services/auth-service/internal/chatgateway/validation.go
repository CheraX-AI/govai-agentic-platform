package chatgateway

import (
	"bytes"
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

	decoder := json.NewDecoder(bytes.NewReader(body))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return Question{}, ErrInvalid
	}

	if !decoder.More() {
		return Question{}, ErrInvalid
	}

	key, err := decoder.Token()
	if err != nil || key != "message" {
		return Question{}, ErrInvalid
	}

	var raw json.RawMessage
	if decoder.Decode(&raw) != nil || len(raw) == 0 || raw[0] != '"' {
		return Question{}, ErrInvalid
	}

	var message string
	if json.Unmarshal(raw, &message) != nil || decoder.More() {
		return Question{}, ErrInvalid
	}

	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return Question{}, ErrInvalid
	}

	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return Question{}, ErrInvalid
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

	return Question{message: message}, nil
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

	encoded, err := json.Marshal(reply)
	if err != nil || len(encoded) > MaxResponseBytes {
		return nil, ErrResponse
	}

	return encoded, nil
}
