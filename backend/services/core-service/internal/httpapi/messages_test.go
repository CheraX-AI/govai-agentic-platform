package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const storedQuestion = "Synthetic private question about my NIC"

// exchange sends the first question of a new conversation and returns its ID.
func (f *fixture) exchange(t *testing.T, token string) string {
	t.Helper()
	r := f.send(token, chatBody(storedQuestion, ""))
	if r.Code != 200 {
		t.Fatalf("first question failed: %d", r.Code)
	}

	return decodeReply(t, r).ConversationID
}

func TestExchangeIsStoredEncrypted(t *testing.T) {
	f := newFixture(t)
	id := f.exchange(t, f.token(t, nil))
	rows := f.store.Messages(id)
	if len(rows) != 2 ||
		rows[0].Role != "citizen" || rows[0].Status != "completed" ||
		rows[1].Role != "assistant" || rows[1].Status != "completed" || rows[1].ReplyTo != rows[0].ID {
		t.Fatalf("unexpected rows: %+v", rows)
	}

	for _, row := range rows {
		if bytes.Contains(row.Sealed, []byte("NIC")) || bytes.Contains(row.Sealed, []byte("Synthetic answer")) {
			t.Fatal("plaintext stored")
		}
	}

	if strings.Contains(f.logs.String(), "NIC") || strings.Contains(f.logs.String(), "Synthetic answer") {
		t.Fatal("message content logged")
	}
}

func TestFailedAgentCallsAreRecorded(t *testing.T) {
	for name, broken := range map[string]func(f *fixture){
		"agent error":       func(f *fixture) { f.agent.err = errors.New("private upstream detail") },
		"wrong echoed ID":   func(f *fixture) { f.agent.reply.ConversationID = unknownConversation },
		"unencodable reply": func(f *fixture) { f.agent.reply.Answer = " " },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			token := f.token(t, nil)
			id := f.exchange(t, token)
			broken(f)
			if r := f.send(token, chatBody("Synthetic follow-up", id)); r.Code < 500 {
				t.Fatalf("failure reported as %d", r.Code)
			}

			rows := f.store.Messages(id)
			if len(rows) != 3 || rows[2].Role != "citizen" || rows[2].Status != "failed" {
				t.Fatalf("failed call not recorded: %+v", rows)
			}
		})
	}
}

func TestRetryOfAnsweredQuestionDoesNotDuplicateExchange(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	id := f.exchange(t, token)

	// The client lost the reply and sends the same question again
	r := f.send(token, chatBody(storedQuestion, id))
	if r.Code != 200 || f.agent.calls != 1 || len(f.store.Messages(id)) != 2 {
		t.Fatalf("retry duplicated the exchange: status=%d calls=%d rows=%d", r.Code, f.agent.calls, len(f.store.Messages(id)))
	}

	// Same response format: the stored answer, empty sources, the same ID
	var body map[string]any
	if json.Unmarshal(r.Body.Bytes(), &body) != nil || len(body) != 3 ||
		body["answer"] != "Synthetic answer only." || body["conversation_id"] != id {
		t.Fatalf("unexpected replay body: %s", r.Body.String())
	}

	if sources, ok := body["sources"].([]any); !ok || len(sources) != 0 {
		t.Fatal("replay must keep the sources field")
	}
}

func TestRetryAfterFailedAgentCallReusesTheQuestion(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	id := f.exchange(t, token)
	f.agent.err = errors.New("synthetic outage")
	if r := f.send(token, chatBody("Synthetic follow-up", id)); r.Code != 503 {
		t.Fatalf("outage reported as %d", r.Code)
	}

	f.agent.err = nil
	if r := f.send(token, chatBody("Synthetic follow-up", id)); r.Code != 200 {
		t.Fatalf("retry failed: %d", r.Code)
	}

	rows := f.store.Messages(id)
	if len(rows) != 4 || f.agent.calls != 3 || rows[2].Status != "completed" || rows[3].ReplyTo != rows[2].ID {
		t.Fatalf("retry added a turn: %+v", rows)
	}
}

func TestSaveFailureIsNotReportedAsSuccess(t *testing.T) {
	f := newFixture(t)
	f.store.CompleteErr = errors.New("private database detail")
	r := f.send(f.token(t, nil), chatBody(storedQuestion, ""))
	if r.Code != 503 || errorCode(t, r) != "chat_unavailable" || strings.Contains(r.Body.String(), "Synthetic answer") {
		t.Fatalf("unsaved answer returned: %d %s", r.Code, r.Body.String())
	}

	if strings.Contains(f.logs.String(), "private database detail") {
		t.Fatal("database error logged")
	}
}

func TestStartFailureSkipsTheAgent(t *testing.T) {
	f := newFixture(t)
	f.store.StartErr = errors.New("private database detail")
	r := f.send(f.token(t, nil), chatBody(storedQuestion, ""))
	if r.Code != 503 || errorCode(t, r) != "chat_unavailable" || f.agent.calls != 0 {
		t.Fatalf("unsaved question reached the agent: %d calls=%d", r.Code, f.agent.calls)
	}
}

func TestDuplicateWhileInProgressIsBusy(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	id := f.exchange(t, token)

	// Another request is still answering this question
	if _, err := f.messages.Start(context.Background(), "synthetic-subject", id, "Synthetic slow question"); err != nil {
		t.Fatal(err)
	}

	r := f.send(token, chatBody("Synthetic slow question", id))
	if r.Code != 503 || errorCode(t, r) != "chat_busy" || r.Header().Get("Retry-After") != "2" || f.agent.calls != 1 {
		t.Fatalf("duplicate in-flight question: %d calls=%d", r.Code, f.agent.calls)
	}
}
