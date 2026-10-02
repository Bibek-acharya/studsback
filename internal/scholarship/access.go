package scholarship

import (
	"errors"
	"strings"
)

// WHO MAY READ AND WRITE THE SCHOLARSHIP ADMIN QUEUE — the module behind /admin.
//
// This module's admin group was mounted on the shared roleMW from
// cmd/server/main.go: RequireRole("admin", "super_admin", "scholarship_provider",
// "scholarship-provider", "Scholarship Provider", "scholarship_provider_subuser",
// "institution"). That list is a multi-tenant list shared by 17 modules, each wired
// to it for its own reasons, and for this module it admitted ordinary paying
// tenants to:
//
//	GET    /admin/scholarship-applications        every application, platform-wide
//	GET    /admin/scholarship-applications/:id     any single application
//	GET    /admin/scholarship-applications/scholarship/:id  one scholarship's
//	PUT    /admin/scholarship-applications/:id/status        change any status
//	POST   /admin/payments/verify-esewa            settle payments platform-wide
//	POST   /admin/payments/send-admit-cards        mail admit cards platform-wide
//	plus the whole scholarship catalogue CRUD
//
// ScholarshipApplication is the most sensitive row this codebase has. It carries
// FullName, Gender, Ethnicity, DateOfBirthBS and DateOfBirthAD, Age, PhoneNumber,
// Email, PhotoURL, the school address down to tole, the PERMANENT and TEMPORARY
// address down to ward and tole, GuardianName/Phone/Email, both parents'
// occupations, FamilyMonthlyIncome, FamilyMembersCount, and Documents as a jsonb
// blob of whatever the applicant uploaded. A parent's occupation and a family's
// monthly income are not fields a scholarship platform needs to hand to any tenant
// that asks.
//
// So the rule is PLATFORM ADMIN ONLY, and it is the same answer as
// internal/admission's — see admission/access.go for why tenant scoping by
// college was deliberately left undecided there. It applies here for the same
// reason and more strongly: the row is a minor's family financial profile, and
// "should a scholarship provider see the household income of an applicant to
// another provider's scholarship" is a product question with a compliance tail,
// not something to settle by widening a role list.
//
// ## What providers legitimately get, and where
//
// A scholarship_provider reviewing applications to its OWN scholarships is a real
// and necessary capability, and this module does NOT take it away. It lives in
// internal/scholarshipprovider, on /api/v1/scholarship-providers/applications,
// and every one of those queries filters on provider_id — for example
// GetApplicationsByProvider and GetApplicationByIDAndProvider both carry
// `provider_scholarships.provider_id = ?` in their WHERE. So a provider sees its own
// applicants and only its own.
//
// Which is exactly why the /admin copy in THIS module has no provider filter to
// lose: ApplicationFindAll has no provider predicate at all, and a provider account
// reaching it through roleMW was not seeing its own applicants, it was seeing
// everybody's, including applicants to scholarships it does not administer and
// applications whose provider is another tenant entirely.
//
// Narrowing this group costs no provider anything, and the routes that carry a
// provider's real work are untouched.

// ErrForbidden is a ROLE refusal: the caller is told they lack the role and learns
// nothing about any particular record. Asked before any lookup, so it is 403.
var ErrForbidden = errors.New("insufficient permissions")

// PlatformAdminRoles are the roles that run the platform itself: admin, super_admin
// and superadmin. The same three aliases system, university and admission all use —
// an operator role that differs between modules is its own class of bug.
func PlatformAdminRoles() []string {
	return []string{"admin", "super_admin", "superadmin"}
}

// IsPlatformAdmin reports whether a role string names a platform operator.
// Case-insensitive and whitespace-tolerant, because roles arrive from a JWT claim
// and the codebase contains both "superadmin" and "super_admin".
func IsPlatformAdmin(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	for _, adminRole := range PlatformAdminRoles() {
		if r == adminRole {
			return true
		}
	}
	return false
}
