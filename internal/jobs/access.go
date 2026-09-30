package jobs

import (
	"errors"
	"log"
	"strings"

	"studsphere/backend/internal/shared/httpx"

	"github.com/gin-gonic/gin"
)

// WHO OWNS A JOB APPLICATION — the finding that shaped this file.
//
// The brief for this fix was to scope every applicant route to "the tenant that
// owns the job". That turns out not to be expressible against the model as it
// stands, and the reason is worth writing down rather than papering over.
//
// Job has no tenant column. Not institution_id, not employer_id, not created_by
// — there is nothing on the row that names the organisation behind the posting,
// and CreateJob is never handed the caller's identity, so even a posting created
// by an admin records no owner. The jobs table is a single first-party catalogue
// behind the platform's own /careers page. An institution account has no job of
// its own to be the owner of, so "the posting organisation may read applicants to
// its job" resolves, for this schema, to "the platform's admins may read any
// applicant" — which is exactly the set the superadmin dashboard already calls.
//
// That also settles the scholarship_provider question without appealing to the
// route name: a scholarship provider owns ProviderScholarship rows in
// internal/scholarshipprovider, a different table with a different owner column
// and its own scoped queries (GetApplicationByIDAndProvider). It is never the
// owner of a Job. Granting it applicant reads here would have been granting on
// the strength of a URL path, which is the mistake that caused the original leak.
//
// The leak itself: these routes were mounted behind the shared roleMW, which
// admits "institution" and "scholarship_provider" as well as the admins, and no
// handler below did an ownership check of any kind. Any paying customer could
// therefore read every applicant's resume and cover letter on the platform by
// walking sequential ids, including applicants who applied to other tenants'
// postings. Enforcing here rather than in the handlers means a route added later
// inherits the check instead of forgetting it.

// ErrApplicationNotFound is the single answer every applicant-scoped endpoint
// gives to a caller who is not entitled to the record: an id that never existed,
// an id that belongs to somebody else, and — on the write routes — an id the
// caller could legitimately read but not modify.
//
// One sentinel for all three, deliberately. See the 404-vs-403 note on the
// refuse helpers below.
var ErrApplicationNotFound = errors.New("application not found")

// PlatformAdminRoles are the roles that run the platform itself. They match the
// two aliases the rest of the codebase treats as the same operator
// ("superadmin" in auth.Service, "super_admin" in cmd/server/main.go's roleMW).
//
// Exported because cmd/server/main.go builds the job-catalogue gate from this
// list — middleware.RequireRole(PlatformAdminRoles()...) — so the middleware at
// the edge and the checks in this file cannot drift apart. The service is the
// authority either way; see authorizeJobAdmin.
func PlatformAdminRoles() []string {
	return []string{"admin", "super_admin", "superadmin"}
}

// Viewer is the authenticated principal behind an applicant-scoped request.
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
// zero Viewer when nothing resolved, which is a Viewer with no role and no id —
// therefore not an admin, and matching no application's submitter. A missing
// context therefore denies by default rather than by accident.
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

// OwnedBy reports whether this application was submitted by the given account.
//
// The only applicant-side rule. Everything else in this file is about denying.
func (a *JobApplication) OwnedBy(userID uint) bool {
	return userID != 0 && a.SubmittedBy() == userID
}

// canRead reports whether the viewer may read this application at all.
func (a *JobApplication) canRead(v Viewer) bool {
	return v.IsPlatformAdmin() || a.OwnedBy(v.UserID)
}

// refuse records why an applicant-scoped request was denied and returns the
// error the handler turns into a response.
//
// The log line is the whole answer to the "a bare 404 makes debugging harder"
// objection. The 403 alternative was rejected on purpose: these ids are
// sequential and the attack being closed here IS the walk over them, so a 403
// would distinguish "this exists but is not yours" from "this never existed" and
// hand the caller a map of the platform's applicant table, plus confirmation
// that a specific named person had applied somewhere. 404 for both makes
// enumeration return nothing to learn. The denial is still recorded server-side
// with the id, the caller and the reason, so the debugging that a 403 would have
// served is served by the log rather than by the response.
func refuse(subject string, id uint, v Viewer, reason string) error {
	log.Printf("jobs: denied %s access id=%d user_id=%d role=%q reason=%s", subject, id, v.UserID, v.Role, reason)
	return ErrApplicationNotFound
}

// authorizeApplication loads an application for a read, refusing anyone the
// model does not entitle.
//
// Not-found and not-yours are collapsed into ErrApplicationNotFound, which is
// also why the lookup still runs for a caller who will be refused: refusing
// before the query would make "denied" fast and observable, and refusing after
// keeps the response identical either way.
func (s *Service) authorizeApplicationRead(id uint, v Viewer) (*JobApplication, error) {
	app, err := s.repo.FindApplicationByID(id)
	if err != nil {
		return nil, ErrApplicationNotFound
	}
	if !app.canRead(v) {
		return nil, refuse("application", id, v, "not an admin and not the applicant")
	}
	return app, nil
}

