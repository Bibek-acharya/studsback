// internal/coins/unlock_test.go
//
// The entitlement rules, with no database.
//
// Everything asserted here is a rule that would otherwise only be observable
// through a round trip to Postgres, and two of them are the ones most likely to
// be got wrong in a way that only shows up for one student: the expiry boundary
// and the "expired means zero regardless of what is unused" rule. Both are
// single-comparison rules and both are worth a table test.
//
// The constraint half — that the UNIQUE actually refuses a duplicate and that
// the CHECK actually refuses an inconsistent funding triple — cannot be asserted
// here, because a Go function that returns false is not a database constraint.
// Those are in unlock_pg_test.go, against a real server.
package coins

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ── resource type ────────────────────────────────────────────────────────────

func TestValidateResourceTypeAcceptsExactlyTheThreeClasses(t *testing.T) {
	for _, class := range []string{ResourceTypeStudyResource, ResourceTypeVideo, ResourceTypeMockTest} {
		if err := validateResourceType(class); err != nil {
			t.Errorf("validateResourceType(%q) = %v, want nil", class, err)
		}
	}
}

func TestValidateResourceTypeRejectsEverythingElse(t *testing.T) {
	// The class names are the ledger's ref_type values and they are lower_snake
	// case. A mixed-case or trailing-space variant is the realistic mistake, from
	// a form field or a hand-written integration, and each must be rejected
	// rather than normalised — a normalised class would create an unlock no gate
	// matches, so the student pays and gets nothing.
	for _, bad := range []string{
		"", " ", "STUDY_RESOURCE", "Study_Resource", "study_resource ",
		"documents", "document", "resource", "mocktest", "mock test",
		"user_referral", "coin_journal", "video;", "study_resource' OR '1'='1",
	} {
		err := validateResourceType(bad)
		if err == nil {
			t.Errorf("validateResourceType(%q) = nil, want a refusal", bad)
			continue
		}
		if !errors.Is(err, ErrInvalidResourceType) {
			t.Errorf("validateResourceType(%q) matched %v, want ErrInvalidResourceType", bad, err)
		}
	}
}

// ResourceTypes and the CHECK constraint are two representations of one list —
// a Go slice and a SQL string — and nothing but a test stops them drifting. The
// SQL side is asserted in unlock_pg_test.go; this asserts the Go side agrees with
// the ledger's ref_type constants, since those are what a gate will pass.
func TestResourceTypesAreTheLedgerRefTypes(t *testing.T) {
	want := []string{RefStudyResource, RefVideo, RefMockTest}
	if len(ResourceTypes) != len(want) {
		t.Fatalf("ResourceTypes has %d entries, want %d", len(ResourceTypes), len(want))
	}
	for i, class := range ResourceTypes {
		if class != want[i] {
			t.Errorf("ResourceTypes[%d] = %q, want %q (the ledger's ref_type, or spendPrice and the gate will disagree)", i, class, want[i])
		}
	}
	for _, class := range ResourceTypes {
		if _, ok := allowanceClassFor(class); !ok {
			t.Errorf("class %q is in ResourceTypes but has no allowance column, so its allowance could never be counted", class)
		}
	}
}

// ── source and the funding triple ────────────────────────────────────────────

func TestValidateUnlockSource(t *testing.T) {
	for _, source := range []string{UnlockSourceCoins, UnlockSourceAllowance} {
		if err := validateUnlockSource(source); err != nil {
			t.Errorf("validateUnlockSource(%q) = %v, want nil", source, err)
		}
	}
	for _, bad := range []string{"", "coins", "ALLOWANCE ", "FREE", "GRANT", "REFERRAL"} {
		if err := validateUnlockSource(bad); err == nil {
			t.Errorf("validateUnlockSource(%q) = nil, want a refusal", bad)
		} else if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("validateUnlockSource(%q) matched %v, want ErrInvalidArgument", bad, err)
		}
	}
}

