package handlers

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/inbox"
)

func TestPaginationRejectsUnboundedOrMalformedValues(t *testing.T) {
	for _, query := range []string{
		"limit=0", "limit=-1", "limit=01", "limit=+1", "limit=101", "limit=",
		"cursor=", "limit=5&limit=5", "user_id=bob", "limit=%xx", "cursor=abc=",
		"cursor=" + strings.Repeat("a", 257),
	} {
		if _, err := parsePage(query); err == nil {
			t.Fatal("invalid pagination accepted")
		}
	}
	if page, err := parsePage(""); err != nil || page.Limit != 20 || page.Before != nil {
		t.Fatal("default pagination incorrect")
	}
	for _, raw := range []string{
		"bad-time|" + inboxID,
		"2026-11-26T05:00:00Z|bad-id",
		"2026-11-26T05:00:00+00:00|" + inboxID,
		"1969-11-26T05:00:00Z|" + inboxID,
	} {
		if _, err := decodeCursor(base64.RawURLEncoding.EncodeToString([]byte(raw))); err == nil {
			t.Fatal("malformed cursor accepted")
		}
	}
}

func TestInvalidStoredPageNeverReachesClient(t *testing.T) {
	row := inbox.Notification{
		ID: inboxID, EventType: "citizen.login.succeeded", Priority: "high", Locale: "en",
		Title: "New sign-in", Body: "Synthetic text.", CreatedAt: time.Now().UTC(),
	}
	for _, change := range []func(*inbox.Notification){
		func(row *inbox.Notification) { row.Priority = "normal" },
		func(row *inbox.Notification) { row.ID = "bad-id" },
		func(row *inbox.Notification) { row.Locale = "unknown" },
		func(row *inbox.Notification) { row.Title = "" },
		func(row *inbox.Notification) { row.Body = "bad\x00body" },
		func(row *inbox.Notification) { row.CreatedAt = time.Time{} },
		func(row *inbox.Notification) {
			before := row.CreatedAt.Add(-time.Hour)
			row.ReadAt = &before
		},
	} {
		invalid := row
		change(&invalid)
		if validPage(inbox.Result{Items: []inbox.Notification{invalid}}, inbox.Page{Limit: 20}) {
			t.Fatal("invalid stored row accepted")
		}
	}
	for _, result := range []inbox.Result{
		{HasMore: true},
		{Items: []inbox.Notification{row, row}},
	} {
		if validPage(result, inbox.Page{Limit: 20}) {
			t.Fatal("empty continuation or duplicate rows accepted")
		}
	}
	boundary := inbox.Cursor{CreatedAt: row.CreatedAt, ID: row.ID}
	if validPage(inbox.Result{Items: []inbox.Notification{row}}, inbox.Page{
		Limit: 20, Before: &boundary,
	}) {
		t.Fatal("cursor boundary row repeated")
	}
	if err := RegisterCitizenRoutes(nil, nil, nil, nil); err == nil {
		t.Fatal("missing API dependencies accepted")
	}
}
