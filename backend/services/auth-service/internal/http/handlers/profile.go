package handler

import (
	"context"
	"errors"
	"mime"
	"strings"
	"unicode"

	"github.com/CheraX-AI/govai-agentic-platform/backend/pkg/authn"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/profile"
	"github.com/CheraX-AI/govai-agentic-platform/backend/services/auth-service/internal/features/websession"
	"github.com/gin-gonic/gin"
)

type ProfileService interface {
	Read(context.Context, profile.Caller) (profile.Snapshot, error)
	Update(context.Context, profile.Caller, profile.UpdateRequest) (profile.Snapshot, error)
}

type ProfileHandler struct{ service ProfileService }

func NewProfileHandler(service ProfileService) (*ProfileHandler, error) {
	if service == nil {
		return nil, profile.ErrUnavailable
	}
	return &ProfileHandler{service: service}, nil
}

func (p *Protected) EnableProfile(service ProfileService) error {
	h, err := NewProfileHandler(service)
	if err != nil {
		return err
	}
	p.profile = h
	return nil
}

type profileSession interface {
	ProfileIdentity(context.Context, string, string, bool) (profile.Caller, error)
}

func (h *WebSessionHandler) EnableProfile(service ProfileService) error {
	if _, ok := h.service.(profileSession); !ok {
		return profile.ErrUnavailable
	}
	p, err := NewProfileHandler(service)
	if err != nil {
		return err
	}
	h.profile = p
	return nil
}

func profileHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
}

func (h *ProfileHandler) Native(c *gin.Context) {
	profileHeaders(c)
	// Never turn a cookie into native identity or accept ambiguous credentials.
	for _, cookie := range c.Request.Cookies() {
		if cookie.Name == WebSessionCookie {
			errorResponse(c, 403, "ambiguous_authentication", "Use one authentication method.")
			return
		}
	}
	if len(c.Request.Header.Values("X-GovAI-Web")) > 0 {
		errorResponse(c, 403, "ambiguous_authentication", "Use the browser profile endpoint.")
		return
	}
	identity, ok := authn.Current(c)
	if !ok {
		errorResponse(c, 401, "unauthorized", "Authentication required.")
		return
	}
	h.serve(c, c.Request.Context(), profile.Caller{Issuer: identity.Issuer, Subject: identity.Subject, Email: identity.Email})
}

func (h *WebSessionHandler) Profile(c *gin.Context) {
	profileHeaders(c)
	if h.profile == nil {
		c.Status(404)
		return
	}
	if len(c.Request.Header.Values("Authorization")) > 0 || len(c.Request.Header.Values("X-GovAI-Web")) != 1 {
		webFail(c, 403, "web_request_rejected")
		return
	}
	write := c.Request.Method == "PUT"
	ctx, done, ok := h.begin(c, write)
	if !ok {
		return
	}
	defer done()
	raw, ok := webCookie(c)
	if !ok {
		webFail(c, 401, "session_rejected")
		return
	}
	csrf := ""
	if write {
		values := c.Request.Header.Values("X-CSRF-Token")
		if len(values) != 1 || !validWebSecret(values[0]) {
			webFail(c, 403, "csrf_required")
			return
		}
		csrf = values[0]
	}
	identity, err := h.service.(profileSession).ProfileIdentity(ctx, raw, csrf, write)
	if err != nil {
		if errors.Is(err, websession.ErrDenied) {
			webFail(c, 401, "session_rejected")
		} else {
			webFail(c, 503, "session_unavailable")
		}
		return
	}
	h.profile.serve(c, ctx, identity)
}

func (h *ProfileHandler) serve(c *gin.Context, ctx context.Context, identity profile.Caller) {
	if c.Request.URL.RawQuery != "" {
		errorResponse(c, 400, "invalid_request", "Profile requests do not accept query parameters.")
		return
	}
	var result profile.Snapshot
	var err error
	if c.Request.Method == "GET" {
		result, err = h.service.Read(ctx, identity)
	} else {
		media, params, parseErr := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if parseErr != nil || media != "application/json" || len(c.Request.Header.Values("Content-Type")) != 1 || c.GetHeader("Content-Encoding") != "" {
			errorResponse(c, 415, "unsupported_media_type", "Use UTF-8 JSON.")
			return
		}
		for key, value := range params {
			if key != "charset" || !strings.EqualFold(value, "utf-8") {
				errorResponse(c, 415, "unsupported_media_type", "Use UTF-8 JSON.")
				return
			}
		}
		request, invalid := profile.ParseUpdate(c.Request.Body)
		if invalid != nil {
			status := 400
			if invalid.Code == "request_too_large" {
				status = 413
			}
			if invalid.Code == "validation_failed" {
				status = 422
			}
			errorResponse(c, status, invalid.Code, "Check the permitted profile fields and revision.")
			return
		}
		result, err = h.service.Update(ctx, identity, request)
	}
	if err != nil {
		profileError(c, err)
		return
	}
	if result.Details.Revision < 0 || result.Details.Revision > profile.MaxClientRevision+1 {
		profileError(c, profile.ErrUnavailable)
		return
	}
	var email any
	if result.Email != "" && len(result.Email) <= 254 && strings.IndexFunc(result.Email, unicode.IsControl) < 0 {
		email = result.Email
	}
	f := result.Details.Fields
	// Map every public field explicitly: never serialize internal redacted DTOs
	// or expose issuer, subject, tokens, credentials or SeaweedFS object keys.
	c.JSON(200, gin.H{
		"full_name": result.Registered.FullName, "nic": result.Registered.NIC.RestrictedOriginal(), "mobile_number": result.Registered.MobileNumber,
		"email": email, "name_si": f.NameSI, "name_ta": f.NameTA, "date_of_birth": f.DateOfBirth, "gender": f.Gender,
		"address":  gin.H{"line1": f.Address.Line1, "line2": f.Address.Line2, "city": f.Address.City},
		"revision": result.Details.Revision, "has_photo": result.Details.PhotoObjectKey != "", "photo_url": nil,
	})
}

func profileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, profile.ErrNotFound):
		errorResponse(c, 404, "profile_not_found", "Profile not found.")
	case errors.Is(err, profile.ErrConflict):
		errorResponse(c, 409, "revision_conflict", "Your profile changed. Reload it before saving again.")
	case errors.Is(err, profile.ErrDOBMismatch):
		errorResponse(c, 422, "dob_nic_mismatch", "Date of birth must match your registered NIC.")
	case errors.Is(err, profile.ErrGenderChoice):
		errorResponse(c, 422, "invalid_gender", "Choose Male or Female.")
	case errors.Is(err, profile.ErrInvalid):
		errorResponse(c, 422, "validation_failed", "Check your profile details.")
	case errors.Is(err, profile.ErrRulesUnavailable):
		errorResponse(c, 503, "profile_validation_unavailable", "Validation for this field is not available yet. Your changes were not saved.")
	default:
		errorResponse(c, 503, "profile_unavailable", "Profile is temporarily unavailable.")
	}
}
