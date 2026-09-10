package analytics

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func parseRange(c *gin.Context) (from, to time.Time, gran string, ok bool) {
	to = time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	from = to.Add(-30 * 24 * time.Hour)
	gran = c.DefaultQuery("granularity", "day")
	if gran != "day" && gran != "week" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "granularity must be day or week"})
		return time.Time{}, time.Time{}, "", false
	}
	const layout = "2006-01-02"
	if v := c.Query("from"); v != "" {
		t, err := time.Parse(layout, v)
		if err != nil || t.After(to) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from must be YYYY-MM-DD"})
			return time.Time{}, time.Time{}, "", false
		}
		from = t.UTC()
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse(layout, v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be YYYY-MM-DD"})
			return time.Time{}, time.Time{}, "", false
		}
		to = t.UTC().Add(24 * time.Hour)
	}
	if !from.Before(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from must precede to"})
		return time.Time{}, time.Time{}, "", false
	}
	return from, to, gran, true
}

func (h *Handler) getUsers(c *gin.Context) {
	from, to, gran, ok := parseRange(c)
	if !ok {
		return
	}
	out, err := h.svc.Users(from, to, gran)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) getFunnel(c *gin.Context) {
	from, to, gran, ok := parseRange(c)
	if !ok {
		return
	}
	out, err := h.svc.Funnel(from, to, gran)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) getSupply(c *gin.Context) {
	from, to, gran, ok := parseRange(c)
	if !ok {
		return
	}
	out, err := h.svc.Supply(from, to, gran)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
