package chatcontract

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Presentation is optional untrusted display data, never HTML or an action plan.
// Invalid extensions are discarded as a unit; Answer remains the fallback.
const MaxPresentationBlocks = 20

var presentationID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func presentationText(raw json.RawMessage, limit int) bool {
	s, err := strictString(raw)
	return err == nil && utf8.ValidString(s) && strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= limit
}

func presentationArray(raw json.RawMessage, max int) ([]json.RawMessage, bool) {
	var items []json.RawMessage
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &items) != nil || len(items) == 0 || len(items) > max {
		return nil, false
	}
	return items, true
}

func presentationRefs(raw json.RawMessage, sources map[int]bool) bool {
	if raw == nil {
		return true
	}
	var refs []json.RawMessage
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &refs) != nil || len(refs) > MaxSources {
		return false
	}
	seen := map[int]bool{}
	for _, value := range refs {
		n, err := strictPositiveInt(value, MaxSourceNumber)
		if err != nil || !sources[n] || seen[n] {
			return false
		}
		seen[n] = true
	}
	return true
}

func presentationUniqueID(raw json.RawMessage, seen map[string]bool) bool {
	s, err := strictString(raw)
	if err != nil || !presentationID.MatchString(s) || seen[s] {
		return false
	}
	seen[s] = true
	return true
}

// validPresentation is deliberately strict about keys and duplicate fields.
// It validates structure/references, not factual accuracy or URL authority.
func validPresentation(raw json.RawMessage, sources []Source) bool {
	if len(raw) == 0 || len(raw) > MaxResponseBytes || !utf8.Valid(raw) || !json.Valid(raw) || !validSurrogates(raw) {
		return false
	}
	root, err := fields(raw, "version", "blocks")
	if err != nil || string(bytes.TrimSpace(root["version"])) != "1" {
		return false
	}
	blocks, ok := presentationArray(root["blocks"], MaxPresentationBlocks)
	if !ok {
		return false
	}
	numbers := map[int]bool{}
	for _, source := range sources {
		numbers[source.Number] = true
	}
	ids := map[string]bool{}
	for _, rawBlock := range blocks {
		// First inspect only the type; fields below rejects unknown/duplicate keys.
		var discriminator struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(rawBlock, &discriminator) != nil {
			return false
		}
		allowed := []string{"id", "type", "title", "source_numbers"}
		switch discriminator.Type {
		case "text":
			allowed = append(allowed, "markdown")
		case "procedure":
			allowed = append(allowed, "steps")
		case "checklist", "faq":
			allowed = append(allowed, "items")
		case "table":
			allowed = append(allowed, "columns", "rows", "caption")
		case "notice":
			allowed = append(allowed, "severity", "text", "date", "date_label")
		default:
			return false // Includes future GA-289 action types until approved.
		}
		b, err := fields(rawBlock, allowed...)
		if err != nil || !presentationUniqueID(b["id"], ids) || !presentationRefs(b["source_numbers"], numbers) {
			return false
		}
		if title, exists := b["title"]; exists && !presentationText(title, 200) {
			return false
		}
		switch discriminator.Type {
		case "text":
			if !presentationText(b["markdown"], 4000) {
				return false
			}
		case "notice":
			severity, _ := strictString(b["severity"])
			if (severity != "info" && severity != "warning") || !presentationText(b["text"], 4000) {
				return false
			}
			date, hasDate := b["date"]
			label, hasLabel := b["date_label"]
			if hasDate != hasLabel {
				return false
			}
			if hasDate {
				s, err := strictString(date)
				parsed, parseErr := time.Parse("2006-01-02", s)
				if err != nil || parseErr != nil || parsed.Format("2006-01-02") != s || !presentationText(label, 200) {
					return false
				}
			}
		case "procedure", "checklist", "faq":
			key := "items"
			if discriminator.Type == "procedure" {
				key = "steps"
			}
			items, ok := presentationArray(b[key], 20)
			if !ok {
				return false
			}
			itemIDs := map[string]bool{}
			for _, rawItem := range items {
				keys := []string{"id", "source_numbers"}
				switch discriminator.Type {
				case "procedure":
					keys = append(keys, "title", "detail")
				case "checklist":
					keys = append(keys, "text")
				case "faq":
					keys = append(keys, "question", "answer")
				}
				item, err := fields(rawItem, keys...)
				if err != nil || !presentationUniqueID(item["id"], itemIDs) || !presentationRefs(item["source_numbers"], numbers) {
					return false
				}
				for _, key := range keys[2:] {
					limit := 4000
					if key == "title" || key == "question" {
						limit = 200
					}
					if !presentationText(item[key], limit) {
						return false
					}
				}
			}
		case "table":
			columns, ok := presentationArray(b["columns"], 8)
			if !ok {
				return false
			}
			columnIDs := map[string]bool{}
			for _, column := range columns {
				c, err := fields(column, "id", "label")
				if err != nil || !presentationUniqueID(c["id"], columnIDs) || !presentationText(c["label"], 200) {
					return false
				}
			}
			rows, ok := presentationArray(b["rows"], 30)
			if !ok {
				return false
			}
			for _, row := range rows {
				cells, ok := presentationArray(row, 8)
				if !ok || len(cells) != len(columns) {
					return false
				}
				for _, cell := range cells {
					if !presentationText(cell, 500) {
						return false
					}
				}
			}
			if caption, exists := b["caption"]; exists && !presentationText(caption, 4000) {
				return false
			}
		}
	}
	return true
}