// This table is the Go mirror of chk_resource_unlock_source_funding, and it is
// the pair of impossibilities the spec calls out by name: a COINS unlock with no
// journal, and an ALLOWANCE unlock with one.
//
// The two rows that deserve the most attention are the zero-price COINS row and
// the negative rows. A zero-amount COINS unlock is permitted because refusing it
// here would be a second pricing policy — a price of zero is the ledger's
// business, and this package records what the ledger settled. A NEGATIVE
// coins_paid is refused from both sources: coins_paid is a snapshot of what a
// student was charged, and a negative snapshot is a bug that reads as a refund.
func TestValidUnlockFunding(t *testing.T) {
	journal := "6f1d6b1e-0000-4000-8000-000000000001"
	empty := ""
	cases := []struct {
		name      string
		source    string
		journalID *string
		coinsPaid int64
		want      bool
	}{
		{"coins with a journal and a price", UnlockSourceCoins, &journal, 40, true},
		{"coins with a journal and a zero price", UnlockSourceCoins, &journal, 0, true},
		{"coins with a journal and a large price", UnlockSourceCoins, &journal, 1_000_000_000, true},
		{"coins with NO journal", UnlockSourceCoins, nil, 40, false},
		{"coins with an EMPTY journal id", UnlockSourceCoins, &empty, 40, false},
		{"coins with a negative price", UnlockSourceCoins, &journal, -1, false},
		{"allowance with no journal and no coins", UnlockSourceAllowance, nil, 0, true},
		{"allowance WITH a journal", UnlockSourceAllowance, &journal, 0, false},
		{"allowance that claims coins were paid", UnlockSourceAllowance, nil, 1, false},
		{"allowance with a negative amount", UnlockSourceAllowance, nil, -1, false},
		{"an unknown source", "FREEBIE", nil, 0, false},
		{"an empty source", "", nil, 0, false},
	}
	for _, c := range cases {
		if got := validUnlockFunding(c.source, c.journalID, c.coinsPaid); got != c.want {
			t.Errorf("%s: validUnlockFunding(%q, %v, %d) = %v, want %v",
				c.name, c.source, c.journalID, c.coinsPaid, got, c.want)
		}
	}
}

// The two sentinels travel together for one reason: a caller asking "can the
// starter allowance cover this" wants ErrNoAllowanceRemaining, and a handler
// mapping 423 ALLOWANCE_EXPIRED wants ErrAllowanceExpired. Asserting that they
// are distinct errors keeps the pairing honest: it has to be created by wrapping
// both, never by aliasing one to the other.
func TestAllowanceSentinelsAreDistinct(t *testing.T) {
	if !errors.Is(ErrNoAllowanceRemaining, ErrNoAllowanceRemaining) {
		t.Fatal("ErrNoAllowanceRemaining does not match itself")
	}
	if errors.Is(ErrNoAllowanceRemaining, ErrAllowanceExpired) {
		t.Error("ErrNoAllowanceRemaining matches ErrAllowanceExpired on its own; the pairing is created by wrapping both")
	}
	if errors.Is(ErrNoAllowanceRemaining, ErrAlreadyUnlocked) {
		t.Error("an exhausted allowance and an existing unlock are different answers and must be different sentinels")
	}
	if errors.Is(ErrUnlockRevoked, ErrAlreadyUnlocked) {
		t.Error("a revoked unlock is not the same as an active one; conflating them would tell a student they still have access")
	}
}

// ── the expiry boundary ──────────────────────────────────────────────────────

// The boundary is the whole point. An allowance whose ExpiresAt is EXACTLY now is
// expired, and the window is half-open: [granted_at, expires_at). An inclusive
// comparison would make one request behave differently from the one a
// microsecond earlier with no change in the world, which is the kind of bug that
// gets reported as "sometimes the third download fails".
func TestAllowanceExpiryBoundaryIsHalfOpen(t *testing.T) {
	granted := time.Date(2026, 9, 27, 4, 12, 0, 0, time.UTC)
	expires := granted.AddDate(0, 0, 30)

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"the instant it is granted", granted, false},
		{"the day before", expires.Add(-24 * time.Hour), false},
		{"one microsecond before", expires.Add(-time.Microsecond), false},
		{"exactly at expires_at", expires, true},
		{"one microsecond after", expires.Add(time.Microsecond), true},
		{"a year after", expires.AddDate(1, 0, 0), true},
	}
	for _, c := range cases {
		if got := allowanceExpired(c.now, expires); got != c.want {
			t.Errorf("%s: allowanceExpired(%s, %s) = %v, want %v",
				c.name, c.now.Format(time.RFC3339Nano), expires.Format(time.RFC3339), got, c.want)
		}
	}
}

// The zero time is not a sentinel for "never expires" here. An allowance always
// has an ExpiresAt — the column is NOT NULL — so there is no "never" case to get
// wrong, and this asserts the zero time reads as long expired rather than as
// infinite validity.
func TestAllowanceExpiredTreatsTheZeroTimeAsExpired(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	if !allowanceExpired(now, time.Time{}) {
		t.Error("a zero ExpiresAt must read as expired, not as never-expiring")
	}
}

