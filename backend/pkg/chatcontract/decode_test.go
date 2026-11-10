package chatcontract

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestDecodeReplyAcceptsSources(t *testing.T) {
	for name, tc := range map[string]struct {
		body        string
		sourceCount int
	}{
		"no sources":        {`{"answer":"An answer","sources":[]}`, 0},
		"null sources":      {`{"answer":"An answer","sources":null}`, 0},
		"sources left out":  {`{"answer":"An answer"}`, 0},
		"number and title":  {`{"answer":"See [1]","sources":[{"number":1,"title":"TIN guide"}]}`, 1},
		"with page":         {`{"answer":"See [1]","sources":[{"number":1,"title":"TIN guide","page":3}]}`, 1},
		"with https url":    {`{"answer":"See [1]","sources":[{"number":1,"title":"TIN guide","url":"https://example.test/tin"}]}`, 1},
		"null page and url": {`{"answer":"See [1]","sources":[{"number":1,"title":"TIN guide","page":null,"url":null}]}`, 1},
		"several":           {`{"answer":"See [1][2]","sources":[{"number":1,"title":"A","page":1},{"number":2,"title":"B"}]}`, 2},
	} {
		t.Run(name, func(t *testing.T) {
			reply, err := DecodeReply([]byte(tc.body))
			if err != nil {
				t.Fatalf("rejected: %v", err)
			}

			if reply.Sources == nil || len(reply.Sources) != tc.sourceCount {
				t.Fatalf("expected %d sources, got %+v", tc.sourceCount, reply.Sources)
			}

			if _, err := Encode(reply); err != nil {
				t.Fatalf("cannot re-encode: %v", err)
			}
		})
	}
}

func TestDecodeReplyKeepsSourceFields(t *testing.T) {
	reply, err := DecodeReply(
		[]byte(
			`{"answer":"See [2][4]","sources":[` +
				`{"number":2,"title":"TIN guide","page":7,"url":"https://example.test/tin"},` +
				`{"number":4,"title":"Other","page":null,"url":null}]}`,
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []Source{
		{Number: 2, Title: "TIN guide", Page: 7, URL: "https://example.test/tin"},
		{Number: 4, Title: "Other"},
	}
	if !slices.Equal(reply.Sources, want) {
		t.Fatalf("got %+v", reply.Sources)
	}
}

func TestDecodeReplyRejectsInvalidReply(t *testing.T) {
	overflow := make([]string, MaxSources+1)
	for i := range overflow {
		overflow[i] = fmt.Sprintf(`{"number":%d,"title":"A"}`, i+1)
	}

	for name, body := range map[string]string{
		"null reply":         `null`,
		"array reply":        `[]`,
		"wrong key case":     `{"Answer":"x"}`,
		"null answer":        `{"answer":null}`,
		"extra reply field":  `{"answer":"x","identity":"caller"}`,
		"lone surrogate":     `{"answer":"\ud800"}`,
		"trailing JSON":      `{"answer":"x"}{}`,
		"missing number":     `{"answer":"x","sources":[{"title":"A"}]}`,
		"missing title":      `{"answer":"x","sources":[{"number":1}]}`,
		"empty title":        `{"answer":"x","sources":[{"number":1,"title":" "}]}`,
		"zero number":        `{"answer":"x","sources":[{"number":0,"title":"A"}]}`,
		"negative number":    `{"answer":"x","sources":[{"number":-1,"title":"A"}]}`,
		"number too large":   `{"answer":"x","sources":[{"number":101,"title":"A"}]}`,
		"fraction number":    `{"answer":"x","sources":[{"number":1.5,"title":"A"}]}`,
		"exponent number":    `{"answer":"x","sources":[{"number":1e1,"title":"A"}]}`,
		"string number":      `{"answer":"x","sources":[{"number":"1","title":"A"}]}`,
		"duplicate number":   `{"answer":"x","sources":[{"number":1,"title":"A"},{"number":1,"title":"B"}]}`,
		"zero page":          `{"answer":"x","sources":[{"number":1,"title":"A","page":0}]}`,
		"string page":        `{"answer":"x","sources":[{"number":1,"title":"A","page":"3"}]}`,
		"page too large":     `{"answer":"x","sources":[{"number":1,"title":"A","page":100001}]}`,
		"empty url":          `{"answer":"x","sources":[{"number":1,"title":"A","url":""}]}`,
		"http url":           `{"answer":"x","sources":[{"number":1,"title":"A","url":"http://example.test"}]}`,
		"javascript url":     `{"answer":"x","sources":[{"number":1,"title":"A","url":"javascript:bad"}]}`,
		"unknown field":      `{"answer":"x","sources":[{"number":1,"title":"A","user":"someone"}]}`,
		"repeated field":     `{"answer":"x","sources":[{"number":1,"title":"A","title":"B"}]}`,
		"sources not a list": `{"answer":"x","sources":{"number":1,"title":"A"}}`,
		"too many sources":   `{"answer":"x","sources":[` + strings.Join(overflow, ",") + `]}`,
		"empty answer":       `{"answer":" ","sources":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReply([]byte(body)); !errors.Is(err, ErrResponse) {
				t.Fatalf("accepted or wrong error: %v", err)
			}
		})
	}
}
