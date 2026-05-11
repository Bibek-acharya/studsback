package education

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		education := v1.Group("/education")
		{
			education.GET("/rankings", h.GetEducationRankings)
			education.GET("/exams", h.GetEducationExams)
			education.GET("/exams/:id", h.GetEducationExamByID)
			education.GET("/courses", h.GetEducationCourses)
			education.GET("/courses/filter-counts", h.GetCourseFilterCounts)
			education.GET("/courses/:id", h.GetEducationCourseByID)
			education.GET("/courses/:id/details", h.GetEducationCourseDetailsByID)
			education.GET("/news", h.GetEducationNews)
			education.GET("/news/filter-counts", h.GetNewsFilterCounts)
			education.GET("/news/:id", h.GetEducationNewsByID)
			education.GET("/events", h.GetEducationEvents)
			education.GET("/events/filter-counts", h.GetEventFilterCounts)
			education.GET("/events/:id", h.GetEducationEventByID)
			education.GET("/blogs", h.GetEducationBlogs)
			education.GET("/blogs/filter-counts", h.GetBlogFilterCounts)
			education.GET("/blogs/:id", h.GetEducationBlogByID)
			education.POST("/blogs/:id/view", h.IncrementBlogView)
		}

		// Public entrance endpoints
		entrances := v1.Group("/entrances")
		{
			entrances.POST("", h.GetPublicEntrances)
			entrances.GET("/filter-counts", h.GetEntranceFilterCounts)
			entrances.GET("/:id", h.GetPublicEntranceByID)
		}

		// Admin blog management routes
		adminBlogs := v1.Group("/admin/blogs")
		adminBlogs.Use(authMW)
		adminBlogs.Use(roleMW)
		{
			adminBlogs.GET("", h.AdminGetBlogs)
			adminBlogs.POST("", h.CreateBlog)
			adminBlogs.PUT("/:id", h.UpdateBlog)
			adminBlogs.DELETE("/:id", h.DeleteBlog)
			adminBlogs.POST("/upload-image", h.UploadBlogImage)
		}

		// Admin event management routes
		adminEvents := v1.Group("/admin/events")
		adminEvents.Use(authMW)
		adminEvents.Use(roleMW)
		{
			adminEvents.GET("", h.AdminGetEvents)
			adminEvents.GET("/:id", h.AdminGetEventByID)
			adminEvents.POST("", h.CreateEvent)
			adminEvents.PUT("/:id", h.UpdateEvent)
			adminEvents.DELETE("/:id", h.DeleteEvent)
			adminEvents.PUT("/:id/feature", h.ToggleEventFeatured)
		}

		// Blog comments (public)
		v1.GET("/blogs/:id/comments", h.GetBlogComments)
		v1.POST("/blogs/:id/comments", h.CreateBlogComment)
	}
}