func TestAllowanceExpiryIsGrantedPlusTheConfiguredDays(t *testing.T) {
	granted := time.Date(2026, 9, 27, 4, 12, 0, 0, time.UTC)

	got, err := allowanceExpiry(granted, 30)
	if err != nil {
		t.Fatalf("allowanceExpiry(30 days) = %v, want nil", err)
	}
	if want := granted.AddDate(0, 0, 30); !got.Equal(want) {
		t.Errorf("expiry = %s, want %s", got.UTC().Format(time.RFC3339), want.UTC().Format(time.RFC3339))
	}

	// Zero days is a legitimate "disabled" configuration, not an error: it yields
	// an allowance that is expired the moment it is granted, which is the honest
	// reading rather than a crash on a config an admin is allowed to write.
	got, err = allowanceExpiry(granted, 0)
	if err != nil {
		t.Fatalf("allowanceExpiry(0 days) = %v, want nil", err)
	}
	if !got.Equal(granted) {
		t.Errorf("a 0-day allowance expires at %s, want %s (i.e. immediately)", got, granted)
	}
	if !allowanceExpired(granted, got) {
		t.Error("a 0-day allowance must be expired at its own grant instant")
	}

	// Negative is refused rather than turned into a date in the past.
	// ValidateEconomyConfig already rejects it, so this can only be a bug, and a
	// silently-expired allowance would hide the bug.
	if _, err := allowanceExpiry(granted, -1); err == nil {
		t.Error("allowanceExpiry(-1 day) = nil, want ErrInvalidArgument")
	} else if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("allowanceExpiry(-1 day) matched %v, want ErrInvalidArgument", err)
	}
}

// ── the per-class arithmetic ─────────────────────────────────────────────────

func TestRemainingUnlocks(t *testing.T) {
	cases := []struct {
		name    string
		granted int64
		used    int64
		expired bool
		want    int64
	}{
		{"untouched", 3, 0, false, 3},
		{"partly used", 3, 1, false, 2},
		{"exhausted", 3, 3, false, 0},
		{"over-used cannot go negative", 3, 4, false, 0},
		{"a zero grant", 0, 0, false, 0},
		{"a zero grant with a stray count", 0, 2, false, 0},
		// The rule that stops "save them for the weekend" from being a strategy.
		{"expired with nothing used", 3, 0, true, 0},
		{"expired with one used", 3, 1, true, 0},
		{"expired with nothing granted", 0, 0, true, 0},
	}
	for _, c := range cases {
		if got := remainingUnlocks(c.granted, c.used, c.expired); got != c.want {
			t.Errorf("%s: remainingUnlocks(%d, %d, expired=%v) = %d, want %d",
				c.name, c.granted, c.used, c.expired, got, c.want)
		}
	}
}

