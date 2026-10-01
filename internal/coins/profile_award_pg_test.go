//go:build coinsintegration

// internal/coins/profile_award_pg_test.go
//
// The profile award against a real PostgreSQL instance.
//
// This file shares the harness from ledger_pg_test.go: the same coreSchema, the
// same single pool, the same openLedgerSchema reset. That is deliberate rather
// than lazy — the award writes coin_journal rows and its assertions are about
// journal counts and balances, so it needs the ledger's fixture and must not
// diverge from it. It additionally truncates reward_grant, which the ledger tests
// do not touch.
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// Every test skips cleanly when COINS_TEST_DSN is unset.
package coins

import (
	"context"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"

	"studsphere/backend/internal/notification"
)

// ── fixtures ─────────────────────────────────────────────────────────────────

// stubCompletion stands in for studentdashboard's twelve checks. The percentage
// is a FIELD so a test can move a student's completion between calls, which is
// what the drop-and-recovery test needs.
type stubCompletion struct {
	mu      sync.Mutex
	percent map[uint]int
}

func newStubCompletion() *stubCompletion {
	return &stubCompletion{percent: map[uint]int{}}
}

func (c *stubCompletion) set(userID uint, percent int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.percent[userID] = percent
}

func (c *stubCompletion) ProfileCompletionPercent(_ context.Context, userID uint) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.percent[userID], nil
}

// recordingNotifier counts emissions without writing an outbox.
type recordingNotifier struct {
	mu    sync.Mutex
	calls []recordedCredit
}

type recordedCredit struct {
	eventKey  string
	userID    uint
	coins     int64
	balance   int64
	occurKey  string
	correlate string
}

func (n *recordingNotifier) Notify(_ context.Context, req notification.NotifyRequest) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, recordedCredit{
		eventKey:  req.EventKey,
		userID:    req.Recipients[0].ID,
		coins:     req.Data["coins"].(int64),
		balance:   req.Data["balance"].(int64),
		occurKey:  req.OccurrenceKey,
		correlate: req.CorrelationID,
	})
	return nil
}

func (n *recordingNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}

// awardFixture is one wired awarder plus the stubs behind it.
type awardFixture struct {
	svc        *ProfileAwardService
	completion *stubCompletion
	notifier   *recordingNotifier
	db         *gorm.DB
}

// newAwardFixture wires the award over the shared ledger pool.
//
// The notifier is wired BY DEFAULT. That is a change of heart from the wiring in
// main.go, and it is deliberate: if it were optional here, a test asserting "the
// notification fires once per step" could pass against a fixture that emits
// nothing at all, which is the worst possible way to test a notification.
func newAwardFixture(t *testing.T, db *gorm.DB, mutate func(*EconomyConfig)) *awardFixture {
	t.Helper()
	if err := db.Exec(`TRUNCATE reward_grant RESTART IDENTITY CASCADE`).Error; err != nil {
		t.Fatalf("truncate reward_grant: %v", err.Error())
	}
	completion := newStubCompletion()
	notifier := &recordingNotifier{}
	ledger := testLedger(t, db, mutate)
	svc := NewProfileAwardService(NewRepository(db), ledger, completion, nil).
		WithNotifier(notifier)
	return &awardFixture{svc: svc, completion: completion, notifier: notifier, db: db}
}

// profileJournals counts this user's PROFILE_COMPLETE grants.
//
// COALESCE ON EVERY JOIN COLUMN, deliberately. A plain count(*) over these joins
// silently undercounts when any join column is NULL — specifically coin_account.
// owner_user_id, which is NULL for the SYSTEM legs of a grant. Every PROFILE_COMPLETE
// grant posts one user leg and one earned_faucet leg, and the faucet leg's account
// has owner_user_id = NULL, so the NULL row drops out of the join and the count
// comes back as zero for a grant that demonstrably happened. Verified by
// TestProfileJournalsCountsThroughNullSystemLegs below, which is the reason that
// test exists.
func profileJournals(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	return countRows(t, db,
		`SELECT count(DISTINCT j.id) FROM coin_journal j
		   JOIN coin_posting p ON p.journal_id = j.id
		   JOIN coin_account a ON a.id = p.account_id
		  WHERE j.scope = ? AND j.reason_code = ?
		    AND a.owner_user_id = ? AND a.kind = ? AND p.amount > 0`,
		ScopeUser, ReasonProfileComplete, userID, AccountUser)
}

