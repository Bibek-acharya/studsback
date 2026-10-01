// internal/auth/referral.go
//
// Where referral attribution is triggered, and why it is triggered from every
// user-creation path in this module rather than from one obvious place.
//
// ── the enumeration, and it is longer than four ───────────────────────────────
//
// docs/coin-system/02-architecture.md §10.1 lists four user-creation paths and
// they are four FUNCTIONS. Counting where a row is actually INSERTED gives six
// reachable sites, because VerifyOTP is not one path — it is three, dispatching on
// what the OTP store staged:
//
//	#  row created in      function                                    code arrives on
//	1  users              VerifyOTP (student branch)                 RegisterRequest
//	2  institution_users  VerifyOTP (institution branch)             RegisterRequest
//	3  scholarship_       VerifyOTP (provider branch)                RegisterRequest
//	provider_users
//	4  users              GoogleLoginOrRegister                      the Google callback
//	5  institution_users  InstitutionGoogleLoginOrRegister           the Google callback
//	6  scholarship_       ScholarshipProviderGoogleLoginOrRegister    the Google callback
//	provider_users
//
// So "a test per user-creation path" is six tests, not four. The four in §10.1 are
// the four functions; this file is the enumeration that closes the gap between the
// two, and TestUserCreationPathsAreAllAttributed below asserts the count so a
// future fifth INSERT into an account table fails the suite rather than silently
// becoming an unattributed signup.
//
// Two further sites create accounts and are DELIBERATELY not attributed:
//
//	CreateInstitution      (service.go:959) — a staff-created institutional
//	                       profile, reachable only from an authenticated
//	                       superadmin console. There is no invite link and no
//	                       code to capture, so there is nothing to attribute.
//	SuperadminRegister     (service.go:1764) — behind an access code, for the
//	                       platform's own staff. Crediting a referral for
//	                       recruiting an employee is a payout nobody agreed to.
//
// Both are listed here rather than left undiscovered because "a path not on the
// list is a bug" is the standard this file holds itself to, and an undocumented
// exception is indistinguishable from an oversight six months from now.
//
// ── the three tables, and what that cost ──────────────────────────────────────
//
// Rows 2, 3, 5 and 6 do NOT go into `users`. This module has three account models
// with independent id sequences — User, InstitutionUser, ScholarshipProviderUser —
// all AutoMigrate'd side by side in cmd/server/main.go. §10.1's
// `referred_user_id` with a UNIQUE on it is therefore under-specified: student id
// 7 and institution id 7 are different accounts, so a bare id is ambiguous as a
// key and a UNIQUE on it would refuse a legitimate attribution of one for the
// other. Every call site below therefore names the TABLE it created a row in.
// See internal/coins/referral_model.go for the schema side of this.
//
// ── why not hook auth.Repository.CreateUser ───────────────────────────────────
//
// The same two disqualifications profile_award.go gives for SaveUser, and they
// apply verbatim: the repository is the lowest layer of this module and hanging a
// coin-economy call off it inverts the dependency (internal/auth would import
// internal/coins), and it is INCOMPLETE — CreateUser, CreateInstitutionUser and
// CreateScholarshipProviderUser are three separate methods, so a chokepoint on any
// one of them misses the other two. A chokepoint that misses two thirds of the
// writers is worse than none, because it reads as handled.
//
// ── the direction of the dependency ───────────────────────────────────────────
//
// auth declares the port below and main wires the implementation. Neither module
// imports the other. The kind vocabulary is DUPLICATED here on purpose: internal/auth
// must be able to compile and run its tests with no coin economy present, and it
// gets there by naming three subjects it can create rather than importing three
// constants it cannot. The mapping is one switch, in cmd/server, and it is
// asserted there.
package auth

import (
	"context"
	"log"
)

// The referred-account kinds, in this module's own vocabulary.
//
// Duplicated rather than imported from internal/coins. See the package comment:
// internal/auth must compile with no coin economy configured, so it names what it
// can create. cmd/server maps these to the coin system's constants and
// TestReferralSubjectMappingIsExhaustiveInWiring asserts the two sets agree, so
// the duplication cannot drift into a silent mis-mapping where a new account table
// attributes as the wrong subject.
const (
	// ReferralSubjectUser is a row in `users`.
	ReferralSubjectUser = "user"
	// ReferralSubjectInstitution is a row in `institution_users`.
	ReferralSubjectInstitution = "institution"
	// ReferralSubjectProvider is a row in `scholarship_provider_users`.
	ReferralSubjectProvider = "provider"
)

