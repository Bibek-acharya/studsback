package system

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		system := v1.Group("/system")
		{
			system.POST("/contact", h.SubmitContactInquiry)
			system.GET("/ads", h.GetActiveAds)
			system.POST("/ads/:id/click", h.TrackAdClick)
			system.GET("/carousels", h.GetCarousels)
			system.GET("/notifications", h.GetPublicNotifications)
			system.GET("/landing-courses", h.GetPublicLandingCourses)
			system.GET("/course-ads", h.GetActiveCourseAdCards)
			system.POST("/course-ads/:id/click", h.TrackCourseAdCardClick)
			system.GET("/college-ads/trending", h.GetPublicCollegeAdTrending)
			system.POST("/college-ad-feedback", h.SubmitCollegeAdFeedback)
		}

		admin := v1.Group("/admin")
		admin.Use(authMW)
		admin.Use(roleMW)
		{
			admin.GET("/inquiries", h.GetContactInquiries)
			admin.GET("/inquiries/:id", h.GetContactInquiryByID)
			admin.PUT("/inquiries/:id/status", h.UpdateContactInquiryStatus)
			admin.DELETE("/inquiries/:id", h.DeleteContactInquiry)

			admin.GET("/ads", h.GetAds)
			admin.GET("/ads/:id", h.GetAdByID)
			admin.POST("/ads", h.CreateAd)
			admin.PUT("/ads/:id", h.UpdateAd)
			admin.DELETE("/ads/:id", h.DeleteAd)
			admin.POST("/ads/:id/click", h.TrackAdClick)

			admin.GET("/carousels", h.GetCarousels)
			admin.GET("/carousels/:id", h.GetCarouselSlideByID)
			admin.POST("/carousels", h.CreateCarouselSlide)
			admin.PUT("/carousels/:id", h.UpdateCarouselSlide)
			admin.DELETE("/carousels/:id", h.DeleteCarouselSlide)
			admin.PUT("/carousels/reorder", h.ReorderCarouselSlides)

			admin.GET("/landing-courses", h.GetAdminLandingCourses)
			admin.PUT("/landing-courses/fields/:id", h.UpdateLandingField)
			admin.PUT("/landing-courses/fields/reorder", h.ReorderLandingFields)
			admin.POST("/landing-courses", h.LinkLandingInstitution)
			admin.DELETE("/landing-courses/:id", h.UnlinkLandingInstitution)
			admin.PUT("/landing-courses/reorder", h.ReorderLandingInstitutions)
			admin.GET("/landing-courses/search", h.SearchLandingInstitutions)

			admin.GET("/course-ads", h.GetCourseAdCards)
			admin.GET("/course-ads/:id", h.GetCourseAdCardByID)
			admin.POST("/course-ads", h.CreateCourseAdCard)
			admin.PUT("/course-ads/:id", h.UpdateCourseAdCard)
			admin.DELETE("/course-ads/:id", h.DeleteCourseAdCard)

			admin.GET("/college-ads/trending", h.GetCollegeAdTrending)
			admin.POST("/college-ads/trending", h.CreateCollegeAdTrending)
			admin.PUT("/college-ads/trending/:id", h.UpdateCollegeAdTrending)
			admin.DELETE("/college-ads/trending/:id", h.DeleteCollegeAdTrending)
			admin.GET("/college-ad-feedback", h.GetCollegeAdFeedback)

			admin.GET("/advertise-requests", h.GetAdvertiseRequests)
			admin.PUT("/advertise-requests/:id/status", h.UpdateAdvertiseRequestStatus)
		}

		// Institution-zone advertise requests (authenticated institution users).
		instZone := v1.Group("/institution")
		instZone.Use(authMW)
		instZone.Use(roleMW)
		{
			instZone.POST("/ad-requests", h.SubmitAdvertiseRequest)
			instZone.GET("/ad-requests", h.GetInstitutionAdvertiseRequests)
		}
	}
}
