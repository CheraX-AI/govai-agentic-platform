package handler

import (
	"context"
	"errors"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/gin-gonic/gin"
	"mime"
	"strconv"
	"strings"
)

func (p *Protected) EnablePhoto(service *profile.PhotoService) error {
	if p.profile == nil || service == nil {
		return profile.ErrUnavailable
	}
	p.profile.photo = service
	return nil
}
func (h *WebSessionHandler) EnablePhoto(service *profile.PhotoService) error {
	if h.profile == nil || service == nil {
		return profile.ErrUnavailable
	}
	h.profile.photo = service
	return nil
}

func (h *ProfileHandler) servePhoto(c *gin.Context, ctx context.Context, caller profile.Caller) {
	if c.Request.Method == "GET" {
		data, media, revision, err := h.photo.Read(ctx, caller)
		if err != nil {
			profileError(c, err)
			return
		}
		c.Header("ETag", `"`+strconv.FormatInt(revision, 10)+`"`)
		c.Header("Content-Disposition", "inline; filename=profile-photo")
		c.Data(200, media, data)
		return
	}
	values := c.Request.Header.Values("If-Match")
	if len(values) == 0 {
		errorResponse(c, 428, "revision_required", "Load your profile before replacing the photo.")
		return
	}
	if len(values) != 1 || len(values[0]) < 3 || !strings.HasPrefix(values[0], `"`) || !strings.HasSuffix(values[0], `"`) {
		errorResponse(c, 400, "invalid_revision", "Use the quoted profile revision in If-Match.")
		return
	}
	raw := strings.Trim(values[0], `"`)
	revision, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || revision < 0 || revision > profile.MaxClientRevision || values[0] != `"`+strconv.FormatInt(revision, 10)+`"` {
		errorResponse(c, 400, "invalid_revision", "Use the quoted profile revision in If-Match.")
		return
	}
	media, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || len(params) != 0 || len(c.Request.Header.Values("Content-Type")) != 1 || c.GetHeader("Content-Encoding") != "" {
		errorResponse(c, 415, "unsupported_photo_type", "Upload a JPEG or PNG image.")
		return
	}
	saved, err := h.photo.Replace(ctx, caller, revision, c.Request.Body, media)
	if err != nil {
		switch {
		case errors.Is(err, profile.ErrPhotoSize):
			errorResponse(c, 413, "photo_too_large", "Choose a photo no larger than 2 MiB.")
		case errors.Is(err, profile.ErrPhotoType):
			errorResponse(c, 415, "unsupported_photo_type", "Choose a valid JPEG or PNG photo.")
		default:
			profileError(c, err)
		}
		return
	}
	c.Header("ETag", `"`+strconv.FormatInt(saved.Revision, 10)+`"`)
	c.JSON(200, gin.H{"revision": saved.Revision, "has_photo": true, "photo_url": c.Request.URL.Path})
}