// The status is the whole of what 03-api-contract.md §2.1 renders, so it is
// asserted as the numbers a student would see rather than as a struct shape.
func TestComputeAllowanceStatus(t *testing.T) {
	granted := time.Date(2026, 9, 27, 4, 12, 0, 0, time.UTC)
	expires := granted.AddDate(0, 0, 30)
	row := &UserFreeAllowance{
		ID:              1,
		UserID:          7,
		GrantedAt:       granted,
		ExpiresAt:       expires,
		DocumentUnlocks: 3,
		VideoUnlocks:    1,
		MockTestUnlocks: 1,
	}

	t.Run("before expiry", func(t *testing.T) {
		status := computeAllowanceStatus(row, map[string]int64{
			ResourceTypeStudyResource: 1,
		}, granted.AddDate(0, 0, 1))

		if status.Expired {
			t.Error("an allowance inside its window must not report as expired")
		}
		if !status.GrantedAt.Equal(granted) || !status.ExpiresAt.Equal(expires) {
			t.Errorf("window = %s..%s, want %s..%s", status.GrantedAt, status.ExpiresAt, granted, expires)
		}
		doc := status.Classes[ResourceTypeStudyResource]
		if doc.Granted != 3 || doc.Used != 1 || doc.Remaining != 2 {
			t.Errorf("documents = %d granted / %d used / %d remaining, want 3 / 1 / 2", doc.Granted, doc.Used, doc.Remaining)
		}
		// A class that was never touched is still present with a full count, so a
		// renderer prints "1 of 1" rather than having to know absence means zero.
		video := status.Classes[ResourceTypeVideo]
		if video.Granted != 1 || video.Used != 0 || video.Remaining != 1 {
			t.Errorf("video = %d/%d/%d, want 1/0/1", video.Granted, video.Used, video.Remaining)
		}
		if status.TotalRemaining != 4 {
			t.Errorf("total remaining = %d, want 4", status.TotalRemaining)
		}
		if got := status.Remaining(ResourceTypeMockTest); got != 1 {
			t.Errorf("Remaining(mock_test) = %d, want 1", got)
		}
		if got := status.Remaining("nonexistent"); got != 0 {
			t.Errorf("Remaining of an unknown class = %d, want 0", got)
		}
	})

	t.Run("at the boundary", func(t *testing.T) {
		// ExpiresAt == now. Expired, and every class reads zero even though two of
		// them are entirely unused.
		status := computeAllowanceStatus(row, map[string]int64{ResourceTypeStudyResource: 1}, expires)
		if !status.Expired {
			t.Error("an allowance whose ExpiresAt is exactly now must be expired")
		}
		for _, class := range ResourceTypes {
			c := status.Classes[class]
			if c.Remaining != 0 {
				t.Errorf("%s remaining = %d on an expired allowance, want 0", class, c.Remaining)
			}
		}
		if status.TotalRemaining != 0 {
			t.Errorf("total remaining = %d on an expired allowance, want 0", status.TotalRemaining)
		}
		// The history survives the expiry. "You used 1 of 3, and they expired on
		// the 26th" is a better answer than a bare zero.
		if got := status.Classes[ResourceTypeStudyResource].Used; got != 1 {
			t.Errorf("used on an expired allowance = %d, want 1 — expiry must not erase the history", got)
		}
		if got := status.Classes[ResourceTypeStudyResource].Granted; got != 3 {
			t.Errorf("granted on an expired allowance = %d, want 3", got)
		}
	})

	t.Run("no row at all", func(t *testing.T) {
		// A user who registered before the hook was wired. Every class reads zero
		// and, crucially, NOT expired: an allowance that was never granted has not
		// lapsed, and reporting it as expired would tell a student their free
		// unlocks ran out when they were never issued.
		status := computeAllowanceStatus(nil, nil, granted)
		if status.Expired {
			t.Error("a missing allowance must not report as expired; it was never granted")
		}
		if status.TotalRemaining != 0 {
			t.Errorf("total remaining = %d with no allowance row, want 0", status.TotalRemaining)
		}
		if len(status.Classes) != len(ResourceTypes) {
			t.Errorf("Classes has %d entries, want %d — a renderer must not have to special-case absence",
				len(status.Classes), len(ResourceTypes))
		}
		for _, class := range ResourceTypes {
			if c := status.Classes[class]; c.Granted != 0 || c.Remaining != 0 {
				t.Errorf("%s = %d granted / %d remaining with no row, want 0/0", class, c.Granted, c.Remaining)
			}
		}
	})

	t.Run("a revocation gives the unlock back", func(t *testing.T) {
		// "Used" is derived, so a revoked allowance unlock is not counted. This is
		// the concrete reason there is no stored counter: a counter that was not
		// decremented on revocation would permanently cost this student one of
		// their three, with no row anywhere explaining why.
		status := computeAllowanceStatus(row, map[string]int64{}, granted)
		if got := status.Remaining(ResourceTypeStudyResource); got != 3 {
			t.Errorf("remaining after a revocation = %d, want 3 — a revoked unlock is not used", got)
		}
	})
}

// The grant is a snapshot of the ROW, never of the config. If this ever reads the
// config, lowering a default silently takes entitlements away from every existing
// student, and there is no row that records that it happened.
func TestComputeAllowanceStatusReadsTheRowNotTheConfig(t *testing.T) {
	row := &UserFreeAllowance{
		GrantedAt: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC),
		// Deliberately not the default 3/1/1: this user was granted 10 documents.
		DocumentUnlocks: 10,
		VideoUnlocks:    2,
		MockTestUnlocks: 5,
	}
	status := computeAllowanceStatus(row, map[string]int64{ResourceTypeStudyResource: 9}, row.GrantedAt)
	if got := status.Remaining(ResourceTypeStudyResource); got != 1 {
		t.Errorf("remaining = %d, want 1 from the row's 10 grants less 9 used", got)
	}
	if got := status.Remaining(ResourceTypeMockTest); got != 5 {
		t.Errorf("mock test remaining = %d, want the row's 5, not the default 1", got)
	}
	if got := status.Remaining(ResourceTypeVideo); got != 2 {
		t.Errorf("video remaining = %d, want the row's 2, not the default 1", got)
	}
}

