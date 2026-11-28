package chatcontract

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPresentationFixtures(t *testing.T) {
	body, err := os.ReadFile("testdata/presentation.json")
	if err != nil {
		t.Fatal(err)
	}
	reply, err := DecodeReply(body)
	if err != nil || len(reply.Presentation) == 0 {
		t.Fatal("valid templates rejected", err)
	}
	encoded, err := Encode(reply)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeReply(encoded)
	var compact bytes.Buffer
	if json.Compact(&compact, reply.Presentation) != nil {
		t.Fatal("invalid fixture JSON")
	}
	if err != nil || string(again.Presentation) != compact.String() {
		t.Fatal("presentation roundtrip failed")
	}
	var legacy struct {
		Answer  string
		Sources []Source
	}
	if json.Unmarshal(encoded, &legacy) != nil || legacy.Answer != reply.Answer || len(legacy.Sources) != 1 {
		t.Fatal("legacy consumer lost answer/evidence")
	}
}

func TestPresentationFallback(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown version":  `{"version":2,"blocks":[{"id":"t","type":"text","markdown":"Hi"}]}`,
		"unknown block":    `{"version":1,"blocks":[{"id":"t","type":"office","url":"javascript:alert(1)"}]}`,
		"missing field":    `{"version":1,"blocks":[{"id":"t","type":"text"}]}`,
		"duplicate key":    `{"version":1,"version":1,"blocks":[]}`,
		"duplicate ids":    `{"version":1,"blocks":[{"id":"t","type":"text","markdown":"A"},{"id":"t","type":"text","markdown":"B"}]}`,
		"broken source":    `{"version":1,"blocks":[{"id":"t","type":"text","markdown":"A","source_numbers":[2]}]}`,
		"duplicate source": `{"version":1,"blocks":[{"id":"t","type":"text","markdown":"A","source_numbers":[1,1]}]}`,
		"html field":       `{"version":1,"blocks":[{"id":"t","type":"text","markdown":"A","html":"<script>alert(1)</script>"}]}`,
		"event handler":    `{"version":1,"blocks":[{"id":"t","type":"text","markdown":"A","onclick":"run()"}]}`,
		"ragged table":     `{"version":1,"blocks":[{"id":"t","type":"table","columns":[{"id":"a","label":"A"}],"rows":[["A","B"]]}]}`,
		"bad date":         `{"version":1,"blocks":[{"id":"t","type":"notice","severity":"warning","text":"A","date":"2026-02-30","date_label":"Issued"}]}`,
		"partial notice":   `{"version":1,"blocks":[{"id":"t","type":"notice","severity":"warning","text":"A","date_label":"Issued"}]}`,
		"null":             `null`, "array": `[]`, "empty": `{}`, "empty blocks": `{"version":1,"blocks":[]}`,
		"null field":     `{"version":1,"blocks":[{"id":"t","type":"text","markdown":null}]}`,
		"oversized text": `{"version":1,"blocks":[{"id":"t","type":"text","markdown":"` + strings.Repeat("අ", 4001) + `"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			body := `{"answer":"Safe fallback [1]","sources":[{"number":1,"title":"Synthetic"}],"presentation":` + raw + `}`
			reply, err := DecodeReply([]byte(body))
			if err != nil || reply.Presentation != nil || reply.Answer != "Safe fallback [1]" || len(reply.Sources) != 1 {
				t.Fatal("optional content broke fallback", err)
			}
			reply.Presentation = json.RawMessage(raw)
			encoded, err := Encode(reply)
			if err != nil || strings.Contains(string(encoded), `"presentation"`) {
				t.Fatal("encode forwarded invalid extension")
			}
		})
	}
}

func TestPresentationUntrustedTextAndEnvelope(t *testing.T) {
	// HTML-looking text is inert data, not sanitized/trusted HTML. Renderers must
	// disable HTML and apply URL policy; Go contract validation cannot prove XSS safety.
	raw := `{"version":1,"blocks":[{"id":"t","type":"text","markdown":"<script>alert(1)</script> [bad](javascript:alert) අවශ්‍ය ලේඛන தேவையான ஆவணங்கள்"}]}`
	reply, err := DecodeReply([]byte(`{"answer":"Fallback","presentation":` + raw + `}`))
	if err != nil || len(reply.Presentation) == 0 {
		t.Fatal("literal text rejected")
	}
	for _, body := range []string{
		`{"answer":"","presentation":` + raw + `}`,
		`{"answer":"A","presentation":{},"presentation":{}}`,
		`{"answer":"A","presentation":{"version":1,`,
		`{"answer":"A","presentation":{},"identity":"forged"}`,
	} {
		if _, err := DecodeReply([]byte(body)); err == nil {
			t.Fatal("invalid base envelope accepted")
		}
	}
}

func TestPresentationLimits(t *testing.T) {
	text := func(s string) json.RawMessage {
		value, _ := json.Marshal(map[string]any{"version": 1, "blocks": []any{map[string]any{"id": "t", "type": "text", "markdown": s}}})
		return value
	}
	if !validPresentation(text(strings.Repeat("අ", 4000)), nil) || validPresentation(text(strings.Repeat("අ", 4001)), nil) {
		t.Fatal("text limit must count Unicode code points")
	}
	var blocks []any
	for i := 0; i < 21; i++ {
		blocks = append(blocks, map[string]any{"id": strings.Repeat("x", i+1), "type": "text", "markdown": "A"})
		value, _ := json.Marshal(map[string]any{"version": 1, "blocks": blocks})
		if validPresentation(value, nil) != (i < 20) {
			t.Fatal("block limit not enforced")
		}
	}
	// Individually valid fields can exceed the envelope budget when combined.
	blocks = nil
	for i := 0; i < 12; i++ {
		blocks = append(blocks, map[string]any{"id": strings.Repeat("x", i+1), "type": "text", "markdown": strings.Repeat("a", 4000)})
	}
	value, _ := json.Marshal(map[string]any{"version": 1, "blocks": blocks})
	encoded, err := Encode(Reply{Answer: strings.Repeat("a", 32000), Presentation: value})
	if err != nil || strings.Contains(string(encoded), `"presentation"`) {
		t.Fatal("optional extension exceeded legacy response budget")
	}
}