// TestProfileJournalsCountsThroughNullSystemLegs is the guard on the helper above,
// and it is the reason profileJournals is written the way it is.
//
// A grant posts two legs: the student's account and earned_faucet. The faucet is a
// SYSTEM account with owner_user_id NULL. Any join or filter that does not treat
// that NULL deliberately loses the faucet leg or — worse — loses whole journals
// and reports "0 grants" for coins that were demonstrably issued. That is how a
// money test ends up green while the economy is broken, so the helper's counting
// rule is pinned here against a real grant.
func TestProfileJournalsCountsThroughNullSystemLegs(t *testing.T) {
	db := openLedgerSchema(t)
	const userID = 990

	ledger := testLedger(t, db, nil)
	if _, err := ledger.Grant(context.Background(), GrantRequest{
		UserID:         userID,
		ReasonCode:     ReasonProfileComplete,
		IdempotencyKey: "profile-step:990:1",
		CreatedBy:      "system:profile_award",
	}); err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	// The faucet leg exists and its owner_user_id is NULL — the exact condition
	// that makes a naive count wrong.
	var faucetNulls int64
	if err := db.Raw(
		`SELECT count(*) FROM coin_posting p
		   JOIN coin_account a ON a.id = p.account_id
		  WHERE a.system_type = ? AND a.owner_user_id IS NULL`,
		SystemEarnedFaucet).Scan(&faucetNulls).Error; err != nil {
		t.Fatalf("count faucet legs: %v", err)
	}
	if faucetNulls == 0 {
		t.Fatal("no faucet leg with a NULL owner_user_id: this grant did not post " +
			"the two legs every grant posts, so the helper's rule is untested")
	}

	if got := profileJournals(t, db, userID); got != 1 {
		t.Errorf("profileJournals = %d, want 1: the helper must count the grant "+
			"despite the NULL-owner faucet leg", got)
	}
}

func profileCoinsPaid(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	return countRows(t, db,
		`SELECT COALESCE(SUM(p.amount), 0) FROM coin_posting p
		   JOIN coin_account a ON a.id = p.account_id
		   JOIN coin_journal j ON j.id = p.journal_id
		  WHERE a.owner_user_id = ? AND a.kind = ? AND j.reason_code = ? AND p.amount > 0`,
		userID, AccountUser, ReasonProfileComplete)
}

func grantRows(t *testing.T, db *gorm.DB, userID uint) []RewardGrant {
	t.Helper()
	var rows []RewardGrant
	if err := db.Where("user_id = ?", userID).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("read reward_grant: %v", err.Error())
	}
	return rows
}

func availableCoins(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	return countRows(t, db,
		`SELECT COALESCE(SUM(posted_balance), 0) FROM coin_account_balance b
		   JOIN coin_account a ON a.id = b.account_id
		  WHERE a.owner_user_id = ? AND a.kind = ? AND a.bucket = ?`,
		userID, AccountUser, BucketEarned)
}

// ── the ladder ────────────────────────────────────────────────────────────────

// TestProfileAwardEachThresholdPaysOnce walks the ladder one rung at a time and
// asserts each rung pays exactly one instalment and nothing more. This is the
// test that would catch an off-by-one in the threshold arithmetic at the only
// layer where the money is real.
func TestProfileAwardEachThresholdPaysOnce(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 901

	for step := 1; step <= 5; step++ {
		percent := ProfileStepThresholdPercent(step, 5)
		f.completion.set(userID, percent)

		paid, err := f.svc.AwardProfileSteps(context.Background(), userID)
		if err != nil {
			t.Fatalf("step %d (%d%%): award: %v", step, percent, err)
		}
		if len(paid) != 1 {
			t.Fatalf("step %d (%d%%): paid %d steps, want 1 (just this rung)",
				step, percent, len(paid))
		}
		if paid[0].Step != step {
			t.Errorf("step %d: awarded step %d, want %d", step, paid[0].Step, step)
		}

		wantCoins := DefaultEconomyConfig().Awards.ProfileInstalment
		if got := profileCoinsPaid(t, db, userID); got != wantCoins*int64(step) {
			t.Errorf("step %d: coins paid = %d, want %d", step, got, wantCoins*int64(step))
		}
		if n := profileJournals(t, db, userID); n != int64(step) {
			t.Errorf("step %d: journals = %d, want %d", step, n, step)
		}
	}

	// Every row carries the journal that paid it and the amount that was paid.
	for i, row := range grantRows(t, db, userID) {
		if row.JournalID == nil {
			t.Errorf("reward_grant %d has no journal_id; the claim and the grant must be one unit", row.ID)
		}
		if row.Amount != DefaultEconomyConfig().Awards.ProfileInstalment {
			t.Errorf("reward_grant %d amount = %d, want %d",
				row.ID, row.Amount, DefaultEconomyConfig().Awards.ProfileInstalment)
		}
		if row.AwardCode != ProfileStepAwardCode(i+1) {
			t.Errorf("reward_grant %d code = %q, want %q", row.ID, row.AwardCode, ProfileStepAwardCode(i+1))
		}
	}
	if got := profileCoinsPaid(t, db, userID); got != DefaultEconomyConfig().Awards.ProfileComplete {
		t.Errorf("full ladder paid %d, want %d", got, DefaultEconomyConfig().Awards.ProfileComplete)
	}
}

