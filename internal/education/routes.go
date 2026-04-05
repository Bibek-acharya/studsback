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
			education.GET("/courses/:id", h.GetEducationCourseByID)
			education.GET("/courses/:id/details", h.GetEducationCourseDetailsByID)
			education.GET("/news", h.GetEducationNews)
			education.GET("/news/:id", h.GetEducationNewsByID)
			education.GET("/events", h.GetEducationEvents)
			education.GET("/events/:id", h.GetEducationEventByID)
			education.GET("/blogs", h.GetEducationBlogs)
			education.GET("/blogs/:id", h.GetEducationBlogByID)
		}
	}
}
