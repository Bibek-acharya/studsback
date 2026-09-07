// internal/notification/routes.go
package notification

import (
	"os"

	"github.com/gin-gonic/gin"
)

// V2Enabled reports whether this module owns the /api/v1/notifications*
// routes. Default on; NOTIFICATIONS_V2=off falls back to the legacy handlers
// (rollback path, readiness review C4). Shared by every gate so the flag has
// one source of truth.
func V2Enabled() bool { return os.Getenv("NOTIFICATIONS_V2") != "off" }

// RegisterRoutes mounts the notification API. Single owner of route
// registration (readiness review H1); callers gate on NOTIFICATIONS_V2.
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

	// Provider legacy proxy — disjoint path, pure legacy shape (doc 05 §1).
	pg := rg.Group("/scholarship-providers/notifications")
	pg.GET("", h.providerList)
	pg.PUT("/:id/read", h.providerMarkRead)
	pg.PUT("/read-all", h.providerMarkAllRead)

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
