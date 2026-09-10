package analytics

import "github.com/gin-gonic/gin"

func (h *Handler) RegisterRoutes(e *gin.Engine, authMW, superadminMW gin.HandlerFunc) {
	g := e.Group("/api/v1/superadmin/analytics")
	g.Use(authMW, superadminMW)
	g.GET("/users", h.getUsers)
	g.GET("/funnel", h.notImplemented)
	g.GET("/supply", h.notImplemented)
	g.GET("/ops", h.notImplemented)
	g.GET("/health", h.notImplemented)
}
