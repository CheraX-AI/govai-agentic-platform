package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/preferences"
	"github.com/gin-gonic/gin"
)

func emptyBody(c *gin.Context) bool {
	if c.Request.Body == nil {
		return true
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
	return err == nil && len(data) == 0
}

// parsePreferences requires a complete two-boolean replacement. Explicit token
// decoding rejects duplicate keys, nulls, extra fields and trailing JSON values.

func parsePreferences(c *gin.Context) (preferences.Channels, int) {
	var channels preferences.Channels
	media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" ||
		len(c.Request.Header.Values("Content-Type")) != 1 ||
		len(c.Request.Header.Values("Content-Encoding")) != 0 {
		return channels, http.StatusUnsupportedMediaType
	}
	for key, value := range params {
		if key != "charset" || !strings.EqualFold(value, "utf-8") {
			return channels, http.StatusUnsupportedMediaType
		}
	}
	if c.Request.Body == nil {
		return channels, http.StatusBadRequest
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, 1025))
	if len(data) > 1024 {
		return channels, http.StatusRequestEntityTooLarge
	}
	if err != nil {
		return channels, http.StatusBadRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return channels, http.StatusBadRequest
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] ||
			(key != "email_enabled" && key != "in_app_enabled") {
			return preferences.Channels{}, http.StatusBadRequest
		}
		value, err := decoder.Token()
		enabled, ok := value.(bool)
		if err != nil || !ok {
			return preferences.Channels{}, http.StatusBadRequest
		}
		seen[key] = true
		if key == "email_enabled" {
			channels.EmailEnabled = enabled
		} else {
			channels.InAppEnabled = enabled
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || len(seen) != 2 {
		return preferences.Channels{}, http.StatusBadRequest
	}
	if _, err := decoder.Token(); err != io.EOF {
		return preferences.Channels{}, http.StatusBadRequest
	}
	return channels, 0
}
