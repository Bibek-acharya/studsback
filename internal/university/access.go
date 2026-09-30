package university

import (
	"errors"
	"log"
	"strings"

	"studsphere/backend/internal/shared/httpx"

	"github.com/gin-gonic/gin"
)

// WHO MAY EDIT THE UNIVERSITY CATALOGUE.
//
// The same shape as the college module and the same question, and the same
// answer — reached from the model rather than from the route name.
//
// IS THERE A LEGITIMATE PER-UNIVERSITY PRINCIPAL? The field that would decide it
// is institution_users.university_id, and the honest reading of that column is
// that it is an AFFILIATION DESCRIPTOR, not an ownership claim. It sits in
// InstitutionUser immediately beside Affiliation, UniversityAffiliations and
// NonUniversityAffiliation; it is a nullable *uint; and it answers "which
// university is this college attached to", not "which university does this
// account run". The contrast with the college module is the point: there, an
// institution account is trusted to write its college's map pin and a live route
// does it. Here, no route anywhere grants an institution account a single write
// on a University, and the only University fields an institution would gain by
// editing one — Verified, Popular, Rank, ReviewCount, and the About/Leadership/
// Faculties/Events JSON blobs — are the platform's curation of a public directory
// page.
//
// University has no owner column either: no institution_id, no created_by, and
// CreateUniversity is never handed the caller's identity, so not even an
// admin-created record notes who owns it. The universities are first-party
// reference data, the same conclusion internal/jobs reaches for the job
// catalogue.
//
// So the rule is platform-admin only for the four privileged routes, and admin
// only
// is a FINISHED answer here precisely because no non-admin principal was left
// over once the rule was applied. There is nothing to scope TO: scoping would
// mean inventing a tenant relationship the code does not have.
//
// The two reads are a different matter and are deliberately NOT narrowed:
//
//   - GET /admin/universities is the same handler the PUBLIC
//     GET /api/v1/universities mounts, with no auth on the public one at all.
//     It returns the published directory. It grants an institution account
//     nothing it cannot already fetch signed out, so gating it would be
//     theatre — and it would break a live caller: the institution dashboard's
//     ProfilePage.tsx fetches /api/v1/admin/universities?limit=500 with the
//     institution token to populate its university picker. Gating that list is
//     the lockout this file is written to avoid.
//
//   - GET /admin/universities/:id is NOT the public read. It goes through
//     FindByIDFull rather than FindByID, so it returns rows the public route
//     does not — including anything not yet published. That is a genuine
//     privilege, so it is platform-admin only.
//
// Two spellings of the same check exist on purpose: authorizeCatalogue answers
// 403 for a role refusal, and the service is the authority for every route
// below, so passing the wide roleMW back in cannot re-open them.

// ErrForbidden is what a university-catalogue call from a non-admin returns.
//
// 403, not 404, and the reason is that a University is public data:
// GET /api/v1/universities serves the published directory to anyone with no
// auth. There is no existence secret to protect, so a 403 costs an enumerator
// nothing and tells an operator the truth — the request failed on authorization,
// not because a university was missing. The handler's own 404 for a row that is
// genuinely absent is a different question and is left alone.
var ErrForbidden = errors.New("insufficient permissions")

// PlatformAdminRoles are the roles that run the platform itself, matching the
// two aliases the rest of the codebase treats as the same operator
// ("superadmin" in auth.Service, "super_admin" in cmd/server/main.go's roleMW).
//
// Exported because cmd/server/main.go builds the university gate from this list,
// so the middleware at the edge and the checks in this file cannot drift apart.
// The service is the authority either way.
func PlatformAdminRoles() []string {
	return []string{"admin", "super_admin", "superadmin"}
}

// Viewer is the authenticated principal behind a university-catalogue request.
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
// zero Viewer when nothing resolved — no role, therefore not an admin. A missing
// context denies by default rather than by accident.
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

// authorizeCatalogue refuses a university-catalogue call to anyone who is not a
// platform operator, and is the authority for the create, update, delete and
// full-read routes.
//
// Refused before any lookup and without consulting the repository: a role
// question, so there is no existence information to hide. The caller and role go
// to the log so a 403 in production stays debuggable.
func (s *Service) authorizeCatalogue(v Viewer, action string) error {
	if v.IsPlatformAdmin() {
		return nil
	}
	log.Printf("university: denied %s user_id=%d role=%q reason=university catalogue is platform-admin only", action, v.UserID, v.Role)
	return ErrForbidden
}
