package system

import (
	"errors"
	"log"
	"strings"

	"studsphere/backend/internal/shared/httpx"

	"github.com/gin-gonic/gin"
)

// WHO MAY READ THE INQUIRY INBOX, AND WHO MAY REWRITE THE AD PLACEMENTS.
//
// Two route groups in routes.go — GET/GET :id/PUT :id/status/DELETE :id on
// /admin/inquiries, and the five CRUD routes on /admin/ads — were mounted behind
// the shared roleMW from cmd/server/main.go. That gate is
// RequireRole("admin", "super_admin", "scholarship_provider", …, "institution"):
// a multi-tenant list shared by 17 modules, each wired to it for its own reasons.
// It admits "institution" and "scholarship_provider" alongside the platform
// operators, and nothing below it checked anything. So an ordinary paying
// customer could read every visitor's contact message on the platform and delete
// them, and could rewrite the site-wide ad configuration.
//
// The inbox is the more serious of the two and it is worth being precise about
// what is exposed. ContactInquiry holds Name, Email, Phone, Subject, Message and
// a Type — messages sent through the public POST /system/contact form, which has
// no auth at all, by people who were looking for a college course and have no
// relationship to any account on the site. DELETE is the sharp edge: a soft
// delete of somebody else's message to the platform, by an account with no
// standing to decide whether it should be retained.
//
// The rule is platform-admin only, and it follows from the model:
//
//   - The inbox is the platform's. SubmitContactInquiry notifies
//     ForRoles("superadmin", "admin") and nobody else — the module already
//     decided who the audience for an inquiry is, in code, before this file
//     existed.
//   - The tenant that legitimately needs to see AND WRITE inquiries addressed to
//     it ALREADY HAS ITS OWN ROUTE, and it is now scoped on both sides:
//     institution.GET("/inquiries") calls GetInstitutionInquiries with the id off
//         the caller's own authenticated account, and institution.PUT("/inquiries/:id/status")
//         and institution.DELETE("/inquiries/:id") call
//         UpdateContactInquiryStatusForInstitution and
//         DeleteContactInquiryForInstitution, which refuse anything not addressed
//         to that id. That route is the answer to "which non-admin principal needs
//     the inbox", and it means admin-only here locks nobody out. There is no
//     second inbox to widen.
//
// Ad is a different shape of record and gets the same answer for a different
// reason. Ad has no institution column at all: it carries Page, Position,
// Priority, Active and a click/impression counter, i.e. it describes a slot on
// the platform's own pages. GET /system/ads serves the active ones to anonymous
// visitors with no auth. So an Ad is first-party site configuration, owned by
// whoever runs the site, and there is no row on it that could name a tenant.
//
// So this is a gate, not a scope: unlike internal/jobs, no non-admin principal is
// left over once the rule is applied. The middleware answers the role question
// before the handler runs and before the body is parsed, and the service repeats
// it so that a caller who passes the wide roleMW back in cannot re-open this.
// Both read PlatformAdminRoles, and both answer 403.
//
// THE SECOND HALF OF THAT FINDING IS NOW FIXED, and it is worth recording what it
// was because the read side was always scoped and only the write side was not.
//
// internal/institution mounted institution.PUT("/inquiries/:id/status") and
// institution.DELETE("/inquiries/:id") against the UNSCOPED
// UpdateContactInquiryStatus and DeleteContactInquiry below, with an id straight
// from the URL. So an institution account could read only its own inbox and yet
// could move ANY visitor's message on the platform into any status, overwrite the
// internal Notes field, and delete it — rows it could not even see.
//
// Both tenant routes now go through UpdateContactInquiryStatusForInstitution and
// DeleteContactInquiryForInstitution, which check ownership in the WHERE clause
// and answer ErrInquiryNotFound (404, not 403: a caller probing for another
// tenant's ids must not learn that an id exists). A platform-level inquiry
// (institution_id NULL) is unreachable to every tenant through the tenant path,
// which is consistent with GetInstitutionInquiries never returning those rows.
//
// The unscoped pair is KEPT rather than deleted, on purpose and for a narrower
// reason than before: they are the primitive the admin path and the tenant path
// both sit on, and UpdateContactInquiryStatusAsAdmin guards the admin one by role.
// Nothing outside this module should call either directly.

// ErrForbidden is what an inbox or ad call from a non-admin returns.
//
// 403, not the 404 the applicant routes in internal/jobs use, and the reason is
// that there is no ownership question here to hide. These are role refusals: the
// caller is told they lack the role and learns nothing about whether a given
// inquiry id exists. Nothing in this file is a "not yours" answer, because on
// this model there is no "yours" — the inbox and the ad slots belong to the
// platform. A 403 also says the truth to an operator: the request failed on
// authorization, it did not fail because an inquiry was missing.
var ErrForbidden = errors.New("insufficient permissions")

// PlatformAdminRoles are the roles that run the platform itself, matching the
// two aliases the rest of the codebase treats as the same operator
// ("superadmin" in auth.Service, "super_admin" in cmd/server/main.go's roleMW).
//
// Exported because cmd/server/main.go builds the inbox and ad gates from this
// list, so the middleware at the edge and the checks in this file cannot drift
// apart. The service is the authority either way; see authorizePlatformAdmin.
func PlatformAdminRoles() []string {
	return []string{"admin", "super_admin", "superadmin"}
}

// Viewer is the authenticated principal behind an inbox or ad request.
type Viewer struct {
	UserID uint
	Role   string
}

// IsPlatformAdmin reports whether this viewer is a platform operator.
func (v Viewer) IsPlatformAdmin() bool {
	role := strings.ToLower(strings.TrimSpace(v.Role))
	for _, adminRole := range PlatformAdminRoles() {
		if role == adminRole {
			return true
		}
	}
	return false
}

// ViewerFrom reads the authenticated principal off the gin context. It returns a
// zero Viewer when nothing resolved — no role and no id, therefore not an admin.
// A missing context denies by default rather than by accident.
func ViewerFrom(c *gin.Context) Viewer {
	v := Viewer{}
	if id, ok := httpx.CurrentUserID(c); ok {
		v.UserID = id
	}
	if role, ok := c.Get("user_role"); ok {
		if s, ok := role.(string); ok {
			v.Role = s
		}
	}
	return v
}

// authorizePlatformAdmin refuses an inbox or ad call to anyone who is not a
// platform operator, and is the authority for all nine routes.
//
// Refused before any lookup and without touching the repository: this is a role
// question, so there is no existence information to hide and nothing to gain by
// querying. The caller and role go to the log so a 403 in production is still
// debuggable without making the response distinguishable.
func (s *Service) authorizePlatformAdmin(v Viewer, action string) error {
	if v.IsPlatformAdmin() {
		return nil
	}
	log.Printf("system: denied %s user_id=%d role=%q reason=inbox and ad config are platform-admin only", action, v.UserID, v.Role)
	return ErrForbidden
}