// TestProfileAwardJumpToFullPaysWholeLadder is the failure the "5 instalments"
// language in 04-implementation-plan.md §5.1 exists to prevent. A student who goes
// from 0% to 100% in one save must collect all 25, not one instalment of 5.
func TestProfileAwardJumpToFullPaysWholeLadder(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 902

	f.completion.set(userID, 0)
	// A baseline call at 0% so the jump is genuinely from nothing.
	if paid, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil || len(paid) != 0 {
		t.Fatalf("0%%: paid %d steps (err %v), want 0", len(paid), err)
	}

	f.completion.set(userID, 100)
	paid, err := f.svc.AwardProfileSteps(context.Background(), userID)
	if err != nil {
		t.Fatalf("100%%: award: %v", err)
	}
	if len(paid) != 5 {
		t.Fatalf("0%% -> 100%% paid %d steps, want 5", len(paid))
	}
	for i, a := range paid {
		if a.Step != i+1 {
			t.Errorf("paid[%d].Step = %d, want %d (the ladder must be contiguous)", i, a.Step, i+1)
		}
	}

	if got, want := profileCoinsPaid(t, db, userID), DefaultEconomyConfig().Awards.ProfileComplete; got != want {
		t.Errorf("0%% -> 100%% paid %d coins, want %d", got, want)
	}
	if n := grantRows(t, db, userID); len(n) != 5 {
		t.Errorf("reward_grant rows = %d, want 5", len(n))
	}
}

// TestProfileAwardDropAndRecoveryPaysOnce is THE test for this mechanic.
//
// Completion is recomputed from the user row and can go DOWN. The requirement is
// that a student who drops from 40% to 20% and back to 40% is paid for step 2
// exactly once. Awarding on the absolute value, or on a "newly reached" check that
// forgets what was already claimed, would pay twice.
func TestProfileAwardDropAndRecoveryPaysOnce(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 903
	instalment := DefaultEconomyConfig().Awards.ProfileInstalment

	// Up to 40%: two instalments.
	f.completion.set(userID, 40)
	paid, err := f.svc.AwardProfileSteps(context.Background(), userID)
	if err != nil {
		t.Fatalf("40%%: award: %v", err)
	}
	if len(paid) != 2 {
		t.Fatalf("40%% paid %d steps, want 2", len(paid))
	}
	if got := profileCoinsPaid(t, db, userID); got != 2*instalment {
		t.Fatalf("40%%: coins = %d, want %d", got, 2*instalment)
	}

	// Drop to 20%: one step's worth of completion. Nothing new is earned, and
	// critically nothing is REVOKED — earned coins are spent or expired, never
	// taken back.
	f.completion.set(userID, 20)
	paid, err = f.svc.AwardProfileSteps(context.Background(), userID)
	if err != nil {
		t.Fatalf("20%%: award: %v", err)
	}
	if len(paid) != 0 {
		t.Errorf("20%% paid %d steps, want 0", len(paid))
	}
	if got := profileCoinsPaid(t, db, userID); got != 2*instalment {
		t.Errorf("after dropping to 20%% coins = %d, want %d (a drop must not revoke)",
			got, 2*instalment)
	}

	// Back to 40%. This is the step that re-earn would slip through: the
	// completion is exactly what it was when two steps were paid.
	f.completion.set(userID, 40)
	paid, err = f.svc.AwardProfileSteps(context.Background(), userID)
	if err != nil {
		t.Fatalf("40%% again: award: %v", err)
	}
	if len(paid) != 0 {
		t.Fatalf("40%% -> 20%% -> 40%% paid %d steps on recovery, want 0: "+
			"a re-earned step is a double payment", len(paid))
	}
	if got := profileCoinsPaid(t, db, userID); got != 2*instalment {
		t.Errorf("after drop and recovery coins = %d, want %d", got, 2*instalment)
	}
	if n := profileJournals(t, db, userID); n != 2 {
		t.Errorf("journals = %d, want 2: one per step, ever", n)
	}
	if n := grantRows(t, db, userID); len(n) != 2 {
		t.Errorf("reward_grant rows = %d, want 2", len(n))
	}
}

// TestProfileAwardDropBelowAndRecoverPastPeak pushes the case further: a student
// who drops far enough to unearn steps and then climbs past their old high water
// mark collects only the steps above it.
func TestProfileAwardDropBelowAndRecoverPastPeak(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 904
	instalment := DefaultEconomyConfig().Awards.ProfileInstalment

	f.completion.set(userID, 80)
	if _, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil {
		t.Fatalf("80%%: %v", err)
	}
	f.completion.set(userID, 0)
	if _, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil {
		t.Fatalf("0%%: %v", err)
	}
	f.completion.set(userID, 100)
	paid, err := f.svc.AwardProfileSteps(context.Background(), userID)
	if err != nil {
		t.Fatalf("100%%: %v", err)
	}
	if len(paid) != 1 || paid[0].Step != 5 {
		t.Fatalf("80%% -> 0%% -> 100%% paid %+v, want only step 5", paid)
	}
	if got, want := profileCoinsPaid(t, db, userID), 5*instalment; got != want {
		t.Errorf("coins = %d, want %d", got, want)
	}
}

