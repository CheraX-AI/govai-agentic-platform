package chatgateway

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

// fields accepts exact-case field names once each, without silently accepting
// extra identity/history/tool-call fields not in the provisional contract.
func fields(body []byte, allowed ...string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(body))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return nil, ErrResponse
	}
	result := map[string]json.RawMessage{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, ErrResponse
		}
		name, ok := key.(string)
		if !ok {
			return nil, ErrResponse
		}
		known := false
		for _, value := range allowed {
			if name == value {
				known = true
			}
		}
		if _, seen := result[name]; !known || seen {
			return nil, ErrResponse
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, ErrResponse
		}
		result[name] = value
	}
	if last, err := d.Token(); err != nil || last != json.Delim('}') {
		return nil, ErrResponse
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, ErrResponse
	}
	return result, nil
}
func strictString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || raw[0] != '"' {
		return "", ErrResponse
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", ErrResponse
	}
	return value, nil
}
func DecodeReply(body []byte) (Reply, error) {
	if len(body) > MaxResponseBytes || !utf8.Valid(body) || !json.Valid(body) || !validSurrogates(body) {
		return Reply{}, ErrResponse
	}
	object, err := fields(body, "answer", "sources")
	if err != nil {
		return Reply{}, err
	}
	answer, err := strictString(object["answer"])
	if err != nil {
		return Reply{}, ErrResponse
	}
	reply := Reply{Answer: answer, Sources: []Source{}}
	if raw, ok := object["sources"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || raw[0] != '[' {
			return Reply{}, ErrResponse
		}
		var sources []json.RawMessage
		if json.Unmarshal(raw, &sources) != nil || len(sources) > 20 {
			return Reply{}, ErrResponse
		}
		for _, rawSource := range sources {
			values, err := fields(rawSource, "title", "url")
			if err != nil || len(values) != 2 {
				return Reply{}, ErrResponse
			}
			title, err := strictString(values["title"])
			if err != nil {
				return Reply{}, err
			}
			location, err := strictString(values["url"])
			if err != nil {
				return Reply{}, err
			}
			reply.Sources = append(reply.Sources, Source{Title: title, URL: location})
		}
	}
	if _, err := Encode(reply); err != nil {
		return Reply{}, ErrResponse
	}
	return reply, nil
}
