// internal/coins/reward_grant_model.go
//
// reward_grant is the once-per-user-per-award ledger of what has been paid out.
// It exists because the profile award is a LADDER that can be re-entered, and a
// ladder cannot be policed by the absolute value of a recomputed percentage.
//
// The reason it is a table and not a column is worth stating, because the
// obvious alternative is worse. A "profile_paid_up_to int" column on the user
// would have to be incremented by whoever grants, which means a read-modify-write
// on a number that five different code paths want to move. Two concurrent profile
// saves both read 2, both write 3, and the student is short one instalment with no
// record that it happened. The UNIQUE (user_id, award_code) below cannot be
// miscounted that way: it is the constraint that does the work, and a bug in this
// package can at worst fail to award, never award twice.
//
// Design notes, including the two places I departed from the DDL sketched in
// docs/coin-system/02-architecture.md §6:
//
//   - journal_id is NULLABLE, where the sketch said NOT NULL REFERENCES. The
//     sketch's shape is unworkable for this award. The row has to be CLAIMED
//     before the journal exists — claiming first is what makes the grant
//     exactly-once, because a no-row-returned from the claim insert means another
//     writer already has this step and must skip the grant entirely. So the row
//     is inserted without a journal id and the SAME transaction fills it in
//     immediately afterwards, from the Grant it just performed. The nullability
//     is therefore only observable mid-transaction; a committed row always
//     carries its journal. That is a deliberate trade of a NOT NULL constraint
//     for a correct once-only guarantee, and the constraint would have bought
//     nothing here anyway (see the FK note below).
//
//   - there is NO FOREIGN KEY to coin_journal, deliberately matching
//     coin_posting.journal_id and coin_lot.journal_id, which are plain uuid
//     columns with indexes and no FK. coin_journal is append-only under trigger
//     (internal/coins/ensure_indexes.go), so a journal row cannot be deleted and
//     an FK could never fire. It would only add a boot-order coupling between two
//     tables AutoMigrate creates in one list, which is exactly the kind of
//     coupling that turns into a migration incident.
//
// amount is a SNAPSHOT of what this step actually paid, not a pointer into
// config. If an operator later changes awards.profile_instalment from 5 to 10,
// this row still records that this student was paid 5, which is the only way the
// 25-coin ceiling stays explainable after a re-pricing.
package coins

import (
	"time"
)

// RewardGrant is one award step that has been paid to one user.
//
// The row's existence IS the award. Nothing else records it, so
// (user_id, award_code) being unique is what makes "at most once per user per
// award type" a database guarantee rather than a service-layer check that a bug
// can bypass — which is the whole reason this table is not a column.
type RewardGrant struct {
	ID uint `gorm:"primarykey" json:"id"`
	// UserID is the student. Deliberately not a GORM association: internal/coins
	// never learns the shape of a user, and an association here would import
	// internal/auth into the ledger.
	// UserID carries the uniqueIndex tag as well as AwardCode, and that is not
	// optional. GORM builds a COMPOSITE unique index only when every column in it
	// names the same index; tagging one column produces a single-column index and
	// leaves the ON CONFLICT (user_id, award_code) with nothing to match — which
	// Postgres reports as 42P10 at runtime, not at migration time.
	//
	// That is the exact failure documented at internal/notification/ensure_indexes.go,
	// where a fresh-boot server rejected every preferences PUT with 42P10 ON
	// CONFLICT without a matching constraint. It cost a production incident there,
	// so here it is prevented three ways: both columns tagged, the constraint
	// asserted by a schema test, and the claim insert's RowsAffected==0 treated as
	// the "already awarded" signal rather than an error being swallowed.
	UserID uint `gorm:"not null;index;uniqueIndex:reward_grant_uniq,priority:1" json:"user_id"`
	// AwardCode is the step identity. For this slice it is
	// ProfileStepCodePrefix + the step number, e.g. "PROFILE_STEP:3". The
	// referral phase appends its own key (per 02-architecture.md §10.1,
	// 'REFERRAL_BONUS:' || referral_id), which is why the column is text and not
	// an enum over this slice's values.
	AwardCode string `gorm:"type:varchar(128);not null;uniqueIndex:reward_grant_uniq,priority:2" json:"award_code"`
	// Amount is coins paid for THIS step, snapshotted at grant time.
	Amount int64 `gorm:"not null" json:"amount"`
	// JournalID links the award to the money. See the file header for why it is
	// nullable and why there is no FK.
	JournalID *string `gorm:"type:uuid;index" json:"journal_id,omitempty"`
	// GrantedAt is when the step was awarded, not when the profile reached the
	// threshold. Those differ by however long the student took to finish the next
	// field, and the ladder reads far better as "you got step 3 on the 4th" than
	// as a burst of timestamps on one save.
	GrantedAt time.Time `gorm:"not null" json:"granted_at"`
}

// TableName pins the singular name, matching the rest of this package and the DDL
// in 02-architecture.md §6. GORM's default pluraliser would produce
// "reward_grants".
func (RewardGrant) TableName() string { return "reward_grant" }

// RewardGrantModels is the AutoMigrate slice for the award ledger. It is kept
// separate from LedgerModels because reward_grant is not part of double entry —
// it has no postings and no account — and lumping it in with the five ledger
// tables would suggest otherwise to the next reader, and would put an award table
// on the reconciliation path (internal/coins/reconcile.go) that has no business
// walking it.
var RewardGrantModels = []any{&RewardGrant{}}
