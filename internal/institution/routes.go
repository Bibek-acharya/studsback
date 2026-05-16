package institution

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		// Public institution routes (no auth)
		v1.GET("/institutions/public", h.ListPublicInstitutions)
		v1.GET("/institutions/public/:id", h.GetPublicInstitution)
		v1.GET("/admissions/published", h.GetPublishedAdmissionPages)

		institution := v1.Group("/institution")
		institution.Use(authMW)
		institution.Use(roleMW)
		{
			institution.GET("/dashboard", h.GetDashboard)
			institution.GET("/analytics", h.GetAnalytics)

			institution.GET("/programs", h.GetPrograms)
			institution.GET("/programs/:id", h.GetProgramByID)
			institution.POST("/programs", h.CreateProgram)
			institution.PUT("/programs/:id", h.UpdateProgram)
			institution.DELETE("/programs/:id", h.DeleteProgram)

			institution.GET("/profile", h.GetProfile)
			institution.PUT("/profile", h.UpdateProfile)
			institution.GET("/media", h.GetMedia)
			institution.POST("/media", h.CreateMedia)
			institution.DELETE("/media/:id", h.DeleteMedia)

			institution.GET("/counselling/sessions", h.GetCounsellingSessions)
			institution.GET("/counselling/bookings", h.GetCounsellingBookings)
			institution.PUT("/counselling/bookings/:id/status", h.UpdateBookingStatus)

			institution.GET("/entrances", h.GetEntrances)
			institution.GET("/entrances/:id", h.GetEntranceByID)
			institution.POST("/entrances", h.CreateEntrance)
			institution.PUT("/entrances/:id", h.UpdateEntrance)
			institution.DELETE("/entrances/:id", h.DeleteEntrance)
			institution.GET("/entrances/:id/applicants", h.GetEntranceApplicants)

			institution.GET("/events", h.GetEvents)
			institution.GET("/events/:id", h.GetEventByID)
			institution.POST("/events", h.CreateEvent)
			institution.PUT("/events/:id", h.UpdateEvent)
			institution.DELETE("/events/:id", h.DeleteEvent)

			institution.GET("/news", h.GetNews)
			institution.GET("/news/:id", h.GetNewsByID)
			institution.POST("/news", h.CreateNews)
			institution.PUT("/news/:id", h.UpdateNews)
			institution.DELETE("/news/:id", h.DeleteNews)

			institution.GET("/blogs", h.GetBlogs)
			institution.GET("/blogs/:id", h.GetBlogByID)
			institution.POST("/blogs", h.CreateBlog)
			institution.PUT("/blogs/:id", h.UpdateBlog)
			institution.DELETE("/blogs/:id", h.DeleteBlog)

			institution.GET("/qms", h.GetQMS)
			institution.GET("/qms/:id", h.GetQMSByID)
			institution.POST("/qms", h.CreateQMS)
			institution.PUT("/qms/:id", h.UpdateQMS)
			institution.DELETE("/qms/:id", h.DeleteQMS)

			institution.GET("/messages", h.GetMessages)
			institution.GET("/messages/:id", h.GetMessageByID)
			institution.POST("/messages", h.CreateMessage)
			institution.GET("/messages/students", h.GetMessageStudents)

			institution.POST("/upload", h.UploadFile)
			institution.GET("/settings", h.GetSettings)
			institution.PUT("/settings", h.UpdateSettings)
			institution.PUT("/settings/password", h.UpdatePassword)

			institution.GET("/scholarships", h.GetScholarships)
			institution.POST("/scholarships", h.CreateScholarship)
			institution.PUT("/scholarships/:id", h.UpdateScholarship)
			institution.DELETE("/scholarships/:id", h.DeleteScholarship)

			institution.GET("/admissions", h.GetAdmissions)
			institution.PUT("/admissions/:id/status", h.UpdateAdmissionStatus)

			institution.POST("/admission-pages", h.CreateAdmissionPage)
			institution.GET("/admission-pages", h.GetAdmissionPages)
			institution.GET("/admission-pages/:id", h.GetAdmissionPage)
			institution.PUT("/admission-pages/:id", h.UpdateAdmissionPage)
			institution.DELETE("/admission-pages/:id", h.DeleteAdmissionPage)

			institution.GET("/scholarship-applications", h.GetScholarshipApplications)
			institution.PUT("/scholarship-applications/:id/status", h.UpdateScholarshipApplicationStatus)
		}
	}
}