// ReferralSubject is a newly created account and the code it arrived with.
//
// Code is the code AS PRESENTED. Nothing here normalises it: normalisation lives in
// internal/shared/utils and is called by the implementation, so every caller
// normalises identically and a second spelling of a code cannot exist.
type ReferralSubject struct {
	// Kind is one of the ReferralSubject* constants — which table.
	Kind string
	// ID is the new account's id in that table.
	ID uint
	// Code is the referral code the account arrived with, empty when it arrived
	// with none.
	Code string
	// Path records which creation path this was, for the reason in
	// internal/coins/referral_model.go: a fraud investigation needs to ask which
	// of the sites is under-counting, and after the row exists there is no other
	// way to ask.
	Path string
}

// ReferralAttribution is what the coin system did with a code.
//
// Its own type rather than the coin system's, for the reason
// profile_award.go gives: this module's callers must not reach into coin internals
// through the seam.
type ReferralAttribution struct {
	// Applied is true only when a NEW attribution row was written.
	Applied bool
	// Reason is the machine-readable code for every non-applied outcome, empty when
	// Applied. Matching it is how a test distinguishes "the signup carried no
	// code" from "the code was already claimed", which are very different bugs.
	Reason string
}

// ReferralAttributor is the coin economy's attribution, as this module sees it.
//
// One method, on purpose. It is idempotent, cheap, and safe to call from every
// creation path, so over-calling is the safe failure — and a narrow port means
// wiring a stub in a test is one line.
type ReferralAttributor interface {
	// ApplyReferral records that this account arrived carrying this code. It must
	// never block the signup that created the account, and must never return an
	// error for a policy outcome (an unknown code, a duplicate claim, a capped
	// referrer): every one of those is a successful signup. See applyAttribution.
	ApplyReferral(ctx context.Context, subject ReferralSubject) (ReferralAttribution, error)
}

// referralAttributorInstance is nil until main wires it. Every call site tolerates
// nil, which keeps this module's own tests — the seeder, and any deployment that
// has not configured a coin economy — working unchanged. The same shape and the
// same reason as profileAwarderInstance.
var referralAttributorInstance ReferralAttributor

// SetReferralAttributor wires attribution. Called once from main.
func SetReferralAttributor(a ReferralAttributor) { referralAttributorInstance = a }

// applyAttribution is the single funnel every user-creation path calls.
//
// It returns nothing and swallows nothing silently. A failure is logged and NOT
// propagated, for exactly the reason awardProfile does: the alternative is that a
// coin-system problem — a locked table, an unreadable config, a coin schema that
// has not been migrated yet — makes students unable to create an account. That is
// a far worse outcome than a missed referral, and the referral is recoverable:
// every subsequent call from any other creation path would attribute, and the code
// stays valid until the invitee uses it.
//
// The one asymmetry against awardProfile is deliberate. awardProfile is
// self-healing because the profile write repeats, many times, forever. A referral
// is captured ONCE, at signup, so a dropped attribution is a lost referral with no
// later opportunity. That does not change the error handling — failing the signup
// over a coin-system fault is still worse, and an account that exists with an
// unattributed invite is strictly better than no account — but it is why the log
// line names the path: the four paths are the ones worth alerting on, and a
// silent attribution failure is otherwise indistinguishable from a signup that
// legitimately carried no code.
//
// ctx is the caller's context so a cancelled request does not leave a
// half-attributed signup, and context.Background() for the paths with no request
// context in scope (the Google callbacks run after a redirect, so the request that
// carried the code has long since been answered — which is why those paths get the
// code as an argument instead).
func (s *Service) applyAttribution(ctx context.Context, subject ReferralSubject) {
	if referralAttributorInstance == nil {
		return
	}
	// The overwhelming majority of signups carry no code, and an unattributed
	// attribution is the correct outcome for them. Returning before the call keeps
	// a coin-system query off the hot path for every signup that never came from
	// an invite — the same reasoning profile_award.go gives for not calling the
	// award on Login.
	if subject.Code == "" {
		return
	}
	if subject.ID == 0 {
		log.Printf("auth: referral attribution on path %s has no account id; nothing was recorded",
			subject.Path)
		return
	}
	res, err := referralAttributorInstance.ApplyReferral(ctx, subject)
	if err != nil {
		log.Printf("auth: referral attribution on path %s did not complete for %s %d: %v",
			subject.Path, subject.Kind, subject.ID, err)
		return
	}
	if !res.Applied {
		log.Printf("auth: referral attribution on path %s for %s %d was not applied (%s)",
			subject.Path, subject.Kind, subject.ID, res.Reason)
	}
}
