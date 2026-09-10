package analytics

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) getUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"totals": gin.H{}, "series": []SeriesPoint{}}})
}

func (h *Handler) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
