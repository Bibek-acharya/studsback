// internal/coins/adjust_test.go
//
// 03 §3.2: POST /admin/coins/adjust — the signed correction.
//
// THE PROHIBITION, which is the whole design: this is NOT a "set balance" endpoint.
// There is no way to make a balance be a number; adjustments MOVE it, and every one
// carries a reason code and an idempotency key.
//
// Why the prohibition rather than an audit log after the fact: an endpoint that can
// write an arbitrary figure is an endpoint whose abuse is undetectable. If an
// operator's session is compromised, "set this student's balance to 1,000,000" is
// one request with no semantic content to review. "Move 1,000,000 with reason
// X" is a request that appears in a list of money movements with an author and a
// stated cause — and it is bounded by the same invariant as every other write.
//
// The negative direction matters as much as the positive. An adjustment that takes a
// student below zero has to FAIL, and it has to fail with the no-overdraft CHECK
// rather than by clamping — a clamped correction leaves the operator believing a
// balance was corrected when it was not.

package coins

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// The vocabulary and the prohibition live in adjustment_reasons.go and adjust.go;
// this file is the table of properties.

func TestAdjustmentReasonSetIsClosed(t *testing.T) {
	if len(AdjustmentReasons) != 5 {
		t.Fatalf("%d adjustment reasons, want exactly 5", len(AdjustmentReasons))
	}
	seen := map[string]bool{}
	for _, reason := range AdjustmentReasons {
		if reason == "" {
			t.Error("an empty reason code is in the set; it cannot be grouped by")
		}
		if seen[reason] {
			t.Errorf("reason %q appears twice", reason)
		}
		seen[reason] = true
	}
	// None of them may collide with an EARN reason. reason_code is one column on
	// one table, and the health metrics group by it — a collision would merge
	// "support gave this back" into "a referral paid", which is precisely the
	// distinction 08's fraud-ratio metric needs.
	for _, earn := range []string{ReasonProfileComplete, ReasonReferralQualified, ReasonResourceApproved, ReasonResourceUnlock, ReasonGrantReversal, ReasonCoinExpired} {
		if seen[earn] {
			t.Errorf("adjustment reason %q collides with an earn reason", earn)
		}
	}
}

// An unknown reason is refused. This is the control that makes the audit trail
// worth anything: an operator cannot invent their own reason string at the call site.
func TestAnAdjustmentWithAnUnknownReasonIsRefused(t *testing.T) {
	ledger := &Ledger{}
	_, err := ledger.Adjust(context.Background(), AdjustRequest{
		UserID: 42, Amount: 100, Reason: "BECAUSE_I_SAID_SO", CreatedBy: "admin:1",
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("an invented reason got %v, want ErrInvalidArgument", err)
	}
}

// A reason is mandatory, and so is an author. A journal with created_by = ” is
// unattributable, and an unattributable correction is the thing this endpoint exists
// to prevent.
func TestAnAdjustmentRequiresAReasonAndAnAuthor(t *testing.T) {
	ledger := &Ledger{}
	if _, err := ledger.Adjust(context.Background(), AdjustRequest{UserID: 42, Amount: 100, CreatedBy: "admin:1"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a reason-less adjustment got %v, want ErrInvalidArgument", err)
	}
	if _, err := ledger.Adjust(context.Background(), AdjustRequest{UserID: 42, Amount: 100, Reason: AdjustGoodwill}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("an author-less adjustment got %v, want ErrInvalidArgument", err)
	}
}

// A ZERO adjustment is refused. It is the closest thing this endpoint has to a
// no-op, and a no-op that writes a journal is noise in the audit trail and a
// wasted transaction.
func TestAZeroAdjustmentIsRefused(t *testing.T) {
	ledger := &Ledger{}
	if _, err := ledger.Adjust(context.Background(), AdjustRequest{
		UserID: 42, Amount: 0, Reason: AdjustGoodwill, CreatedBy: "admin:1",
	}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a zero adjustment got %v, want ErrInvalidArgument", err)
	}
}

// There is deliberately no Bucket field on AdjustRequest. A correction belongs in
// the bucket the value came from, and letting the CALLER name it is how a goodwill
// credit ends up in FREE — and therefore non-expiring — when it should behave like
// the earned value it is replacing. The field's absence is a design constraint, so
// it is asserted rather than left to a reviewer to notice.
//
// This is a compile-time property expressed as a reflection test, which is unusual
// and is the lesser evil: adding `Bucket string` to the struct is a one-line change,
// and nothing else in the suite would fail.
func TestAdjustmentRequestHasNoBucketField(t *testing.T) {
	if _, ok := reflect.TypeOf(AdjustRequest{}).FieldByName("Bucket"); ok {
		t.Error("AdjustRequest has a Bucket field; a correction must land in the bucket the value came from, " +
			"and a caller-named bucket is how goodwill ends up in FREE and never expires")
	}
}

// The reason set must be reachable from the DDL too — the same drift check the
// approval constraint has.
func TestAdjustmentReasonsRenderAsSQLLiteralList(t *testing.T) {
	got := AdjustmentReasonsSQLList()
	for _, reason := range AdjustmentReasons {
		if !strings.Contains(got, "'"+reason+"'") {
			t.Errorf("the SQL list %q is missing %q", got, reason)
		}
	}
	if strings.Contains(got, "DUPLICATE_AWARD, DUPLICATE_AWARD") {
		t.Error("the SQL list has a duplicate")
	}
}