// authorizeApplicationWrite loads an application for a mutation.
//
// Admin only, with no applicant exception. The three write routes set the
// pipeline's shortlist/reject status, edit the recruiter's internal notes, and
// send outbound email on the platform's behalf — none of which an applicant may
// do to their own record, and the last of which is the sharpest edge here: it
// lets a non-owner use the platform as a mail relay to any applicant.
func (s *Service) authorizeApplicationWrite(id uint, v Viewer) (*JobApplication, error) {
	if !v.IsPlatformAdmin() {
		// Refuse without loading. This is a role refusal, not an ownership
		// question, so there is no existence information to hide from it.
		return nil, refuse("application", id, v, "write routes are admin only")
	}
	app, err := s.repo.FindApplicationByID(id)
	if err != nil {
		return nil, ErrApplicationNotFound
	}
	return app, nil
}

// authorizeJobRead guards the routes that return applicant PII for a whole job
// rather than for one application.
//
// GET /superadmin/jobs/:id/applicants hands back full name, email and phone for
// every applicant to that posting, so it is the same disclosure as the resume
// route and one step earlier in an enumeration: a single call replaces the walk.
// It was behind the same broad roleMW and was just as unowned.
func (s *Service) authorizeJobRead(jobID uint, v Viewer) error {
	if !v.IsPlatformAdmin() {
		return refuse("applicant listing for job", jobID, v, "admin only")
	}
	return nil
}

// WHO MAY EDIT THE JOB CATALOGUE — the second half of this module, and the part
// the applicant fix deliberately left alone.
//
// These five routes (list, read, create, update, delete) were mounted behind the
// shared roleMW in cmd/server/main.go, which admits "institution",
// "scholarship_provider" and "scholarship_provider_subuser" alongside the
// admins, and none of the handlers or service methods below checked anything.
// An ordinary paying customer could therefore create postings on the platform's
// own /careers page, rewrite an existing one, and delete one.
//
// Delete is the serious one. DeleteJob deletes every JobApplication row for the
// posting and calls storage.DeleteObject on each applicant's resume and cover
// letter — irreversible destruction of other people's personal documents, by an
// account with no relationship to any of them. That is worse than the disclosure
// fixed above: a read leaks, a delete cannot be undone.
//
// The rule is platform-admin only, and it follows from the model rather than
// from a route name:
//
//   - Job has no owner column. Not institution_id, not employer_id, not
//     created_by, and CreateJob is never handed the caller's identity, so not
//     even an admin-created posting records who owns it. The catalogue behind
//     /careers is first-party, the same conclusion access.go reaches for
//     applicant reads.
//   - A scholarship provider owns ProviderScholarship rows in
//     internal/scholarshipprovider — a different table, already scoped by
//     GetApplicationByIDAndProvider. It is never the owner of a Job, so
//     granting it catalogue writes would again be granting on the strength of a
//     URL path.
//   - An institution account has no postings of its own here, so "the employer
//     that posted the job may edit its job" resolves, in this schema, to "the
//     platform's admins may edit any job". There is no non-admin caller left
//     over the way there was for applicant self-reads.
//
// So unlike the applicant group there is no principal that a broad role gate
// would wrongly exclude, which is why this one is a gate at all rather than a
// service-only check: the middleware answers the role question before the
// handler is reached and before the request body is parsed, and the service
// repeats the check so that widening the gate at the call site cannot re-open
// this. Both read the same list (PlatformAdminRoles) and both answer 403, so
// there is one rule and one answer for it.

// ErrJobForbidden is what a job-catalogue call from a non-admin returns.
//
// 403 here, not the 404 the applicant routes use, and the difference is
// deliberate rather than stylistic. The applicant routes return 404 because the
// record may not be the caller's and a 403 would confirm that a given id exists
// — an existence oracle over sequential applicant ids. Job ids carry no such
// secret: /careers/:id serves a published posting to anyone, and
// /api/v1/careers lists them all. A caller refused here learns only their own
// role, which they already know. Reporting it as "insufficient permissions"
// also tells an operator the truth: the request failed on authorization, it did
// not fail because a job was missing. A 403 is also what the gate at the route
// returns for the same refusal, and one rule should not answer two ways.
var ErrJobForbidden = errors.New("insufficient permissions")

// authorizeJobAdmin refuses a catalogue call to anyone who is not a platform
// operator, and is the authority for all five routes.
//
// Refused before any lookup, and without consulting the repository: this is a
// role question, so there is no existence information to hide and nothing to
// gain by touching the database. The reason is logged with the caller and role
// so a 403 in production is still debuggable — the same reasoning that moved
// the applicant routes' diagnostics into the log.
func (s *Service) authorizeJobAdmin(v Viewer, action string) error {
	if v.IsPlatformAdmin() {
		return nil
	}
	log.Printf("jobs: denied %s job user_id=%d role=%q reason=job catalogue is platform-admin only", action, v.UserID, v.Role)
	return ErrJobForbidden
}