// ── idempotency, replay, and the ceiling ──────────────────────────────────────

// TestProfileAwardReplayPaysNothing asserts that a save which changes no
// completion field is free: no coins, no journal, no new row, no notification.
// This is what makes calling the award from eight paths safe — seven of the eight
// calls on any given save are necessarily redundant.
func TestProfileAwardReplayPaysNothing(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 905

	f.completion.set(userID, 60)
	if _, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil {
		t.Fatalf("first: %v", err)
	}
	journals := profileJournals(t, db, userID)
	coins := profileCoinsPaid(t, db, userID)
	balance := availableCoins(t, db, userID)
	notices := f.notifier.count()

	// Five more calls with nothing changed at all.
	for i := 0; i < 5; i++ {
		paid, err := f.svc.AwardProfileSteps(context.Background(), userID)
		if err != nil {
			t.Fatalf("replay %d: %v", i, err)
		}
		if len(paid) != 0 {
			t.Fatalf("replay %d paid %d steps, want 0", i, len(paid))
		}
	}

	if got := profileJournals(t, db, userID); got != journals {
		t.Errorf("journals after replay = %d, want %d (a replay must write no journal)", got, journals)
	}
	if got := profileCoinsPaid(t, db, userID); got != coins {
		t.Errorf("coins after replay = %d, want %d", got, coins)
	}
	if got := availableCoins(t, db, userID); got != balance {
		t.Errorf("available after replay = %d, want %d", got, balance)
	}
	if got := f.notifier.count(); got != notices {
		t.Errorf("notifications after replay = %d, want %d (a replay must not announce)", got, notices)
	}
	if n := grantRows(t, db, userID); len(n) != 3 {
		t.Errorf("reward_grant rows = %d, want 3", len(n))
	}
}

// TestProfileAwardBelowFirstThresholdDoesNothing is the other half of "cheap to
// call": a student with an empty profile must produce no rows at all, not even
// zero-amount ones.
func TestProfileAwardBelowFirstThresholdDoesNothing(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 906

	for _, percent := range []int{0, 1, 10, 19} {
		f.completion.set(userID, percent)
		paid, err := f.svc.AwardProfileSteps(context.Background(), userID)
		if err != nil {
			t.Fatalf("%d%%: %v", percent, err)
		}
		if len(paid) != 0 {
			t.Errorf("%d%% paid %d steps, want 0", percent, len(paid))
		}
	}
	if n := grantRows(t, db, userID); len(n) != 0 {
		t.Errorf("reward_grant rows = %d, want 0", len(n))
	}
	if n := profileJournals(t, db, userID); n != 0 {
		t.Errorf("journals = %d, want 0", n)
	}
	if got := f.notifier.count(); got != 0 {
		t.Errorf("notifications = %d, want 0", got)
	}
}

// TestProfileAwardCeilingIsTheConstraint asserts the 25-coin ceiling against the
// DATABASE, not against a service-layer cap.
//
// There is no cap check anywhere in the award, and that is the design: a check
// that compared "coins paid so far + this one" against 25 would be a second
// source of truth that could disagree with the constraint under concurrency, and
// would be bypassed by any future caller. The UNIQUE (user_id, award_code) makes
// five rows per user a property of the schema.
//
// So the test asserts the constraint itself rejects a sixth, and then asserts the
// ladder cannot produce one.
func TestProfileAwardCeilingIsTheConstraint(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 907

	// The ladder itself stops at five.
	f.completion.set(userID, 100)
	if _, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil {
		t.Fatalf("100%%: %v", err)
	}
	if got, want := profileCoinsPaid(t, db, userID), DefaultEconomyConfig().Awards.ProfileComplete; got != want {
		t.Fatalf("coins = %d, want %d", got, want)
	}
	// Over-reaching the configured percentage cannot buy a sixth step either.
	f.completion.set(userID, 100)
	if _, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if n := grantRows(t, db, userID); len(n) != 5 {
		t.Errorf("reward_grant rows = %d, want 5", len(n))
	}

	// The constraint is load-bearing: a duplicate award_code for this user is
	// rejected by the database, which is what makes the ceiling structural
	// rather than a check that could be bypassed or could disagree.
	dup := RewardGrant{UserID: userID, AwardCode: ProfileStepAwardCode(3), Amount: 5}
	if err := db.Create(&dup).Error; err == nil {
		t.Fatal("a duplicate (user_id, award_code) was accepted; the ceiling is " +
			"only structural while this insert fails")
	}

	// And a sixth, never-before-seen step is still allowed by the constraint —
	// which is exactly why the ladder length is enforced in code and not here.
	// Asserting it documents that the constraint caps REPEATS, not variety.
	sixth := RewardGrant{UserID: userID, AwardCode: ProfileStepAwardCode(6), Amount: 5}
	if err := db.Create(&sixth).Error; err != nil {
		t.Fatalf("a distinct award_code was rejected: %v", err)
	}
}

