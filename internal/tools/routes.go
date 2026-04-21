package tools

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		tools := v1.Group("/tools")
		{
			tools.POST("/scholarship-finder/recommendations", h.GetScholarshipFinderRecommendations)
			tools.POST("/college-recommender/recommendations", h.GetCollegeRecommenderRecommendations)

			// Logo management endpoints
			admin := tools.Group("")
			admin.Use(roleMW)
			{
				admin.POST("/logo/upload", h.UploadLogo)
				admin.GET("/logo", h.GetLogoURL)
			}

			// Public logo serving endpoint (no auth required)
			tools.GET("/logo/serve", h.ServeLogo)
		}
	}
}
