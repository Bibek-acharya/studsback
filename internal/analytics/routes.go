package analytics

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterRoutes(e *gin.Engine, authMW, superadminMW gin.HandlerFunc) {
	g := e.Group("/api/v1/superadmin/analytics")
	g.Use(authMW, superadminMW)
	g.GET("/users", h.getUsers)
	g.GET("/funnel", h.getFunnel)
	g.GET("/supply", h.getSupply)
	g.GET("/ops", h.getOps)
	g.GET("/health", h.getHealth)
	g.GET("/pages", h.getPages)
}

// RegisterPublicRoutes wires unauthenticated endpoints (no cookies exist).
func (h *Handler) RegisterPublicRoutes(e *gin.Engine) {
	g := e.Group("/api/v1/track")
	g.POST("/visit", h.trackVisit)
}
