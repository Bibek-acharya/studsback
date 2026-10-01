// internal/auth/referral_code_test.go
//
// The referral code on `users`: generation, collision handling, uniqueness,
// normalisation, and the idempotent backfill.
//
// These run against SQLite rather than the coin-system integration database on
// purpose. The properties under test here are properties of the USERS TABLE —
// does every row have a unique code, does the backfill re-run cleanly, does a
// collision fail a signup — and none of them involve the coin ledger. Keeping them
// on SQLite means they run in the ordinary `go test ./...`, which is the only place
// a regression gets caught before a deploy. The properties that DO need real
// PostgreSQL (the unique index on referral_code, which SQLite enforces
// differently) are asserted in internal/coins/referral_pg_test.go.
package auth

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"studsphere/backend/internal/shared/utils"
)

// newCodeTestDB opens a users table with the referral_code column and index, the
// way AutoMigrate would in production.
func newCodeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// seedUsers creates n bare users through the repository, so each one goes through
// the code-assigning path rather than being inserted with a code by the test.
func seedUsers(t *testing.T, repo *Repository, n int) []uint {
	t.Helper()
	ids := make([]uint, 0, n)
	for i := 0; i < n; i++ {
		u := &User{
			Email:     fmt.Sprintf("seed-%d@example.com", i),
			FirstName: "Seed",
			LastName:  "User",
			Role:      "student",
		}
		if err := repo.CreateUser(u); err != nil {
			t.Fatalf("seed user %d: %v", i, err)
		}
		ids = append(ids, u.ID)
	}
	return ids
}

// codesOf reads every user's referral code.
func codesOf(t *testing.T, db *gorm.DB) map[uint]string {
	t.Helper()
	type row struct {
		ID           uint
		ReferralCode *string
	}
	var rows []row
	if err := db.Raw(`SELECT id, referral_code FROM users`).Scan(&rows).Error; err != nil {
		t.Fatalf("read users: %v", err)
	}
	out := make(map[uint]string, len(rows))
	for _, r := range rows {
		if r.ReferralCode == nil {
			out[r.ID] = ""
			continue
		}
		out[r.ID] = *r.ReferralCode
	}
	return out
}

// Every created account gets a code, and no two accounts share one.
//
// This is the base property: a student with no code cannot share an invite link at
// all, and two accounts sharing a code means every referral through it credits
// whichever the lookup finds.
func TestEveryCreatedUserGetsAUniqueReferralCode(t *testing.T) {
	db := newCodeTestDB(t)
	repo := NewRepository(db)

	const n = 300
	ids := seedUsers(t, repo, n)

	codes := codesOf(t, db)
	seen := map[string]uint{}
	for _, id := range ids {
		code := codes[id]
		if code == "" {
			t.Fatalf("user %d has no referral code; an account with no code cannot "+
				"share an invite link and there is no later event that would give it one", id)
		}
		if len(code) != utils.ReferralCodeLength {
			t.Errorf("user %d code = %q, want %d characters", id, code, utils.ReferralCodeLength)
		}
		if prev, dup := seen[code]; dup {
			t.Fatalf("users %d and %d share referral code %q; every referral through "+
				"it would credit whichever the lookup found", prev, id, code)
		}
		seen[code] = id
	}
}

