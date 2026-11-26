package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/preferences"
)

func TestCitizenPreferenceReplacement(t *testing.T) {
	f := newAPIFixture(t)
	alice, bob := f.token(t, "alice", nil), f.token(t, "bob", nil)
	body := `{"email_enabled":false,"in_app_enabled":true}`
	response := f.send("PUT", "/api/notifications/preferences", alice, body)
	if response.Code != 200 || f.store.channels["alice"] != (preferences.Channels{
		EmailEnabled: false, InAppEnabled: true,
	}) || !strings.Contains(response.Body.String(), `"security_in_app_required":true`) {
		t.Fatal("channel choices were not saved for the authenticated owner")
	}
	response = f.send("GET", "/api/notifications/preferences", alice, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"in_app_enabled":true`) {
		t.Fatal("saved choices did not round-trip")
	}
	response = f.send("GET", "/api/notifications/preferences", bob, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"in_app_enabled":false`) {
		t.Fatal("other citizen inherited changed preferences")
	}
}

func TestCitizenRejectsAmbiguousInputBeforeStorage(t *testing.T) {
	f := newAPIFixture(t)
	token := f.token(t, "alice", nil)
	for _, body := range []string{
		`{}`, `null`, `[]`, `{"email_enabled":true}`,
		`{"email_enabled":null,"in_app_enabled":true}`,
		`{"email_enabled":"false","in_app_enabled":true}`,
		`{"email_enabled":true,"email_enabled":false,"in_app_enabled":true}`,
		`{"email_enabled":true,"in_app_enabled":true,"subject":"bob"}`,
		`{"email_enabled":true,"in_app_enabled":true,"locale":"ta"}`,
		`{"email_enabled":true,"in_app_enabled":true} {}`,
	} {
		response := f.send("PUT", "/api/notifications/preferences", token, body)
		if response.Code != 400 || f.store.calls != 0 {
			t.Fatal("ambiguous preference request reached storage")
		}
	}
	response := f.send("PUT", "/api/notifications/preferences", token, strings.Repeat("x", 1025))
	if response.Code != 413 || f.store.calls != 0 {
		t.Fatal("oversized preferences reached storage")
	}
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/api/notifications?subject=bob", ""},
		{"GET", "/api/notifications?limit=10&limit=20", ""},
		{"GET", "/api/notifications?limit=101", ""},
		{"GET", "/api/notifications?cursor=invalid", ""},
		{"GET", "/api/notifications/unread-count?owner=bob", ""},
		{"GET", "/api/notifications/preferences?subject=bob", ""},
		{"PATCH", "/api/notifications/not-a-uuid/read", ""},
		{"PATCH", "/api/notifications/" + inboxID + "/read", `{"subject":"bob"}`},
	} {
		response := f.send(route.method, route.path, token, route.body)
		if response.Code != 400 || f.store.calls != 0 {
			t.Fatal("unsupported owner selector or pagination reached storage")
		}
	}
	for _, contentType := range []string{"text/plain", "application/json; charset=latin1"} {
		request := httptest.NewRequest("PUT",
			"https://notifications.test/api/notifications/preferences",
			strings.NewReader(`{"email_enabled":false,"in_app_enabled":true}`),
		)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", contentType)
		response := httptest.NewRecorder()
		f.router.ServeHTTP(response, request)
		if response.Code != 415 || f.store.calls != 0 {
			t.Fatal("unsupported content type reached storage")
		}
	}
}

func TestCitizenCursorRoundTrip(t *testing.T) {
	f := newAPIFixture(t)
	f.store.result.HasMore = true
	response := f.send("GET", "/api/notifications?limit=1", f.token(t, "alice", nil), "")
	var page listResponse
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil ||
		page.NextCursor == nil || f.store.page.Limit != 1 {
		t.Fatal("first page did not return a bounded cursor")
	}
	if strings.Contains(*page.NextCursor, "alice") {
		t.Fatal("cursor contains owner data")
	}
	f.store.result.Items[0].ID = "123e4567-e89b-42d3-a456-426614173999"
	f.store.result.HasMore = false
	response = f.send("GET", "/api/notifications?limit=1&cursor="+*page.NextCursor,
		f.token(t, "alice", nil), "")
	if response.Code != 200 || f.store.page.Before == nil ||
		f.store.page.Before.ID != inboxID {
		t.Fatal("second page did not use the original ordering boundary")
	}
}
