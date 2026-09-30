package system

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the public system surface, the platform admin surface,
// and the institution-zone advertise surface.
//
// adminRoleMW is passed in rather than built here, and it is deliberately NOT
// the shared roleMW from cmd/server/main.go. That gate is
// RequireRole("admin", "super_admin", "scholarship_provider", …, "institution")
// — a multi-tenant list shared by 17 modules, each wired to it for its own
// reasons. Behind it, on the two groups below, an ordinary paying customer could
// read every visitor's contact message on the platform and delete it, and could
// rewrite the site-wide ad configuration. access.go has the full argument.
//
// So the inbox and the ad config get their own gate, built from
// system.PlatformAdminRoles() so the middleware here and the service checks in
// access.go cannot drift. Every handler under it also passes a Viewer into the
// service, which re-checks the role, so passing a wider gate back in at the call
// site cannot re-open this. TestSystemInboxAndAdGateCannotBeWidenedByTheCaller
// pins that.
//
// roleMW itself is left alone: narrowing a list 17 modules depend on would break
// all of them. It still governs the rest of the /admin group below and the
// institution zone, both of which have reasons of their own.
func RegisterRoutes(r *gin.Engine, authMW, roleMW, adminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		system := v1.Group("/system")
		{
			// POST /contact takes no auth on purpose: it is the public contact
			// form, and the people who use it are prospective students with no
			// account. That is exactly why the inbox that collects them is
			// platform-admin only — see access.go.
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
			system.GET("/college-ad-card-settings", h.GetCollegeAdCardSettings)
			system.GET("/college-type-counts", h.GetCollegeTypeCounts)
		}

		// The platform inbox and the ad configuration. Behind adminRoleMW, not
		// roleMW. This group is split out from the /admin group below purely so
		// the gate can differ; the handlers are unchanged.
		//
		// GET /inquiries/:id and GET /ads/:id answer 404 for a missing row, and
		// 403 for a caller without the role. Those are different questions — "no
		// such record" versus "not permitted to ask" — and access.go explains why
		// the role refusal is not a 404 here.
		inbox := v1.Group("/admin")
		inbox.Use(authMW)
		inbox.Use(adminRoleMW)
		{
			inbox.GET("/inquiries", h.GetContactInquiries)
			inbox.GET("/inquiries/:id", h.GetContactInquiryByID)
			inbox.PUT("/inquiries/:id/status", h.UpdateContactInquiryStatus)
			inbox.DELETE("/inquiries/:id", h.DeleteContactInquiry)

			inbox.GET("/ads", h.GetAds)
			inbox.GET("/ads/:id", h.GetAdByID)
			inbox.POST("/ads", h.CreateAd)
			inbox.PUT("/ads/:id", h.UpdateAd)
			inbox.DELETE("/ads/:id", h.DeleteAd)
			// The click counter is also on the public route below, where it is
			// written by anonymous page views. Exposing it here adds nothing, but
			// it is mounted for the admin panel and so it belongs on this gate.
			inbox.POST("/ads/:id/click", h.TrackAdClick)
		}

		// The rest of the admin surface. Still on roleMW, and deliberately not
		// changed by this fix: these are the carousel, landing-page, course-ad,
		// college-ad and advertise-request editors, whose tenant story is a
		// separate question from the inbox and is not answered here. Flagged, not
		// fixed — see the report.
		admin := v1.Group("/admin")
		admin.Use(authMW)
		admin.Use(roleMW)
		{
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
			admin.PUT("/college-ad-card-settings", h.UpdateCollegeAdCardSettings)
			admin.GET("/college-ad-feedback", h.GetCollegeAdFeedback)

			admin.GET("/advertise-requests", h.GetAdvertiseRequests)
			admin.PUT("/advertise-requests/:id/status", h.UpdateAdvertiseRequestStatus)
		}

		// Institution-zone advertise requests (authenticated institution users).
		// Left on roleMW: this group is how an institution account asks to buy
		// placement, and the reads are already scoped to the caller by
		// GetInstitutionAdvertiseRequests. The counterpart that actually approves
		// the request is on the /admin group above.
		instZone := v1.Group("/institution")
		instZone.Use(authMW)
		instZone.Use(roleMW)
		{
			instZone.POST("/ad-requests", h.SubmitAdvertiseRequest)
			instZone.GET("/ad-requests", h.GetInstitutionAdvertiseRequests)
		}
	}
}
