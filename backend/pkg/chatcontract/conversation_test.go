package chatcontract

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	conversationV4 = "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"
	conversationV7 = "01936f3e-7b2a-7c4d-9e5f-0a1b2c3d4e5f"
)

// Each value is wrapped in a JSON string, so these test the UUID check itself.
var malformedConversationIDs = map[string]string{
	"empty":             "",
	"uppercase":         strings.ToUpper(conversationV4),
	"no hyphens":        strings.ReplaceAll(conversationV4, "-", ""),
	"braces":            "{" + conversationV4 + "}",
	"urn prefix":        "urn:uuid:" + conversationV4,
	"leading space":     " " + conversationV4,
	"trailing space":    conversationV4 + " ",
	"too long":          conversationV4 + "0",
	"too short":         conversationV4[:35],
	"non-hex":           conversationV4[:35] + "g",
	"hyphen moved":      "3f2b8c1e9-a4d-4f6b-8c2e-1d7a5b9e0f34",
	"nil uuid":          "00000000-0000-0000-0000-000000000000",
	"max uuid":          "ffffffff-ffff-ffff-ffff-ffffffffffff",
	"version 0":         "3f2b8c1e-9a4d-0f6b-8c2e-1d7a5b9e0f34",
	"version 9":         "3f2b8c1e-9a4d-9f6b-8c2e-1d7a5b9e0f34",
	"ncs variant":       "3f2b8c1e-9a4d-4f6b-0c2e-1d7a5b9e0f34",
	"microsoft variant": "3f2b8c1e-9a4d-4f6b-cc2e-1d7a5b9e0f34",
}

func TestParseConversationID(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"left out (old request)": {`{"message":"Passport documents?"}`, ""},
		"uuid v4":                {`{"message":"Passport documents?","conversation_id":"` + conversationV4 + `"}`, conversationV4},
		"uuid v7":                {`{"message":"Passport documents?","conversation_id":"` + conversationV7 + `"}`, conversationV7},
		"id first":               {`{"conversation_id":"` + conversationV4 + `","message":"Passport documents?"}`, conversationV4},
	} {
		t.Run(name, func(t *testing.T) {
			q, err := Parse(strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("rejected: %v", err)
			}

			if q.Text() != "Passport documents?" || q.ConversationID() != tc.want {
				t.Fatalf("got conversation ID %q", q.ConversationID())
			}
		})
	}
}

func TestParseRejectsMalformedConversationID(t *testing.T) {
	for name, id := range malformedConversationIDs {
		t.Run(name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"message": "hello", "conversation_id": id})
			if _, err := Parse(strings.NewReader(string(body))); !errors.Is(err, ErrInvalid) {
				t.Fatalf("accepted or wrong error: %v", err)
			}
		})
	}

	for name, body := range map[string]string{
		"null":           `{"message":"hello","conversation_id":null}`,
		"number":         `{"message":"hello","conversation_id":1}`,
		"object":         `{"message":"hello","conversation_id":{}}`,
		"wrong key case": `{"message":"hello","Conversation_Id":"` + conversationV4 + `"}`,
		"repeated":       `{"message":"hello","conversation_id":"` + conversationV4 + `","conversation_id":"` + conversationV7 + `"}`,
		"no message":     `{"conversation_id":"` + conversationV4 + `"}`,
		"extra field":    `{"message":"hello","conversation_id":"` + conversationV4 + `","history":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(body)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("accepted or wrong error: %v", err)
			}
		})
	}
}

func TestQuestionHidesConversationID(t *testing.T) {
	q, err := Parse(strings.NewReader(`{"message":"hello","conversation_id":"` + conversationV4 + `"}`))
	if err != nil {
		t.Fatal(err)
	}

	encoded, _ := json.Marshal(q)
	if strings.Contains(string(encoded)+fmt.Sprintf("%+v", q), conversationV4) {
		t.Fatal("conversation ID leaked through formatting")
	}
}

func TestEncodeConversationID(t *testing.T) {
	encoded, err := Encode(Reply{Answer: "Synthetic answer.", ConversationID: conversationV4})
	if err != nil ||
		string(encoded) != `{"answer":"Synthetic answer.","sources":[],"conversation_id":"`+conversationV4+`"}` {
		t.Fatalf("conversation ID not encoded: %s %v", encoded, err)
	}

	encoded, err = Encode(Reply{Answer: "Synthetic answer."})
	if err != nil || strings.Contains(string(encoded), "conversation_id") {
		t.Fatalf("empty conversation ID not left out: %s %v", encoded, err)
	}

	for name, id := range malformedConversationIDs {
		if id == "" {
			continue
		}

		t.Run(name, func(t *testing.T) {
			if _, err := Encode(Reply{Answer: "Synthetic answer.", ConversationID: id}); !errors.Is(err, ErrResponse) {
				t.Fatalf("accepted or wrong error: %v", err)
			}
		})
	}
}

func TestDecodeReplyConversationID(t *testing.T) {
	reply, err := DecodeReply([]byte(`{"answer":"See [1]","sources":[{"number":1,"title":"A"}],"conversation_id":"` + conversationV7 + `"}`))
	if err != nil || reply.ConversationID != conversationV7 || len(reply.Sources) != 1 {
		t.Fatalf("conversation ID not kept: %+v %v", reply.ConversationID, err)
	}

	encoded, err := Encode(reply)
	if err != nil || !strings.Contains(string(encoded), `"conversation_id":"`+conversationV7+`"`) {
		t.Fatalf("conversation ID lost on re-encode: %v", err)
	}

	reply, err = DecodeReply([]byte(`{"answer":"An answer"}`))
	if err != nil || reply.ConversationID != "" {
		t.Fatalf("reply without conversation ID rejected: %v", err)
	}

	for name, id := range malformedConversationIDs {
		t.Run(name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"answer": "x", "conversation_id": id})
			if _, err := DecodeReply(body); !errors.Is(err, ErrResponse) {
				t.Fatalf("accepted or wrong error: %v", err)
			}
		})
	}

	for name, body := range map[string]string{
		"null":     `{"answer":"x","conversation_id":null}`,
		"number":   `{"answer":"x","conversation_id":7}`,
		"repeated": `{"answer":"x","conversation_id":"` + conversationV4 + `","conversation_id":"` + conversationV4 + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReply([]byte(body)); !errors.Is(err, ErrResponse) {
				t.Fatalf("accepted or wrong error: %v", err)
			}
		})
	}
}
