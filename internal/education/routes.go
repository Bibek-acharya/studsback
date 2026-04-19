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

		// Superadmin blog management routes
		adminBlogs := v1.Group("/superadmin/blogs")
		adminBlogs.Use(authMW)
		{
			adminBlogs.GET("", h.AdminGetBlogs)
			adminBlogs.POST("", h.CreateBlog)
			adminBlogs.PUT("/:id", h.UpdateBlog)
			adminBlogs.DELETE("/:id", h.DeleteBlog)
		}
	}
}
