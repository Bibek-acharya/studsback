// internal/coins/unlock_model.go
//
// Entitlements, as opposed to money. This file is 02-architecture.md §5.
//
// ── why these tables are not in the ledger ────────────────────────────────────
//
// A balance answers "how much". It cannot answer "may this student open this
// document", and it cannot answer that without a second source of truth that
// will eventually disagree with the first. An unlock is not a coin movement: it
// is a fact about a relationship between a user and a resource, and the three
// properties that make it worth its own tables are all properties a double-entry
// posting cannot have.
//
//  1. A free unlock moves no coins at all. Putting one in the ledger would mean
//     inventing a zero-value transfer so that the entitlement could live in the
//     journal — and CHECK (amount <> 0) on coin_posting, correctly, refuses
//     exactly that. The shape of the problem is a rule about entitlement shape
//     ("three documents, one video, one mock test, ever"), not a quantity of
//     currency.
//
//  2. Revocation is not a reversal. Clawing back coins is a reversal, and the
//     ledger does that beautifully. Un-revoking access to a resource moves no
//     money, so it needs no journal, but it absolutely needs a record: "was
//     this revoked, when, and by whom" is a support question and a fraud
//     question, and a DELETE answers neither.
//
//  3. The allowance is a cap on a COUNT, and a count is not a balance. Nothing
//     in the ledger can express "at most three of these, per user" because the
//     ledger only knows about movement.
//
// ── the one constraint that earns this file its keep ─────────────────────────
//
// UNIQUE (user_id, resource_type, resource_id). It is the single most valuable
// constraint in the system, and it is worth being blunt about why: a fraud ring
// that farms a thousand fraudulent accounts can still only ever unlock the
// resources it actually pays for. The worst case is capped at "they got the
// three starter unlocks each" rather than "they drained the catalogue", and
// that cap is structural — it holds independently of balance, independently of
// config, and independently of whether the Go code that writes these rows has a
// bug. One index buys that. See EnsureEntitlementIndexes.
//
// ── the counter that is deliberately absent ──────────────────────────────────
//
// There is no `used` column on user_free_allowance, and adding one is the most
// likely way to break this. It is derived:
//
//	SELECT count(*) FROM resource_unlock
//	 WHERE user_id = ? AND source = 'ALLOWANCE'
//	   AND resource_type = ? AND revoked_at IS NULL
//
// A stored counter duplicates the truth, and the two disagree the first time an
// unlock is revoked — the revocation has to decrement the counter or the student
// loses an entitlement they still hold, and forgetting that is silent. Deriving
// it costs one indexed count (resource_unlock_allowance_open_idx is exactly
// this query's index) and cannot drift.
package coins

import "time"

// Unlockable classes. These are compile-time aliases of the ledger's ref_type
// constants rather than new string literals, because the class that selects the
// PRICE (ledger.go spendPrice) and the class that selects the entitlement are
// the same vocabulary, and two sets of constants for one vocabulary is how they
// end up disagreeing. See spendPrice and 03-api-contract.md §2.3.
const (
	ResourceTypeStudyResource = RefStudyResource
	ResourceTypeVideo         = RefVideo
	ResourceTypeMockTest      = RefMockTest
)

// ResourceTypes is the closed set of unlockable classes, in the order the API
// contract lists them. The CHECK constraint on resource_unlock.resource_type
// restates these three values as SQL; that duplication is deliberate and
// unavoidable — a CHECK is a string in the database and a Go var is a slice in
// the binary — and TestEnsureEntitlementIndexesRestatesTheResourceTypeCheck is
// what keeps them from drifting.
var ResourceTypes = []string{ResourceTypeStudyResource, ResourceTypeVideo, ResourceTypeMockTest}

// Unlock sources. COINS means the ledger already settled a spend and this row
// is the entitlement half of that event. ALLOWANCE means a starter entitlement
// burned and no coins moved at all.
const (
	UnlockSourceCoins     = "COINS"
	UnlockSourceAllowance = "ALLOWANCE"
)

// UnlockSources is the closed set of sources, in the same spirit as
// ResourceTypes.
var UnlockSources = []string{UnlockSourceCoins, UnlockSourceAllowance}

