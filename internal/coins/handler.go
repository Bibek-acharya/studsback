package coins

import (
	"errors"
	"net/http"

	"studsphere/backend/internal/shared/httpx"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetEconomyConfig handles GET /api/v1/admin/coins/economy.
func (h *Handler) GetEconomyConfig(c *gin.Context) {
	config, err := h.service.GetEconomyConfig()
	if err != nil {
		response.Error(c, statusForError(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Coin economy config fetched", config)
}

// UpdateEconomyConfig handles PUT /api/v1/admin/coins/economy. The body is a
// partial update: an omitted key keeps its current value.
func (h *Handler) UpdateEconomyConfig(c *gin.Context) {
	var req UpdateEconomyConfigRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// The actor is the audit trail for a pricing change, so it is resolved the
	// strict way (httpx.CurrentUserID) and a missing id fails the request
	// rather than recording changed_by_user_id = 0, which is
	// indistinguishable from a real user. This route sits behind Auth
	// (routes.go), so the id is set in the normal path; the check is a
	// backstop against a route wired without the middleware.
	actorID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	config, err := h.service.UpdateEconomyConfig(req, actorID)
	if err != nil {
		response.Error(c, statusForError(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Coin economy config updated", config)
}

// statusForError maps domain errors onto HTTP status codes, per
// internal/mocktests/handler.go:247-258. A rejected config is a 400 carrying
// the field-level reason; unreadable storage is a 500 because nothing the
// caller sent is wrong.
func statusForError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrInvalidConfig):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
