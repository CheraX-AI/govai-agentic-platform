package handlers

import (
	"net/http"

	"github.com/CheraX-AI/govai-agentic-platform/backend/services/notification-service/internal/features/inbox"
	"github.com/gin-gonic/gin"
)

type listResponse struct {
	Items      []inbox.Notification `json:"items"`
	NextCursor *string              `json:"next_cursor"`
}

func (api *citizenAPI) list(c *gin.Context) {
	owner, ok := requestOwner(c)
	if !ok {
		return
	}
	page, err := parsePage(c.Request.URL.RawQuery)
	if err != nil {
		failCitizen(c, http.StatusBadRequest, "invalid_pagination")
		return
	}
	ctx, cancel := requestContext(c)
	defer cancel()
	result, err := api.inbox.List(ctx, owner, page)
	if err != nil || ctx.Err() != nil || !validPage(result, page) {
		failCitizen(c, http.StatusServiceUnavailable, "notifications_unavailable")
		return
	}
	response := listResponse{Items: result.Items}
	if response.Items == nil {
		response.Items = []inbox.Notification{}
	}
	if result.HasMore {
		last := result.Items[len(result.Items)-1]
		cursor := encodeCursor(inbox.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
		response.NextCursor = &cursor
	}
	c.JSON(http.StatusOK, response)
}

func (api *citizenAPI) markRead(c *gin.Context) {
	owner, ok := requestOwner(c)
	if !ok {
		return
	}
	if c.Request.URL.RawQuery != "" || !notificationID.MatchString(c.Param("id")) ||
		!emptyBody(c) {
		failCitizen(c, http.StatusBadRequest, "invalid_notification_request")
		return
	}
	ctx, cancel := requestContext(c)
	defer cancel()
	found, err := api.inbox.MarkRead(ctx, owner, c.Param("id"))
	if err != nil || ctx.Err() != nil {
		failCitizen(c, http.StatusServiceUnavailable, "notifications_unavailable")
		return
	}
	if !found {
		failCitizen(c, http.StatusNotFound, "notification_not_found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *citizenAPI) unreadCount(c *gin.Context) {
	owner, ok := requestOwner(c)
	if !ok {
		return
	}
	if c.Request.URL.RawQuery != "" {
		failCitizen(c, http.StatusBadRequest, "invalid_notification_request")
		return
	}
	ctx, cancel := requestContext(c)
	defer cancel()
	count, err := api.inbox.UnreadCount(ctx, owner)
	if err != nil || ctx.Err() != nil || count < 0 {
		failCitizen(c, http.StatusServiceUnavailable, "notifications_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}
