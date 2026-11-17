package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/chatcontract"
)

const unknownConversation = "3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34"

func chatBody(message, conversationID string) string {
	fields := map[string]string{"message": message}
	if conversationID != "" {
		fields["conversation_id"] = conversationID
	}

	body, _ := json.Marshal(fields)
	return string(body)
}

func decodeReply(t *testing.T, r *httptest.ResponseRecorder) chatcontract.Reply {
	t.Helper()
	var reply chatcontract.Reply
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &reply) != nil {
		t.Fatalf("chat failed: %d %s", r.Code, r.Body.String())
	}

	return reply
}

func errorCode(t *testing.T, r *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if json.Unmarshal(r.Body.Bytes(), &body) != nil || len(body) != 2 || body["error"] == "" {
		t.Fatalf("unsafe error envelope: %s", r.Body.String())
	}

	return body["code"]
}

func TestNewConversationReceivesUUID(t *testing.T) {
	f := newFixture(t)

	// An existing client that never sends conversation_id keeps working
	reply := decodeReply(t, f.send(f.token(t, nil), `{"message":"hello"}`))

	if !chatcontract.ValidConversationID(reply.ConversationID) {
		t.Fatalf("no conversation ID in reply: %q", reply.ConversationID)
	}

	// The owner is the verified token subject
	if f.store.owner(reply.ConversationID) != "synthetic-subject" {
		t.Fatal("conversation not owned by the verified citizen")
	}

	if reply.Answer != f.agent.reply.Answer || f.agent.calls != 1 {
		t.Fatal("existing chat answer changed")
	}
}

func TestExistingConversationKeepsItsID(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	first := decodeReply(t, f.send(token, chatBody("hello", ""))).ConversationID

	for range 2 {
		again := decodeReply(t, f.send(token, chatBody("follow-up", first))).ConversationID
		if again != first {
			t.Fatalf("conversation ID changed: %q -> %q", first, again)
		}
	}

	if f.store.creates != 1 {
		t.Fatalf("reused ID created new conversations: %d", f.store.creates)
	}
}

func TestCitizenCannotAccessAnotherCitizensConversation(t *testing.T) {
	f := newFixture(t)
	citizenA := f.token(t, map[string]any{"sub": "synthetic-citizen-a"})
	citizenB := f.token(t, map[string]any{"sub": "synthetic-citizen-b"})
	conversationA := decodeReply(t, f.send(citizenA, chatBody("hello", ""))).ConversationID
	calls := f.agent.calls

	r := f.send(citizenB, chatBody("show me", conversationA))

	// Same answer as an unknown ID, so B cannot learn that A's ID exists
	if r.Code != 404 || errorCode(t, r) != "conversation_not_found" || f.agent.calls != calls {
		t.Fatalf("citizen B reached citizen A's conversation: %d", r.Code)
	}

	if strings.Contains(r.Body.String(), conversationA) || strings.Contains(f.logs.String(), "synthetic-citizen") {
		t.Fatal("conversation or owner disclosed")
	}

	if f.store.owner(conversationA) != "synthetic-citizen-a" {
		t.Fatal("conversation owner changed")
	}
}

func TestUnknownConversationIsRejected(t *testing.T) {
	f := newFixture(t)

	r := f.send(f.token(t, nil), chatBody("hello", unknownConversation))

	if r.Code != 404 || errorCode(t, r) != "conversation_not_found" || f.agent.calls != 0 || f.store.creates != 0 {
		t.Fatalf("unknown conversation accepted: %d", r.Code)
	}
}

func TestMalformedConversationIDIsBadRequest(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	for _, body := range []string{
		chatBody("hello", strings.ToUpper(unknownConversation)),
		chatBody("hello", "not-a-uuid"),
		chatBody("hello", "00000000-0000-0000-0000-000000000000"),
		`{"message":"hello","conversation_id":null}`,
		`{"message":"hello","conversation_id":""}`,
	} {
		r := f.send(token, body)
		if r.Code != 400 || errorCode(t, r) != "invalid_conversation_id" || f.agent.calls != 0 {
			t.Fatalf("malformed conversation ID: %d %s", r.Code, body)
		}
	}

	// Clients can never name the owner: unknown fields are rejected outright
	if r := f.send(token, `{"message":"hello","citizen_id":"someone-else"}`); r.Code != 400 || f.agent.calls != 0 {
		t.Fatal("client-supplied citizen ID accepted")
	}
}

func TestConversationStorageFailureIsUnavailable(t *testing.T) {
	f := newFixture(t)
	f.store.err = errors.New("private database host and password")

	r := f.send(f.token(t, nil), `{"message":"hello"}`)

	if r.Code != 503 || errorCode(t, r) != "chat_unavailable" || f.agent.calls != 0 {
		t.Fatalf("storage failure: %d", r.Code)
	}

	if strings.Contains(r.Body.String()+f.logs.String(), "private") {
		t.Fatal("storage details disclosed")
	}
}

func TestAuthenticationPrecedesConversationLookup(t *testing.T) {
	f := newFixture(t)

	if r := f.send("", chatBody("hello", unknownConversation)); r.Code != 401 || f.store.creates != 0 {
		t.Fatal("unauthenticated request reached conversation storage")
	}
}

func TestAgentEchoOfConversationID(t *testing.T) {
	f := newFixture(t)
	token := f.token(t, nil)
	conversationID := decodeReply(t, f.send(token, `{"message":"hello"}`)).ConversationID

	// An agent that returns the same ID is fine
	f.agent.reply.ConversationID = conversationID
	if reply := decodeReply(t, f.send(token, chatBody("again", conversationID))); reply.ConversationID != conversationID {
		t.Fatal("matching agent conversation ID rejected")
	}

	// An agent that returns another ID is not trusted
	f.agent.reply.ConversationID = unknownConversation
	r := f.send(token, chatBody("again", conversationID))
	if r.Code != 502 || errorCode(t, r) != "invalid_agent_response" || strings.Contains(r.Body.String(), unknownConversation) {
		t.Fatalf("agent chose the conversation: %d", r.Code)
	}
}
