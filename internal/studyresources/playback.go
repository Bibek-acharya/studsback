package studyresources

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

// playbackTokenQueryParam carries the short-lived playback grant on the stream
// URL. It is deliberately NOT the session token parameter.
const playbackTokenQueryParam = "pt"

// IssuePlaybackToken handles GET /api/v1/study-resources/:id/playback-token.
//
// It sits behind the session Auth middleware and mints a short-lived token that
// is bound to this one published video resource. The response carries the
// token, its expiry and a ready-to-use stream URL — never the object key, and
// never a presigned/blob URL.
func (h *Handler) IssuePlaybackToken(c *gin.Context) {
	// A grant must never be cached by browsers or intermediaries.
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")

	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}

	resource, err := h.service.GetPlayableVideoResource(uint(id))
	if err != nil {
		if errors.Is(err, ErrNotAVideoResource) {
			response.Error(c, http.StatusBadRequest, "Only published video lectures can be played")
			return
		}
		response.Error(c, http.StatusNotFound, "Resource not found")
		return
	}

	ttl := utils.DefaultPlaybackTokenTTL
	token, expiresAt, err := utils.IssuePlaybackToken(userID, resource.ID, ttl)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to issue playback token")
		return
	}

	streamURL := fmt.Sprintf("/api/v1/study-resources/%d/stream?%s=%s",
		resource.ID, playbackTokenQueryParam, url.QueryEscape(token))

	response.Success(c, http.StatusOK, "Playback token issued", gin.H{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
		"stream_url": streamURL,
	})
}

// currentUserID reads the authenticated user set by the auth middleware. The
// value is a uint, but int/int64 are tolerated for custom middleware.
func currentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	switch v := value.(type) {
	case uint:
		return v, true
	case uint64:
		return uint(v), true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	default:
		return 0, false
	}
}
