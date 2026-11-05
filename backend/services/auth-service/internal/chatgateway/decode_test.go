package chatgateway

import (
	"errors"
	"strings"
	"testing"
)

// The reply format is core-service's (core-service/internal/chat): sources carry
// number and title, with optional page and url.
func TestDecodeReplyAcceptsCoreSources(t *testing.T) {
	for name, body := range map[string]string{
		"no sources":        `{"answer":"An answer","sources":[]}`,
		"null sources":      `{"answer":"An answer","sources":null}`,
		"sources left out":  `{"answer":"An answer"}`,
		"number and title":  `{"answer":"See [1]","sources":[{"number":1,"title":"TIN guide"}]}`,
		"with page":         `{"answer":"See [1]","sources":[{"number":1,"title":"TIN guide","page":3}]}`,
		"with https url":    `{"answer":"See [1]","sources":[{"number":1,"title":"TIN guide","url":"https://example.test/tin"}]}`,
		"null page and url": `{"answer":"See [1]","sources":[{"number":1,"title":"TIN guide","page":null,"url":null}]}`,
		"several":           `{"answer":"See [1][2]","sources":[{"number":1,"title":"A","page":1},{"number":2,"title":"B"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			reply, err := DecodeReply([]byte(body))
			if err != nil {
				t.Fatalf("rejected: %v", err)
			}
			if _, err := Encode(reply); err != nil {
				t.Fatalf("cannot re-encode: %v", err)
			}
		})
	}
}

func TestDecodeReplyKeepsSourceFields(t *testing.T) {
	reply, err := DecodeReply([]byte(`{"answer":"See [2]","sources":[{"number":2,"title":"TIN guide","page":7,"url":"https://example.test/tin"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := Source{Number: 2, Title: "TIN guide", Page: 7, URL: "https://example.test/tin"}
	if len(reply.Sources) != 1 || reply.Sources[0] != want {
		t.Fatalf("got %+v", reply.Sources)
	}
	encoded, err := Encode(Reply{Answer: "See [1]", Sources: []Source{{Number: 1, Title: "TIN guide"}}})
	if err != nil {
		t.Fatal(err)
	}
	// Unknown page and url are left out, as core-service does
	if !strings.Contains(string(encoded), `{"number":1,"title":"TIN guide"}`) {
		t.Fatalf("got %s", encoded)
	}
}

func TestDecodeReplyRejectsBadSources(t *testing.T) {
	for name, body := range map[string]string{
		"missing number":     `{"answer":"x","sources":[{"title":"A"}]}`,
		"missing title":      `{"answer":"x","sources":[{"number":1}]}`,
		"empty title":        `{"answer":"x","sources":[{"number":1,"title":" "}]}`,
		"zero number":        `{"answer":"x","sources":[{"number":0,"title":"A"}]}`,
		"negative number":    `{"answer":"x","sources":[{"number":-1,"title":"A"}]}`,
		"number too large":   `{"answer":"x","sources":[{"number":101,"title":"A"}]}`,
		"fraction number":    `{"answer":"x","sources":[{"number":1.5,"title":"A"}]}`,
		"string number":      `{"answer":"x","sources":[{"number":"1","title":"A"}]}`,
		"duplicate number":   `{"answer":"x","sources":[{"number":1,"title":"A"},{"number":1,"title":"B"}]}`,
		"zero page":          `{"answer":"x","sources":[{"number":1,"title":"A","page":0}]}`,
		"page too large":     `{"answer":"x","sources":[{"number":1,"title":"A","page":100001}]}`,
		"empty url":          `{"answer":"x","sources":[{"number":1,"title":"A","url":""}]}`,
		"http url":           `{"answer":"x","sources":[{"number":1,"title":"A","url":"http://example.test"}]}`,
		"javascript url":     `{"answer":"x","sources":[{"number":1,"title":"A","url":"javascript:bad"}]}`,
		"unknown field":      `{"answer":"x","sources":[{"number":1,"title":"A","user":"someone"}]}`,
		"repeated field":     `{"answer":"x","sources":[{"number":1,"title":"A","title":"B"}]}`,
		"sources not a list": `{"answer":"x","sources":{"number":1,"title":"A"}}`,
		"too many sources":   `{"answer":"x","sources":[` + strings.Repeat(`{"number":1,"title":"A"},`, 20) + `{"number":21,"title":"A"}]}`,
		"empty answer":       `{"answer":" ","sources":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReply([]byte(body)); !errors.Is(err, ErrResponse) {
				t.Fatalf("accepted or wrong error: %v", err)
			}
		})
	}
}