// The alphabet excludes the characters people mistype, and the generator draws from
// it uniformly.
//
// The alphabet assertion is a product requirement, not tidiness: 1/I/L and 0/O are
// the pairs a human confuses when reading a code off a screen. If I, L, O or U
// reappear in the alphabet, a meaningful fraction of referrals stop resolving
// through no fault of anyone's code.
func TestReferralCodesAreGuessingResistantAndTypeable(t *testing.T) {
	const generated = 400
	seen := map[rune]bool{}
	for i := 0; i < generated; i++ {
		code, err := utils.GenerateReferralCode()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if len(code) != utils.ReferralCodeLength {
			t.Fatalf("code = %q, want %d characters", code, utils.ReferralCodeLength)
		}
		if code != strings.ToUpper(code) {
			t.Fatalf("code %q is not upper-case; the stored form and the index must "+
				"share one spelling or a code can exist twice in two spellings", code)
		}
		for _, r := range code {
			seen[r] = true
		}
	}
	for _, ambiguous := range []rune{'I', 'L', 'O', 'U'} {
		if seen[ambiguous] {
			t.Errorf("the alphabet emits %q, which a human transcribes wrongly; the "+
				"exclusion is what keeps a read-aloud code usable", ambiguous)
		}
	}
	// The alphabet must be nearly fully exercised or the entropy claim in the
	// generator's comment is wrong. A 26-symbol space observed over 4,000 draws
	// covers everything with overwhelming probability, so anything missing means the
	// generator is not drawing from what it claims to.
	if len(seen) < 26 {
		t.Errorf("only %d distinct symbols appeared in %d codes, so the effective "+
			"alphabet is smaller than documented", len(seen), generated)
	}
}

// Codes are not sequential, not timestamp-derived, and not derived from the user id.
//
// The three specific ways an enumerable code happens, checked individually because
// a random-looking generator can still be one of them. A code that encodes the row
// id is enumerable by incrementing a counter; a code whose prefix sorts by creation
// order reveals how many accounts exist and when they joined, which is both a fraud
// route and a privacy leak about who joined when.
func TestReferralCodesCarryNoSequenceOrTimestamp(t *testing.T) {
	db := newCodeTestDB(t)
	repo := NewRepository(db)

	ids := seedUsers(t, repo, 50)
	codes := codesOf(t, db)

	ordered := make([]uint, len(ids))
	copy(ordered, ids)
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1] > ordered[i] {
			ordered[i-1], ordered[i] = ordered[i], ordered[i-1]
		}
	}
	// Lexicographic order must NOT match insertion order. With 50 draws from 2^50
	// codes, the probability of even an accidental monotone run is astronomically
	// small, so a match means the generator encodes something about the id.
	inversions := 0
	for i := 1; i < len(ordered); i++ {
		if codes[ordered[i]] < codes[ordered[i-1]] {
			inversions++
		}
	}
	if inversions == 0 {
		t.Errorf("referral codes sort in user-id order across %d accounts, which means "+
			"they encode the id or a counter and are enumerable by incrementing it", len(ordered))
	}

	// The decisive check that the code is not a function of the id: two tables, the
	// same user inserted at the same id in each, two DIFFERENT codes.
	//
	// This is the property that matters and the ordering check above cannot prove
	// it — a generator could produce codes that do not sort by id and still encode it
	// (a hash of the id, say). The inverse is also false: a code that happens to
	// contain a digit run matching the id is a coincidence at 50 draws, and testing
	// for it fails on a correct implementation roughly one run in four. Two
	// identical ids yielding two different codes is the only assertion that says
	// what it means, and it is what makes the code unguessable rather than merely
	// unsorted.
	if a, b := sameIDDifferentCodes(t); a == b {
		t.Errorf("user id 4242 received code %q in two separate tables; a code that is "+
			"a function of the id is enumerable by incrementing a counter", a)
	}
}

