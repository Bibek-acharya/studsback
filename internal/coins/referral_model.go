// internal/coins/referral_model.go
//
// user_referral: who referred whom, and everything the fraud controls and the
// next slice's state machine need to hang off that fact.
//
// ── what this table is, and what it is not ───────────────────────────────────
//
// It is the ATTRIBUTION record: a code was accepted from this account, and here
// is who gets the credit. It is created the moment a signup carries a code, before
// any qualification, before any coins.
//
// It is deliberately not an award ledger. reward_grant answers "what has this user
// been paid for"; user_referral answers "what relationships exist". They are
// joined by the next slice through RefType=RefUserReferral, not by sharing
// columns, because a relationship exists long before it is paid for and deleting a
// failed attribution would destroy the record that somebody tried.
//
// ── why referred_kind exists, which is the finding ───────────────────────────
//
// docs/coin-system/02-architecture.md §10.1 specifies:
//
//	referred_user_id bigint NOT NULL,
//	CONSTRAINT user_referral_uniq UNIQUE (referred_user_id)
//
// That is only correct if every user-creation path writes to `users`. It does not.
// This codebase has THREE account tables, confirmed by AutoMigrate in
// cmd/server/main.go:204-207 and by the models in internal/auth/model.go:
//
//	users                      auth.User
//	institution_users          auth.InstitutionUser
//	scholarship_provider_users auth.ScholarshipProviderUser
//
// and two of the four user-creation paths in §10.1 write to the latter two.
// InstitutionGoogleLoginOrRegister creates an InstitutionUser;
// ScholarshipProviderGoogleLoginOrRegister creates a ScholarshipProviderUser. They
// have independent id sequences, so institution id 7 and student id 7 are
// different accounts.
//
// A User-keyed attribution therefore silently misses both of those classes: a
// student who invites a college to join, or a scholarship provider, records
// nothing at all. That is an under-counted referral AND an open fraud route —
// precisely the failure the four-path tests exist to catch — except that no amount
// of testing the student path would reveal it, because the student path works.
// The id is therefore namespaced by the table it came from, and the uniqueness is
// on the PAIR. See ReferredKinds.
//
// Note the direction of the fix: the REferrer is always a student (only `users`
// holds referral codes and coin balances), so referrer_user_id needs no namespace.
// Only the referred side does.
//
// ── why the states are declared but not reached ───────────────────────────────
//
// Qualification, the 7-day reserved hold, the release and the clawback are the
// next slice. The states and their CHECK constraint are created now anyway,
// because a CHECK that gains a value later is an ALTER against live rows, and
// because the shape of a state machine is worth reviewing before it is written
// into. This slice writes exactly one of them: 'pending'.
package coins

import "time"

