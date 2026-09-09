// internal/notification/routes.go
package notification

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the notification API. Sole owner of the
// /api/v1/notifications* route family (readiness review H1); registration is
// unconditional — the NOTIFICATIONS_V2 rollback flag is retired (P2.5).
// superadminMW guards the broadcast endpoints (wiring-time, per Task 7);
// nil skips the guard and is for tests only.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, superadminMW gin.HandlerFunc) {
	g := rg.Group("/notifications")
	g.GET("", h.list)
	g.GET("/unread-count", h.unreadCount)
	g.PUT("/:id/read", h.markRead)
	g.PUT("/read-all", h.markAllRead)
	g.PUT("/:id/archive", h.archive(true))
	g.PUT("/:id/unarchive", h.archive(false))
	g.DELETE("/:id", h.remove)

	// Broadcast — superadmin-only via the middleware supplied at wiring time
	// (same pattern as auth/routes.go's superadmin group).
	bg := rg.Group("/notifications")
	if superadminMW != nil {
		bg.Use(superadminMW)
	}
	bg.POST("/broadcast", h.createBroadcast)
	bg.GET("/broadcasts", h.listBroadcasts)
	bg.POST("/broadcasts/:id/cancel", h.cancelBroadcast)

	// Public banner admin CRUD (Task 13) — same /system/notifications path
	// family as the system module's public GET (method-disjoint, so no Gin
	// route conflict). Superadmin-only via the middleware supplied at wiring
	// time; nil skips the guard and is for tests only.
	sg := rg.Group("/system/notifications")
	if superadminMW != nil {
		sg.Use(superadminMW)
	}
	sg.POST("", h.createPublicNotification)
	sg.GET("/all", h.listAllPublicNotifications)
	sg.PUT("/:id", h.updatePublicNotification)
	sg.DELETE("/:id", h.deletePublicNotification)

	// Preferences — per-account preference management (Task 4).
	rg.GET("/notifications/preferences", h.GetPreferences)
	rg.PUT("/notifications/preferences", h.UpdatePreferences)
}
