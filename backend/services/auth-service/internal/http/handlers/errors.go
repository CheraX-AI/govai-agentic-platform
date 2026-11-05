package handler

import "github.com/gin-gonic/gin"

// abortJSON preserves existing endpoint payloads, including code-only recovery
// errors and registration receipts. Callers own headers, timing and status mapping.
func abortJSON(c *gin.Context, status int, body any) {
	c.AbortWithStatusJSON(status, body)
}

// errorResponse accepts only deliberate public messages, never raw provider errors.
// Empty fields are omitted to preserve the existing endpoint contracts.
func errorResponse(c *gin.Context, status int, code, message string) {
	body := gin.H{}
	if code != "" {
		body["code"] = code
	}

	if message != "" {
		body["error"] = message
	}

	abortJSON(c, status, body)
}
