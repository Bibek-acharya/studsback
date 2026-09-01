package review

import (
	"github.com/gin-gonic/gin"

	"studsphere/backend/internal/shared/middleware"
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
			education.GET("/university/:universityId", h.GetUniversityReviews)
			education.POST("/:id/helpful", h.MarkHelpful)
		}

		// College reviews are public; auth is optional so the response can
		// include the current user's own vote when logged in.
		publicEducation := v1.Group("/education/reviews")
		publicEducation.Use(middleware.OptionalAuth())
		{
			publicEducation.GET("/college/:collegeId", h.GetCollegeReviews)
		}

		university := v1.Group("/user/university-reviews")
		university.Use(authMW)
		{
			university.POST("", h.SubmitUniversityReview)
			university.GET("/:universityId", h.GetMyUniversityReview)
			university.PUT("/:universityId", h.UpdateUniversityReview)
		}

		// Public date report endpoint
		v1.POST("/reports", h.CreateDateReport)

		// Admin review management routes
		adminReviews := v1.Group("/admin/university-reviews")
		adminReviews.Use(authMW)
		adminReviews.Use(roleMW)
		{
			adminReviews.GET("/:universityId", h.AdminGetUniversityReviews)
			adminReviews.DELETE("/:id", h.AdminDeleteReview)
		}

		// Institution review management routes
		instReviews := v1.Group("/institution/reviews")
		instReviews.Use(authMW)
		instReviews.Use(roleMW)
		{
			instReviews.GET("", h.GetInstitutionReviews)
			instReviews.GET("/college/:collegeId", h.GetCollegeReviews)
			instReviews.DELETE("/:id", h.AdminDeleteReview)
		}

		// Admin date report management routes
		adminDateReports := v1.Group("/admin/date-reports")
		adminDateReports.Use(authMW)
		adminDateReports.Use(roleMW)
		{
			adminDateReports.GET("", h.GetAllDateReports)
			adminDateReports.PUT("/:id", h.UpdateDateReportStatus)
			adminDateReports.DELETE("/:id", h.DeleteDateReport)
		}
	}
}