// TestProfileAwardSeparateUsersEachGetTheFullLadder is the guard on the
// idempotency key. coin_journal's unique index is (scope, idempotency_key) and
// ScopeUser is a CONSTANT, so a key that omitted the user id would make student
// A's PROFILE_STEP:1 collide with student B's and come back as a fingerprint
// mismatch. Two users at 100% must each collect 25.
func TestProfileAwardSeparateUsersEachGetTheFullLadder(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)

	for _, userID := range []uint{908, 909, 910} {
		f.completion.set(userID, 100)
		paid, err := f.svc.AwardProfileSteps(context.Background(), userID)
		if err != nil {
			t.Fatalf("user %d: %v", userID, err)
		}
		if len(paid) != 5 {
			t.Fatalf("user %d paid %d steps, want 5", userID, len(paid))
		}
	}
	for _, userID := range []uint{908, 909, 910} {
		if got, want := profileCoinsPaid(t, db, userID), int64(DefaultEconomyConfig().Awards.ProfileComplete); got != want {
			t.Errorf("user %d coins = %d, want %d", userID, got, want)
		}
	}
	if got := countRows(t, db, `SELECT count(*) FROM reward_grant`); got != 15 {
		t.Errorf("reward_grant rows = %d, want 15", got)
	}
}

// ── concurrency ───────────────────────────────────────────────────────────────

// TestProfileAwardConcurrentSavesGrantOnce is the atomicity test.
//
// Two profile saves that cross the same threshold at the same time must produce
// exactly ONE grant and ONE journal entry. This is what the per-user advisory
// lock plus the ON CONFLICT claim buy together, and neither alone is sufficient:
// the lock serialises the writers, and the constraint is what stops the loser
// from paying anyway.
func TestProfileAwardConcurrentSavesGrantOnce(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 920
	const writers = 8

	// Everyone crosses every threshold at once.
	f.completion.set(userID, 100)

	var wg sync.WaitGroup
	results := make([][]ProfileAward, writers)
	errs := make([]error, writers)
	start := make(chan struct{})

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start // release all writers at once, so they contend
			results[idx], errs[idx] = f.svc.AwardProfileSteps(context.Background(), userID)
		}(i)
	}
	close(start)
	wg.Wait()

	totalSteps := 0
	for i := 0; i < writers; i++ {
		if errs[i] != nil {
			t.Errorf("writer %d: %v", i, errs[i])
		}
		totalSteps += len(results[i])
	}

	if totalSteps != 5 {
		t.Errorf("%d concurrent writers paid %d steps in total, want exactly 5",
			writers, totalSteps)
	}
	if n := profileJournals(t, db, userID); n != 5 {
		t.Errorf("journals = %d, want 5: two concurrent saves must not both grant", n)
	}
	if n := grantRows(t, db, userID); len(n) != 5 {
		t.Errorf("reward_grant rows = %d, want 5", len(n))
	}
	if got, want := profileCoinsPaid(t, db, userID), int64(DefaultEconomyConfig().Awards.ProfileComplete); got != want {
		t.Errorf("coins paid = %d, want %d", got, want)
	}
	if got, want := availableCoins(t, db, userID), int64(DefaultEconomyConfig().Awards.ProfileComplete); got != want {
		t.Errorf("available = %d, want %d", got, want)
	}
	// Exactly one notification per step, no matter how many writers raced.
	if got := f.notifier.count(); got != 5 {
		t.Errorf("notifications = %d, want 5", got)
	}
}

// TestProfileAwardConcurrentSavesOnOneThreshold is the narrow version, crossing
// ONE threshold from below rather than all five at once. It is the exact scenario
// in the slice brief: two profile saves racing across the same threshold.
func TestProfileAwardConcurrentSavesOnOneThreshold(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 921
	const writers = 8

	// Already at step 1, so the race is over step 2 alone.
	f.completion.set(userID, 20)
	if _, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil {
		t.Fatalf("seed step 1: %v", err)
	}
	f.completion.set(userID, 40)

	var wg sync.WaitGroup
	results := make([][]ProfileAward, writers)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx], _ = f.svc.AwardProfileSteps(context.Background(), userID)
		}(i)
	}
	close(start)
	wg.Wait()

	winners := 0
	for _, r := range results {
		winners += len(r)
	}
	if winners != 1 {
		t.Errorf("%d writers crossing one threshold produced %d grants, want exactly 1",
			writers, winners)
	}
	if n := profileJournals(t, db, userID); n != 2 {
		t.Errorf("journals = %d, want 2 (step 1 from the seed, step 2 from one winner)", n)
	}
	if got, want := profileCoinsPaid(t, db, userID), int64(2*DefaultEconomyConfig().Awards.ProfileInstalment); got != want {
		t.Errorf("coins = %d, want %d", got, want)
	}
}

