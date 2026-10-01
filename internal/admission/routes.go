package admission

import (
	"net/http"

	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the applicant routes and the operator queue.
//
// ## The two groups, and why the operator one is NOT the shared roleMW
//
// The admin group used to take `roleMW` from cmd/server/main.go, which is
// RequireRole("admin", "super_admin", "scholarship_provider", "scholarship-provider",
// "Scholarship Provider", "scholarship_provider_subuser", "institution") — a
// multi-tenant list shared by 17 modules, each wired to it for its own reasons. It
// admits `institution` and `scholarship_provider`, so an ordinary paying tenant
// could GET every applicant's name, email, phone, date of birth, address and
// uploaded documents across every college on the platform, and PUT a status onto
// any application.
//
// So the admin group now derives its own gate from PlatformAdminRoles(). The
// parameter is dropped rather than ignored: leaving a wide `roleMW` in the
// signature would let the next caller pass it back in and re-open the hole, which
// is the failure mode access.go's authorizePlatformAdmin is written to defeat.
//
// ## Why the two groups both have a GET /:id
//
// They are different resources with different rules, and sharing one handler
// hid that. The applicant GET answers "is this my application" and now goes
// through GetByIDForApplicant, which refuses anything else with
// ErrAdmissionNotFound — the refusal lives in the service, not in the handler,
// so every caller of the service is covered rather than the one route that
// happens to remember to check. The operator GET answers "show me this
// application" behind the admin gate. Same handler, different service method, and
// the difference is now in the signature rather than in a branch.
func RegisterRoutes(r *gin.Engine, authMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		protected := v1.Group("")
		protected.Use(authMW)
		{
			protected.POST("/admissions", h.Create)
			protected.GET("/admissions/my", h.GetMyAdmissions)
			protected.GET("/admissions/:id", h.GetMyAdmissionByID)
			protected.PUT("/admissions/:id", h.Update)
			protected.DELETE("/admissions/:id", h.Delete)
		}

		admin := v1.Group("/admin/admissions")
		admin.Use(authMW)
		admin.Use(RequirePlatformAdmin())
		{
			admin.GET("", h.GetAll)
			admin.GET("/:id", h.GetByID)
			admin.PUT("/:id/status", h.UpdateStatus)
			admin.GET("/college/:collegeId", h.GetByCollegeID)
		}
	}
}

// RequirePlatformAdmin is the operator gate for /admin/admissions.
//
// Refused before the handler and before the body is parsed, so it is a role
// question answered with no lookup and nothing to disclose. It reads the same
// PlatformAdminRoles() the service's own IsPlatformAdmin reads, so the middleware
// at the edge and any service-level check cannot drift apart.
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
