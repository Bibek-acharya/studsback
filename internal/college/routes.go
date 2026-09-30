package college

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the public find-college surface, the college-catalogue
// admin surface, and the institution's own location route.
//
// roleMW is passed in and is deliberately still the shared list from
// cmd/server/main.go. That gate is
// RequireRole("admin", "super_admin", "scholarship_provider", …, "institution"),
// and narrowing it is NOT the fix here, for a reason specific to this module: an
// institution account is a legitimate principal for one of these routes. It
// administers a college, institution_users.college_id says which one, and
// PUT /admin/colleges/:id/location has to stay reachable by that account or the
// capability internal/institution and the institution dashboard already depend on
// stops working. access.go has the full argument.
//
// So the role gate stays wide and the DECISION moves into the service, which is
// the shape the jobs module already uses for a tenant-scoped route. The service
// compares the college in the URL against the caller's own college and refuses
// with 403 when they differ, and every catalogue route that no tenant owns is
// platform-admin only there. TestCollegeLocationIsTenantScoped pins both halves —
// a guard that only denied would pass every negative test here while breaking the
// product.
//
// The service is the authority, not this file: passing a wider roleMW at the call
// site cannot re-open anything, because nothing above the handler reads the role.
// TestCollegeGateCannotBeWidenedByTheCaller pins that.
func RegisterRoutes(r *gin.Engine, authMW, roleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		colleges := v1.Group("/colleges")
		{
			colleges.GET("", h.GetColleges)
			colleges.GET("/filter-counts", h.GetCollegeFilterCounts)
			colleges.GET("/featured", h.GetFeaturedColleges)
			colleges.GET("/popular-comparisons", h.GetPopularComparisons)
			colleges.GET("/compare", h.CompareColleges)
			colleges.GET("/:id", h.GetCollegeByID)
			colleges.POST("/recommend", h.RecommendColleges)
			colleges.POST("/log-comparison", h.LogComparison)
		}

		admissions := v1.Group("/admissions")
		{
			admissionColleges := admissions.Group("/colleges")
			{
				admissionColleges.GET("", h.GetColleges)
				admissionColleges.GET("/filter-counts", h.GetCollegeFilterCounts)
				admissionColleges.GET("/featured", h.GetFeaturedColleges)
				admissionColleges.GET("/:id", h.GetCollegeByID)
				admissionColleges.POST("/recommend", h.RecommendColleges)
			}
			admissions.GET("/direct", h.GetColleges)
		}

		// Public map routes
		v1.GET("/map/colleges", h.GetMapColleges)

		// Super admin college location
		admin := v1.Group("/admin/colleges")
		admin.Use(authMW)
		admin.Use(roleMW)
		{
			admin.GET("", h.GetColleges)
			admin.GET("/:id", h.GetCollegeByID)
			admin.POST("", h.CreateCollege)
			admin.POST("/upload-image", h.UploadCollegeImage)
			admin.PUT("/:id", h.UpdateCollege)
			admin.DELETE("/:id", h.DeleteCollege)
			admin.PUT("/:id/approve", h.ApproveCollege)
			admin.PUT("/:id/featured", h.ToggleCollegeFeatured)
			admin.PUT("/:id/location", h.UpdateCollegeLocation)
		}

		// Institution college location
		inst := v1.Group("/institution")
		inst.Use(authMW)
		{
			inst.PUT("/college/location", h.UpdateInstitutionCollegeLocation)
		}
	}
}
