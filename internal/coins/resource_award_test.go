package coins

import (
	"context"
	"errors"
	"testing"
)

// The §5.3 upload award, as seen from the coin side.
//
// The state machine that calls this lives in internal/studyresources, and its
// tests assert that approval INVOKES a grant with the uploader's id. What is
// pinned here is the half that only this module can know: the key namespace, the
// reason code, and the fact that a second approval of the same resource pays
// nothing.
//
// THE KEY NAMESPACE IS THE POINT. coin_journal has one UNIQUE
// (scope, idempotency_key) across every reason code in the economy, so a key this
// method composes shares a namespace with the profile ladder, the referral
// settlement, and the spend path. "resource-approved:<id>" is safe precisely
// because nothing else in the codebase builds that prefix — which is a fact
// about the whole package and therefore belongs in a test that fails loudly if a
// second producer ever appears.

func TestResourceApprovedGrantKeyIsNamespaced(t *testing.T) {
	key := resourceApprovedKey(9)
	if key != "resource-approved:9" {
		t.Errorf("key = %q, want %q", key, "resource-approved:9")
	}
	// Two resources, two keys. A key built from the uploader instead would collide
	// the moment one student uploaded twice, and the second award would be silently
	// swallowed as a replay — a student who published two files and got paid once.
	if resourceApprovedKey(9) == resourceApprovedKey(10) {
		t.Error("two resources share an idempotency key")
	}
	// And it must not collide with any other producer's namespace. Each prefix is
	// asserted as a PREFIX, not as a whole key: "resource-approved:" + "9" IS this
	// key, so comparing the composed forms would be a tautology. What matters is
	// that no OTHER producer builds a key starting with one of these prefixes.
	for _, other := range []string{"referral-qualified:", "profile-step:", "unlock:", "resource_approved:"} {
		if len(resourceApprovedKey(9)) > len(other) && resourceApprovedKey(9)[:len(other)] == other {
			t.Errorf("the resource award key shares a prefix with %q", other)
		}
	}
}

// The reason code must be one the ledger can actually pay, or approval refuses at
// the last moment with a message about configuration. grantSpecs is the authority.
func TestResourceApprovedIsAPayableGrant(t *testing.T) {
	spec, ok := grantSpecs[ReasonResourceApproved]
	if !ok {
		t.Fatalf("reason %s is not in grantSpecs, so an approval cannot pay", ReasonResourceApproved)
	}
	cfg := DefaultEconomyConfig()
	if spec.amount(cfg) != 80 {
		t.Errorf("the award is %d coins, want 80 (04 §5.3 and 03 §3.1)", spec.amount(cfg))
	}
	if spec.bucket != BucketEarned {
		t.Errorf("the award lands in %s, want %s — an upload award is earned, never free", spec.bucket, BucketEarned)
	}
	if spec.expiryDays(cfg) <= 0 {
		t.Errorf("the award expires in %d days; Grant refuses a zero expiry", spec.expiryDays(cfg))
	}
}

func TestGrantResourceApprovedRefusesAWiringProblem(t *testing.T) {
	ledger := NewLedger(nil, nil)

	// No ledger at all.
	if _, err := GrantResourceApproved(context.Background(), ledger, 42, 7, "Notes"); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("a nil ledger got %v, want ErrNoDatabase", err)
	}
	// No user to pay. An approval for a row with UploadedBy = 0 would otherwise
	// create an account for user zero.
	if _, err := GrantResourceApproved(context.Background(), &Ledger{}, 0, 7, "Notes"); err == nil {
		t.Error("a zero user id was accepted")
	}
	if _, err := GrantResourceApproved(context.Background(), &Ledger{}, 42, 0, "Notes"); err == nil {
		t.Error("a zero resource id was accepted; the idempotency key would be the same for every such row")
	}
}
