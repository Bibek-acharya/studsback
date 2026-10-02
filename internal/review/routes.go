package review

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"studsphere/backend/internal/shared/middleware"
	"studsphere/backend/internal/shared/response"
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

		// Moderation of UNIVERSITY reviews. Platform admin only — see access.go.
		adminReviews := v1.Group("/admin/university-reviews")
		adminReviews.Use(authMW)
		adminReviews.Use(RequirePlatformAdmin())
		{
			adminReviews.GET("/:universityId", h.AdminGetUniversityReviews)
			adminReviews.DELETE("/:id", h.AdminDeleteReview)
		}

		// An institution's OWN reviews. The two READS stay on roleMW because a
		// tenant legitimately needs them and GetInstitutionReviews is keyed on the
		// caller's own id — see access.go.
		//
		// The DELETE IS REMOVED, and that is the fix. It called the same unscoped
		// Service.AdminDeleteReview as the admin group above, with an id from the
		// URL and no ownership comparison at any layer, so an institution could
		// delete any review on the platform — another tenant's, or one naming an
		// individual student — and the author got a moderation notification for it.
		// A tenant that wants a review taken down has ReviewReport, on the public
		// side, which is how the queue is meant to be fed.
		instReviews := v1.Group("/institution/reviews")
		instReviews.Use(authMW)
		instReviews.Use(roleMW)
		{
			instReviews.GET("", h.GetInstitutionReviews)
			instReviews.GET("/college/:collegeId", h.GetCollegeReviews)
		}

		// Platform admin only: DateReport is submitted through a PUBLIC endpoint
		// with no auth and carries Contact — a phone number — plus free text.
		adminDateReports := v1.Group("/admin/date-reports")
		adminDateReports.Use(authMW)
		adminDateReports.Use(RequirePlatformAdmin())
		{
			adminDateReports.GET("", h.GetAllDateReports)
			adminDateReports.PUT("/:id", h.UpdateDateReportStatus)
			adminDateReports.DELETE("/:id", h.DeleteDateReport)
		}
	}
}

// RequirePlatformAdmin is the operator gate for the /admin groups and, explicitly,
// NOT for the tenant reads on /institution/reviews.
//
// Refused before the handler and before the body is parsed, so it is a role
// question answered with no lookup and nothing to disclose. It reads the same
// PlatformAdminRoles() as the module's IsPlatformAdmin.
func RequirePlatformAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("user_role")
		roleStr, _ := role.(string)
		if !IsPlatformAdmin(roleStr) {
			response.Error(c, http.StatusForbidden, "This route is restricted to platform administrators")
			c.Abort()
			return
		}
		c.Next()
	}
}
