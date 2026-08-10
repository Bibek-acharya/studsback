package studentdashboard

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		protected := v1.Group("")
		protected.Use(authMW)
		{
			protected.GET("/calendar/events", h.GetCalendarEvents)
			protected.GET("/calendar/events/:id", h.GetCalendarEventByID)
			protected.POST("/calendar/events", h.CreateCalendarEvent)
			protected.PUT("/calendar/events/:id", h.UpdateCalendarEvent)
			protected.DELETE("/calendar/events/:id", h.DeleteCalendarEvent)

			protected.GET("/invites", h.GetInvites)
			protected.GET("/invites/:id", h.GetInviteByID)
			protected.PUT("/invites/:id/accept", h.AcceptInvite)
			protected.PUT("/invites/:id/decline", h.DeclineInvite)
			protected.PUT("/invites/:id/save", h.SaveInvite)

			protected.GET("/bookmarks", h.GetBookmarks)
			protected.POST("/bookmarks", h.CreateBookmark)
			protected.DELETE("/bookmarks/:id", h.DeleteBookmark)
			protected.GET("/bookmarks/:type", h.GetBookmarksByType)

			protected.GET("/dashboard/stats", h.GetDashboardStats)
			protected.GET("/dashboard/recent-applications", h.GetRecentApplications)
			protected.GET("/my-applications", h.GetMyApplications)

			protected.GET("/notifications", h.GetNotifications)
			protected.PUT("/notifications/:id/read", h.MarkNotificationRead)
			protected.PUT("/notifications/read-all", h.MarkAllNotificationsRead)
		}
	}
}
