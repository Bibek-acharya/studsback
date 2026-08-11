package institution

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		// Public institution routes (no auth)
		v1.GET("/institutions/public/filter-counts", h.GetPublicInstitutionFilterCounts)
		v1.GET("/institutions/public", h.ListPublicInstitutions)
		v1.GET("/institutions/public/sponsored/:universityId", h.GetSponsoredInstitutions)
		v1.GET("/institutions/public/by-university/:universityId", h.GetInstitutionsByUniversity)
		v1.GET("/institutions/public/by-universities", h.GetInstitutionsByUniversities)
		v1.GET("/institutions/public/:id", h.GetPublicInstitution)
		v1.GET("/institutions/public/blogs", h.ListPublicBlogs)
		v1.GET("/institutions/public/news/by-slug/:slug", h.GetPublicNewsBySlug)
		v1.GET("/institutions/public/events/by-slug/:slug", h.GetPublicEventBySlug)
		v1.GET("/institutions/public/blogs/by-slug/:slug", h.GetPublicBlogBySlug)
		v1.GET("/institutions/public/:id/counselling-sessions", h.GetPublicCounsellingSessions)
		v1.GET("/institutions/public/news", h.ListPublicNews)
		v1.GET("/institutions/public/news/:id", h.GetPublicNewsByID)
		v1.GET("/institutions/public/events", h.ListPublicEvents)
		v1.GET("/institutions/public/events/:id", h.GetPublicEventByID)
		v1.GET("/institutions/public/scholarships", h.ListPublicScholarships)
		v1.GET("/institutions/public/scholarships/:id", h.GetPublicScholarshipByID)
		v1.GET("/admissions/published", h.GetPublishedAdmissionPages)
		v1.GET("/admissions/published/institutions", h.GetPublishedAdmissionInstitutions)
		v1.GET("/admissions/published/institutions/:id", h.GetPublishedAdmissionInstitutionByID)
		v1.GET("/admissions/published/pages/:id", h.GetPublishedAdmissionByPageID)

		adminInst := v1.Group("/admin/institutions")
		adminInst.Use(authMW, roleMW)
		{
			adminInst.PUT("/:id/sponsored", h.ToggleInstitutionSponsored)
		}

		// Public counselling booking (auth only, no role check)
		publicBooking := v1.Group("")
		publicBooking.Use(authMW)
		{
			publicBooking.POST("/counselling/sessions/book", h.CreatePublicCounsellingBooking)
		}

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
			institution.POST("/counselling/sessions", h.CreateCounsellingSession)
			institution.PUT("/counselling/sessions/:id", h.UpdateCounsellingSession)
			institution.DELETE("/counselling/sessions/:id", h.DeleteCounsellingSession)
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
			institution.POST("/admission-pages/:id/generate-whats-new", h.GenerateWhatsNew)

			institution.GET("/scholarship-applications", h.GetScholarshipApplications)
			institution.PUT("/scholarship-applications/:id/status", h.UpdateScholarshipApplicationStatus)

			institution.GET("/students/:id", h.GetStudentProfile)

			institution.GET("/inquiries", h.GetInstitutionInquiries)
			institution.PUT("/inquiries/:id/status", h.UpdateInquiryStatus)
			institution.DELETE("/inquiries/:id", h.DeleteInquiry)

			institution.GET("/followers", h.GetInstitutionFollowers)
		}
	}
}
