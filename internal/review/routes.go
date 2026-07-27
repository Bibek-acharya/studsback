package review

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		user := v1.Group("/user/reviews")
		user.Use(authMW)
		{
			user.POST("", h.SubmitReview)
			user.GET("", h.GetUserReviews)
			user.PUT("/:id", h.UpdateReview)
			user.DELETE("/:id", h.DeleteReview)
			user.POST("/:id/report", h.ReportReview)
		}

		education := v1.Group("/education/reviews")
		education.Use(authMW)
		{
			education.GET("/college/:collegeId", h.GetCollegeReviews)
			education.GET("/university/:universityId", h.GetUniversityReviews)
			education.POST("/:id/helpful", h.MarkHelpful)
		}

		university := v1.Group("/user/university-reviews")
		university.Use(authMW)
		{
			university.POST("", h.SubmitUniversityReview)
			university.GET("/:universityId", h.GetMyUniversityReview)
		}
	}
}
