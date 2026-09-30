package college

import (
	"errors"
	"log"
	"strings"

	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/shared/httpx"

	"github.com/gin-gonic/gin"
)

// WHO MAY EDIT THE COLLEGE CATALOGUE — and the one place a tenant may.
//
// This module is the second of the four audited behind the shared roleMW, and
// it is the only one of the four where the honest answer is NOT "admins only".
// The route name says /admin/colleges and the gate that used to sit on it says
// admin, and both suggest a platform catalogue with no tenant. The model says
// otherwise, and the model wins.
//
// DOES A COLLEGE HAVE AN INSTITUTION OF ITS OWN? Yes, and the field that decides
// it is institution_users.college_id — InstitutionUser.CollegeID, a non-nullable
// uint that names the College row a given institution account administers. The
// proof that this is a real ownership claim rather than a coincidence is already
// in this module and predates any of this: UpdateInstitutionCollegeLocation
// reads it, and when it is zero the handler refuses with "No college associated
// with your account". The module has always treated that column as "my college".
//
// So an institution account is a legitimate principal for exactly one thing here,
// and the codebase has already built it a route for it: institution.PUT
// ("/college/location"), mounted behind authMW alone, correctly scoped to
// instUser.CollegeID. That is the whole tenant story. Which means:
//
//   - PUT /admin/colleges/:id/location is tenant-scoped, not admin-only. It takes
//     an arbitrary :id, and under roleMW an institution account could use it to
//     move ANY college's map pin — corrupting a competitor's location in the
//     find-college map. The rule: platform admin for any college, or the
//     institution whose CollegeID is that college. Denying an institution its
//     OWN college's pin here costs it nothing, because the dedicated
//     /institution/college/location route still serves it.
//
//   - Everything else in the group — list, read, create, full profile update,
//     delete, approve, toggle-featured, upload-image — is platform-admin only.
//     College has no institution_id, no created_by and no owner column, so the
//     only tenant link in the whole schema is the one on institution_users. That
//     link names a single college, and the only field an institution is trusted
//     to write on it is the map pin. Curation is the platform's: Verified,
//     Claimed, Featured, Popular, Rating, Reviews and the programme JSON blobs
//     are what a paying institution would most want to edit, and a "tenant may
//     edit its own record" rule that reached them would be handing over exactly
//     the fields that make the catalogue trustworthy. Approve and toggle-featured
//     are moderation decisions by name.
//
// This is therefore the "scope, don't gate" answer, and it is the case the brief
// warned about: an admin-only rule here would have broken a real, live,
// already-correct institution capability, and a rule that only ever says "admin"
// is not finished.

// ErrForbidden is what a college-catalogue call from a caller with no claim on
// the record returns.
//
// 403 and not 404, and the reason is the same as internal/jobs' job-catalogue
// rule: a College is public data. GET /api/v1/colleges/:id serves it to anyone
// with no auth at all, and the compare, filter-counts and map routes serve the
// whole set. So there is no existence secret to protect — a caller refused here
// learns only its own role, which it already knows, and a 403 tells an operator
// the truth: this failed on authorization, not because a college was missing.
//
// Not-found inside the service therefore still surfaces as the handler's own 404,
// and the two answers are not confused.
var ErrForbidden = errors.New("insufficient permissions")

// PlatformAdminRoles are the roles that run the platform itself, matching the
// two aliases the rest of the codebase treats as the same operator
// ("superadmin" in auth.Service, "super_admin" in cmd/server/main.go's roleMW).
//
// Exported because cmd/server/main.go builds the college-catalogue gate from this
// list, so the middleware at the edge and the checks in this file cannot drift
// apart. The service is the authority either way.
func PlatformAdminRoles() []string {
	return []string{"admin", "super_admin", "superadmin"}
}

// TenantCollegeLookup is the narrow slice of internal/institution this package
// needs in order to answer "which college does this caller administer?".
//
// An interface rather than a concrete *institution.Repository so the ownership
// rule can be tested against a stub without standing up that module's schema.
// The production wiring in cmd/server/main.go passes institutionRepo, the same
// value college.NewHandler already receives.
type TenantCollegeLookup interface {
	FindInstitutionUserByID(id uint) (*institution.InstitutionUser, error)
}

// Viewer is the authenticated principal behind a college-catalogue request.
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
// zero Viewer when nothing resolved — no role and no id, therefore not an admin,
// and matching no institution_users row either. A missing context denies by
// default rather than by accident.
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

// TenantCollegeID is the field the whole file turns on: the College row this
// viewer administers, or 0 when the viewer is not an institution account or has
// no college attached.
//
// It is resolved from the viewer's own authenticated user id through
// institution_users, never from anything the request body or a path parameter
// supplies. That is what stops a caller naming a college that is not theirs.
func (s *Service) TenantCollegeID(v Viewer) uint {
	if v.IsPlatformAdmin() || v.UserID == 0 || s.tenants == nil {
		return 0
	}
	user, err := s.tenants.FindInstitutionUserByID(v.UserID)
	if err != nil || user == nil {
		return 0
	}
	return user.CollegeID
}

// ownsCollege reports whether the viewer administers this college, by
// institution_users.college_id.
func (s *Service) ownsCollege(v Viewer, collegeID uint) bool {
	return collegeID != 0 && s.TenantCollegeID(v) == collegeID
}

// authorizeCatalogue refuses a college-catalogue call to anyone who is not a
// platform operator. Used for every route in the group except the map pin.
func (s *Service) authorizeCatalogue(v Viewer, action string) error {
	if v.IsPlatformAdmin() {
		return nil
	}
	log.Printf("college: denied %s user_id=%d role=%q reason=college catalogue is platform-admin only", action, v.UserID, v.Role)
	return ErrForbidden
}

// authorizeLocation is the tenant-scoped rule: a platform operator may move any
// college's pin, and an institution account may move its own.
//
// This is the one route in the group where a non-admin principal survives, so the
// check is an ownership comparison rather than a role test, and it is done here
// rather than in the handler so that a route added later inherits it.
func (s *Service) authorizeLocation(v Viewer, collegeID uint) error {
	if v.IsPlatformAdmin() || s.ownsCollege(v, collegeID) {
		return nil
	}
	log.Printf("college: denied location update on college_id=%d user_id=%d role=%q reason=not this institution's college", collegeID, v.UserID, v.Role)
	return ErrForbidden
}
