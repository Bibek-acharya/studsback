// internal/notification/routes.go
package notification

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the notification API. Single owner of route
// registration (readiness review H1); callers gate on NOTIFICATIONS_V2.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/notifications")
	g.GET("", h.list)
	g.GET("/unread-count", h.unreadCount)
	g.PUT("/:id/read", h.markRead)
	g.PUT("/read-all", h.markAllRead)
	g.PUT("/:id/archive", h.archive(true))
	g.PUT("/:id/unarchive", h.archive(false))
	g.DELETE("/:id", h.remove)
}