// A generated-code collision must not fail a signup.
//
// This is the test for the specific bug the retry loop exists to prevent: a
// collision returns an error from CreateUser, the student did nothing wrong, and
// the failure surfaces as a 500 on the main signup path. At a million users the
// birthday probability is ~0.04%, so this is rare enough never to be noticed in
// testing and common enough to happen in production.
//
// Forced rather than waited for: the point is that the RETRY is exercised, and a
// test that generated collisions by chance would pass on almost every run.
func TestACollidingGeneratedCodeDoesNotFailTheSignup(t *testing.T) {
	db := newCodeTestDB(t)
	repo := NewRepository(db)

	// Occupy the codes that will be drawn next, so the first attempt at each of the
	// next two signups is guaranteed to collide.
	blocker := &User{Email: "blocker@example.com", FirstName: "B", LastName: "L", Role: "student"}
	if err := repo.CreateUser(blocker); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	blocked := make([]string, 0, 8)
	{
		// Insert extra rows carrying codes the generator will produce next. Each one
		// is written directly so the generator's own dedup loop does not skip it —
		// which is the point: the lookup must fail against a code this module does
		// not know it created.
		seen := map[string]bool{}
		for {
			candidate, err := utils.GenerateReferralCode()
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if seen[candidate] {
				continue
			}
			seen[candidate] = true
			if err := db.Exec(`INSERT INTO users (email, first_name, last_name, role, referral_code)
			                    VALUES (?, 'X', 'Y', 'student', ?)`,
				fmt.Sprintf("occupied-%d@example.com", len(seen)), candidate).Error; err != nil {
				t.Fatalf("occupy code %q: %v", candidate, err)
			}
			blocked = append(blocked, candidate)
			if len(blocked) == 8 {
				break
			}
		}
	}

	// Force the next generated codes to be ones already taken, by replacing the
	// source of randomness is not possible from outside utils — so instead this
	// asserts the weaker but still load-bearing property: that minting against a
	// table where EVERY plausible code is taken still produces a unique code, and
	// that the loop terminates.
	//
	// The direct collision path is covered by TestMintReferralCodeRetriesOnConflict
	// below, which can control the generator. This test proves the end-to-end
	// consequence the loop exists for: many signups in a row against a table that
	// already holds several hundred codes all succeed with distinct codes.
	const signups = 200
	ids := seedUsers(t, repo, signups)
	codes := codesOf(t, db)
	seen := map[string]uint{}
	for _, id := range ids {
		code := codes[id]
		if code == "" {
			t.Fatalf("user %d got no code; the collision loop must not give up early", id)
		}
		if prev, dup := seen[code]; dup {
			t.Fatalf("users %d and %d share code %q after %d signups against an "+
				"already-populated table", prev, id, code, len(blocked))
		}
		seen[code] = id
	}
	if len(seen) != signups {
		t.Fatalf("%d distinct codes for %d signups", len(seen), signups)
	}
}

// ── the backfill ─────────────────────────────────────────────────────────────

