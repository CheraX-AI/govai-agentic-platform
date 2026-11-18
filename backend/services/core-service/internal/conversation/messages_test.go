package conversation_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/conversation"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/core-service/internal/conversation/conversationtest"
)

// Synthetic keys and texts only.
var (
	keyA = bytes.Repeat([]byte{0x11}, 32)
	keyB = bytes.Repeat([]byte{0x22}, 32)
)

const (
	ownerA   = "synthetic-citizen-a"
	ownerB   = "synthetic-citizen-b"
	inFlight = time.Minute
	question = "Synthetic question about passport renewal"
	answer   = "Synthetic answer only."
)

type setup struct {
	store    *conversationtest.Store
	messages *conversation.Messages
	id       string
}

func newSetup(t *testing.T) setup {
	t.Helper()
	store := conversationtest.New()
	service, err := conversation.NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	id, err := service.Resolve(context.Background(), ownerA, "")
	if err != nil {
		t.Fatal(err)
	}

	messages, err := conversation.NewMessages(store, keyA, inFlight)
	if err != nil {
		t.Fatal(err)
	}

	return setup{store: store, messages: messages, id: id}
}

func (s setup) start(t *testing.T, text string) conversation.Turn {
	t.Helper()
	turn, err := s.messages.Start(context.Background(), ownerA, s.id, text)
	if err != nil {
		t.Fatal(err)
	}

	return turn
}

// open decrypts a stored row and fails the test if it does not decrypt.
func open(t *testing.T, key []byte, m conversationtest.Message) string {
	t.Helper()
	text, err := conversation.Open(key, m.Sealed, m.ConversationID, m.ID, m.Role)
	if err != nil {
		t.Fatalf("stored %s message does not decrypt", m.Role)
	}

	return text
}

func TestQuestionIsStoredPendingAndEncrypted(t *testing.T) {
	s := newSetup(t)
	s.start(t, question)
	rows := s.store.Messages(s.id)
	if len(rows) != 1 || rows[0].Role != "citizen" || rows[0].Status != "pending" || rows[0].ReplyTo != "" {
		t.Fatalf("unexpected rows: %+v", rows)
	}

	if bytes.Contains(rows[0].Sealed, []byte("passport")) || len(rows[0].Sealed) != 12+len(question)+16 {
		t.Fatal("question stored in plaintext or not as nonce + ciphertext + tag")
	}

	if open(t, keyA, rows[0]) != question {
		t.Fatal("stored question does not decrypt to the original text")
	}
}

func TestAnswerIsStoredAndTurnCompleted(t *testing.T) {
	s := newSetup(t)
	turn := s.start(t, question)
	if err := s.messages.Complete(context.Background(), turn, answer); err != nil {
		t.Fatal(err)
	}

	rows := s.store.Messages(s.id)
	if len(rows) != 2 || rows[0].Status != "completed" || rows[1].Role != "assistant" ||
		rows[1].Status != "completed" || rows[1].ReplyTo != rows[0].ID {
		t.Fatalf("unexpected rows: %+v", rows)
	}

	if open(t, keyA, rows[0]) != question || open(t, keyA, rows[1]) != answer {
		t.Fatal("stored content does not decrypt to the original text")
	}
}

func TestCiphertextIsBoundToItsRowAndKey(t *testing.T) {
	s := newSetup(t)
	s.messages.Complete(context.Background(), s.start(t, question), answer)
	rows := s.store.Messages(s.id)
	q, a := rows[0], rows[1]
	for name, c := range map[string]struct {
		key                         []byte
		conversation, message, role string
	}{
		"wrong key":          {keyB, q.ConversationID, q.ID, q.Role},
		"other message ID":   {keyA, q.ConversationID, a.ID, q.Role},
		"other role":         {keyA, q.ConversationID, q.ID, "assistant"},
		"other conversation": {keyA, a.ID, q.ID, q.Role},
	} {
		if _, err := conversation.Open(c.key, q.Sealed, c.conversation, c.message, c.role); err == nil {
			t.Fatalf("%s: ciphertext opened", name)
		}
	}
}