// ResourceUnlock is one row per (user, resource_type, resource_id). The UNIQUE on
// those three columns is the fraud control described in the file header.
//
// The other fields are all consequences of a purchase having happened, not of a
// user having a balance:
//
//   - JournalID is NULL when Source is ALLOWANCE and NOT NULL when Source is
//     COINS, enforced by chk_resource_unlock_source_funding so the two cannot
//     disagree. An allowance unlock pointing at a journal claims coins were
//     charged for it; a coin unlock with no journal is money that moved and left
//     no trace of what for. Both are unrecoverable audit gaps, so both are
//     refused at the database rather than at a call site that might be bypassed.
//
//   - CoinsPaid is a SNAPSHOT and is never re-read from config. Prices change: a
//     document that costs 40 today may cost 60 next month, and a user who bought
//     at 40 keeps that unlock at 40. Re-deriving the price at query time
//     retroactively re-prices history and turns an entitlement into an argument
//     (02-architecture.md §5). It is 0 for every allowance unlock, which is what
//     makes "the three free ones cost nothing" a fact about the rows rather than
//     a fact about today's config.
//
//   - RevokedAt and RevokeReason are a reversal, not a delete. Nothing in this
//     table is ever removed: a deleted row cannot answer "was this ever granted",
//     and that question is asked during fraud review.
type ResourceUnlock struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	UserID       uint   `gorm:"not null;index" json:"user_id"`
	ResourceType string `gorm:"type:varchar(32);not null" json:"resource_type"`
	ResourceID   uint64 `gorm:"not null" json:"resource_id"`
	// JournalID has a foreign key to coin_journal(id) when it is not NULL. That
	// FK is one of the two ways a COINS unlock proves it is real: it must name a
	// journal that actually exists.
	JournalID    *string    `gorm:"type:uuid" json:"journal_id,omitempty"`
	Source       string     `gorm:"type:varchar(16);not null" json:"source"`
	CoinsPaid    int64      `gorm:"not null;default:0" json:"coins_paid"`
	UnlockedAt   time.Time  `gorm:"not null;default:now()" json:"unlocked_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokeReason *string    `gorm:"type:varchar(255)" json:"revoke_reason,omitempty"`
}

// UserFreeAllowance is one row per user. It holds the entitlement QUANTITIES
// and the expiry, and nothing else — in particular it holds no `used` counter,
// for the reason in the file header.
//
// The quantities are snapshots of the economy config at grant time, for the same
// reason CoinsPaid is: a student who was granted three document unlocks was
// granted three, and an admin lowering the default to two afterwards must not
// retroactively take one away from everybody. Re-granting is not "raise it to
// the current default" either, which is why EnsureAllowance never rewrites an
// existing row.
type UserFreeAllowance struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	UserID          uint      `gorm:"not null;index" json:"user_id"`
	GrantedAt       time.Time `gorm:"not null;default:now()" json:"granted_at"`
	ExpiresAt       time.Time `gorm:"not null" json:"expires_at"`
	DocumentUnlocks int64     `gorm:"not null;default:0" json:"document_unlocks"`
	VideoUnlocks    int64     `gorm:"not null;default:0" json:"video_unlocks"`
	MockTestUnlocks int64     `gorm:"not null;default:0" json:"mock_test_unlocks"`
}

func (ResourceUnlock) TableName() string    { return "resource_unlock" }
func (UserFreeAllowance) TableName() string { return "user_free_allowance" }

// EntitlementModels is what cmd/server/main.go adds to the AutoMigrate list,
// alongside the individual &coins.ResourceUnlock{} / &coins.UserFreeAllowance{}
// entries. AutoMigrate creates the tables and the plain non-unique indexes;
// every CHECK constraint, both UNIQUE constraints, the foreign key and the
// partial index that makes the derived `used` count cheap come from
// EnsureEntitlementIndexes, which main.go must call in the same non-fatal —
// rather, same !config.IsSQLite — block as coins.EnsurePostgresIndexes.
var EntitlementModels = []any{
	&ResourceUnlock{},
	&UserFreeAllowance{},
}
