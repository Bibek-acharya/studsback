package search

import (
	"errors"
	"log"
	"strings"

	"studsphere/backend/internal/shared/httpx"

	"github.com/gin-gonic/gin"
)

// WHO MAY REINDEX THE EMBEDDINGS — the fourth module audited behind the shared
// roleMW, and the one whose real exposure was worse than the flag said.
//
// WHAT THE ROUTE ACTUALLY DOES. Reindex is not a rebuild of a search index in the
// usual sense. With ?force=true it calls embedding.ReindexAllForce, and for each
// of the seventeen tables in embeddingTablesList that has an embedding column it
// runs, unparameterised:
//
//	UPDATE <table> SET embedding = NULL, embedded_at = NULL WHERE embedding IS NOT NULL
//
// That is a mass write of every embedding column on the platform. The table list
// is not a tenant's own data — it spans colleges, courses, exams, scholarships,
// universities, and also institution_users, institution_news, institution_blogs,
// provider_news, provider_blogs and admission_pages. So one request nulls the
// derived vectors of every tenant on the platform and then queues a full retrain
// of all of them, in a background goroutine, at embedding-provider cost. It is
// destructive, global, and cheap to ask for.
//
// THE HOLE IS BIGGER THAN roleMW. Two copies of this handler are mounted. The
// admin one sits behind roleMW, which admits institution and
// scholarship_provider. The other — v1.POST("/search/reindex") — was mounted on
// the bare /api/v1 group with no middleware whatsoever: not authMW, not roleMW,
// nothing. Reindex is a public, unauthenticated, destructive global operation.
// That copy is closed here as well, and it is the reason this module is not
// simply "the same fix as the others".
//
// THE RULE, and why there is no tenant-scoped version. Platform-admin only, and
// not because the route is named /admin. There is no field on any of the seventeen
// tables that could scope the operation, because the operation is not scoped to
// anything: it is defined as a sweep over all of them. A tenant-scoped reindex
// would have to be a different operation against a different table list, and
// inventing one here would be inventing a product feature. So the honest rule is
// the platform operator, and — unlike the college module — there is no non-admin
// principal left over to break: the only caller in the repo is the superadmin
// dashboard's SettingsSection.tsx.
//
// THE CHECK IS FIRST, BEFORE embedding.IsEnabled(). IsEnabled is a
// configuration flag, and the handler's disabled-branch answers 202 with a
// message naming it. Refusing after that check would let any authenticated tenant
// read the platform's embedding configuration by calling the route and branching
// on the status code, and would make "disabled" and "refused" indistinguishable in
// production. So authorizeReindex runs first and the flag is never disclosed to a
// caller who has no claim on it.
//
// 403 rather than 404: there is no record here, so there is no existence question
// to hide and nothing an enumerator could learn. A 403 also says the truth to an
// operator — this failed on authorization, not because a reindex was missing.

// ErrForbidden is what a reindex call from a non-admin returns.
var ErrForbidden = errors.New("insufficient permissions")

// PlatformAdminRoles are the roles that run the platform itself, matching the
// two aliases the rest of the codebase treats as the same operator
// ("superadmin" in auth.Service, "super_admin" in cmd/server/main.go's roleMW).
//
// Exported because cmd/server/main.go builds the reindex gate from this list, so
// the middleware at the edge and the check here cannot drift apart.
func PlatformAdminRoles() []string {
	return []string{"admin", "super_admin", "superadmin"}
}

// Viewer is the authenticated principal behind a reindex request.
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
// zero Viewer when nothing resolved — no role, therefore not an admin. For the
// formerly-public copy of this route that is the load-bearing case: a request with
// no token at all now produces a Viewer with an empty role and is refused by the
// service even if a caller ever mounted this handler without the middleware.
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

// authorizeReindex refuses a reindex to anyone who is not a platform operator.
//
// The service is the authority, so a caller who passes the wide roleMW — or who
// mounts this handler with no middleware at all — still cannot trigger the sweep.
// The caller, role and force flag go to the log, because a refusal on an
// operation this destructive is worth being able to investigate after the fact.
func (s *SearchService) authorizeReindex(v Viewer, force bool) error {
	if v.IsPlatformAdmin() {
		return nil
	}
	log.Printf("search: denied reindex user_id=%d role=%q force=%t reason=reindex is platform-admin only", v.UserID, v.Role, force)
	return ErrForbidden
}
