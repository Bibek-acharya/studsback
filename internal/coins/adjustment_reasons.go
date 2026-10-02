// internal/coins/adjustment_reasons.go
//
// The closed vocabulary of reasons an operator may give for a manual correction.
//
// SEPARATE FROM adjust.go because it is a CONTRACT rather than a mechanism, and
// because the reason set is what the DDL CHECK must agree with. Two places consume
// it — the validation in Adjust and the constraint added by the migration — and a
// single definition is the only way they cannot drift.

package coins

import "strings"

// The adjustment reason codes.
//
// Closed because every code is a different support sentence, a different metric
// bucket, and a different thing an auditor looks for. A free-text reason is a reason
// nobody can group by, which is the same mistake as a free-text reason_code on the
// earn path.
//
// The set is deliberately SMALL. Three of these five are corrections of the ledger's
// own mistakes (duplicate award, erroneous spend, data fix) and two are operator
// decisions (goodwill, migration correction). A set that grew to fifteen would mean
// the distinctions are not being drawn at the point of the correction, which is the
// moment they are cheapest to draw.
const (
	// AdjustDuplicateAward reverses an award that was made twice — a double-submit,
	// a retried job, or two paths racing for the same milestone.
	AdjustDuplicateAward = "DUPLICATE_AWARD"

	// AdjustErroneousSpend takes back a spend that should not have happened. Note
	// the asymmetry with GOODWILL: this is the operator admitting their own system's
	// error, while GOODWILL is a decision on the platform's initiative.
	AdjustErroneousSpend = "ERRONEOUS_SPEND"

	// AdjustGoodwill restores value a student was wrongly denied. This is the only
	// code whose existence is a policy statement, and it is the reason the endpoint
	// is not "set balance": goodwill is a MOVEMENT with a name attached, which can
	// be counted per month and argued about, where a balance overwrite cannot be
	// argued about at all.
	AdjustGoodwill = "GOODWILL"

	// AdjustMigration corrects a balance carried in from before the ledger existed.
	AdjustMigration = "MIGRATION_CORRECTION"

	// AdjustDataFix corrects a balance whose cached projection drifted from its
	// postings. This is the code for the defect RunReconcile finds: the reconciler
	// reports the drift, a human resolves it here, and the reason says which.
	AdjustDataFix = "DATA_FIX"
)

// AdjustmentReasons is the closed set, in a stable order. The order is the order the
// dashboard groups by and the order the CHECK constraint lists, so it is a constant
// rather than a set literal.
var AdjustmentReasons = []string{
	AdjustDuplicateAward,
	AdjustErroneousSpend,
	AdjustGoodwill,
	AdjustMigration,
	AdjustDataFix,
}

// AdjustmentReasonsSQLList renders the set as a SQL literal list for a CHECK
// constraint: 'A', 'B', 'C'.
//
// A helper rather than a raw string at the migration, so the constraint and the
// validator cannot be written from two different recollections of the vocabulary.
func AdjustmentReasonsSQLList() string {
	quoted := make([]string, 0, len(AdjustmentReasons))
	for _, reason := range AdjustmentReasons {
		quoted = append(quoted, "'"+reason+"'")
	}
	return strings.Join(quoted, ", ")
}