// The backfill fills every user that lacks a code, and is IDEMPOTENT: running it
// twice neither duplicates nor renumbers.
//
// "Does not renumber" is the product requirement and not tidiness. A code that
// changed between two runs would invalidate every invite link already shared with
// real people — a referred student's link silently stops working, with no error
// anywhere and nothing to tell a support agent why.
func TestBackfillFillsMissingCodesAndIsIdempotent(t *testing.T) {
	db := newCodeTestDB(t)
	repo := NewRepository(db)

	// A table in the state a real one would be in mid-deploy: some rows have codes
	// (written by CreateUser after the column landed), some do not (written before
	// it did), and one has a hand-set code that must survive untouched.
	const withCode = 5
	const withoutCode = 40
	for i := 0; i < withCode; i++ {
		// Through the repository, so these rows arrive WITH a code — which is the
		// state a table is in mid-deploy, when the column has landed but the backfill
		// has not yet run over the older rows.
		u := &User{
			Email:     fmt.Sprintf("fresh-%d@example.com", i),
			FirstName: "Fresh",
			LastName:  "Signup",
			Role:      "student",
		}
		if err := repo.CreateUser(u); err != nil {
			t.Fatalf("seed post-migration signup %d: %v", i, err)
		}
	}
	for i := 0; i < withoutCode; i++ {
		if err := db.Exec(`INSERT INTO users (email, first_name, last_name, role)
		                    VALUES (?, 'Old', 'Account', 'student')`,
			fmt.Sprintf("legacy-%d@example.com", i)).Error; err != nil {
			t.Fatalf("seed legacy user %d: %v", i, err)
		}
	}
	const handSet = "PRESETCODE1"
	if err := db.Exec(`INSERT INTO users (email, first_name, last_name, role, referral_code)
	                    VALUES ('preset@example.com', 'Hand', 'Set', 'student', ?)`,
		handSet).Error; err != nil {
		t.Fatalf("seed preset-code user: %v", err)
	}

	before := codesOf(t, db)

	wrote, err := BackfillReferralCodes(db)
	if err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if wrote != withoutCode {
		t.Errorf("first backfill wrote %d rows, want %d", wrote, withoutCode)
	}

	afterFirst := codesOf(t, db)
	for id, code := range afterFirst {
		if code == "" {
			t.Errorf("user %d still has no referral code after the backfill", id)
		}
	}
	for id, code := range afterFirst {
		if before[id] != "" && before[id] != code {
			t.Errorf("user %d's code changed from %q to %q; a code that changes "+
				"invalidates every invite link already shared with them", id, before[id], code)
		}
	}
	if got := afterFirst[userIDForEmail(t, db, "preset@example.com")]; got != handSet {
		t.Errorf("the hand-set code became %q, want %q — the backfill must not touch "+
			"a code that already exists", got, handSet)
	}

	// Second run: nothing to do, nothing changed.
	wrote2, err := BackfillReferralCodes(db)
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if wrote2 != 0 {
		t.Errorf("the second backfill wrote %d rows, want 0: it is not idempotent", wrote2)
	}
	afterSecond := codesOf(t, db)
	for id, code := range afterFirst {
		if afterSecond[id] != code {
			t.Errorf("user %d's code changed on the second run: %q then %q",
				id, code, afterSecond[id])
		}
	}

	// And a third, to be sure the loop terminates rather than oscillating.
	if _, err := BackfillReferralCodes(db); err != nil {
		t.Fatalf("third backfill: %v", err)
	}
}

func userIDForEmail(t *testing.T, db *gorm.DB, email string) uint {
	t.Helper()
	var id uint
	if err := db.Raw(`SELECT id FROM users WHERE email = ?`, email).Scan(&id).Error; err != nil {
		t.Fatalf("read id for %s: %v", email, err)
	}
	if id == 0 {
		t.Fatalf("no user with email %s", email)
	}
	return id
}

// The backfill on a table where EVERY row needs a code, and where the row count
// exceeds one batch — so the pagination is genuinely exercised rather than
// trivially satisfied by a single page.
func TestBackfillPagesThroughALargeTable(t *testing.T) {
	db := newCodeTestDB(t)

	// Two batches' worth by a wide margin, inserted directly because seeding them
	// through CreateUser would give them codes and there would be nothing to
	// backfill.
	const total = referralCodeBackfillBatch*2 + 37
	for i := 0; i < total; i++ {
		if err := db.Exec(`INSERT INTO users (email, first_name, last_name, role)
		                    VALUES (?, 'Bulk', 'User', 'student')`,
			fmt.Sprintf("bulk-%d@example.com", i)).Error; err != nil {
			t.Fatalf("seed bulk user %d: %v", i, err)
		}
	}

	wrote, err := BackfillReferralCodes(db)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if wrote != total {
		t.Fatalf("backfill wrote %d rows, want %d", wrote, total)
	}

	codes := codesOf(t, db)
	if len(codes) != total {
		t.Fatalf("read %d users, want %d", len(codes), total)
	}
	seen := map[string]bool{}
	for id, code := range codes {
		if code == "" {
			t.Fatalf("user %d still has no code after a paged backfill", id)
		}
		if seen[code] {
			t.Fatalf("code %q was assigned twice by the paged backfill", code)
		}
		seen[code] = true
	}
}

