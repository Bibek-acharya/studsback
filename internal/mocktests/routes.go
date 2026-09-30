package mocktests

import (
	"studsphere/backend/internal/shared/middleware"

	"github.com/gin-gonic/gin"
)

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

		// The paper itself: public, but behind OptionalAuth rather than Auth.
		//
		// OptionalAuth resolves a session when one is present and lets an anonymous
		// request through when it is not, which is what the coin gate on this route
		// needs to tell "a student who has not paid" from "a visitor with no
		// account". Auth would have been the wrong choice: it would 401 a
		// logged-out visitor for a paper that may well be ungated, so switching the
		// economy on would have changed who can BROWSE — a product change this slice
		// must not make silently. With OptionalAuth the gate decides, and it decides
		// after reading the kill switch, so an ungated paper is served to anybody
		// exactly as before and only a GATED paper asks an anonymous reader to sign
		// in. See handler.go GetTest.
		//
		// The list route above stays entirely unauthenticated and is unaffected: it
		// returns summaries with no questions and no options, so there is nothing in
		// it to gate.
		papers := v1.Group("/mock-tests")
		papers.Use(middleware.OptionalAuth())
		{
			papers.GET("/:id", h.GetTest)
		}

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