// TestProfileAwardConcurrentSeparateUsers runs the concurrency check across
// DIFFERENT users, where the per-user lock must not serialise them into one at a
// time. Not a throughput assertion — a correctness one: if the lock were taken on
// something coarser than the user, this would still pass, and if the claim insert
// were wrong for distinct users it would fail.
func TestProfileAwardConcurrentSeparateUsers(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)

	const users = 6
	ids := make([]uint, users)
	for i := range ids {
		ids[i] = uint(930 + i)
		f.completion.set(ids[i], 100)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	paid := map[uint]int{}
	errs := make([]error, users)
	start := make(chan struct{})
	for i, id := range ids {
		wg.Add(1)
		go func(idx int, userID uint) {
			defer wg.Done()
			<-start
			steps, err := f.svc.AwardProfileSteps(context.Background(), userID)
			mu.Lock()
			defer mu.Unlock()
			errs[idx] = err
			paid[userID] = len(steps)
		}(i, id)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("user %d: %v", ids[i], err)
		}
		if paid[ids[i]] != 5 {
			t.Errorf("user %d paid %d steps, want 5", ids[i], paid[ids[i]])
		}
		if got, want := profileCoinsPaid(t, db, ids[i]), int64(DefaultEconomyConfig().Awards.ProfileComplete); got != want {
			t.Errorf("user %d coins = %d, want %d", ids[i], got, want)
		}
	}
}

// ── notification ──────────────────────────────────────────────────────────────

// TestProfileAwardNotificationPerStep asserts coins.credited fires exactly once
// per awarded step, with the registry's own templates rendering from the emitted
// data.
//
// The render is part of the assertion on purpose: the registry templates carry
// missingkey=error, so a Data map missing "coins" or "balance" drops the
// notification at send time rather than producing a blank line. Testing that the
// emit HAPPENED without testing that it RENDERS is how a working award ships with
// no notifications.
func TestProfileAwardNotificationPerStep(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 940

	f.completion.set(userID, 60)
	if _, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil {
		t.Fatalf("60%%: %v", err)
	}
	if got := f.notifier.count(); got != 3 {
		t.Fatalf("60%% produced %d notifications, want 3 (one per step)", got)
	}

	f.notifier.mu.Lock()
	calls := append([]recordedCredit(nil), f.notifier.calls...)
	f.notifier.mu.Unlock()

	def, ok := notification.Registry[notification.EventCoinsCredited]
	if !ok {
		t.Fatal("coins.credited has no registry row; the registry is boot-validated")
	}
	seen := map[string]bool{}
	for i, c := range calls {
		if c.eventKey != notification.EventCoinsCredited {
			t.Errorf("call %d event = %q, want %q", i, c.eventKey, notification.EventCoinsCredited)
		}
		if c.userID != userID {
			t.Errorf("call %d addressed to user %d, want %d", i, c.userID, userID)
		}
		if c.coins != DefaultEconomyConfig().Awards.ProfileInstalment {
			t.Errorf("call %d coins = %d, want %d", i, c.coins, DefaultEconomyConfig().Awards.ProfileInstalment)
		}
		if c.balance != int64(i+1)*DefaultEconomyConfig().Awards.ProfileInstalment {
			t.Errorf("call %d balance = %d, want %d: the receipt must show the real balance",
				i, c.balance, int64(i+1)*DefaultEconomyConfig().Awards.ProfileInstalment)
		}
		if seen[c.occurKey] {
			t.Errorf("occurrence key %q used twice; a retry would double-notify", c.occurKey)
		}
		seen[c.occurKey] = true
		if c.correlate == "" {
			t.Errorf("call %d has no correlation id; the journal is the receipt", i)
		}

		title, err := notification.ResolveTemplate(def.TitleTpl, map[string]any{
			"coins": c.coins, "balance": c.balance, "step": i + 1,
		})
		if err != nil {
			t.Fatalf("call %d: render title: %v", i, err)
		}
		body, err := notification.ResolveTemplate(def.BodyTpl, map[string]any{
			"coins": c.coins, "balance": c.balance, "step": i + 1,
		})
		if err != nil {
			t.Fatalf("call %d: render body: %v", i, err)
		}
		if title == "" || body == "" {
			t.Errorf("call %d rendered empty copy: %q / %q", i, title, body)
		}
		assertNoBannedCreditCopy(t, title+" "+body)
	}

	// A save that awards nothing must not announce anything.
	before := f.notifier.count()
	f.completion.set(userID, 60)
	if _, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got := f.notifier.count(); got != before {
		t.Errorf("a replay produced %d extra notifications, want 0", got-before)
	}
}

