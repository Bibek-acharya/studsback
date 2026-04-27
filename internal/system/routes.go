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
			system.GET("/carousels", h.GetCarousels)
			system.GET("/notifications", h.GetPublicNotifications)
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
		}
	}
}