// UserReferral is one accepted referral: a code arrived on a signup and was
// attributed.
//
// Its existence is the attribution. Nothing else records it, so the uniqueness
// constraints below are what make "one inviter per invitee" a property of the
// database rather than of a code path that can be bypassed.
type UserReferral struct {
	ID uint `gorm:"primarykey" json:"id"`

	// ReferrerUserID is the student whose code was used. NOT NULL, and no
	// foreign key — the same rule every coin table documents: a coin table that
	// cannot be created until another module's table exists couples two
	// deployments with no other coupling, and a user delete must not cascade into
	// an audit record.
	ReferrerUserID uint `gorm:"not null;index" json:"referrer_user_id"`

	// ReferredKind and ReferredUserID together identify the referred account.
	//
	// They are unique as a PAIR and not referred_user_id alone. See the file
	// header: three tables, three id sequences, and a UNIQUE on the bare id would
	// both refuse a legitimate attribution (student 7 already attributed, so
	// institution 7 can never be) and permit a double-claim across tables.
	ReferredKind   string `gorm:"type:varchar(24);not null;uniqueIndex:user_referral_referred_uniq,priority:1" json:"referred_kind"`
	ReferredUserID uint   `gorm:"not null;uniqueIndex:user_referral_referred_uniq,priority:2" json:"referred_user_id"`

	// ReferralCode is the code AS PRESENTED, normalised. Stored rather than
	// re-derived from users.referral_code so the row answers "what did this
	// person actually type" after the referrer's code has been rotated, and so a
	// referral that arrived through a code since deleted is still legible.
	//
	// Length 32 is ReferralCodeLength (10) with headroom, because a code that
	// failed normalisation — every character stripped — still gets recorded here
	// rather than rejected, and an unbounded text column on a table an attacker
	// can write to is an unbounded storage cost.
	ReferralCode string `gorm:"type:varchar(32);not null" json:"referral_code"`

	// Status is the state machine. See ReferralStatuses.
	Status string `gorm:"type:varchar(24);not null;default:pending" json:"status"`

	// AwardedCoins is what was actually paid, snapshotted for the same reason
	// reward_grant.amount is: an operator re-prising referral_referrer from 60 to
	// 30 must not retroactively change what this row says was paid.
	AwardedCoins int64 `gorm:"not null;default:0" json:"awarded_coins"`

	// HoldJournalID and GrantJournalID are the ledger's own ids. Nullable for the
	// same reason reward_grant.journal_id is: the claim has to exist before the
	// journal does, or a retry would double-pay. No foreign key, matching every
	// other coin table's journal reference — coin_journal is append-only under
	// trigger, so a foreign key could never fire.
	HoldJournalID  *string `gorm:"type:uuid;index" json:"hold_journal_id,omitempty"`
	GrantJournalID *string `gorm:"type:uuid;index" json:"grant_journal_id,omitempty"`

	// CapPeriod and CapSlot record which of the referrer's monthly cap slots this
	// referral consumed. 'YYYY-MM' in UTC and 1..monthly_cap. Written by the
	// qualification slice; the columns exist now so the cap accounting is part of
	// the reviewed shape rather than an ALTER.
	CapPeriod *string `gorm:"type:char(7);index" json:"cap_period,omitempty"`
	CapSlot   *int    `json:"cap_slot,omitempty"`

	QualifiedAt  *time.Time `json:"qualified_at,omitempty"`
	ReleasedAt   *time.Time `json:"released_at,omitempty"`
	ClawedBackAt *time.Time `json:"clawed_back_at,omitempty"`
	// ClawbackReason is NOT NULL with an empty default rather than nullable,
	// because a clawback with no recorded reason is the thing a ledger exists to
	// make impossible, and a nullable reason is how one gets written.
	ClawbackReason string `gorm:"type:varchar(255);not null;default:''" json:"clawback_reason,omitempty"`

	// PhoneHash and DeviceHash are keyed HMACs, never plaintext. 05-economy-and-fraud.md
	// §2.4 calls this table "explicitly a fraud-matching table", and a table whose
	// entire purpose is to be joined against is the most attractive target in the
	// database if it holds raw phone numbers. The hash is produced by the caller;
	// the key never enters this package.
	PhoneHash  []byte `gorm:"type:bytea" json:"-"`
	DeviceHash []byte `gorm:"type:bytea" json:"-"`

	PhoneVerifiedAt *time.Time `json:"phone_verified_at,omitempty"`

	// SourcePath records WHICH user-creation path accepted the code. Not a fraud
	// control — the four paths are equally trusted, since all four are reached by
	// an unauthenticated caller and equally in need of attribution. It is here
	// because the four-path tests and the fraud investigation both need to answer
	// "which of the four paths is under-counting", and there is no other way to
	// ask once the account exists.
	SourcePath string `gorm:"type:varchar(32);not null;default:''" json:"source_path"`

	CreatedAt time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

// ReferralPending is the only state this slice writes: the code was accepted and
// recorded, and the invitee has not yet qualified.
const ReferralPending = "pending"

// ReferralQualified means the invitee met the qualification bar and coins were
// RESERVED. The next slice writes it.
const ReferralQualified = "qualified"

// ReferralClawedBack means reserved coins were taken back on a fraud finding. The
// next slice writes it.
const ReferralClawedBack = "clawedback"

// ReferralRejected means the invitee never qualified, or failed a fraud check
// before any coins were reserved. The next slice writes it.
//
// Distinct from clawedback on purpose: a rejection never paid anything, a clawback
// takes back something that was. Collapsing them into one "failed" state would
// lose the answer to "was this student ever credited for this referral", which is
// the first question a support or fraud review asks.
const ReferralRejected = "rejected"

// ReferralStatuses is the closed set the CHECK constraint enforces.
var ReferralStatuses = []string{
	ReferralPending,
	ReferralQualified,
	ReferralClawedBack,
	ReferralRejected,
}

// ReferredKinds are the three tables an account can be created in. See the file
// header for why this is a finding rather than a choice.
const (
	// SubjectUser is a row in `users`.
	SubjectUser = "user"
	// SubjectInstitution is a row in `institution_users`.
	SubjectInstitution = "institution"
	// SubjectProvider is a row in `scholarship_provider_users`.
	SubjectProvider = "provider"
)

// ReferredKinds is the closed set the CHECK constraint enforces.
var ReferredKinds = []string{SubjectUser, SubjectInstitution, SubjectProvider}

// TableName pins the singular name, matching the DDL in 02-architecture.md §10.1
// and the rest of this package. GORM's pluraliser would produce
// "user_referrals".
func (UserReferral) TableName() string { return "user_referral" }

// UserReferralModels is the AutoMigrate slice for the referral table. Separate
// from LedgerModels and from RewardGrantModels because it has no postings and no
// account, and putting it on the reconciliation path would suggest otherwise.
var UserReferralModels = []any{&UserReferral{}}
