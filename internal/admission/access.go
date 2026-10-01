package admission

import (
	"errors"
	"strings"
)

// ErrAdmissionNotFound is the refusal for a row that does not exist OR does not
// belong to the caller.
//
// The two are deliberately indistinguishable, and the reason is specific to what
// this table holds: an Admission row carries StudentName, StudentEmail,
// StudentPhone, DateOfBirth, Address and Documents. Whether a given application id
// exists is itself information about a person who applied to a college, so a 403
// ("you may not see this, but it is there") leaks something a 404 does not.
// internal/jobs already scopes applicant routes this way.
var ErrAdmissionNotFound = errors.New("admission not found")

// ErrForbidden is a ROLE refusal.
//
// Distinct from ErrAdmissionNotFound on purpose. This one is the answer to "may
// this kind of account use an admin route at all" — asked before any lookup, so it
// discloses nothing about any row and is 403 because it is true. That is the same
// distinction system's ErrForbidden documents.
var ErrForbidden = errors.New("insufficient permissions")

// PlatformAdminRoles are the roles that run the platform itself, matching the two
// aliases the rest of the codebase treats as the same operator ("superadmin" in
// auth.Service, "super_admin" in cmd/server/main.go's roleMW).
//
// THIS LIST IS THE FIX for the read routes, so it is worth being explicit about
// what it replaced. internal/admission mounted every admin read and the status
// write on the shared roleMW, which is
// RequireRole("admin", "super_admin", "scholarship_provider", "scholarship-provider",
// "Scholarship Provider", "scholarship_provider_subuser", "institution") — a list
// shared by 17 modules, each wired to it for its own reasons. That admitted
// `institution` and `scholarship_provider` accounts to GET /admin/admissions, to
// GET /admin/admissions/:id, and to GET /admin/admissions/college/:collegeId, so
// an ordinary paying tenant could read every applicant's name, email, phone, date
// of birth, address and uploaded documents across every college on the platform.
//
// Note what is NOT here and why. A `superadmin`/`super_admin` only list is
// deliberately NOT what this is: those two aliases plus `admin` are the operator
// set, and `admin` is included because system's own PlatformAdminRoles() and
// university's include it and an operator role that differs between modules is its
// own class of bug.
//
// The scope is admin-only rather than tenant-scoped, which is narrower than
// technically possible: institution_users.college_id would allow scoping a college's
// own applicants to the institution that owns it. That is left as a product
// decision — what a college-scoped institution should see of a Documents blob is a
// data-minimisation question, and deciding it by accident here would be worse than
// not deciding it. Narrowing to admins closes the disclosure now; the tenant
// question can be answered deliberately afterwards.
func PlatformAdminRoles() []string {
	return []string{"admin", "super_admin", "superadmin"}
}

// IsPlatformAdmin reports whether a role string names a platform operator.
//
// Case-insensitive and whitespace-tolerant, matching system's IsPlatformAdmin:
// roles arrive from a JWT claim and from registration forms, and the codebase
// contains both "superadmin" and "super_admin" spellings.
func IsPlatformAdmin(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	for _, adminRole := range PlatformAdminRoles() {
		if r == adminRole {
			return true
		}
	}
	return false
}
