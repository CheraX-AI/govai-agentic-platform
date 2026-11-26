package handlers

import (
	"net/http"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/preferences"
	"github.com/gin-gonic/gin"
)

type preferenceResponse struct {
	preferences.Channels
	Locale                string `json:"locale"`
	SecurityInAppRequired bool   `json:"security_in_app_required"`
}

func (api *citizenAPI) getPreferences(c *gin.Context) {
	owner, ok := requestOwner(c)
	if !ok {
		return
	}
	if c.Request.URL.RawQuery != "" {
		failCitizen(c, http.StatusBadRequest, "invalid_preferences")
		return
	}
	ctx, cancel := requestContext(c)
	defer cancel()
	channels, err := api.preferences.Get(ctx, owner)
	if err != nil || ctx.Err() != nil {
		failCitizen(c, http.StatusServiceUnavailable, "preferences_unavailable")
		return
	}
	c.JSON(http.StatusOK, preferenceResponse{
		Channels: channels, Locale: "en", SecurityInAppRequired: true,
	})
}

func (api *citizenAPI) replacePreferences(c *gin.Context) {
	owner, ok := requestOwner(c)
	if !ok {
		return
	}
	if c.Request.URL.RawQuery != "" {
		failCitizen(c, http.StatusBadRequest, "invalid_preferences")
		return
	}
	channels, status := parsePreferences(c)
	if status != 0 {
		failCitizen(c, status, "invalid_preferences")
		return
	}
	ctx, cancel := requestContext(c)
	defer cancel()
	if api.preferences.Replace(ctx, owner, channels) != nil || ctx.Err() != nil {
		failCitizen(c, http.StatusServiceUnavailable, "preferences_unavailable")
		return
	}
	c.JSON(http.StatusOK, preferenceResponse{
		Channels: channels, Locale: "en", SecurityInAppRequired: true,
	})
}
