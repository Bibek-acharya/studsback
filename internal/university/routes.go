package university

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the public university directory and the admin surface.
//
// adminRoleMW is passed in rather than built here, and it is deliberately NOT the
// shared roleMW from cmd/server/main.go. That gate is
// RequireRole("admin", "super_admin", "scholarship_provider", …, "institution"):
// a multi-tenant list shared by 17 modules, behind which an ordinary paying
// customer could add, rewrite and delete rows in the platform's university
// directory, and read entries through the privileged FindByIDFull path that the
// public route does not serve. access.go has the full argument.
//
// It is applied to four routes and NOT to the list, and that asymmetry is the
// design rather than an oversight:
//
//   - GET /admin/universities stays on roleMW because it is the same handler the
//     public GET /api/v1/universities mounts, which has no auth at all. It returns
//     the published directory, so gating it would close nothing. Worse, it would
//     break a live caller: the institution dashboard's ProfilePage.tsx fetches
//     /api/v1/admin/universities?limit=500 with the institution token to fill its
//     university picker. Narrowing that list is precisely the lockout this fix is
//     supposed to avoid, and the anti-lockout test in university_access_test.go
//     pins it.
//
//   - The other four are platform-admin only: the single read (privileged
//     FindByIDFull) and the three writes.
//
// The service is the authority for all four regardless: each handler passes a
// Viewer in and the service re-checks the role, so passing the wide roleMW back in
// at the call site cannot re-open them. TestUniversityGateCannotBeWidenedByTheCaller
// pins that.
//
// roleMW itself is left alone: narrowing a list 17 modules depend on would break
// all of them.
func RegisterRoutes(r *gin.Engine, authMW, roleMW, adminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	universities := r.Group("/api/v1/universities")
	{
		universities.GET("", h.GetUniversities)
		universities.GET("/filter-counts", h.GetUniversityFilterCounts)
		universities.GET("/:id", h.GetUniversityByID)
		universities.GET("/:id/courses", h.GetUniversityCourses)
		universities.GET("/:id/scholarships", h.GetUniversityScholarships)
		universities.GET("/:id/affiliated-colleges", h.GetAffiliatedColleges)
		universities.GET("/:id/:tab", h.GetUniversityTab)
	}

	// The published directory, on roleMW. See the note on RegisterRoutes: this is
	// the public listing behind a redundant admin path, kept reachable by tenants
	// because the institution dashboard depends on it.
	admin := r.Group("/api/v1/admin")
	admin.Use(authMW)
	admin.Use(roleMW)
	{
		admin.GET("/universities", h.GetUniversities)
	}

	// Everything that is actually privileged in this module.
	adminOnly := r.Group("/api/v1/admin")
	adminOnly.Use(authMW)
	adminOnly.Use(adminRoleMW)
	{
		adminOnly.GET("/universities/:id", h.AdminGetUniversityByID)
		adminOnly.POST("/universities", h.CreateUniversity)
		adminOnly.PUT("/universities/:id", h.UpdateUniversity)
		adminOnly.DELETE("/universities/:id", h.DeleteUniversity)
	}
}