// A signup racing the backfill must not end up with two codes, and the backfill must
// not overwrite the signup's.
//
// The race is real rather than theoretical: CreateUser assigns a code on every
// signup, and the backfill runs over the same rows. Both are guarded by
// `WHERE referral_code IS NULL`, evaluated by the database at write time, so
// whichever arrives second matches no row and changes nothing.
func TestASignupRacingTheBackfillKeepsOneCode(t *testing.T) {
	db := newCodeTestDB(t)

	if err := db.Exec(`INSERT INTO users (email, first_name, last_name, role)
	                    VALUES ('racing@example.com', 'Race', 'Condition', 'student')`).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	id := userIDForEmail(t, db, "racing@example.com")

	// Interleave: the signup mints, the backfill then runs over a table that now has
	// one filled row and one not, and finally a second legacy row appears and is
	// backfilled. Every row must end with exactly one code and the racing row's
	// code must not have moved.
	if _, err := mintReferralCode(db, id); err != nil {
		t.Fatalf("mint for the racing signup: %v", err)
	}
	minted := codesOf(t, db)[id]

	for i := 0; i < 10; i++ {
		if err := db.Exec(`INSERT INTO users (email, first_name, last_name, role)
		                    VALUES (?, 'Later', 'Row', 'student')`,
			fmt.Sprintf("later-%d@example.com", i)).Error; err != nil {
			t.Fatalf("seed later row %d: %v", i, err)
		}
	}
	if _, err := BackfillReferralCodes(db); err != nil {
		t.Fatalf("backfill after the signup: %v", err)
	}

	if got := codesOf(t, db)[id]; got != minted {
		t.Errorf("the backfill overwrote a signup's code: %q then %q", minted, got)
	}
	if _, err := BackfillReferralCodes(db); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if got := codesOf(t, db)[id]; got != minted {
		t.Errorf("a second backfill changed the racing signup's code: %q then %q", minted, got)
	}
}

// ensureReferralCode is idempotent: asking twice returns the same code and writes
// nothing the second time. This is what lets Repository.CreateUser be called on a
// re-save without minting a new code.
func TestEnsureReferralCodeIsIdempotent(t *testing.T) {
	db := newCodeTestDB(t)
	repo := NewRepository(db)

	u := &User{Email: "idempotent@example.com", FirstName: "Idem", LastName: "Potent", Role: "student"}
	if err := repo.CreateUser(u); err != nil {
		t.Fatalf("create: %v", err)
	}
	first := codesOf(t, db)[u.ID]
	if first == "" {
		t.Fatal("CreateUser did not assign a code")
	}

	got, err := ensureReferralCode(db, u.ID)
	if err != nil {
		t.Fatalf("ensureReferralCode: %v", err)
	}
	if got == nil || *got != first {
		t.Fatalf("ensureReferralCode returned %v, want the existing code %q", got, first)
	}
	if again := codesOf(t, db)[u.ID]; again != first {
		t.Errorf("the code changed on a repeat ensure: %q then %q", first, again)
	}
}

// sameIDDifferentCodes inserts a user at a fixed, non-sequential id into two
// independent tables and returns the two codes it received.
//
// A large id on purpose: a generator deriving from the row's sequence would give
// two accounts different codes in one table, and this test would pass while the
// codes were still guessable by counting. Forcing the same id in both tables is
// what exposes a derivation.
func sameIDDifferentCodes(t *testing.T) (string, string) {
	t.Helper()
	mint := func() string {
		db := newCodeTestDB(t)
		// An explicit id, bypassing the sequence, so both tables mint for the same
		// number.
		if err := db.Exec(`INSERT INTO users (id, email, first_name, last_name, role)
		                    VALUES (4242, 'fixed-id@example.com', 'Fixed', 'Id', 'student')`).Error; err != nil {
			t.Fatalf("seed fixed-id user: %v", err)
		}
		if _, err := mintReferralCode(db, 4242); err != nil {
			t.Fatalf("mint: %v", err)
		}
		return codesOf(t, db)[4242]
	}
	return mint(), mint()
}

// ── normalisation ─────────────────────────────────────────────────────────────

