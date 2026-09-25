package mocktests

import "github.com/gin-gonic/gin"

// RegisterRoutes wires the public browsing endpoints, the authenticated
// submit/attempt endpoints, and the superadmin-guarded CRUD endpoints.
//
// Route note: the attempt result lives under /mock-tests/:id/attempts/:attempt_id
// rather than /mock-tests/attempts/:id because gin cannot register a static
// path segment next to the :id wildcard.
func RegisterRoutes(r *gin.Engine, authMW, superadminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		// Public browsing.
		v1.GET("/mock-tests", h.ListTests)
		v1.GET("/mock-tests/:id", h.GetTest)

		// Authenticated: grading and owner-scoped attempt results.
		authenticated := v1.Group("/mock-tests")
		authenticated.Use(authMW)
		{
			authenticated.POST("/:id/submit", h.SubmitTest)
			authenticated.GET("/:id/attempts/:attempt_id", h.GetAttempt)
		}

		// Admin CRUD (superadmin/super_admin only).
		admin := v1.Group("/admin/mock-tests")
		admin.Use(authMW)
		admin.Use(superadminRoleMW)
		{
			admin.GET("", h.AdminListTests)
			admin.GET("/:id", h.AdminGetTest)
			admin.POST("", h.CreateTest)
			admin.PUT("/:id", h.UpdateTest)
			admin.DELETE("/:id", h.DeleteTest)
		}
	}
}
