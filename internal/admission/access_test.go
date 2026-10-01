package admission

import "testing"

// WHO MAY READ AND WRITE AN ADMISSION, as pinned by the tests below:
//
//	platform admin (admin / superadmin / super_admin) — every route
//	the applicant                                        — their own row only
//	every other tenant, including institution and
//	  scholarship_provider                               — nothing
//
// The module mounted all four admin reads and the status write on roleMW, which
// cmd/server/main.go builds as
// RequireRole("admin", "super_admin", "scholarship_provider", …, "institution").
// That is a multi-tenant list shared by 17 modules. So `institution` and
// `scholarship_provider` accounts — ordinary paying customers — could read every
// applicant's name, email, phone, date of birth, address and uploaded documents,
// across every college on the platform.
//
// The scope chosen is ADMIN ONLY, and deliberately narrower than what is
// technically possible. An institution account could be scoped to the colleges it
// owns through institution_users.college_id, and that is left as a product
// decision: what a college-scoped institution should be shown of a `Documents`
// blob is a data-minimisation question, not an authorisation one, and inheriting
// it from a broken route would decide it by accident. Narrowing to admins closes
// the disclosure now; tenant scoping can then be designed on purpose.
//
// WHY 404 AND NOT 403. An ownership refusal must not confirm that an id exists,
// and these rows are applicant PII where existence is itself information. This
// matches how internal/jobs scopes its applicant routes.

func TestPlatformAdminRoleListDoesNotAdmitTenants(t *testing.T) {
	// The role list is the whole fix for the read routes, so it is pinned
	// explicitly rather than only through the routes that use it. Every one of
	// these was admitted before.
	for _, tenant := range []string{
		"institution", "scholarship_provider", "scholarship-provider",
		"Scholarship Provider", "scholarship_provider_subuser", "student", "user",
	} {
		if IsPlatformAdmin(tenant) {
			t.Errorf("IsPlatformAdmin(%q) = true; this list is the only thing standing between a tenant and every applicant's PII", tenant)
		}
	}
	for _, admin := range []string{"admin", "superadmin", "super_admin", "Admin", " SUPER_ADMIN "} {
		if !IsPlatformAdmin(admin) {
			t.Errorf("IsPlatformAdmin(%q) = false; the operator would be locked out of the queue", admin)
		}
	}
}