// ── the class → column binding ───────────────────────────────────────────────

// The three classes must map to the three distinct allowance columns. A copy-paste
// that pointed video and mock_test at the same column would let a student burn
// one mock-test unlock twice, and the arithmetic tests above would all still
// pass, because they read whatever the binding said.
func TestAllowanceClassesBindDistinctColumns(t *testing.T) {
	cfg := AllowanceConfig{DocumentUnlocks: 7, VideoUnlocks: 11, MockTestUnlocks: 13}
	seen := map[int64]string{}
	for _, class := range ResourceTypes {
		c, ok := allowanceClassFor(class)
		if !ok {
			t.Fatalf("no allowance column for %q", class)
		}
		quota := c.quota(cfg)
		if other, dup := seen[quota]; dup {
			t.Errorf("%q and %q both read column %d from a config of %+v — the binding is not one-to-one",
				class, other, quota, cfg)
		}
		seen[quota] = class
	}
	if _, ok := allowanceClassFor("user_referral"); ok {
		t.Error("user_referral is a journal ref_type, not an unlockable class, and must not resolve to an allowance column")
	}
}

// ── argument validation, and the order it happens in ─────────────────────────

// Every entitlement method validates its arguments before it touches the
// repository, so a malformed request never opens a transaction and never takes a
// user's advisory lock to find out it was bad. This is the ledger's stated rule
// ("an unknown reason code, a zero award or a missing key must not open a
// transaction"), and the nil-repo Service is how it is asserted without a
// database: a validation error surfaces instead of ErrNoDatabase.
func TestEntitlementMethodsValidateBeforeReachingTheDatabase(t *testing.T) {
	svc := NewService(nil, nil) // no repository, no config: nothing can be reached
	ctx := context.Background()
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	const journal = "6f1d6b1e-0000-4000-8000-000000000001"

	// resource_type is validated before the repository is consulted.
	if _, err := svc.HasAccess(ctx, 7, "documents", 812); !errors.Is(err, ErrInvalidResourceType) {
		t.Errorf("HasAccess with a bad class = %v, want ErrInvalidResourceType", err)
	}
	if _, err := svc.ConsumeAllowance(ctx, 7, "documents", 812, now); !errors.Is(err, ErrInvalidResourceType) {
		t.Errorf("ConsumeAllowance with a bad class = %v, want ErrInvalidResourceType", err)
	}
	if _, err := svc.RecordCoinUnlock(ctx, 7, "documents", 812, journal, 40, now); !errors.Is(err, ErrInvalidResourceType) {
		t.Errorf("RecordCoinUnlock with a bad class = %v, want ErrInvalidResourceType", err)
	}
	if err := svc.Revoke(ctx, 7, "documents", 812, "fraud", now); !errors.Is(err, ErrInvalidResourceType) {
		t.Errorf("Revoke with a bad class = %v, want ErrInvalidResourceType", err)
	}

	// A zero user is a zero user whatever the class is.
	if _, err := svc.HasAccess(ctx, 0, ResourceTypeStudyResource, 812); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("HasAccess with user 0 = %v, want ErrInvalidArgument", err)
	}
	if _, err := svc.EnsureAllowance(ctx, 0, now); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("EnsureAllowance with user 0 = %v, want ErrInvalidArgument", err)
	}
	if _, err := svc.RemainingAllowance(ctx, 0, now); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("RemainingAllowance with user 0 = %v, want ErrInvalidArgument", err)
	}

	// Resource 0 is nothing, so an unlock of it is a row that grants access to no
	// resource. It is refused rather than stored.
	if _, err := svc.HasAccess(ctx, 7, ResourceTypeStudyResource, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("HasAccess with resource 0 = %v, want ErrInvalidArgument", err)
	}
	if _, err := svc.ConsumeAllowance(ctx, 7, ResourceTypeStudyResource, 0, now); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("ConsumeAllowance with resource 0 = %v, want ErrInvalidArgument", err)
	}
	if _, err := svc.RecordCoinUnlock(ctx, 7, ResourceTypeStudyResource, 0, journal, 40, now); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("RecordCoinUnlock with resource 0 = %v, want ErrInvalidArgument", err)
	}
	if err := svc.Revoke(ctx, 7, ResourceTypeStudyResource, 0, "fraud", now); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("Revoke with resource 0 = %v, want ErrInvalidArgument", err)
	}

	// A well-formed request on a repository-less Service reports the wiring
	// mistake rather than panicking.
	if _, err := svc.HasAccess(ctx, 7, ResourceTypeStudyResource, 812); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("HasAccess with no repository = %v, want ErrNoDatabase", err)
	}
	if _, err := svc.RemainingAllowance(ctx, 7, now); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("RemainingAllowance with no repository = %v, want ErrNoDatabase", err)
	}
	if _, err := svc.EnsureAllowance(ctx, 7, now); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("EnsureAllowance with no repository = %v, want ErrNoDatabase", err)
	}
	if _, err := svc.ConsumeAllowance(ctx, 7, ResourceTypeStudyResource, 812, now); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("ConsumeAllowance with no repository = %v, want ErrNoDatabase", err)
	}
	if _, err := svc.RecordCoinUnlock(ctx, 7, ResourceTypeStudyResource, 812, journal, 40, now); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("RecordCoinUnlock with no repository = %v, want ErrNoDatabase", err)
	}
	if err := svc.Revoke(ctx, 7, ResourceTypeStudyResource, 812, "fraud", now); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("Revoke with no repository = %v, want ErrNoDatabase", err)
	}
}

