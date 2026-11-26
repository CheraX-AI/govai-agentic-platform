package handlers

import (
	"encoding/base64"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	contract "github.com/CheraX-AI/govai-agentic-platform/backend/pkg/notificationcontract"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/inbox"
)

var errPage = errors.New("invalid notification page")

var notificationID = regexp.MustCompile(
	"^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
)

func parsePage(raw string) (inbox.Page, error) {
	page := inbox.Page{Limit: 20}
	query, err := url.ParseQuery(raw)
	if err != nil || len(raw) > 1024 {
		return inbox.Page{}, errPage
	}
	for key, values := range query {
		if len(values) != 1 || values[0] == "" {
			return inbox.Page{}, errPage
		}
		switch key {
		case "limit":
			limit, err := strconv.Atoi(values[0])
			if err != nil || limit < 1 || limit > 100 || strconv.Itoa(limit) != values[0] {
				return inbox.Page{}, errPage
			}
			page.Limit = limit
		case "cursor":
			cursor, err := decodeCursor(values[0])
			if err != nil {
				return inbox.Page{}, errPage
			}
			page.Before = &cursor
		default:
			return inbox.Page{}, errPage
		}
	}
	return page, nil
}

// Cursors contain ordering keys only. They grant no authority: the SQL adapter
// must apply the verified owner filter independently on every page.

func encodeCursor(cursor inbox.Cursor) string {
	text := cursor.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + cursor.ID
	return base64.RawURLEncoding.EncodeToString([]byte(text))
}

func decodeCursor(raw string) (inbox.Cursor, error) {
	if len(raw) > 256 {
		return inbox.Cursor{}, errPage
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(data) != raw {
		return inbox.Cursor{}, errPage
	}
	stamp, id, ok := strings.Cut(string(data), "|")
	created, err := time.Parse(time.RFC3339Nano, stamp)
	if !ok || err != nil || created.Year() < 1970 || !notificationID.MatchString(id) ||
		created.UTC().Format(time.RFC3339Nano) != stamp {
		return inbox.Cursor{}, errPage
	}
	return inbox.Cursor{CreatedAt: created, ID: id}, nil
}

func before(row inbox.Notification, cursor inbox.Cursor) bool {
	return row.CreatedAt.Before(cursor.CreatedAt) ||
		(row.CreatedAt.Equal(cursor.CreatedAt) && row.ID < cursor.ID)
}

func validPage(result inbox.Result, page inbox.Page) bool {
	if len(result.Items) > page.Limit || (result.HasMore && len(result.Items) == 0) {
		return false
	}
	seen := make(map[string]bool, len(result.Items))
	previous := page.Before
	for _, row := range result.Items {
		priority, err := contract.PriorityFor(row.EventType)
		if err != nil || string(priority) != row.Priority || row.Locale != "en" ||
			!notificationID.MatchString(row.ID) || seen[row.ID] ||
			row.CreatedAt.IsZero() || row.CreatedAt.Year() < 1970 ||
			!safeText(row.Title, 256, false) || !safeText(row.Body, 8192, true) ||
			(row.ReadAt != nil && row.ReadAt.Before(row.CreatedAt)) ||
			(previous != nil && !before(row, *previous)) {
			return false
		}
		seen[row.ID] = true
		previous = &inbox.Cursor{CreatedAt: row.CreatedAt, ID: row.ID}
	}
	return true
}

func safeText(text string, limit int, multiline bool) bool {
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || len(text) > limit {
		return false
	}
	for _, character := range text {
		if character == 127 || (character < 32 &&
			(!multiline || (character != '\n' && character != '\t'))) {
			return false
		}
	}
	return true
}
