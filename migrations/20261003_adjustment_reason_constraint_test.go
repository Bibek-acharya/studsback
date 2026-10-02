package migrations

import (
	"testing"

	"studsphere/backend/internal/coins"
)

// The CHECK constraint must carry the same vocabulary the service validates.
//
// Two layers, tested in two places, because they fail differently: the service check
// produces an error message an operator can act on, and the constraint is what stops
// a future code path — a bulk correction, a backfill, an admin maintenance path —
// from inventing a reason. A service-only check is bypassable; a constraint-only check
// produces a PostgreSQL error nobody can act on.
func TestAdjustmentReasonConstraintMatchesTheServiceVocabulary(t *testing.T) {
	want := "'DUPLICATE_AWARD', 'ERRONEOUS_SPEND', 'GOODWILL', 'MIGRATION_CORRECTION', 'DATA_FIX'"
	got := coins.AdjustmentReasonsSQLList()
	if got != want {
		t.Errorf("AdjustmentReasonsSQLList() = %q, want %q", got, want)
	}
	// Every reason the constraint lists must be one the service accepts, or the
	// database would permit a journal the server would refuse to write.
	for _, reason := range coins.AdjustmentReasons {
		if !coins.IsAdjustmentReason(reason) {
			t.Errorf("the constraint lists %q but the service rejects it", reason)
		}
	}
	if coins.IsAdjustmentReason("NOT_A_REASON") {
		t.Error("the service accepted a reason outside the set")
	}
	// And the earn reasons must NOT be in the adjustment set, or "a referral paid"
	// and "support gave this back" would merge into one metric bucket.
	for _, earn := range []string{coins.ReasonProfileComplete, coins.ReasonReferralQualified, coins.ReasonResourceApproved, coins.ReasonCoinExpired} {
		if coins.IsAdjustmentReason(earn) {
			t.Errorf("earn reason %q is valid as an adjustment reason", earn)
		}
	}
}

func TestAddAdjustmentReasonConstraintRefusesNoDatabase(t *testing.T) {
	if err := AddAdjustmentReasonConstraint(nil); err == nil {
		t.Error("a nil handle was accepted")
	}
}