func TestFailedAgentCallMarksQuestionFailed(t *testing.T) {
	s := newSetup(t)
	if err := s.messages.Fail(context.Background(), s.start(t, question)); err != nil {
		t.Fatal(err)
	}

	if rows := s.store.Messages(s.id); len(rows) != 1 || rows[0].Status != "failed" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestRetryAfterFailureReusesTheQuestion(t *testing.T) {
	s := newSetup(t)
	s.messages.Fail(context.Background(), s.start(t, question))
	retry := s.start(t, question)
	if _, replayed := retry.Answered(); replayed {
		t.Fatal("failed question has no answer to replay")
	}

	if rows := s.store.Messages(s.id); len(rows) != 1 || rows[0].Status != "pending" {
		t.Fatalf("retry added a second question: %+v", rows)
	}

	if err := s.messages.Complete(context.Background(), retry, answer); err != nil {
		t.Fatal(err)
	}

	if rows := s.store.Messages(s.id); len(rows) != 2 || rows[1].ReplyTo != rows[0].ID {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestRetryAfterAnswerReturnsStoredAnswer(t *testing.T) {
	s := newSetup(t)
	s.messages.Complete(context.Background(), s.start(t, question), answer)
	retry := s.start(t, question)
	if got, replayed := retry.Answered(); !replayed || got != answer {
		t.Fatal("retry of an answered question did not get the stored answer")
	}

	// Completing or failing the replayed turn changes nothing
	if s.messages.Complete(context.Background(), retry, "Other synthetic answer.") != nil ||
		s.messages.Fail(context.Background(), retry) != nil {
		t.Fatal("replayed turn could not be finished")
	}

	if rows := s.store.Messages(s.id); len(rows) != 2 || rows[0].Status != "completed" {
		t.Fatalf("retry duplicated the exchange: %+v", rows)
	}
}

func TestRepeatAfterReplayWindowIsANewQuestion(t *testing.T) {
	s := newSetup(t)
	s.messages.Complete(context.Background(), s.start(t, question), answer)
	s.store.Advance(11 * time.Minute)
	if _, replayed := s.start(t, question).Answered(); replayed {
		t.Fatal("old answer replayed")
	}

	if rows := s.store.Messages(s.id); len(rows) != 3 || rows[2].Status != "pending" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestDifferentQuestionIsANewTurn(t *testing.T) {
	s := newSetup(t)
	s.messages.Fail(context.Background(), s.start(t, question))
	s.start(t, "Synthetic follow-up question")
	rows := s.store.Messages(s.id)
	if len(rows) != 2 || rows[0].Status != "failed" || rows[1].Status != "pending" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestRetryWhileInProgressIsRejectedUntilStale(t *testing.T) {
	s := newSetup(t)
	first := s.start(t, question)
	if _, err := s.messages.Start(context.Background(), ownerA, s.id, question); !errors.Is(err, conversation.ErrInProgress) {
		t.Fatal("duplicate in-flight question accepted")
	}

	// The first request is lost; after inFlight the question can be retried in place
	s.store.Advance(inFlight + time.Second)
	retry := s.start(t, question)
	if rows := s.store.Messages(s.id); len(rows) != 1 || rows[0].Status != "pending" {
		t.Fatalf("unexpected rows: %+v", rows)
	}

	if err := s.messages.Complete(context.Background(), retry, answer); err != nil {
		t.Fatal(err)
	}

	// The lost request cannot complete the same question a second time
	if s.messages.Complete(context.Background(), first, "Late synthetic answer.") == nil {
		t.Fatal("question answered twice")
	}

	if rows := s.store.Messages(s.id); len(rows) != 2 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestOnlyTheOwnerCanAddMessages(t *testing.T) {
	s := newSetup(t)
	if _, err := s.messages.Start(context.Background(), ownerB, s.id, question); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatal("another citizen wrote to the conversation")
	}

	if len(s.store.Messages(s.id)) != 0 {
		t.Fatal("message stored for another citizen")
	}
}

func TestStorageFailuresAreReported(t *testing.T) {
	s := newSetup(t)
	s.store.StartErr = errors.New("private database detail")
	if _, err := s.messages.Start(context.Background(), ownerA, s.id, question); !errors.Is(err, conversation.ErrUnavailable) {
		t.Fatal("start failure hidden")
	}

	s.store.StartErr = nil
	turn := s.start(t, question)
	s.store.CompleteErr = errors.New("private database detail")
	if err := s.messages.Complete(context.Background(), turn, answer); !errors.Is(err, conversation.ErrUnavailable) {
		t.Fatal("complete failure hidden")
	}

	s.store.FailErr = errors.New("private database detail")
	if err := s.messages.Fail(context.Background(), turn); !errors.Is(err, conversation.ErrUnavailable) {
		t.Fatal("fail failure hidden")
	}
}

func TestKeyChangeStartsANewTurnInsteadOfFailing(t *testing.T) {
	s := newSetup(t)
	s.messages.Complete(context.Background(), s.start(t, question), answer)
	rotated, err := conversation.NewMessages(s.store, keyB, inFlight)
	if err != nil {
		t.Fatal(err)
	}

	turn, err := rotated.Start(context.Background(), ownerA, s.id, question)
	if err != nil {
		t.Fatal(err)
	}

	if _, replayed := turn.Answered(); replayed || len(s.store.Messages(s.id)) != 3 {
		t.Fatal("unreadable old turn was treated as a retry")
	}
}

func TestInvalidInputAndSettingsAreRejected(t *testing.T) {
	store := conversationtest.New()
	for name, build := range map[string]func() (*conversation.Messages, error){
		"no store":    func() (*conversation.Messages, error) { return conversation.NewMessages(nil, keyA, inFlight) },
		"short key":   func() (*conversation.Messages, error) { return conversation.NewMessages(store, keyA[:16], inFlight) },
		"no key":      func() (*conversation.Messages, error) { return conversation.NewMessages(store, nil, inFlight) },
		"no inFlight": func() (*conversation.Messages, error) { return conversation.NewMessages(store, keyA, 0) },
	} {
		if _, err := build(); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}

	s := newSetup(t)
	for _, args := range [][3]string{
		{"", s.id, question},
		{ownerA, "not-a-uuid", question},
		{ownerA, s.id, ""},
	} {
		if _, err := s.messages.Start(context.Background(), args[0], args[1], args[2]); !errors.Is(err, conversation.ErrUnavailable) {
			t.Fatalf("invalid input accepted: %q", args)
		}
	}

	if s.messages.Complete(context.Background(), conversation.Turn{}, answer) == nil ||
		s.messages.Fail(context.Background(), conversation.Turn{}) == nil {
		t.Fatal("empty turn accepted")
	}
}

// answered adds one completed turn.
func (s setup) answered(t *testing.T, q, a string) {
	t.Helper()
	if err := s.messages.Complete(context.Background(), s.start(t, q), a); err != nil {
		t.Fatal(err)
	}
}

func TestRecentReturnsCompletedTurnsOldestFirst(t *testing.T) {
	s := newSetup(t)
	s.answered(t, "Synthetic question 1", "Synthetic answer 1")
	s.messages.Fail(context.Background(), s.start(t, "Synthetic failed question"))
	s.answered(t, "Synthetic question 2", "Synthetic answer 2")
	s.start(t, "Synthetic pending question")

	got, err := s.messages.Recent(context.Background(), ownerA, s.id, 10)
	if err != nil {
		t.Fatal(err)
	}

	want := []conversation.Message{
		{Role: "citizen", Text: "Synthetic question 1"}, {Role: "assistant", Text: "Synthetic answer 1"},
		{Role: "citizen", Text: "Synthetic question 2"}, {Role: "assistant", Text: "Synthetic answer 2"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d messages, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("message %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestRecentKeepsOnlyTheLastTurns(t *testing.T) {
	s := newSetup(t)
	for _, n := range []string{"1", "2", "3"} {
		s.answered(t, "Synthetic question "+n, "Synthetic answer "+n)
	}

	got, err := s.messages.Recent(context.Background(), ownerA, s.id, 2)
	if err != nil || len(got) != 4 || got[0].Text != "Synthetic question 2" || got[3].Text != "Synthetic answer 3" {
		t.Fatalf("unexpected window: %+v %v", got, err)
	}
}

func TestRecentChecksOwnership(t *testing.T) {
	s := newSetup(t)
	s.answered(t, question, answer)
	if got, err := s.messages.Recent(context.Background(), ownerB, s.id, 10); !errors.Is(err, conversation.ErrNotFound) || got != nil {
		t.Fatal("another citizen read the conversation")
	}
}

func TestRecentIsEmptyForNewConversation(t *testing.T) {
	s := newSetup(t)
	if got, err := s.messages.Recent(context.Background(), ownerA, s.id, 10); err != nil || len(got) != 0 {
		t.Fatalf("unexpected messages: %+v %v", got, err)
	}
}

func TestRecentSkipsTurnsSealedWithAnotherKey(t *testing.T) {
	s := newSetup(t)
	s.answered(t, question, answer)
	rotated, err := conversation.NewMessages(s.store, keyB, inFlight)
	if err != nil {
		t.Fatal(err)
	}

	if got, err := rotated.Recent(context.Background(), ownerA, s.id, 10); err != nil || len(got) != 0 {
		t.Fatalf("unreadable turn returned: %+v %v", got, err)
	}
}

func TestRecentRejectsBadInputAndStorageFailure(t *testing.T) {
	s := newSetup(t)
	for _, limit := range []int{0, -1, conversation.MaxRecentTurns + 1} {
		if _, err := s.messages.Recent(context.Background(), ownerA, s.id, limit); !errors.Is(err, conversation.ErrUnavailable) {
			t.Fatalf("limit %d accepted", limit)
		}
	}

	if _, err := s.messages.Recent(context.Background(), ownerA, "not-a-uuid", 10); !errors.Is(err, conversation.ErrUnavailable) {
		t.Fatal("malformed conversation ID accepted")
	}

	s.store.Err = errors.New("private database detail")
	if _, err := s.messages.Recent(context.Background(), ownerA, s.id, 10); !errors.Is(err, conversation.ErrUnavailable) {
		t.Fatal("storage failure hidden")
	}
}
