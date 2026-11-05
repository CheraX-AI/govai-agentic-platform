package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestQuestionValidation(t *testing.T) {
	for _, message := range []string{
		"Passport documents?",
		"கடவுச்சீட்டிற்கு என்ன ஆவணங்கள் தேவை?",
		"විදේශ ගමන් බලපත්‍රය",
		"line one\nline two",
		"🙂 help",
	} {
		body, _ := json.Marshal(map[string]string{"message": "  " + message + "  "})
		q, err := Parse(strings.NewReader(string(body)))
		if err != nil || q.Text() != message {
			t.Fatal("valid message altered/rejected")
		}

		encoded, _ := json.Marshal(q)
		if strings.Contains(string(encoded)+fmt.Sprintf("%+v", q), message) {
			t.Fatal("question leaked through formatting")
		}
	}

	for _, body := range []string{
		`{}`,
		`[]`,
		`null`,
		`{"message":null}`,
		`{"message":1}`,
		`{"message":""}`,
		`{"message":" \t\n"}`,
		`{"Message":"hello"}`,
		`{"message":"hi","message":"other"}`,
		`{"message":"hi","role":"admin"}`,
		`{"message":"hi"}{}`,
		`{"message":"\ud800"}`,
		`{"message":"hi\u0000there"}`,
	} {
		if _, err := Parse(strings.NewReader(body)); err == nil {
			t.Fatal("invalid message accepted")
		}
	}

	long, _ := json.Marshal(map[string]string{"message": strings.Repeat("x", MaxMessageRunes+1)})
	if _, err := Parse(strings.NewReader(string(long))); !errors.Is(err, ErrInvalid) {
		t.Fatal("character limit bypassed")
	}

	if _, err := Parse(strings.NewReader(strings.Repeat("x", MaxRequestBytes+1))); !errors.Is(err, ErrTooLarge) {
		t.Fatal("byte limit bypassed")
	}
}

func TestReplyContract(t *testing.T) {
	reply := Reply{
		Answer: "Synthetic test answer.",
		Sources: []Source{
			{
				Number: 1,
				Title:  "Synthetic source",
				Page:   4,
				URL:    "https://official.example.test/guide?lang=ta#docs",
			},
		},
	}
	encoded, err := Encode(reply)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Reply
	if json.Unmarshal(encoded, &decoded) != nil ||
		decoded.Answer != reply.Answer ||
		decoded.Sources[0] != reply.Sources[0] {
		t.Fatal("response changed")
	}

	encoded, err = Encode(Reply{Answer: "Synthetic test answer."})
	if err != nil || !strings.Contains(string(encoded), `"sources":[]`) {
		t.Fatal("missing sources array")
	}

	encoded, err = Encode(
		Reply{Answer: "Synthetic [1].", Sources: []Source{{Number: 1, Title: "Synthetic source"}}},
	)
	if err != nil ||
		string(encoded) != `{"answer":"Synthetic [1].","sources":[{"number":1,"title":"Synthetic source"}]}` {
		t.Fatal("optional page and url not omitted")
	}

	for _, reply := range []Reply{{Answer: " "}, {Answer: strings.Repeat("x", (32<<10)+1)},
		{Answer: "test", Sources: []Source{{Number: 1, Title: "bad", URL: "javascript:alert(1)"}}},
		{
			Answer:  "test",
			Sources: []Source{{Number: 1, Title: "bad", URL: "https://user:pass@example.test/"}},
		},
		{Answer: "test", Sources: []Source{{Title: "no number"}}},
		{Answer: "test", Sources: []Source{{Number: 1, Title: " "}}},
		{Answer: "test", Sources: []Source{{Number: 1, Title: "a", Page: -1}}},
		{Answer: "test", Sources: []Source{{Number: 1, Title: "a"}, {Number: 1, Title: "b"}}}} {
		if _, err := Encode(reply); err == nil {
			t.Fatal("invalid response accepted")
		}
	}
}