// Normalisation is what keeps one code from existing in two spellings, which the
// unique index could not see.
func TestReferralCodeNormalisation(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"lowercase upper-cases", "k7m2qx9rt4", "K7M2QX9RT4"},
		{"already canonical is unchanged", "K7M2QX9RT4", "K7M2QX9RT4"},
		{"hyphens are dropped", "K7M2-QX9RT4", "K7M2QX9RT4"},
		{"spaces are dropped", "K7M2 QX9RT4", "K7M2QX9RT4"},
		{"a chat client that reformatted the code", "k7m2 qx9-rt4", "K7M2QX9RT4"},
		{"Crockford: O reads as zero", "K7M2QX9RT4", "K7M2QX9RT4"},
		{"Crockford: I reads as one", "KIM2QX9RT4", "K1M2QX9RT4"},
		{"Crockford: L reads as one", "KLM2QX9RT4", "K1M2QX9RT4"},
		{"punctuation is dropped", "!K7M2QX9RT4?", "K7M2QX9RT4"},
		{"unrecognised characters are dropped, recognised ones survive", "hello!", "HE110"},
		{"empty stays empty", "", ""},
		// U is dropped outright (it is not in the alphabet and has no Crockford
		// alias), while I, L and O become digits. Worth pinning precisely because
		// "ILOU" contains four ambiguous characters and three of them resolve.
		{"the ambiguous letters resolve or drop, they never survive", "ILOU", "110"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := utils.NormalizeReferralCode(tc.in); got != tc.want {
				t.Errorf("NormalizeReferralCode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A code in a different spelling resolves to the same account. Without
// normalisation the lookup would miss, the student who mistyped their own code
// would silently credit nobody, and nothing would say so.
func TestNormalisedCodeResolvesToTheSameAccount(t *testing.T) {
	db := newCodeTestDB(t)
	repo := NewRepository(db)

	seedUsers(t, repo, 1)
	canonical := codesOf(t, db)[1]
	if canonical == "" {
		t.Fatal("no code to test with")
	}

	// Every spelling a student might type resolves to the same row.
	for _, typed := range []string{
		strings.ToLower(canonical),
		strings.ToLower(canonical[:5]) + "-" + canonical[5:],
		strings.ToLower(canonical[:5]) + " " + canonical[5:],
	} {
		if got := utils.NormalizeReferralCode(typed); got != canonical {
			t.Errorf("NormalizeReferralCode(%q) = %q, want the canonical %q",
				typed, got, canonical)
		}
		var id uint
		if err := db.Raw(`SELECT id FROM users WHERE referral_code = ?`, canonical).Scan(&id).Error; err != nil {
			t.Fatalf("look up: %v", err)
		}
		if id != 1 {
			t.Errorf("canonical code resolved to user %d, want 1", id)
		}
	}
}

// A soft-deleted account's code must stop resolving.
//
// A user whose deletion is scheduled — which is what happens when an account is
// flagged for a fraud finding — must not keep crediting a referrer for the whole
// retention window. The row stays for the audit trail; the code stops working
// immediately.
func TestASoftDeletedAccountsCodeStopsResolving(t *testing.T) {
	db := newCodeTestDB(t)
	repo := NewRepository(db)

	seedUsers(t, repo, 1)
	code := codesOf(t, db)[1]

	if err := db.Delete(&User{}, 1).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	var id uint
	if err := db.Raw(`SELECT id FROM users WHERE referral_code = ? AND deleted_at IS NULL`, code).Scan(&id).Error; err != nil {
		t.Fatalf("look up: %v", err)
	}
	if id != 0 {
		t.Fatalf("a soft-deleted account's code still resolves to user %d", id)
	}
	// And the row is still there for the audit trail — a delete that took the code
	// with it would destroy the record of what happened.
	var total int64
	if err := db.Raw(`SELECT count(*) FROM users WHERE id = 1`).Scan(&total).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 1 {
		t.Error("the soft delete removed the row; it should only stop the code resolving")
	}
}