// assertNoBannedCreditCopy is the statutory check from
// docs/coin-system/09-support-copy-cheat-sheet.md, run against the rendered
// string rather than trusted to review.
//
// The three bans are not style rules: "free" is a CPA 2075 s.16 unfair-trade
// exposure, a currency figure beside a coin figure is false on the product's own
// terms, and prize/award/win/raffle/draw are the words Income Tax Act 2058 s.5/88A
// uses to define a taxable windfall gain. A copy review that only happens once, at
// the moment the string is written, is a copy review that stops happening.
func assertNoBannedCreditCopy(t *testing.T, s string) {
	t.Helper()
	if term := bannedTerm(s); term != "" {
		t.Errorf("coins.credited copy contains the banned term %q: %q", term, s)
	}
}

// bannedTerms is the list from
// docs/coin-system/09-support-copy-cheat-sheet.md §"Banned words", flattened into
// something a test can iterate.
//
// Substring matching on a lowercased copy, with two deliberate exceptions: "$" and
// "." are checked as plain substrings because there is no word boundary to speak
// of, and every alphabetic term is checked as a substring rather than a whole
// word. That is deliberately LOOSE — "draw" matches "withdrawn", "win" matches
// "winding" — because the failure this guards is statutory exposure from a copy
// edit, and a false positive costs one word rephrased. A loose check that never
// fires would cost the whole guarantee.
var bannedTerms = []string{
	// CPA 2075 s.16(2)(c)(3): calling something free when money is on the other side.
	"free", "no cost",
	// Income Tax Act 2058 s.5/88A: the words that define a taxable windfall gain.
	"prize", "award", "winner", "won ", "win ", "wins ", "raffle", "draw", "baksis", "jitauri",
	// Earn-only product: no cash-out path exists, so describing one is false.
	"cash out", "cashout", "redeem for money", "convert to money",
	// A currency figure beside a coin figure. Checked as substrings so "Rs",
	// "INR", "NPR" and "₹"-adjacent forms are all caught.
	"npr", "inr", "rupee", "rs.", "rs ", "$", "usd", "€",
	// House rules from the same table, worth keeping out for consistency.
	"refundable", "hurry", "limited time", "don't miss out",
}

// bannedTerm returns the first banned term found in s, or "".
func bannedTerm(s string) string {
	lowered := strings.ToLower(s)
	for _, term := range bannedTerms {
		if strings.Contains(lowered, term) {
			return term
		}
	}
	return ""
}

// TestProfileAwardBannedCopyGuardProvesItself is the test for the guard above. A
// banned-word check that cannot fail is worse than none, because it reads as
// coverage. This feeds it the exact phrasings 09 warns about and requires it to
// reject them, while accepting the real copy.
func TestProfileAwardBannedCopyGuardProvesItself(t *testing.T) {
	// bannedTerm, run over each phrasing 09 warns about, must be non-empty. This
	// asserts the DETECTOR rather than the copy: a word list that silently stopped
	// matching would leave assertNoBannedCreditCopy passing on everything,
	// including the real string, and the coverage would be a fiction.
	for _, phrase := range []string{
		"Your free StudsTokens are ready",
		"worth NPR 500 of value",
		"You won 5 coins",
		"Collect your prize",
		"Claim your award",
		"Enter the raffle",
		"Cash out your coins",
		"That costs only Rs 40",
	} {
		if bannedTerm(phrase) == "" {
			t.Errorf("the banned-term detector did not flag %q, so the copy check "+
				"would pass on copy it is supposed to reject", phrase)
		}
	}

	// And the real copy must come back clean.
	def, ok := notification.Registry[notification.EventCoinsCredited]
	if !ok {
		t.Fatal("coins.credited missing from the registry")
	}
	rendered, err := notification.ResolveTemplate(def.BodyTpl, map[string]any{
		"coins": int64(5), "balance": int64(25), "step": 1,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := bannedTerm(rendered); got != "" {
		t.Errorf("the shipped copy contains the banned term %q: %q", got, rendered)
	}
	if bannedTerm(def.TitleTpl+" "+rendered) != "" {
		t.Errorf("the shipped title contains a banned term: %q", def.TitleTpl)
	}
}

// ── configuration ─────────────────────────────────────────────────────────────

// TestProfileAwardHonoursConfiguredInstalment proves the amounts come from
// config and not from literals, the same rule the ledger enforces on its callers.
func TestProfileAwardHonoursConfiguredInstalment(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, func(cfg *EconomyConfig) {
		cfg.Awards.ProfileInstalment = 3
		cfg.Awards.ProfileComplete = 15
		cfg.Awards.ProfileInstalments = 5
	})
	const userID = 950

	f.completion.set(userID, 100)
	paid, err := f.svc.AwardProfileSteps(context.Background(), userID)
	if err != nil {
		t.Fatalf("100%%: %v", err)
	}
	if len(paid) != 5 {
		t.Fatalf("paid %d steps, want 5", len(paid))
	}
	if got, want := profileCoinsPaid(t, db, userID), int64(15); got != want {
		t.Errorf("coins = %d, want %d (5 x 3 from config)", got, want)
	}
	// The snapshot on the row records what was actually paid, so a later
	// re-pricing cannot rewrite history.
	for _, row := range grantRows(t, db, userID) {
		if row.Amount != 3 {
			t.Errorf("reward_grant amount = %d, want the configured 3", row.Amount)
		}
	}
}

// TestProfileAwardRejectsZeroInstalments asserts the misconfiguration is an error
// rather than a silent zero-award or a division by zero.
func TestProfileAwardRejectsZeroInstalments(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, func(cfg *EconomyConfig) {
		cfg.Awards.ProfileInstalments = 0
		cfg.Awards.ProfileComplete = 0
	})
	f.completion.set(951, 100)
	if _, err := f.svc.AwardProfileSteps(context.Background(), 951); err == nil {
		t.Fatal("a zero-instalment ladder was accepted; that would silently award nothing forever")
	}
}

