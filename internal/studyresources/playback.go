package studyresources

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"studsphere/backend/internal/shared/httpx"
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

	userID, ok := httpx.CurrentUserID(c)
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

	// ── the coin gate ──────────────────────────────────────────────────────
	//
	// It runs HERE — after the playability check and BEFORE IssuePlaybackToken —
	// and the ordering is the whole contract.
	//
	// AFTER the playability check, for the same reason the download gate runs after
	// its publication check: a draft is a 404 whether or not anyone can pay for it.
	// Gating first would answer 402 to a request for a video the public is not
	// allowed to know exists, which both leaks the draft and leaves a student unable
	// to tell "this does not exist" from "this costs 90 coins". A document asked
	// for here is still a 400, not a price.
	//
	// BEFORE the token, and this is the load-bearing half. A token is a capability:
	// it is what StreamResource accepts in place of a session, and it is minted
	// with no storage access of its own. So a token issued and then ignored is not
	// a gate at all — it is a bypass with extra steps, and a student who is refused
	// here would still walk away holding a grant that opens the stream route for
	// the next five minutes. Refusing BEFORE the mint is the only arrangement in
	// which "gated" means "no bytes, ever".
	//
	// A nil gate means no gate: the check is skipped entirely rather than
	// defaulting to "refuse", so a deployment that has not wired the economy serves
	// playback exactly as it always has. That is the same reason the config ships
	// with every gate off — the kill switch is off, and an absent switch is the off
	// position.
	if h.playbackGate != nil {
		userID, _ := httpx.CurrentUserID(c)
		decision := h.playbackGate.AuthorizePlayback(c.Request.Context(), userID,
			uint64(resource.ID), resource.Title)
		if !decision.Allowed {
			writeGateRefusal(c, decision.Refusal)
			return
		}
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