// A malformed journal id is refused as an argument rather than arriving at the
// foreign key three statements later as an opaque constraint error.
func TestRecordCoinUnlockRejectsBadFundingArguments(t *testing.T) {
	svc := NewService(nil, nil)
	ctx := context.Background()
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	const goodJournal = "6f1d6b1e-0000-4000-8000-000000000001"

	for _, bad := range []string{"", "   ", "not-a-uuid", "812"} {
		_, err := svc.RecordCoinUnlock(ctx, 7, ResourceTypeStudyResource, 812, bad, 40, now)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("RecordCoinUnlock with journal %q = %v, want ErrInvalidArgument", bad, err)
		}
		if errors.Is(err, ErrNoDatabase) {
			t.Errorf("RecordCoinUnlock with journal %q reached the repository, so the argument check is after requireRepo", bad)
		}
	}
	if _, err := svc.RecordCoinUnlock(ctx, 7, ResourceTypeStudyResource, 812, goodJournal, -1, now); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("RecordCoinUnlock with coins_paid -1 = %v, want ErrInvalidArgument", err)
	}
	// A zero-amount coin unlock passes the argument check, so with no repository
	// it reaches requireRepo. A price of zero is the ledger's policy, not this
	// method's.
	if _, err := svc.RecordCoinUnlock(ctx, 7, ResourceTypeStudyResource, 812, goodJournal, 0, now); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("RecordCoinUnlock with coins_paid 0 = %v, want ErrNoDatabase", err)
	}
}

// A revocation with no reason is refused before anything is written. A
// revoked_at with no reason is a support ticket that cannot be answered, and
// chk_resource_unlock_revocation refuses the half-present pair at the database
// as well.
func TestRevokeRequiresAReason(t *testing.T) {
	svc := NewService(nil, nil)
	for _, bad := range []string{"", "   ", "\t\n"} {
		err := svc.Revoke(context.Background(), 7, ResourceTypeStudyResource, 812, bad, time.Now())
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("Revoke with reason %q = %v, want ErrInvalidArgument", bad, err)
		}
		if errors.Is(err, ErrNoDatabase) {
			t.Errorf("Revoke with reason %q reached the repository before the reason was checked", bad)
		}
	}
}

// The table names are pinned explicitly rather than left to GORM's pluraliser,
// for the same reason the ledger's are: the DDL in
// 02-architecture.md §5 was validated against the singular names, and a
// pluralised "resource_unlocks" would be created by AutoMigrate while every
// statement in unlock_repository.go looked for the singular one.
func TestEntitlementTableNamesAreSingular(t *testing.T) {
	if got := (ResourceUnlock{}).TableName(); got != "resource_unlock" {
		t.Errorf("ResourceUnlock.TableName() = %q, want %q", got, "resource_unlock")
	}
	if got := (UserFreeAllowance{}).TableName(); got != "user_free_allowance" {
		t.Errorf("UserFreeAllowance.TableName() = %q, want %q", got, "user_free_allowance")
	}
	if len(EntitlementModels) != 2 {
		t.Errorf("EntitlementModels has %d entries, want 2 — main.go migrates this list", len(EntitlementModels))
	}
}