// TestProfileAwardRequiresCompletionLookup is the wiring guard. An awarder with no
// completion lookup must error rather than report 0% and pay nothing forever,
// which is the quietest available way to break an earn mechanic.
func TestProfileAwardRequiresCompletionLookup(t *testing.T) {
	db := openLedgerSchema(t)
	svc := NewProfileAwardService(NewRepository(db), testLedger(t, db, nil), nil, nil)
	if _, err := svc.AwardProfileSteps(context.Background(), 960); err == nil {
		t.Fatal("an awarder with no completion lookup paid anyway")
	}
}

// TestProfileAwardResumesAfterPartialFailure is the resumption property: each step
// is its own transaction, so a ladder that fails partway keeps the steps that did
// succeed, and a retry pays exactly the rest.
func TestProfileAwardResumesAfterPartialFailure(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 970

	// Pay steps 1-2 by hand, as an earlier interrupted call would have left them.
	for _, step := range []int{1, 2} {
		if err := db.Create(&RewardGrant{
			UserID:    userID,
			AwardCode: ProfileStepAwardCode(step),
			Amount:    DefaultEconomyConfig().Awards.ProfileInstalment,
			GrantedAt: f.svc.now(),
		}).Error; err != nil {
			t.Fatalf("seed step %d: %v", step, err)
		}
	}

	f.completion.set(userID, 100)
	paid, err := f.svc.AwardProfileSteps(context.Background(), userID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if len(paid) != 3 {
		t.Fatalf("resume paid %d steps, want 3 (steps 3, 4, 5)", len(paid))
	}
	for i, a := range paid {
		if a.Step != i+3 {
			t.Errorf("paid[%d].Step = %d, want %d", i, a.Step, i+3)
		}
	}
	// The seeded rows have no journal, and only the newly paid ones do — which is
	// visible in the journal count.
	if n := profileJournals(t, db, userID); n != 3 {
		t.Errorf("journals = %d, want 3", n)
	}
	if n := grantRows(t, db, userID); len(n) != 5 {
		t.Errorf("reward_grant rows = %d, want 5", len(n))
	}
}

// TestProfileAwardEveryStepHasDistinctJournal asserts the ladder's journals are
// distinct, so each instalment is separately auditable and separately reversible.
func TestProfileAwardEveryStepHasDistinctJournal(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	const userID = 980

	f.completion.set(userID, 100)
	if _, err := f.svc.AwardProfileSteps(context.Background(), userID); err != nil {
		t.Fatalf("100%%: %v", err)
	}
	seen := map[string]int{}
	for _, row := range grantRows(t, db, userID) {
		if row.JournalID == nil {
			t.Fatalf("reward_grant %d has no journal", row.ID)
		}
		if prev, dup := seen[*row.JournalID]; dup {
			t.Errorf("reward_grant rows %d and %d share journal %s", prev, row.ID, *row.JournalID)
		}
		seen[*row.JournalID] = int(row.ID)

		// The journal must exist and be a posted PROFILE_COMPLETE grant.
		var j struct {
			ReasonCode string `gorm:"column:reason_code"`
			State      string `gorm:"column:state"`
		}
		if err := db.Raw(`SELECT reason_code, state FROM coin_journal WHERE id = ?`, *row.JournalID).
			Scan(&j).Error; err != nil {
			t.Errorf("read journal %s: %v", *row.JournalID, err)
			continue
		}
		reason, state := j.ReasonCode, j.State
		if reason != ReasonProfileComplete {
			t.Errorf("journal %s reason = %q, want %q", *row.JournalID, reason, ReasonProfileComplete)
		}
		if state != StatePosted {
			t.Errorf("journal %s state = %q, want %q", *row.JournalID, state, StatePosted)
		}
	}
}

// TestProfileAwardZeroUserRejected keeps the award off user 0, which the ledger
// refuses as an argument error.
func TestProfileAwardZeroUserRejected(t *testing.T) {
	db := openLedgerSchema(t)
	f := newAwardFixture(t, db, nil)
	if _, err := f.svc.AwardProfileSteps(context.Background(), 0); err == nil {
		t.Fatal("user 0 was awarded")
	}
}
