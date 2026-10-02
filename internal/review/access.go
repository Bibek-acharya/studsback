package review

import (
	"errors"
	"strings"
)

// WHO MAY MODERATE A REVIEW, AND WHO MAY READ THE MODERATION QUEUES.
//
// Three separate surfaces in this module were all on the shared roleMW from
// cmd/server/main.go — RequireRole("admin", "super_admin", "scholarship_provider",
// "scholarship-provider", "Scholarship Provider", "scholarship_provider_subuser",
// "institution"), a multi-tenant list shared by 17 modules. Each is a different
// question and all three now answer "platform admin only":
//
//  1. adminReviews — GET /admin/university-reviews/:universityId, DELETE /:id
//  2. adminDateReports — GET /admin/date-reports, PUT /:id, DELETE /:id
//  3. instReviews.DELETE("/:id")
//
// ## WHY ALL THREE ARE ADMIN ONLY, WHICH IS NOT OBVIOUS FOR (3)
//
// Surface (3) is the one worth arguing, because its route group is named
// /institution/reviews and it sits next to two reads that a tenant legitimately
// needs. An institution asking for its own reviews, or for a college's reviews, is
// a real capability — and it already exists, scoped, on this same group:
//
//	instReviews.GET("", h.GetInstitutionReviews)      — keyed on getUserID(c)
//	instReviews.GET("/college/:collegeId", h.GetCollegeReviews)
//
// The DELETE was never part of that. It calls the same unscoped
// Service.AdminDeleteReview that surface (1)'s DELETE calls, with an id from the
// URL and no comparison to the caller at any layer — so an institution account
// could delete ANY review on the platform: another college's, another
// institution's, or a review naming an individual student, and the author received
// a moderation notification for it (EventSocialReviewModerated) without anyone
// having moderated anything. This is the same shape as the inquiry bug fixed in
// 85e6d89: the read side was always scoped and only the write side was not.
//
// Nothing is lost by removing it. A tenant that wants a review taken down already
// has the path for it: ReviewReport, on the public side, which is how a
// moderation queue is supposed to be fed. Direct deletion by a tenant is not a
// capability the product needs.
//
// ## WHY (1) AND (2) ARE ADMIN ONLY
//
// DateReport is a row submitted through the PUBLIC endpoint
// POST /api/v1/reports, which has no auth, carrying Contact — a phone number — and
// free-text Feedback. A tenant enumerating /admin/date-reports harvests phone
// numbers of members of the public. Moderation of a public report queue is
// moderation; there is no tenant-scoped version of it to preserve.
//
// ## WHY THE ROLE LIST IS NOT NARROWED IN PLACE
//
// roleMW still goes to the other modules wired to it, each configured for its own
// reasons. This module derives its own list, and RegisterRoutes no longer accepts
// a roleMW parameter at all — an ignored wide gate in a signature is an
// invitation to pass it back in.

// ErrForbidden is a ROLE refusal, answered before any lookup: 403, because there
// is no existence question to hide and nothing has been loaded.
var ErrForbidden = errors.New("insufficient permissions")

// PlatformAdminRoles are the roles that run the platform itself: admin, super_admin
// and superadmin. The same three aliases system, university, admission and
// scholarship all use — an operator role that differs between modules is its own
// class of bug.
func PlatformAdminRoles() []string {
	return []string{"admin", "super_admin", "superadmin"}
}

// IsPlatformAdmin reports whether a role string names a platform operator.
// Case-insensitive and whitespace-tolerant: roles arrive from a JWT claim and the
// codebase contains both "superadmin" and "super_admin".
func IsPlatformAdmin(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	for _, adminRole := range PlatformAdminRoles() {
		if r == adminRole {
			return true
		}
	}
	return false
}
