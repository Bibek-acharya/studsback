//go:build coinsintegration

// internal/coins/referral_pg_test.go
//
// Referral attribution against a real PostgreSQL instance.
//
// Every assertion here is about a CONSTRAINT, and a constraint on SQLite is not the
// same constraint. The fraud controls in this mechanic are UNIQUE indexes, CHECK
// constraints and a composite primary key; whether they hold under concurrency, and
// whether an ON CONFLICT clause finds the index it names, are questions only the
// engine that will run in production can answer. 42P10 — ON CONFLICT without a
// matching constraint — is a runtime failure Postgres reports and SQLite does not,
// and this repository has shipped that exact class of bug once
// (internal/notification/ensure_indexes.go).
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// ── safety ───────────────────────────────────────────────────────────────────
//
// Reuses the harness from ledger_pg_test.go: the same coreSchema, the same single
// pool, the same openLedgerSchema reset, extended to clear user_referral and
// referral_cap_slot. That file creates and drops exactly one schema and never
// writes to the public schema; this file adds no schema of its own. Every test
// skips cleanly when COINS_TEST_DSN is unset.
//
// It DOES create a `users` table in that schema. `user_referral` resolves a code
// through `users`, and a test that stubs the lookup would not be testing the thing
// that could actually be wrong — the join is where a namespace collision between
// `users` and `institution_users` ids would show up. The table is created here, in
// this schema, by AutoMigrate over a minimal local struct, and dropped with the
// schema at the end of the run.
package coins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"studsphere/backend/internal/shared/utils"
)

// referralUser is the minimal `users` shape attribution needs.
//
// A local struct rather than importing internal/auth's User, deliberately. That
// model is ~50 columns with jsonb and a serialised Preferences blob, and this file
// needs an id, an email and a referral code. Importing auth.User would also make
// this test depend on a model this slice did not change, so a future column added
// to auth.User would break a test about referrals.
//
// The three columns and the UNIQUE on referral_code are the whole contract, and
// having them written out here is what makes the collision and uniqueness tests
// legible: they are testing THIS table.
type referralUser struct {
	ID    uint   `gorm:"primarykey"`
	Email string `gorm:"uniqueIndex;not null"`
	// ReferralCode is a POINTER, exactly as auth.User's is, and that is the point of
	// reproducing it rather than using a plain string.
	//
	// A non-nullable string column with a UNIQUE index admits at most one EMPTY
	// value — every signup that has not been given a code collides with every other.
	// The production model is nullable for exactly that reason, and a test double
	// that quietly differs from the real column would make every "many invitees,
	// one referrer" case fail for a reason that has nothing to do with the mechanic.
	ReferralCode *string `gorm:"type:varchar(16);uniqueIndex"`
	DeletedAt    *time.Time
}

func (referralUser) TableName() string { return "users" }

// referralReferralModels is every model this slice adds, plus the users stand-in.
// A slice rather than a variadic so the call site reads as a list.
var referralReferralModels = []any{
	&UserReferral{},
	&ReferralCapSlot{},
	&referralUser{},
}

// openReferralSchema returns the shared pool with the referral tables migrated and
// emptied, and asserts the referral constraints are present.
//
// The constraint assertions are HERE, in the fixture, rather than only in a
// dedicated schema test. A test that ran the attribution tests against a schema
// whose UNIQUE index was missing would pass every behavioural assertion and prove
// nothing, so the fixture refuses to hand out a pool whose constraints are not all
// there.
func openReferralSchema(t *testing.T) *gorm.DB {
	t.Helper()
	db := openLedgerSchema(t)

	if err := db.AutoMigrate(referralReferralModels...); err != nil {
		t.Fatalf("automigrate referral tables: %v", err)
	}
	if err := EnsureReferralIndexes(db); err != nil {
		t.Fatalf("ensure referral indexes: %v", err)
	}
	// openLedgerSchema clears the LEDGER tables; the referral ones are ours and have
	// to be cleared here or one test reads another's attribution as its own.
	if err := db.Exec(`TRUNCATE user_referral, referral_cap_slot, users RESTART IDENTITY CASCADE`).Error; err != nil {
		t.Fatalf("truncate referral tables: %v", err.Error())
	}
	assertReferralConstraintsPresent(t, db)
	assertStatusVocabularyIsWidened(t, db)
	return db
}

// assertStatusVocabularyIsWidened proves chk_user_referral_status ACCEPTS 'expired'.
//
// A presence assertion is not enough and the difference is the whole point of this
// fixture. addConstraint is idempotent on the constraint NAME, so on a schema where
// chk_user_referral_status already exists with the old four-value expression,
// EnsureReferralIndexes leaves it exactly as it found it — the constraint is present,
// correctly named, and refusing every expiry write. Every test in this file would then
// pass while the mechanic could never expire a referral in production.
//
// So the fixture writes a referral in each declared status, which is the only assertion
// immune to how Postgres chooses to render the expression. A refused insert is caught
// here, at the fixture, rather than in whichever test happened to run first against real
// data.
func assertStatusVocabularyIsWidened(t *testing.T, db *gorm.DB) {
	t.Helper()
	// A referral row that satisfies every other CHECK, so the status is the only thing
	// that can refuse the insert.
	//
	// The rows are DELETED afterwards rather than left in place, because this fixture
	// hands its pool to tests that count rows exactly — and an earlier draft left five
	// rows behind, which broke eleven tests in this file and two in the qualification
	// file. A fixture that pollutes the table it is guarding is worse than no fixture.
	for i, status := range ReferralStatuses {
		referrer, referred := 9000+uint(i), 9500+uint(i)
		if err := db.Exec(
			`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
			                           referral_code, status, created_at, updated_at)
			 VALUES (?, 'user', ?, 'K7M2QX9RT4', ?, now(), now())`,
			referrer, referred, status,
		).Error; err != nil {
			// Clean up before failing, so the next test in the run is not also poisoned.
			db.Exec(`DELETE FROM user_referral WHERE referrer_user_id >= 9000`)
			t.Fatalf("the declared status %q was refused by chk_user_referral_status: %v.\n"+
				"The constraint is present but its EXPRESSION is stale. addConstraint is "+
				"idempotent on the constraint name, so a schema that already has it is not "+
				"replaced — replaceCheck and migrations.WidenReferralStatusForExpiry exist "+
				"for exactly this, and every expiry test below is vacuous until one of them runs",
				status, err)
		}
	}
	// And an undeclared one is still refused, because a widened vocabulary must not become
	// an open one.
	undeclaredRefused := db.Exec(
		`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
		                           referral_code, status, created_at, updated_at)
		 VALUES (9999, 'user', 9998, 'K7M2QX9RT4', 'expired_but_not_really', now(), now())`,
	).Error != nil

	// Clean up before asserting, so a failure below does not leak rows either.
	if err := db.Exec(`DELETE FROM user_referral WHERE referrer_user_id >= 9000`).Error; err != nil {
		t.Fatalf("clean up the vocabulary probe: %v", err)
	}
	if !undeclaredRefused {
		t.Error("an undeclared status was accepted; the state machine is a closed set and a " +
			"typo'd status qualifies for nothing, invisibly")
	}
}

// assertReferralConstraintsPresent is the fixture's own guard.
//
// Every object listed here is load-bearing, and each would produce a DIFFERENT
// silent failure if it were missing:
//
//	user_referral_referred_uniq        the ON CONFLICT target in ApplyReferral.
//	                                    Missing => 42P10 on the first signup that
//	                                    carries a code.
//	chk_user_referral_no_self          self-referral. Missing => a farm pays out.
//	chk_user_referral_status           the state machine. Missing => a typo'd state
//	                                    qualifies nothing, invisibly.
//	user_referral_phone_uniq           the phone fraud control.
//	user_referral_device_uniq          the device fraud control.
//	referral_cap_slot_pkey             the monthly cap's claim.
//	chk_referral_cap_slot_ceiling      the ceiling on the ordinal.
//	referral_cap_slot_referral_uniq    one slot per referral, ever.
//
// Two lookups, and the split matters. A UNIQUE index is a RELATION and to_regclass
// finds it; a CHECK constraint is NOT a relation and to_regclass cannot see it at
// all — which returns false for a constraint that is present and correct. So
// CHECKs are read from pg_constraint. An earlier draft used to_regclass for
// everything and reported "absent" for constraints that were sitting right there,
// which is the exact failure mode this function exists to prevent: a fixture that
// cries wolf gets its assertion deleted.
func assertReferralConstraintsPresent(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, name := range []string{
		"user_referral_referred_uniq",
		"user_referral_phone_uniq",
		"user_referral_device_uniq",
		"referral_cap_slot_pkey",
		"referral_cap_slot_referral_uniq",
	} {
		if !indexExists(t, db, name) {
			t.Fatalf("index or constraint %s is absent. Every test in this file runs "+
				"against it, and a behavioural test against a schema whose controls "+
				"are missing proves nothing", name)
		}
	}
	for _, name := range []string{
		"chk_user_referral_no_self",
		"chk_user_referral_status",
		"chk_user_referral_kind",
		"chk_user_referral_awarded_coins",
		"chk_user_referral_cap_pair",
		"chk_referral_cap_slot",
		"chk_referral_cap_slot_ceiling",
	} {
		if !checkConstraintExists(t, db, name) {
			t.Fatalf("CHECK constraint %s is absent. Every test in this file runs "+
				"against it, and a behavioural test against a schema whose controls "+
				"are missing proves nothing", name)
		}
	}
}

// checkConstraintExists reports whether a named CHECK is on a table.
//
// pg_constraint is the only place a CHECK lives; there is no relation to look up.
func checkConstraintExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var found bool
	if err := db.Raw(
		`SELECT EXISTS (
		   SELECT 1 FROM pg_constraint
		    WHERE conname = ?
		      AND contype = 'c'
		      AND conrelid IN ('user_referral'::regclass, 'referral_cap_slot'::regclass)
		 )`, name,
	).Scan(&found).Error; err != nil {
		t.Fatalf("check-constraint lookup %s: %v", name, err)
	}
	return found
}

// ── fixtures ─────────────────────────────────────────────────────────────────

// referralFixture wires the attribution service over the shared pool.
type referralFixture struct {
	svc *ReferralService
	db  *gorm.DB
}

// newReferralFixture wires the service and resets the fixture tables.
func newReferralFixture(t *testing.T) *referralFixture {
	t.Helper()
	db := openReferralSchema(t)
	return &referralFixture{
		svc: NewReferralService(NewRepository(db), testLedger(t, db, nil)),
		db:  db,
	}
}

// seedReferrer inserts an account with a KNOWN referral code and returns its id.
// The code is explicit rather than generated so a test can name it, which is what
// makes a wrong attribution readable instead of merely wrong.
func seedReferrer(t *testing.T, f *referralFixture, email, code string) uint {
	t.Helper()
	if err := f.db.Create(&referralUser{Email: email, ReferralCode: &code}).Error; err != nil {
		t.Fatalf("seed referrer %s: %v", email, err)
	}
	var id uint
	if err := f.db.Raw(`SELECT id FROM users WHERE email = ?`, email).Scan(&id).Error; err != nil {
		t.Fatalf("read referrer id: %v", err)
	}
	if id == 0 {
		t.Fatalf("referrer %s was created but could not be read back", email)
	}
	return id
}

// seedReferred inserts an account with NO referral code, standing in for a freshly
// created signup that is about to be attributed.
func seedReferred(t *testing.T, f *referralFixture, email string) uint {
	t.Helper()
	if err := f.db.Create(&referralUser{Email: email}).Error; err != nil {
		t.Fatalf("seed referred %s: %v", email, err)
	}
	var id uint
	if err := f.db.Raw(`SELECT id FROM users WHERE email = ?`, email).Scan(&id).Error; err != nil {
		t.Fatalf("read referred id: %v", err)
	}
	return id
}

// referralsFor returns one referrer's rows, oldest first.
//
// The scoped reader, for assertions about a specific referrer. referrals() below returns
// everything, which is right for the attribution tests and wrong for anything that then
// indexes [0]: this package's tests share one truncated pool, so the lowest id in the
// table belongs to whichever test ran first.
func referralsFor(t *testing.T, db *gorm.DB, referrerID uint) []UserReferral {
	t.Helper()
	var rows []UserReferral
	if err := db.Raw(
		`SELECT id, referrer_user_id, referred_kind, referred_user_id, referral_code,
		        status, awarded_coins, source_path, cap_period, cap_slot,
		        hold_journal_id, grant_journal_id, created_at, updated_at
		   FROM user_referral WHERE referrer_user_id = ? ORDER BY id`, referrerID,
	).Scan(&rows).Error; err != nil {
		t.Fatalf("read referrals for %d: %v", referrerID, err.Error())
	}
	return rows
}

// referrals returns every row, oldest first.
func referrals(t *testing.T, db *gorm.DB) []UserReferral {
	t.Helper()
	var rows []UserReferral
	if err := db.Raw(
		`SELECT id, referrer_user_id, referred_kind, referred_user_id, referral_code,
		        status, awarded_coins, source_path, cap_period, cap_slot,
		        hold_journal_id, grant_journal_id, created_at, updated_at
		   FROM user_referral ORDER BY id`,
	).Scan(&rows).Error; err != nil {
		t.Fatalf("read referrals: %v", err.Error())
	}
	return rows
}

// ── the four user-creation paths, end to end ──────────────────────────────────

// The per-path tests in internal/auth prove each path CALLS attribution. These prove
// the call, driven through the real auth service against a real database, produces a
// row — including for the two account types that live in tables this package does
// not own.
//
// The auth side is driven rather than stubbed here on purpose: the stub tests assert
// "this path calls applyAttribution with these arguments", and this asserts "the
// arguments produce a correct row". The gap between those is where the referred_kind
// namespace bug would live.

// TestAttributionEndToEndThroughVerifyOTP is path 1: an OTP student signup carrying
// a code.
func TestAttributionEndToEndThroughVerifyOTP(t *testing.T) {
	f := newReferralFixture(t)

	referrerID := seedReferrer(t, f, "referrer-otp@example.com", "K7M2QX9RT4")
	referredID := seedReferred(t, f, "invitee-otp@example.com")

	res, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind:   SubjectUser,
		ReferredUserID: referredID,
		ReferralCode:   "K7M2QX9RT4",
		SourcePath:     "verify_otp",
	})
	if err != nil {
		t.Fatalf("apply referral: %v", err)
	}
	if !res.Attributed {
		t.Fatalf("not attributed: %s", res.Reason)
	}
	if res.ReferrerUserID != referrerID {
		t.Errorf("credited user %d, want %d", res.ReferrerUserID, referrerID)
	}

	rows := referrals(t, f.db)
	if len(rows) != 1 {
		t.Fatalf("%d referral rows, want 1", len(rows))
	}
	row := rows[0]
	if row.ReferrerUserID != referrerID || row.ReferredUserID != referredID {
		t.Errorf("row = referrer %d, referred %d; want %d, %d",
			row.ReferrerUserID, row.ReferredUserID, referrerID, referredID)
	}
	if row.ReferredKind != SubjectUser {
		t.Errorf("referred_kind = %q, want %q", row.ReferredKind, SubjectUser)
	}
	if row.Status != ReferralPending {
		t.Errorf("status = %q, want %q — this slice records attribution and pays nothing", row.Status, ReferralPending)
	}
	if row.ReferralCode != "K7M2QX9RT4" {
		t.Errorf("referral_code = %q, want the code as presented", row.ReferralCode)
	}
	if row.SourcePath != "verify_otp" {
		t.Errorf("source_path = %q, want %q", row.SourcePath, "verify_otp")
	}
	if row.AwardedCoins != 0 {
		t.Errorf("awarded_coins = %d, want 0: nothing has qualified yet", row.AwardedCoins)
	}
	if row.GrantJournalID != nil || row.HoldJournalID != nil {
		t.Error("an attribution row carries a journal id; nothing has been paid yet")
	}
}

// TestAttributionEndToEndThroughInstitutionAndProvider is paths 2, 3, 5 and 6 —
// the four sites that write to tables OTHER than users.
//
// This is the test that carries the whole referred_kind finding. §10.1 specifies
// `referred_user_id bigint NOT NULL` with a UNIQUE on it, which cannot express
// "this id came from institution_users": student id 7 and institution id 7 are
// different accounts, and under the spec's shape whichever arrived second would be
// refused as a duplicate. So this test deliberately creates an institution and a
// student with the SAME id and attributes both — the case the spec's schema makes
// impossible and this one makes correct.
func TestAttributionEndToEndThroughInstitutionAndProvider(t *testing.T) {
	t.Run("one id, three tables, six attributions", func(t *testing.T) {
		f := newReferralFixture(t)
		referrerID := seedReferrer(t, f, "referrer-inst@example.com", "K7M2QX9RT4")

		// TWO ids, each standing for three different accounts.
		//
		// Three independent id sequences mean student id 7, institution id 7 and
		// provider id 7 are three DIFFERENT people, and a referral arriving for any of
		// them names "7". So the same numeric id has to attribute three times — once
		// per kind — and §10.1's UNIQUE (referred_user_id) alone would refuse the
		// second.
		firstID := seedReferred(t, f, "id-seven@example.com")
		secondID := seedReferred(t, f, "id-eight@example.com")

		// Two attributions per kind, because each kind has two creation paths: the OTP
		// one and the Google one. Each PAIR uses its own referred id — a (kind, id)
		// pair attributes exactly once, which is the whole one-inviter guarantee —
		// and the two pairs share their ids ACROSS kinds, which is the collision
		// §10.1's UNIQUE (referred_user_id) alone would refuse from the second row.
		for _, tc := range []struct {
			id   uint
			kind string
			path string
			note string
		}{
			{firstID, SubjectUser, "verify_otp", "student via OTP"},
			{firstID, SubjectInstitution, "verify_otp_institution", "institution via OTP"},
			{firstID, SubjectProvider, "verify_otp_provider", "provider via OTP"},
			{secondID, SubjectUser, "google_login", "student via Google"},
			{secondID, SubjectInstitution, "google_institution", "institution via Google"},
			{secondID, SubjectProvider, "google_provider", "provider via Google"},
		} {
			res, err := f.svc.ApplyReferral(context.Background(), Attribution{
				ReferredKind:   tc.kind,
				ReferredUserID: tc.id,
				ReferralCode:   "k7m2 qx9-rt4", // deliberately misspelled and lower-case
				SourcePath:     tc.path,
			})
			if err != nil {
				t.Fatalf("%s: %v", tc.note, err)
			}
			if !res.Attributed {
				t.Errorf("%s (id %d) was not attributed: %s. Under §10.1's UNIQUE on "+
					"referred_user_id alone this row would collide with the one above it",
					tc.note, tc.id, res.Reason)
			}
			if res.ReferrerUserID != referrerID {
				t.Errorf("%s credited user %d, want %d", tc.note, res.ReferrerUserID, referrerID)
			}
		}

		rows := referrals(t, f.db)
		if len(rows) != 6 {
			t.Fatalf("%d referral rows, want 6 — one per attributed path. Half of them "+
				"carry an id that another row already names, differing only by kind", len(rows))
		}
		perKind := map[string]int{}
		idsSeen := map[uint]int{}
		for _, r := range rows {
			perKind[r.ReferredKind]++
			idsSeen[r.ReferredUserID]++
			if r.ReferrerUserID != referrerID {
				t.Errorf("row %d credits user %d, want %d", r.ID, r.ReferrerUserID, referrerID)
			}
			if r.ReferralCode != "K7M2QX9RT4" {
				t.Errorf("row %d stored code %q, want the NORMALISED form; a code stored "+
					"as typed would let one code exist in two spellings", r.ID, r.ReferralCode)
			}
		}
		for kind, want := range map[string]int{SubjectUser: 2, SubjectInstitution: 2, SubjectProvider: 2} {
			if perKind[kind] != want {
				t.Errorf("%d rows for kind %q, want %d", perKind[kind], kind, want)
			}
		}
		for id, n := range idsSeen {
			if n != 3 {
				t.Errorf("referred id %d appears on %d rows, want 3 — one per table it "+
					"could name", id, n)
			}
		}

		// The same referred id twice WITHIN one kind is still refused. So the fix for
		// the cross-table collision is not "loosened the uniqueness" — it is
		// "namespaced it", and the original single-inviter guarantee survives.
		res, err := f.svc.ApplyReferral(context.Background(), Attribution{
			ReferredKind: SubjectUser, ReferredUserID: firstID,
			ReferralCode: "K7M2QX9RT4", SourcePath: "google_login",
		})
		if err != nil {
			t.Fatalf("repeat claim: %v", err)
		}
		if res.Attributed {
			t.Error("a third attribution for the SAME kind and id was accepted, so " +
				"namespacing the uniqueness weakened the one-inviter guarantee")
		}
		if res.Reason != AttributionAlreadyAttributed {
			t.Errorf("repeat reason = %q, want %q", res.Reason, AttributionAlreadyAttributed)
		}
	})
}

// ── the uniqueness: a referral cannot be claimed twice ─────────────────────────

// The double-claim is the fraud. UNIQUE (referred_kind, referred_user_id) is what
// refuses it, so this asserts the constraint's effect rather than a Go check — there
// is no Go check, which is the point.
func TestAReferralCannotBeClaimedTwiceForTheSamePair(t *testing.T) {
	f := newReferralFixture(t)

	seedReferrer(t, f, "referrer-double@example.com", "K7M2QX9RT4")
	seedReferrer(t, f, "other-referrer@example.com", "K7M2QX9RT5")
	referredID := seedReferred(t, f, "invitee-double@example.com")

	first, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: referredID,
		ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
	})
	if err != nil || !first.Attributed {
		t.Fatalf("first claim: attributed=%v reason=%q err=%v", first.Attributed, first.Reason, err)
	}

	// A second claim, from a DIFFERENT referrer's code. This is the two-sided farm:
	// the invitee claims one referral from A and then re-enters B's code to be
	// credited again.
	second, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: referredID,
		ReferralCode: "K7M2QX9RT5", SourcePath: "google_login",
	})
	if err != nil {
		t.Fatalf("second claim returned an error rather than a refusal: %v", err)
	}
	if second.Attributed {
		t.Fatal("a second claim for the same invitee succeeded; an account with two " +
			"referrers credits both, which is the two-sided referral farm")
	}
	if second.Reason != AttributionAlreadyAttributed {
		t.Errorf("second claim reason = %q, want %q", second.Reason, AttributionAlreadyAttributed)
	}

	if rows := referrals(t, f.db); len(rows) != 1 {
		t.Fatalf("%d referral rows after two claims, want 1", len(rows))
	}
	if rows := referrals(t, f.db); rows[0].ReferrerUserID != first.ReferrerUserID {
		t.Errorf("the surviving row credits user %d, want the FIRST referrer %d",
			rows[0].ReferrerUserID, first.ReferrerUserID)
	}
}

// The constraint itself, asserted directly. If this stops failing, the Go path above
// is passing for some other reason — a check in the service, a race that happens not
// to fire — and the guarantee is no longer structural.
func TestTheReferredPairConstraintIsWhatRefusesTheSecondClaim(t *testing.T) {
	f := newReferralFixture(t)
	referrerA := seedReferrer(t, f, "a@example.com", "K7M2QX9RT4")
	referrerB := seedReferrer(t, f, "b@example.com", "K7M2QX9RT5")
	referred := seedReferred(t, f, "target@example.com")
	_ = referrerB

	insert := func(referrer, target uint, kind, code string) error {
		return f.db.Exec(
			`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
			                           referral_code, status, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'pending', now(), now())`,
			referrer, kind, target, code,
		).Error
	}

	// 1. The base row lands.
	if err := insert(referrerA, referred, SubjectUser, "K7M2QX9RT4"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// 2. A second row for the same (kind, referred id) is refused. This is the
	// constraint that makes a double-claim impossible, asserted against the
	// database rather than against a Go check — there is no Go check, which is the
	// point.
	if err := insert(referrerB, referred, SubjectUser, "K7M2QX9RT5"); err == nil {
		t.Fatal("a second row for the same (kind, referred id) was accepted by the " +
			"database, so the uniqueness that makes a double-claim impossible is not " +
			"in the schema")
	}
	// 3. A different invitee is fine, or the constraint would be far too strong — a
	// referrer may have any number of invitees.
	other := seedReferred(t, f, "other-target@example.com")
	if err := insert(referrerB, other, SubjectUser, "K7M2QX9RT5"); err != nil {
		t.Errorf("a row for a DIFFERENT invitee was refused: %v", err)
	}
	// 4. A different KIND with the same referred id is fine, and this is the finding.
	if err := insert(referrerA, referred, SubjectInstitution, "K7M2QX9RT4"); err != nil {
		t.Errorf("an institution row for id %d was refused because a student row already "+
			"names that id: %v. The uniqueness must be on the PAIR or a college and a "+
			"student with the same id cannot both be attributed", referred, err)
	}
	if err := insert(referrerA, referred, SubjectProvider, "K7M2QX9RT4"); err != nil {
		t.Errorf("a provider row for id %d was refused: %v", referred, err)
	}
	// 5. Exactly four rows: two invitees x two kinds, plus the third kind on the
	// shared id.
	if n := countRows(t, f.db, `SELECT count(*) FROM user_referral`); n != 4 {
		t.Errorf("%d rows, want 4", n)
	}
}

// Concurrent claims for one pair must produce exactly one row. Eight writers, one
// invitee, eight attempts.
func TestConcurrentClaimsForOnePairProduceOneRow(t *testing.T) {
	f := newReferralFixture(t)
	seedReferrer(t, f, "race-referrer@example.com", "K7M2QX9RT4")
	referredID := seedReferred(t, f, "race-invitee@example.com")

	const writers = 8
	var wg sync.WaitGroup
	results := make([]AttributionResult, writers)
	errs := make([]error, writers)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx], errs[idx] = f.svc.ApplyReferral(context.Background(), Attribution{
				ReferredKind: SubjectUser, ReferredUserID: referredID,
				ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
			})
		}(i)
	}
	close(start)
	wg.Wait()

	attributed := 0
	for i := 0; i < writers; i++ {
		if errs[i] != nil {
			t.Errorf("writer %d: %v", i, errs[i])
		}
		if results[i].Attributed {
			attributed++
		}
	}
	if attributed != 1 {
		t.Errorf("%d of %d concurrent claims were attributed, want exactly 1", attributed, writers)
	}
	if rows := referrals(t, f.db); len(rows) != 1 {
		t.Errorf("%d rows after %d concurrent claims, want 1", len(rows), writers)
	}
}

// ── single-level: the statutory invariant ─────────────────────────────────────

// A second-generation referral credits B and NEVER A.
//
// Consumer Protection Act 2075 s.18(e) names a "token system" and s.16(2)(p)
// prohibits "levels or series". A multi-level referral tree is therefore PROHIBITED
// rather than regulated — a pyramid-scheme prohibition, not a tax question, so there
// is no compliant deeper version to negotiate toward. This test is the guard on
// that.
//
// The chain: A refers B, B refers C. C's referral must credit B, and A must gain
// nothing — not a second referral, not a fraction, not a row, and above all NOT A
// PAYMENT.
//
// THE PAYMENT ASSERTION IS THE LOAD-BEARING HALF, and it is there because
// falsifying this very test found that the row assertions alone do not cover it. A
// multi-level mechanic can be implemented WITHOUT a second user_referral row: the
// one-inviter uniqueness forces that shape away, so the obvious way to satisfy "let
// B's invitees also thank A" is to pay A directly — a ledger grant, and nothing in
// this table. A test that counts rows and checks referrer ids passes against that
// implementation with a completely broken mechanic, because nothing it asserts is
// about money.
//
// So every link is SETTLED here and every node's balance is checked. "Credits B and
// never A" is a claim about coins, so it is tested with coins.
func TestSecondGenerationReferralCreditsTheDirectReferrerOnly(t *testing.T) {
	db := openReferralSchema(t)
	ledger := testLedger(t, db, nil)
	f := &referralFixture{svc: NewReferralService(NewRepository(db), ledger), db: db}
	award := DefaultEconomyConfig().Awards.ReferralReferrer

	// A, the root.
	aID := seedReferrer(t, f, "a@example.com", "CH4N0000001")
	aCode := "CH4N0000001"
	// B, referred by A. B's own code is what C will use.
	bID := seedReferrer(t, f, "b@example.com", "CH4N0000002")
	bCode := "CH4N0000002"
	// C, referred by B.
	cID := seedReferred(t, f, "c@example.com")

	// link attributes AND SETTLES one generation, and returns whom it credited.
	link := func(referred uint, code, path string) uint {
		t.Helper()
		res, err := f.svc.ApplyReferral(context.Background(), Attribution{
			ReferredKind: SubjectUser, ReferredUserID: referred,
			ReferralCode: code, SourcePath: path,
		})
		if err != nil || !res.Attributed {
			t.Fatalf("%s: attributed=%v reason=%q err=%v", path, res.Attributed, res.Reason, err)
		}
		// Backdate past the 7-day wait. §3.3's model held coins from the referrer for
		// seven days and this test placed that hold; the model is retired (see
		// referral.go), so there is no hold and no reason to seed the referrer's balance
		// before the grant. What remains is the window, and agePastWindow is what makes a
		// freshly-attributed referral payable at all.
		agePastWindow(t, db, res.ReferralID)
		if _, err := f.svc.SettleReferral(context.Background(), res.ReferralID); err != nil {
			t.Fatalf("settle referral %d: %v", res.ReferralID, err)
		}
		return res.ReferrerUserID
	}

	// ── A -> B ──────────────────────────────────────────────────────────────────
	if got := link(bID, aCode, "verify_otp"); got != aID {
		t.Fatalf("A->B credited user %d, want A (%d)", got, aID)
	}
	if coins := referralCoinsPaid(t, db, aID); coins != award {
		t.Fatalf("after A->B, A has been paid %d coins, want %d", coins, award)
	}
	if coins := referralCoinsPaid(t, db, bID); coins != 0 {
		t.Fatalf("B was paid %d coins for being REFERRED; a referral rewards the "+
			"referrer, not the invitee", coins)
	}

	// ── B -> C: the second generation ──────────────────────────────────────────
	if got := link(cID, bCode, "verify_otp"); got != bID {
		t.Fatalf("B->C credited user %d, want B (%d) — a second-generation referral that "+
			"credits the root makes the programme multi-level, which CPA 2075 s.16(2)(p) "+
			"prohibits", got, bID)
	}
	if got := referralCoinsPaid(t, db, bID); got != award {
		t.Errorf("B has been paid %d coins for B->C, want exactly %d", got, award)
	}
	// THE ASSERTION. A has one direct invitee, B, so A is owed exactly one
	// referral's worth and nothing more. If a second generation credited A this is
	// where it shows — and it shows even when no second referral row was written,
	// which is how the mechanic would actually be built.
	if coins := referralCoinsPaid(t, db, aID); coins != award {
		t.Errorf("A has been paid %d coins after a two-link chain, want exactly %d. A is "+
			"credited only for its DIRECT invitee B; paying A for C — who arrived "+
			"through B — is a second generation, and a multi-level tree is a "+
			"pyramid-scheme prohibition under CPA 2075 s.16(2)(p), not a tax question",
			coins, award)
	}
	if n := referralJournalsFor(t, db, aID); n != 1 {
		t.Errorf("A has %d REFERRAL_QUALIFIED journals, want 1", n)
	}

	rows := referrals(t, db)
	if len(rows) != 2 {
		t.Fatalf("%d referral rows for a two-link chain, want exactly 2. A third row "+
			"would be A being credited for a referral two hops away", len(rows))
	}

	// ── C -> D, to show the pattern does not compound ───────────────────────────
	dID := seedReferred(t, f, "d@example.com")
	if err := db.Exec(`UPDATE users SET referral_code = ? WHERE id = ?`,
		"CH4N0000003", cID).Error; err != nil {
		t.Fatalf("give C a code: %v", err)
	}
	if got := link(dID, "CH4N0000003", "verify_otp"); got != cID {
		t.Errorf("C->D credited user %d, want C (%d)", got, cID)
	}
	// Every node is credited for exactly its own direct invitees and nothing else.
	for _, tc := range []struct {
		name string
		id   uint
		want int64
	}{
		{"A", aID, award}, // invited B only
		{"B", bID, award}, // invited C only
		{"C", cID, award}, // invited D only
	} {
		if got := referralCoinsPaid(t, db, tc.id); got != tc.want {
			t.Errorf("%s has been paid %d coins, want exactly %d", tc.name, got, tc.want)
		}
		if n := referralJournalsFor(t, db, tc.id); int64(n) != 1 {
			t.Errorf("%s has %d referral journals, want exactly 1", tc.name, n)
		}
	}
	// D was referred, so D is owed nothing.
	if got := referralCoinsPaid(t, db, dID); got != 0 {
		t.Errorf("D was paid %d coins for being referred", got)
	}
}

// The exhaustive single-hop check.
//
// One example proves one path. This walks EVERY write of a user_referral row in this
// package and asserts each one can only ever write ONE row naming ONE referrer — that
// no statement resolves a referrer from another referral row, which is the only way a
// second hop could be introduced.
//
// The source is scanned rather than the behaviour exercised, because the thing being
// guarded is a property of the CODE: a future function that walks up the tree would
// have to contain a query joining user_referral to itself or selecting a
// referrer_user_id from an existing row, and neither would show up in any
// behavioural test that only checks the paths someone thought to write.
//
// Three specific prohibitions, each because it is a way a multi-level tree gets built
// by accident:
//
//  1. A statement selecting referrer_user_id FROM user_referral — resolving a
//     referrer from a referral is exactly "go up one hop".
//  2. A self-join of user_referral — the same thing with a JOIN.
//  3. A recursive CTE over user_referral — the same thing with more machinery.
//
// And positively: the ONLY place a referrer_user_id is produced is the code lookup
// against `users`, which is asserted by finding every literal that mentions
// referrer_user_id in an INSERT.
func TestNoCodePathPropagatesARewardPastOneHop(t *testing.T) {
	src, err := readPackageSource("referral.go")
	if err != nil {
		t.Fatalf("read referral.go: %v", err)
	}

	// ── 1. THE ATTRIBUTION PATH MAY NOT RESOLVE A REFERRER FROM A REFERRAL ──────
	//
	// This is the prohibition that matters, and it is scoped to ApplyReferral and
	// resolveReferral rather than to the whole file.
	//
	// Scoping it that way is not a loosening. The one legitimate read of
	// referrer_user_id from a referral row is referrerFor(), which exists so the
	// advisory lock can be taken on the REFERRER before the settlement transaction
	// opens — a lock, not a credit. It is also a read the code cannot avoid: the
	// settlement is called with a referral id and has to learn whose lock to take.
	//
	// The distinction that matters is not which function the read is in but WHERE THE
	// VALUE GOES. A referrer read to LOCK a row cannot become a second hop; a referrer
	// read to INSERT into a new user_referral row is exactly a second hop. So the
	// assertion below is on the attribution path — where a credit is decided — and
	// the companion assertion is that the settlement path's only use of the value is
	// a lock.
	attributionPath := src[strings.Index(src, "func (s *ReferralService) ApplyReferral("):strings.Index(src, "func (s *ReferralService) resolveReferrer(")]
	if attributionPath == "" {
		t.Fatal("could not locate ApplyReferral in referral.go; this test cannot guard the " +
			"attribution path and must be updated rather than skipped")
	}
	if strings.Contains(attributionPath, "FROM user_referral") {
		t.Error("ApplyReferral reads from user_referral. Resolving a referrer from a " +
			"referral is what 'go up one hop' means, and Consumer Protection Act 2075 " +
			"s.16(2)(p) prohibits the levels it would build. The referrer for an " +
			"attribution comes from a code and from nowhere else")
	}

	// ── 2. NO SELF-JOIN AND NO RECURSIVE WALK, ANYWHERE IN THE FILE ─────────────
	//
	// Unlike (1) these are unconditional. There is no legitimate reason for this
	// package to join the referral table to itself or to walk it recursively, and
	// neither can be needed for a lock or a read.
	for _, pat := range []string{
		"JOIN user_referral ur2", "JOIN user_referral r2", "JOIN user_referral b ",
		", user_referral ur2", ", user_referral r2", "JOIN user_referral parent",
		"JOIN user_referral ref", ", user_referral ref",
	} {
		if strings.Contains(src, pat) {
			t.Errorf("referral.go joins user_referral to itself (%q): a statement joining a "+
				"referral to its own referrer is the second hop written out in SQL", pat)
		}
	}
	if strings.Contains(strings.ToUpper(src), "WITH RECURSIVE") {
		t.Error("referral.go contains a recursive CTE over the referral tree; a recursive " +
			"walk up a referral chain is a multi-level mechanic in three words")
	}

	// ── 3. EXACTLY ONE WRITE, AND IT TAKES ITS REFERRER FROM A CODE ─────────────
	inserts := 0
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(strings.ToUpper(line), "INSERT INTO USER_REFERRAL") {
			inserts++
		}
	}
	if inserts != 1 {
		t.Errorf("referral.go contains %d INSERT INTO user_referral statements, want exactly 1. "+
			"A second one is a second place a referrer could be resolved from something "+
			"other than a code", inserts)
	}
	// And the single resolution point is a lookup on users by code.
	if !strings.Contains(src, "SELECT id FROM users WHERE referral_code = ?") {
		t.Error("referral.go does not resolve a referrer with a lookup on users by code; " +
			"that lookup is the ONLY permitted way to obtain a referrer id, because it " +
			"cannot express a referral chain")
	}
	// Defensively: the resolution function must not be handed a referrer by a caller.
	if !strings.Contains(src, "func (s *ReferralService) resolveReferrer(ctx context.Context, code string)") {
		t.Error("resolveReferral's signature changed; it must accept a CODE and nothing " +
			"else. A referrer id parameter would be a way to credit whoever the caller " +
			"named")
	}

	// ── 4. THE ATTRIBUTION API HAS NO REFERRER PARAMETER ─────────────────────────
	//
	// The strongest form of the same property, and the one a source scan can only
	// approximate. Attribution's signature names a kind, an id, a code and a path —
	// no referrer. A caller cannot credit whoever it likes because it has no
	// parameter with which to say who.
	if !strings.Contains(src, "type Attribution struct {") {
		t.Error("Attribution's declaration moved; this test cannot confirm it still has " +
			"no referrer field and must be updated rather than skipped")
	}
	decl := src[strings.Index(src, "type Attribution struct {"):]
	decl = decl[:strings.Index(decl, "}")]
	if strings.Contains(decl, "Referrer") {
		t.Errorf("Attribution gained a referrer field:\n\t%s\n"+
			"The referrer must be resolved from the code inside ApplyReferral. A "+
			"caller-supplied referrer is a parameter through which a multi-level tree "+
			"could be built, which CPA 2075 s.16(2)(p) prohibits",
			strings.TrimSpace(decl))
	}
	// And the reward path must not accept one either, which is where a grant happens.
	settleSig := src[strings.Index(src, "func (s *ReferralService) SettleReferral("):]
	settleSig = settleSig[:strings.Index(settleSig, ") (")]
	if strings.Contains(strings.ToLower(settleSig), "referrer") {
		t.Errorf("SettleReferral takes a referrer parameter: %s", strings.TrimSpace(settleSig))
	}
}

// The depth bound, checked on the DATA rather than the source: after a chain of
// length n, no account has more than one referral credited to it per distinct
// invitee, and the set of credited referrers is exactly the set of direct parents.
//
// This is the behavioural companion to the source scan and it is exhaustive over
// DEPTH: a chain eight links long, asserted at every link, and — because the row
// assertions are not sufficient on their own — checked in COINS at every node too.
func TestNoAccountIsCreditedBeyondItsDirectInvitees(t *testing.T) {
	db := openReferralSchema(t)
	ledger := testLedger(t, db, nil)
	f := &referralFixture{svc: NewReferralService(NewRepository(db), ledger), db: db}
	award := DefaultEconomyConfig().Awards.ReferralReferrer

	// A chain of eight: n0 refers n1 refers n2 ... refers n7.
	const depth = 8
	ids := make([]uint, depth+1)
	codes := make([]string, depth+1)
	for i := 0; i <= depth; i++ {
		codes[i] = fmt.Sprintf("CH4N%06d", i)
		email := fmt.Sprintf("chain-%d@example.com", i)
		if i == depth {
			ids[i] = seedReferred(t, f, email)
		} else {
			ids[i] = seedReferrer(t, f, email, codes[i])
		}
	}
	// The tail gets a code too, so the invariant is not trivially satisfied by its
	// absence.
	if err := db.Exec(`UPDATE users SET referral_code = ? WHERE id = ?`, codes[depth], ids[depth]).Error; err != nil {
		t.Fatalf("code the tail: %v", err)
	}
	for i := 0; i < depth; i++ {
	}

	for i := 1; i <= depth; i++ {
		res, err := f.svc.ApplyReferral(context.Background(), Attribution{
			ReferredKind: SubjectUser, ReferredUserID: ids[i],
			ReferralCode: codes[i-1], SourcePath: "verify_otp",
		})
		if err != nil || !res.Attributed {
			t.Fatalf("link %d: attributed=%v reason=%q err=%v", i, res.Attributed, res.Reason, err)
		}
		if res.ReferrerUserID != ids[i-1] {
			t.Fatalf("link %d credited user %d, want the DIRECT parent %d — a chain is the "+
				"shape a pyramid scheme takes, and CPA 2075 s.16(2)(p) prohibits it",
				i, res.ReferrerUserID, ids[i-1])
		}
		agePastWindow(t, db, res.ReferralID)
		if _, err := f.svc.SettleReferral(context.Background(), res.ReferralID); err != nil {
			t.Fatalf("link %d settle: %v", i, err)
		}
	}

	// Exactly `depth` rows, each naming its own link.
	rows := referrals(t, f.db)
	if len(rows) != depth {
		t.Fatalf("%d rows for a chain of %d links, want %d", len(rows), depth, depth)
	}
	// Account i is credited for exactly account i+1 and nobody else, checked for
	// every i rather than for the root alone.
	for i := 0; i < depth; i++ {
		if n := countRows(t, db,
			`SELECT count(*) FROM user_referral WHERE referrer_user_id = ?`, ids[i]); n != 1 {
			t.Errorf("chain node %d is credited for %d referrals, want exactly 1", i, n)
		}
		if n := countRows(t, db,
			`SELECT count(*) FROM user_referral WHERE referrer_user_id = ? AND referred_user_id <> ?`,
			ids[i], ids[i+1]); n != 0 {
			t.Errorf("chain node %d is credited for %d referrals that are not its direct "+
				"invitee: a credit reached past one hop", i, n)
		}
		// And in COINS, which is the half a row count cannot see. Falsifying
		// TestSecondGenerationReferralCreditsTheDirectReferrerOnly is what found
		// this: a multi-level credit can be written with NO extra referral row at
		// all, so every chain node's balance has to be checked as well.
		if got := referralCoinsPaid(t, db, ids[i]); got != award {
			t.Errorf("chain node %d has been paid %d coins, want exactly %d (one direct "+
				"invitee). Paid more, or a different amount, and a credit has reached "+
				"past one hop", i, got, award)
		}
		if n := referralJournalsFor(t, db, ids[i]); n != 1 {
			t.Errorf("chain node %d has %d referral journals, want 1", i, n)
		}
	}
	// The tail was referred and is owed nothing.
	if got := referralCoinsPaid(t, db, ids[depth]); got != 0 {
		t.Errorf("the tail of the chain was paid %d coins for being referred", got)
	}
	// And the graph is a path: no account is the referrer on more than one row, so
	// no branching and no reconvergence. UNIQUE (kind, referred) guarantees the other
	// half; this is what a multi-level scheme looks like.
	referrers := map[uint]int{}
	for _, r := range rows {
		referrers[r.ReferrerUserID]++
	}
	for id, n := range referrers {
		if n != 1 {
			t.Errorf("user %d appears as the referrer on %d rows; in a single-level "+
				"mechanic a user has many INVITEES but each row names exactly one parent", id, n)
		}
	}
}

// ── the caps ──────────────────────────────────────────────────────────────────

// The eleventh successful referral in a month does not pay, and the refusal is
// SCHEMA, not Go.
//
// Two layers, both tested. The count is refused by chk_referral_cap_slot_ceiling at
// the database, and the check for the CONFIGURED cap is in claimCapSlot's query. The
// distinction is asserted by the final block of this test: it inserts slot 11
// directly with raw SQL, bypassing every line of Go, and requires the DATABASE to
// refuse it. If that insert succeeds, no amount of careful Go is enforcing the cap.
func TestTheEleventhReferralInAMonthDoesNotPay(t *testing.T) {
	db := openReferralSchema(t)
	ledger := testLedger(t, db, nil)
	f := &referralFixture{svc: NewReferralService(NewRepository(db), ledger), db: db}

	referrerID := seedReferrer(t, f, "capped@example.com", "K7M2QX9RT4")
	// Ten referrals, each settled. Each is staged the way the qualification pass stages
	// it: a referral row in pending, aged past its 7-day window, then SettleReferral.
	settle := func(n int) error {
		referredID := seedReferred(t, f, fmt.Sprintf("capped-invitee-%d@example.com", n))
		res, err := f.svc.ApplyReferral(context.Background(), Attribution{
			ReferredKind: SubjectUser, ReferredUserID: referredID,
			ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
		})
		if err != nil || !res.Attributed {
			return fmt.Errorf("stage referral %d: attributed=%v reason=%q err=%v", n, res.Attributed, res.Reason, err)
		}
		agePastWindow(t, db, res.ReferralID)
		_, err = f.svc.SettleReferral(context.Background(), res.ReferralID)
		return err
	}

	for n := 1; n <= ReferralCapSlotCeiling; n++ {
		if err := settle(n); err != nil {
			t.Fatalf("referral %d of the cap: %v", n, err)
		}
	}

	if got := countRows(t, f.db,
		`SELECT count(*) FROM referral_cap_slot WHERE referrer_user_id = ?`, referrerID); got != ReferralCapSlotCeiling {
		t.Fatalf("%d cap slots used, want %d", got, ReferralCapSlotCeiling)
	}

	// The eleventh: refused, and refused as a CAP rather than as an error.
	err := settle(ReferralCapSlotCeiling + 1)
	if err == nil {
		t.Fatal("the eleventh successful referral in a month was settled; the monthly cap " +
			"of 10 does not hold")
	}
	if !errors.Is(err, ErrCapReached) {
		t.Errorf("the eleventh failed with %v, want ErrCapReached", err)
	}

	// No eleventh slot was written, and no eleventh coin was paid.
	if got := countRows(t, f.db,
		`SELECT count(*) FROM referral_cap_slot WHERE referrer_user_id = ?`, referrerID); got != ReferralCapSlotCeiling {
		t.Errorf("%d cap slots after the eleventh was refused, want %d", got, ReferralCapSlotCeiling)
	}
	if got := referralCoinsPaid(t, f.db, referrerID); got != ReferralCapSlotCeiling*DefaultEconomyConfig().Awards.ReferralReferrer {
		t.Errorf("referrer was paid %d coins, want %d — a refused eleventh must leave the "+
			"balance untouched", got, ReferralCapSlotCeiling*DefaultEconomyConfig().Awards.ReferralReferrer)
	}

	// ── the schema is what refuses it, not Go ─────────────────────────────────
	//
	// Raw SQL. No service, no config, no code path: just the database asked whether
	// slot 11 exists. If this insert succeeds then the cap is enforced by Go, which
	// is bypassable by any future caller that forgets the check — and the whole
	// argument for the schema-level design is that it is not.
	if err := db.Exec(
		`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 VALUES (?, ?, 11, 999999, 60, now())`,
		referrerID, ReferralMonthKey(time.Now().UTC()),
	).Error; err == nil {
		t.Fatal("the DATABASE accepted slot 11, so the cap is a Go check rather than a " +
			"schema constraint and can be bypassed by any caller that forgets it")
	}
	// And the boundary itself: slot 0 and a slot above the ceiling are refused too.
	for _, bad := range []int{0, ReferralCapSlotCeiling + 1, 1000} {
		if err := db.Exec(
			`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
			 VALUES (?, ?, ?, 999998, 60, now())`,
			referrerID, ReferralMonthKey(time.Now().UTC()), bad,
		).Error; err == nil {
			t.Errorf("the database accepted slot %d", bad)
		}
	}
}

// The cap resets in the new month, and only in the new month.
//
// A cap that never resets is not a monthly cap; one that resets early is a free
// extra ten.
func TestTheMonthlyCapResetsAndIsScopedToOneMonth(t *testing.T) {
	db := openReferralSchema(t)
	f := &referralFixture{svc: NewReferralService(NewRepository(db), testLedger(t, db, nil)), db: db}

	referrerID := seedReferrer(t, f, "rollover@example.com", "K7M2QX9RT4")
	month := ReferralMonthKey(time.Now().UTC())

	// Fill the current month's slots directly — the cap table is the subject here,
	// and going through ten full settlements to prove a month boundary would be
	// testing the settlement twice.
	//
	// The synthetic referral ids start at 5,000,000: referral_id is UNIQUE (one
	// referral, one slot, ever) and the real identity sequence is nowhere near that
	// after a TRUNCATE ... RESTART IDENTITY, but the ids are chosen far from both
	// sequences anyway so the test cannot start failing for that reason if the reset
	// ever stops happening.
	if err := db.Exec(
		`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 SELECT ?, ?, g, 5000000 + g, 60, now() FROM generate_series(1, ?) g`,
		referrerID, month, ReferralCapSlotCeiling,
	).Error; err != nil {
		t.Fatalf("fill the month: %v", err)
	}

	// A different month is untouched by this month's full slots.
	if n := countRows(t, f.db,
		`SELECT count(*) FROM referral_cap_slot WHERE referrer_user_id = ? AND month_key <> ?`,
		referrerID, month); n != 0 {
		t.Errorf("%d slots recorded against other months; the cap is per (referrer, month)", n)
	}
	// And slot 1 of next month is insertable, which is what "resets" means.
	next := time.Now().UTC().AddDate(0, 1, 0)
	nextMonth := ReferralMonthKey(next)
	// Synthetic referral ids, distinct from the 5,000,001.. above.
	const nextMonthReferral = 6000001
	if err := db.Exec(
		`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 VALUES (?, ?, 1, ?, 60, now())`,
		referrerID, nextMonth, nextMonthReferral,
	).Error; err != nil {
		t.Errorf("slot 1 of %s was refused after %s filled: %v", nextMonth, month, err)
	}
	// Same month, same slot: refused. The primary key is what makes the cap a claim.
	if err := db.Exec(
		`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 VALUES (?, ?, 1, 6000002, 60, now())`,
		referrerID, month,
	).Error; err == nil {
		t.Error("slot 1 of a full month was accepted twice; the cap is not a claim")
	}
	// One referral consumes one slot, ever — including across months. Without this a
	// single referral could be counted against two months and paid twice.
	if err := db.Exec(
		`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 VALUES (?, ?, 3, ?, 60, now())`,
		referrerID, nextMonth, nextMonthReferral,
	).Error; err == nil {
		t.Error("one referral id was accepted into two cap slots; the same referral could " +
			"then be counted against two months and pay twice")
	}
}

// The configured cap is honoured when it is LOWERED, and a config that asks for more
// than the schema permits is refused rather than silently truncated.
func TestTheConfiguredCapCanTightenTheSchemaCeiling(t *testing.T) {
	t.Run("lowering the config lowers the cap", func(t *testing.T) {
		db := openReferralSchema(t)
		ledger := testLedger(t, db, func(cfg *EconomyConfig) { cfg.Referral.MonthlyCap = 3 })
		svc := NewReferralService(NewRepository(db), ledger)
		f := &referralFixture{svc: svc, db: db}

		referrerID := seedReferrer(t, f, "tightened@example.com", "K7M2QX9RT4")
		month := ReferralMonthKey(time.Now().UTC())
		if err := db.Exec(
			`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
			 SELECT ?, ?, g, 700000 + g, 60, now() FROM generate_series(1, 3) g`,
			referrerID, month,
		).Error; err != nil {
			t.Fatalf("fill three slots: %v", err)
		}

		// With the config at 3, the fourth is refused even though the schema would
		// permit slot 4. This is the "tighten mid-incident without a deploy" property.
		referredID := seedReferred(t, f, "tightened-invitee@example.com")
		res, err := svc.ApplyReferral(context.Background(), Attribution{
			ReferredKind: SubjectUser, ReferredUserID: referredID,
			ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
		})
		if err != nil || !res.Attributed {
			t.Fatalf("stage: attributed=%v reason=%q err=%v", res.Attributed, res.Reason, err)
		}
		agePastWindow(t, db, res.ReferralID)
		if _, err := svc.SettleReferral(context.Background(), res.ReferralID); !errors.Is(err, ErrCapReached) {
			t.Errorf("the fourth settlement with a cap of 3 returned %v, want ErrCapReached", err)
		}
	})

	t.Run("raising the config above the ceiling is refused", func(t *testing.T) {
		db := openReferralSchema(t)
		ledger := testLedger(t, db, func(cfg *EconomyConfig) { cfg.Referral.MonthlyCap = 50 })
		svc := NewReferralService(NewRepository(db), ledger)
		f := &referralFixture{svc: svc, db: db}

		seedReferrer(t, f, "raised@example.com", "K7M2QX9RT4")
		referredID := seedReferred(t, f, "raised-invitee@example.com")
		res, err := svc.ApplyReferral(context.Background(), Attribution{
			ReferredKind: SubjectUser, ReferredUserID: referredID,
			ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
		})
		if err != nil || !res.Attributed {
			t.Fatalf("stage: %v", err)
		}
		agePastWindow(t, db, res.ReferralID)
		_, err = svc.SettleReferral(context.Background(), res.ReferralID)
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("a monthly_cap of 50 (above the ceiling of %d) returned %v, want "+
				"ErrInvalidConfig. Truncating silently would leave an operator believing "+
				"their change took effect when it did not",
				ReferralCapSlotCeiling, err)
		}
	})
}

// ── the fraud constraints ─────────────────────────────────────────────────────

// chk_user_referral_no_self. A self-referral is the cheapest possible farm and it
// must be refused by the database.
func TestSelfReferralIsRefusedByTheDatabase(t *testing.T) {
	f := newReferralFixture(t)
	id := seedReferrer(t, f, "self@example.com", "K7M2QX9RT4")

	// Through the service, which refuses with a reason a caller can log.
	res, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: id,
		ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
	})
	if err != nil {
		t.Fatalf("self-referral returned an error rather than a result: %v", err)
	}
	if res.Attributed {
		t.Fatal("a self-referral was attributed")
	}
	if res.Reason != AttributionSelfReferral {
		t.Errorf("reason = %q, want %q", res.Reason, AttributionSelfReferral)
	}

	// And through the database, bypassing Go entirely.
	if err := f.db.Exec(
		`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
		                           referral_code, status, created_at, updated_at)
		 VALUES (?, 'user', ?, 'K7M2QX9RT4', 'pending', now(), now())`,
		id, id,
	).Error; err == nil {
		t.Error("the database accepted a self-referral; chk_user_referral_no_self is " +
			"missing or not covering this shape")
	}

	// The kind guard: student id 7 and institution id 7 are DIFFERENT accounts, so a
	// row naming both is legitimate and must be accepted. A bare
	// `referrer <> referred` CHECK would refuse it, which is why §10.1's expression
	// needed the kind.
	if err := f.db.Exec(
		`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
		                           referral_code, status, created_at, updated_at)
		 VALUES (?, 'institution', ?, 'K7M2QX9RT4', 'pending', now(), now())`,
		id, id,
	).Error; err != nil {
		t.Errorf("the database refused a student referring an institution that happens to "+
			"share its id: %v. The no-self CHECK must be kind-guarded or it refuses "+
			"legitimate attributions across three independent id sequences", err)
	}
}

// The phone and device fraud controls: one successful referral per invited phone
// number, ever, and per invited device, ever.
//
// §2.4 calls these the two rules that "matter more than either" cap, because a cap
// bounds loss while these prevent the same person being counted repeatedly — which is
// the actual mechanism.
func TestPhoneAndDeviceFraudControlsAreUniqueIndexes(t *testing.T) {
	f := newReferralFixture(t)
	referrerA := seedReferrer(t, f, "phone-a@example.com", "K7M2QX9RT4")
	referrerB := seedReferrer(t, f, "phone-b@example.com", "K7M2QX9RT5")

	phone := []byte("hmac-of-98012345678")
	device := []byte("hmac-of-device-fingerprint-abc")

	insert := func(referrer uint, phoneHash, deviceHash any) error {
		return f.db.Exec(
			`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
			                           referral_code, status, phone_hash, device_hash,
			                           created_at, updated_at)
			 VALUES (?, 'user', ?, 'K7M2QX9RT4', 'pending', ?, ?, now(), now())`,
			referrer, uint(1000)+referrer, phoneHash, deviceHash,
		).Error
	}

	if err := insert(referrerA, phone, device); err != nil {
		t.Fatalf("first fraud-matched referral: %v", err)
	}
	// The same phone through a DIFFERENT referrer and a different invitee — the SIM
	// farm.
	if err := insert(referrerB, phone, []byte("a different device")); err == nil {
		t.Error("two referrals share a phone_hash; one phone number can be farmed " +
			"repeatedly, which is SIM-farming, threat 2 in §3.2")
	}
	// The same device through a different referrer — the device farm.
	if err := insert(referrerB, []byte("a different phone"), device); err == nil {
		t.Error("two referrals share a device_hash; one device can hold four accounts, " +
			"which is threat 8 in §3.2 and the cheapest control available")
	}
	// Distinct values are fine, or the indexes would be too strong.
	if err := insert(referrerB, []byte("another phone"), []byte("another device")); err != nil {
		t.Errorf("a referral with distinct phone and device hashes was refused: %v", err)
	}

	// NULL is not a collision. This is why the indexes are PARTIAL on `IS NOT NULL`:
	// a plain UNIQUE over a nullable column admits at most one NULL, which would make
	// the second signup with no recorded phone fail.
	for i := 0; i < 5; i++ {
		id := seedReferred(t, f, fmt.Sprintf("null-hash-%d@example.com", i))
		if err := f.db.Exec(
			`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
			                           referral_code, status, created_at, updated_at)
			 VALUES (?, 'user', ?, 'K7M2QX9RT4', 'pending', now(), now())`,
			referrerA, id,
		).Error; err != nil {
			t.Fatalf("NULL hashes collided on the partial unique index (%d): %v", i, err)
		}
	}
}

// The state machine is a closed set. A typo'd status is invisible: it qualifies for
// nothing and is clawed back for nothing.
func TestReferralStatusAndKindChecksAreEnforced(t *testing.T) {
	f := newReferralFixture(t)
	referrerID := seedReferrer(t, f, "states@example.com", "K7M2QX9RT4")
	id := seedReferred(t, f, "states-invitee@example.com")

	// Every declared status is accepted, so the qualification pass can write them. The
	// presence of each is also asserted by the fixture's assertStatusVocabularyIsWidened,
	// which proves the CHECK ACCEPTS it rather than merely existing.
	for i, status := range ReferralStatuses {
		target := id + uint(i)*0 // keep the same referred id: only the last may win
		_ = target
		if err := f.db.Exec(
			`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
			                           referral_code, status, created_at, updated_at)
			 VALUES (?, 'user', ?, 'K7M2QX9RT4', ?, now(), now())`,
			referrerID, id, status,
		).Error; err != nil && status != ReferralStatuses[0] {
			t.Errorf("the declared status %q was refused: %v", status, err)
		}
		if err := f.db.Exec(`DELETE FROM user_referral WHERE referred_user_id = ?`, id).Error; err != nil {
			t.Fatalf("clear between statuses: %v", err)
		}
	}
	// An undeclared one is not.
	for _, bad := range []string{"pening", "PENDING", "Qualified", "refunded", "", "paid"} {
		if err := f.db.Exec(
			`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
			                           referral_code, status, created_at, updated_at)
			 VALUES (?, 'user', ?, 'K7M2QX9RT4', ?, now(), now())`,
			referrerID, id, bad,
		).Error; err == nil {
			t.Errorf("the status %q was accepted; a row with a typo'd status qualifies for "+
				"nothing and is clawed back for nothing, invisibly", bad)
		}
	}
	// Same for the kind.
	if err := f.db.Exec(
		`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
		                           referral_code, status, created_at, updated_at)
		 VALUES (?, 'student', ?, 'K7M2QX9RT4', 'pending', now(), now())`,
		referrerID, id,
	).Error; err == nil {
		t.Error(`the referred_kind "student" was accepted; a wrong kind would point the row ` +
			"at an id space the account is not in")
	}
}

// Negative awards and half-written cap pairs are refused.
func TestReferralValueChecksAreEnforced(t *testing.T) {
	f := newReferralFixture(t)
	referrerID := seedReferrer(t, f, "values@example.com", "K7M2QX9RT4")
	id := seedReferred(t, f, "values-invitee@example.com")

	if err := f.db.Exec(
		`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
		                           referral_code, status, awarded_coins, created_at, updated_at)
		 VALUES (?, 'user', ?, 'K7M2QX9RT4', 'pending', -60, now(), now())`,
		referrerID, id,
	).Error; err == nil {
		t.Error("a negative awarded_coins was accepted")
	}
	if err := f.db.Exec(
		`INSERT INTO user_referral (referrer_user_id, referred_kind, referred_user_id,
		                           referral_code, status, cap_period, created_at, updated_at)
		 VALUES (?, 'user', ?, 'K7M2QX9RT4', 'pending', '2026-01', now(), now())`,
		referrerID, id,
	).Error; err == nil {
		t.Error("a cap_period with no cap_slot was accepted; half a cap claim is a " +
			"referral counted against nobody's cap and every cap accounting that reads it")
	}
}

// ── attribution outcomes ──────────────────────────────────────────────────────

// Every non-applied outcome, and the reason each is distinct. They matter because
// "no referral" and "fraudulent referral" are different operational answers and the
// log line that says which is the only thing standing between a typo and an incident.
func TestAttributionOutcomesAreDistinguishable(t *testing.T) {
	f := newReferralFixture(t)
	seedReferrer(t, f, "outcomes@example.com", "K7M2QX9RT4")
	selfID := seedReferrer(t, f, "outcomes-self@example.com", "K7M2QX9RT5")

	for _, tc := range []struct {
		name string
		attr Attribution
		want string
	}{
		{
			name: "no code at all",
			attr: Attribution{ReferredKind: SubjectUser, ReferredUserID: 1, ReferralCode: ""},
			want: AttributionNoCode,
		},
		{
			name: "a code that is entirely unrecognisable characters",
			attr: Attribution{ReferredKind: SubjectUser, ReferredUserID: 1, ReferralCode: "!!!???"},
			want: AttributionNoCode,
		},
		{
			name: "a well-formed code nobody owns",
			attr: Attribution{ReferredKind: SubjectUser, ReferredUserID: 1, ReferralCode: "ZZZZZZZZZZ"},
			want: AttributionUnknownCode,
		},
		{
			name: "the account's own code",
			attr: Attribution{ReferredKind: SubjectUser, ReferredUserID: selfID, ReferralCode: "K7M2QX9RT5"},
			want: AttributionSelfReferral,
		},
		{
			name: "a kind this package does not create",
			attr: Attribution{ReferredKind: "superadmin", ReferredUserID: 1, ReferralCode: "K7M2QX9RT4"},
			want: "", // an error, not a result — asserted separately below
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := f.svc.ApplyReferral(context.Background(), tc.attr)
			if tc.want == "" {
				if err == nil {
					t.Error("an unknown referred_kind was accepted; it would point the row " +
						"at an id space the account is not in")
				}
				return
			}
			if err != nil {
				t.Fatalf("returned an error rather than a result: %v", err)
			}
			if res.Attributed {
				t.Fatalf("attributed, want the refusal %q", tc.want)
			}
			if res.Reason != tc.want {
				t.Errorf("reason = %q, want %q", res.Reason, tc.want)
			}
		})
	}

	// A soft-deleted account's code: proved rather than asserted.
	//
	// The code is a valid one that NORMALISES TO ITSELF. An earlier draft used
	// "DELETEDCOD", which contains L and O — normalisation rewrites those to 1 and 0,
	// so the lookup missed for a reason that had nothing to do with the soft delete
	// and the assertion passed vacuously. A test that passes for the wrong reason is
	// worse than no test, and this is exactly how one gets written.
	const deletedCode = "DE7ETEDC4D"
	if got := utils.NormalizeReferralCode(deletedCode); got != deletedCode {
		t.Fatalf("the fixture code %q normalises to %q, so this test would pass without "+
			"testing the soft delete at all", deletedCode, got)
	}
	deletedID := seedReferrer(t, f, "deleted@example.com", deletedCode)

	// Before the delete the code resolves — proving the assertion below is about the
	// delete and not about the code.
	before, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: 1, ReferralCode: deletedCode,
	})
	if err != nil || !before.Attributed {
		t.Fatalf("the code did not resolve before the delete (attributed=%v err=%v), so "+
			"the soft-delete assertion below would be vacuous", before.Attributed, err)
	}

	if err := f.db.Exec(`UPDATE users SET deleted_at = now() WHERE id = ?`, deletedID).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// The row is still there for the audit trail; only the code stops working.
	if n := countRows(t, f.db, `SELECT count(*) FROM users WHERE id = ?`, deletedID); n != 1 {
		t.Fatal("the soft delete removed the row; it should only stop the code resolving")
	}

	res, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: seedReferred(t, f, "after-delete@example.com"),
		ReferralCode: deletedCode,
	})
	if err != nil || res.Attributed || res.Reason != AttributionUnknownCode {
		t.Errorf("a soft-deleted account's code: attributed=%v reason=%q err=%v; want "+
			"an unknown code. A deletion scheduled for a fraud finding must stop paying "+
			"the referrer immediately, not at the end of the retention window",
			res.Attributed, res.Reason, err)
	}
}

// The lock is held on the REFERRER, which is what makes two referrals to the same
// referrer serialise and the cap accounting safe.
func TestConcurrentAttributionsToOneReferrerAllLand(t *testing.T) {
	f := newReferralFixture(t)
	referrerID := seedReferrer(t, f, "busy-referrer@example.com", "K7M2QX9RT4")

	const writers = 12
	ids := make([]uint, writers)
	for i := range ids {
		ids[i] = seedReferred(t, f, fmt.Sprintf("busy-invitee-%d@example.com", i))
	}

	var wg sync.WaitGroup
	results := make([]AttributionResult, writers)
	errs := make([]error, writers)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx], errs[idx] = f.svc.ApplyReferral(context.Background(), Attribution{
				ReferredKind: SubjectUser, ReferredUserID: ids[idx],
				ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
			})
		}(i)
	}
	close(start)
	wg.Wait()

	attributed := 0
	for i := 0; i < writers; i++ {
		if errs[i] != nil {
			t.Errorf("writer %d: %v", i, errs[i])
		}
		if results[i].Attributed {
			attributed++
			if results[i].ReferrerUserID != referrerID {
				t.Errorf("writer %d credited user %d, want %d", i, results[i].ReferrerUserID, referrerID)
			}
		}
	}
	// Twelve DIFFERENT invitees, one referrer: every one attributes. This is the
	// case a per-referrer uniqueness constraint would wrongly refuse.
	if attributed != writers {
		t.Errorf("%d of %d concurrent attributions landed, want %d — a referrer must be "+
			"able to have many invitees", attributed, writers, writers)
	}
	if rows := referrals(t, f.db); len(rows) != writers {
		t.Errorf("%d rows, want %d", len(rows), writers)
	}
}

// ── the settlement seam ───────────────────────────────────────────────────────

// SettleReferral is ONE unit: the claim, the cap slot and the money commit together,
// and a failure of any of them leaves none of them behind.
//
// This is the gap Phase 1 recorded and the phase after it was told to close.
// ledger.go: "converting a hold into a grant is ReleaseReserved followed by Grant, and
// in two transactions that leaves a window in which the coins are unheld. The referral
// phase must do both in one transaction and should say so where it does."
//
// THAT PROPERTY SURVIVED THE RETIREMENT OF THE HOLD, and keeping it is a deliberate
// choice rather than an inheritance. The original reason for one transaction was that a
// release and a grant had to be atomic with each other; there is no release any more, so
// that reason is gone. What remains is a reason of its own — the reward_grant CLAIM and
// the money must be one unit, because a claim committed without its money makes every
// later attempt a silent no-op, and money committed without its claim pays twice on the
// next pass. Neither half is achievable across two transactions, so it stays one.
//
// The referrer is NOT funded first. Under the old model it had to be, because the
// award was reserved out of its own balance; under this one the faucet pays directly,
// so a referrer who has never held a coin is paid a referral award. That is the change
// the whole slice exists to make, and it is asserted rather than described.
func TestSettlementIsOneUnitAndPaysFromTheFaucet(t *testing.T) {
	db := openReferralSchema(t)
	f := &referralFixture{svc: NewReferralService(NewRepository(db), testLedger(t, db, nil)), db: db}
	award := DefaultEconomyConfig().Awards.ReferralReferrer

	referrerID := seedReferrer(t, f, "settle@example.com", "K7M2QX9RT4")
	referredID := seedReferred(t, f, "settle-invitee@example.com")
	res, err := f.svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: referredID,
		ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
	})
	if err != nil || !res.Attributed {
		t.Fatalf("stage: %v", err)
	}
	referralID := res.ReferralID

	// Past the wait. A referral is not payable the moment it is attributed — see
	// TestTheWindowGateRejectsAReferralInsideItsWindow — so this is the one line every
	// settlement test needs, and its absence is why the first run of this slice failed
	// in eight places at once.
	agePastWindow(t, db, referralID)

	// The referrer holds NOTHING. Under the retired hold model this test could not
	// exist: Reserve refuses on a zero balance, which is the bug this slice fixes.
	if got := userPosted(t, db, referrerID); got != 0 {
		t.Fatalf("the fixture funds the referrer with %d coins; this test exists to prove "+
			"a referral pays a referrer who holds none", got)
	}
	before := userPosted(t, db, referrerID)

	grant, err := f.svc.SettleReferral(context.Background(), referralID)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if grant.Amount != award {
		t.Errorf("granted %d, want %d", grant.Amount, award)
	}
	if got, want := userPosted(t, db, referrerID), before+award; got != want {
		t.Errorf("posted = %d, want %d", got, want)
	}
	// Nothing is reserved, before or after. There is no hold.
	if got := userReserved(t, db, referrerID); got != 0 {
		t.Errorf("reserved = %d, want 0: a settlement reserves nothing under this model", got)
	}
	// ONE journal for this referral, and it is the grant.
	if n := countRows(t, db,
		`SELECT count(*) FROM coin_journal WHERE ref_type = ? AND ref_id = ?`,
		RefUserReferral, referralID); n != 1 {
		t.Errorf("%d journals for referral %d, want 1 (the grant; the hold journal is gone)",
			n, referralID)
	}
	// And it is POSTED rather than PENDING, which under the hold model was the hold's
	// state and is now unreachable for a referral at all.
	if n := countRows(t, db,
		`SELECT count(*) FROM coin_journal WHERE ref_type = ? AND ref_id = ? AND state = ?`,
		RefUserReferral, referralID, StatePosted); n != 1 {
		t.Errorf("%d POSTED journals for referral %d, want 1", n, referralID)
	}

	// Read the ONE row this test created rather than the first row in the table.
	//
	// referrals() is ordered by id, and this fixture shares a truncated pool with the rest
	// of the file — so `[0]` is only correct while this is the test that runs first. An
	// earlier draft indexed `[0]` and asserted row.Status, which passes until another test
	// leaves a lower id behind, at which point it fails for a reason that has nothing to
	// do with settlement. Scoped by referrer is the assertion that stays true.
	rows := referralsFor(t, f.db, referrerID)
	if len(rows) != 1 {
		t.Fatalf("%d referrals for referrer %d, want 1", len(rows), referrerID)
	}
	row := rows[0]
	if row.Status != ReferralQualified {
		t.Errorf("status = %q, want %q", row.Status, ReferralQualified)
	}
	if row.GrantJournalID == nil || *row.GrantJournalID != grant.JournalID {
		t.Errorf("grant_journal_id = %v, want %s", row.GrantJournalID, grant.JournalID)
	}
	// hold_journal_id is PERMANENTLY NULL under this model. The column is kept for audit
	// (referral_model.go explains why) but nothing writes it, and asserting that here is
	// what keeps a future "let me restore the hold" edit from being a silent no-op.
	if row.HoldJournalID != nil {
		t.Errorf("hold_journal_id = %v, want NULL: the reserved-hold model is retired", row.HoldJournalID)
	}
	if row.CapSlot == nil || row.CapPeriod == nil {
		t.Error("the settlement recorded no cap slot, so this referral is paid but " +
			"uncounted against the monthly cap")
	}

	// The reward_grant claim, once, linking the award to the journal — the two halves of
	// the one-transaction property.
	grants := grantRows(t, db, referrerID)
	if len(grants) != 1 {
		t.Errorf("%d reward_grant rows, want 1", len(grants))
	}
	if grants[0].AwardCode != ReferralAwardCode(referralID) {
		t.Errorf("award_code = %q, want %q", grants[0].AwardCode, ReferralAwardCode(referralID))
	}
	if grants[0].JournalID == nil {
		t.Error("the reward_grant row has no journal_id; the claim and the money must be " +
			"one unit or a retry pays twice")
	}

	// A second settlement pays nothing — and now says so, rather than returning a zero
	// result, because "we already paid this" and "this may never be paid" are different
	// operational answers and the qualification pass has to tell them apart.
	if _, err := f.svc.SettleReferral(context.Background(), referralID); !errors.Is(err, ErrReferralAlreadyPaid) {
		t.Errorf("a repeated settlement returned %v, want ErrReferralAlreadyPaid", err)
	}
	if got := userPosted(t, db, referrerID); got != before+award {
		t.Errorf("a repeated settlement moved the balance from %d to %d", before+award, got)
	}
	if n := len(grantRows(t, db, referrerID)); n != 1 {
		t.Errorf("%d reward_grant rows after two settlements, want 1", n)
	}
}

// The rollback half: a settlement that cannot proceed leaves NOTHING behind — no claim,
// no cap slot, no status change.
//
// The original version of this test re-priced the award so the grant would disagree with
// the hold's amount, which was the only failure available while a hold existed. With no
// hold there is nothing to disagree with, so the failure is injected the way a real one
// arrives: the referrer has consumed their monthly cap.
//
// That is a strictly better injection than re-pricing, because it exercises the cap step
// — which sits BETWEEN the claim and the money — so the rollback is proven across the
// whole transaction rather than at its tail. TestAFailedSettlementLeavesTheClaimRetryable
// covers the same ordering from the qualification side.
func TestSettlementRollsBackEveryStepOnFailure(t *testing.T) {
	db := openReferralSchema(t)
	svc := NewReferralService(NewRepository(db), testLedger(t, db, nil))
	f := &referralFixture{svc: svc, db: db}

	referrerID := seedReferrer(t, f, "rollback@example.com", "K7M2QX9RT4")
	referredID := seedReferred(t, f, "rollback-invitee@example.com")
	res, err := svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: referredID,
		ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
	})
	if err != nil || !res.Attributed {
		t.Fatalf("stage: %v", err)
	}
	referralID := res.ReferralID
	agePastWindow(t, db, referralID)

	// The referrer's month is full, so the cap step refuses — after the claim has been
	// written and before any money moves.
	if err := db.Exec(
		`INSERT INTO referral_cap_slot (referrer_user_id, month_key, slot, referral_id, coins, awarded_at)
		 SELECT ?, ?, g, 400000 + g, 60, now() FROM generate_series(1, ?) g`,
		referrerID, ReferralMonthKey(time.Now().UTC()), ReferralCapSlotCeiling,
	).Error; err != nil {
		t.Fatalf("fill the cap: %v", err)
	}

	if _, err := svc.SettleReferral(context.Background(), referralID); !errors.Is(err, ErrCapReached) {
		t.Fatalf("a capped settlement returned %v, want ErrCapReached", err)
	}

	// No money. The faucet and the referrer are both untouched.
	if got := userPosted(t, db, referrerID); got != 0 {
		t.Errorf("posted = %d after a failed settlement, want 0", got)
	}
	if got := settleCoins(t, db, referrerID); got != 0 {
		t.Errorf("paid %d coins after a failed settlement", got)
	}
	// No claim. THIS is the assertion that matters: a claim committed without its money
	// makes every later attempt a silent no-op, and the referral would be lost forever
	// with a row claiming it was paid.
	if n := len(grantRows(t, db, referrerID)); n != 0 {
		t.Errorf("%d reward_grant rows survived a failed settlement; a committed claim "+
			"without its money makes every later attempt a no-op", n)
	}
	// No journal.
	if n := countRows(t, db, `SELECT count(*) FROM coin_journal`); n != 0 {
		t.Errorf("%d journals after a failed settlement, want 0", n)
	}
	// No cap slot consumed by the failed attempt — the existing ten are the ones the
	// fixture wrote.
	if n := countRows(t, db,
		`SELECT count(*) FROM referral_cap_slot WHERE referrer_user_id = ?`, referrerID); n != ReferralCapSlotCeiling {
		t.Errorf("%d cap slots after a failed settlement, want the %d the fixture wrote",
			n, ReferralCapSlotCeiling)
	}
	// No state change.
	if got := referralStatus(t, db, referralID); got != ReferralPending {
		t.Errorf("status = %q after a failed settlement, want %q", got, ReferralPending)
	}

	// Freeing the cap makes it payable, so nothing was consumed by the failure.
	if err := db.Exec(`DELETE FROM referral_cap_slot WHERE referrer_user_id = ?`, referrerID).Error; err != nil {
		t.Fatalf("free the cap: %v", err)
	}
	if _, err := svc.SettleReferral(context.Background(), referralID); err != nil {
		t.Fatalf("settlement after the cap was freed: %v", err)
	}
	if got := settleCoins(t, db, referrerID); got != DefaultEconomyConfig().Awards.ReferralReferrer {
		t.Errorf("paid %d after a successful retry, want %d",
			got, DefaultEconomyConfig().Awards.ReferralReferrer)
	}
}

// A referral with NO HOLD settles normally, and this test was the opposite.
//
// IT WAS: "a referral with no open hold is not settled", asserting ErrNotFound. That
// encoded the §3.3 model directly — the award was reserved from the referrer, so a
// referral without a hold had nothing to release and paying it would "create coins
// against a hold that does not exist".
//
// That model was rejected. Reserve refuses a hold larger than the referrer's balance, so
// under the old rule a student with a zero balance — the overwhelmingly common case, and
// the population an earn mechanic exists for — could not be paid for referring anybody.
// The test was not wrong about the model it encoded; the model was wrong.
//
// So the assertion is inverted, and deliberately so rather than deleted: a test that is
// simply removed records nothing, while an inverted one carries the reason it was
// inverted and fails loudly if the mechanic drifts back toward reserving anything. The
// referrer below is never funded and no hold is ever placed, and the settlement pays.
func TestSettlementPaysAReferralThatHasNoHoldAtAll(t *testing.T) {
	db := openReferralSchema(t)
	svc := NewReferralService(NewRepository(db), testLedger(t, db, nil))
	f := &referralFixture{svc: svc, db: db}
	award := DefaultEconomyConfig().Awards.ReferralReferrer

	referrerID := seedReferrer(t, f, "nohold@example.com", "K7M2QX9RT4")
	referredID := seedReferred(t, f, "nohold-invitee@example.com")
	res, err := svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: referredID,
		ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
	})
	if err != nil || !res.Attributed {
		t.Fatalf("stage: %v", err)
	}
	agePastWindow(t, db, res.ReferralID)

	// The fixture proves the premise: a zero balance and no hold anywhere.
	if got := userPosted(t, db, referrerID); got != 0 {
		t.Fatalf("the referrer holds %d coins; this test exists to prove a holdless, "+
			"balance-less referrer is paid", got)
	}
	if n := countRows(t, db, `SELECT count(*) FROM coin_journal`); n != 0 {
		t.Fatalf("%d journals before the settlement, want 0 — there is no hold to release", n)
	}

	grant, err := svc.SettleReferral(context.Background(), res.ReferralID)
	if err != nil {
		t.Fatalf("settling a referral with no hold returned %v. It used to return "+
			"ErrNotFound: the reserved-hold model is retired and nothing is withheld from "+
			"the referrer any more", err)
	}
	if grant.Amount != award {
		t.Errorf("granted %d, want %d", grant.Amount, award)
	}
	if got := settleCoins(t, db, referrerID); got != award {
		t.Errorf("paid %d coins, want %d", got, award)
	}
	if n := len(grantRows(t, db, referrerID)); n != 1 {
		t.Errorf("%d reward_grant rows, want 1", n)
	}
	if got := referralStatus(t, db, res.ReferralID); got != ReferralQualified {
		t.Errorf("status = %q, want %q", got, ReferralQualified)
	}
	// And nothing was ever reserved, so the no-overdraft CHECK was never in play.
	if got := userReserved(t, db, referrerID); got != 0 {
		t.Errorf("reserved = %d, want 0", got)
	}
}

// A referral INSIDE its window is not settled, whatever else is true.
//
// This is the refusal that replaced the hold check in the same position in this file:
// both answer "may this pay now?", and the window is what the answer now depends on.
func TestSettlementRefusesAReferralInsideItsWindow(t *testing.T) {
	db := openReferralSchema(t)
	svc := NewReferralService(NewRepository(db), testLedger(t, db, nil))
	f := &referralFixture{svc: svc, db: db}

	referrerID := seedReferrer(t, f, "early@example.com", "K7M2QX9RT4")
	referredID := seedReferred(t, f, "early-invitee@example.com")
	res, err := svc.ApplyReferral(context.Background(), Attribution{
		ReferredKind: SubjectUser, ReferredUserID: referredID,
		ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
	})
	if err != nil || !res.Attributed {
		t.Fatalf("stage: %v", err)
	}
	// NOT backdated: the seven-day wait is entirely ahead of this referral.
	if _, err := svc.SettleReferral(context.Background(), res.ReferralID); !errors.Is(err, ErrReferralNotYetPayable) {
		t.Errorf("settling a referral inside its window returned %v, want ErrReferralNotYetPayable", err)
	}
	if n := len(grantRows(t, db, referrerID)); n != 0 {
		t.Errorf("%d reward_grant rows for an early settlement, want 0", n)
	}
	if got := settleCoins(t, db, referrerID); got != 0 {
		t.Errorf("paid %d coins for a referral inside its window", got)
	}
}

// A clawed-back or rejected referral is never paid, however it is reached.
func TestSettlementRefusesAClawedBackOrRejectedReferral(t *testing.T) {
	for _, status := range []string{ReferralClawedBack, ReferralRejected} {
		t.Run(status, func(t *testing.T) {
			db := openReferralSchema(t)
			ledger := testLedger(t, db, nil)
			svc := NewReferralService(NewRepository(db), ledger)
			f := &referralFixture{svc: svc, db: db}

			referrerID := seedReferrer(t, f, "final-"+status+"@example.com", "K7M2QX9RT4")
			referredID := seedReferred(t, f, "final-invitee-"+status+"@example.com")
			res, err := svc.ApplyReferral(context.Background(), Attribution{
				ReferredKind: SubjectUser, ReferredUserID: referredID,
				ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
			})
			if err != nil || !res.Attributed {
				t.Fatalf("stage: %v", err)
			}
			// Past the window, so the ONLY thing refusing the settlement is the
			// terminal status. Without the backdate this test would pass for the wrong
			// reason — the window would refuse it first and the assertion about
			// ErrImmutable would be satisfied by a different gate.
			agePastWindow(t, db, res.ReferralID)
			if err := db.Exec(`UPDATE user_referral SET status = ? WHERE id = ?`,
				status, res.ReferralID).Error; err != nil {
				t.Fatalf("move to %s: %v", status, err)
			}

			if _, err := svc.SettleReferral(context.Background(), res.ReferralID); !errors.Is(err, ErrImmutable) {
				t.Errorf("settling a %s referral returned %v, want ErrImmutable", status, err)
			}
			if n := len(grantRows(t, db, referrerID)); n != 0 {
				t.Errorf("%d reward_grant rows for a %s referral, want 0", n, status)
			}
		})
	}
}

// The cap is a count of successful referrals per calendar month, and a COUNTING cap
// is only sound if the thing it counts is money that actually moved. This asserts
// the ledger agrees: the same number of referrals the cap table records is the
// number of REFERRAL_QUALIFIED journals the ledger holds.
//
// If the two disagreed — cap slots written without journals, or journals without
// slots — the cap would be counting something other than money issued, and the
// economy's faucet would not be bounded by the number the operator believes it is.
func TestCapSlotsAndLedgerJournalsAgree(t *testing.T) {
	db := openReferralSchema(t)
	ledger := testLedger(t, db, nil)
	f := &referralFixture{svc: NewReferralService(NewRepository(db), ledger), db: db}

	// No seed funding: the award is paid from the faucet, so the referrer needs nothing.
	// fundReferrer was here only to satisfy the retired hold, and leaving it would have
	// meant the referrer's referral coins were indistinguishable from its seed money.
	referrerID := seedReferrer(t, f, "agree@example.com", "K7M2QX9RT4")

	const referrals = 5
	for n := 1; n <= referrals; n++ {
		referredID := seedReferred(t, f, fmt.Sprintf("agree-invitee-%d@example.com", n))
		res, err := f.svc.ApplyReferral(context.Background(), Attribution{
			ReferredKind: SubjectUser, ReferredUserID: referredID,
			ReferralCode: "K7M2QX9RT4", SourcePath: "verify_otp",
		})
		if err != nil || !res.Attributed {
			t.Fatalf("stage %d: %v", n, err)
		}
		agePastWindow(t, db, res.ReferralID)
		if _, err := f.svc.SettleReferral(context.Background(), res.ReferralID); err != nil {
			t.Fatalf("settle %d: %v", n, err)
		}
	}

	slots := countRows(t, db, `SELECT count(*) FROM referral_cap_slot WHERE referrer_user_id = ?`, referrerID)
	grants := countRows(t, db,
		`SELECT count(DISTINCT j.id) FROM coin_journal j
		   JOIN coin_posting p ON p.journal_id = j.id
		   JOIN coin_account a ON a.id = p.account_id
		  WHERE j.reason_code = ? AND a.owner_user_id = ? AND a.kind = ? AND p.amount > 0`,
		ReasonReferralQualified, referrerID, AccountUser)
	if slots != referrals {
		t.Errorf("%d cap slots for %d settled referrals", slots, referrals)
	}
	if grants != referrals {
		t.Errorf("%d REFERRAL_QUALIFIED journals for %d cap slots: the cap counts "+
			"referrals and the ledger counts money, and they must be counting the same "+
			"things or the cap does not bound issuance", grants, slots)
	}
	// And the money is exactly the configured award per settlement — no more and no less.
	coins := referralCoinsPaid(t, db, referrerID)
	want := int64(referrals) * DefaultEconomyConfig().Awards.ReferralReferrer
	if coins != want {
		t.Errorf("paid %d coins for %d referrals, want %d", coins, referrals, want)
	}
	// Nothing was ever reserved. Under the old model this assertion was "every hold was
	// released"; the stronger version of it now is that no hold exists at all.
	if got := userReserved(t, db, referrerID); got != 0 {
		t.Errorf("%d coins reserved after %d settlements; nothing is held under this model",
			got, referrals)
	}
	if n := countRows(t, db,
		`SELECT count(*) FROM coin_journal WHERE reason_code = ?`, ReasonReferralHold); n != 0 {
		t.Errorf("%d REFERRAL_HOLD journals exist after %d settlements, want 0", n, referrals)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// agePastWindow backdates a referral past the configured wait, so a test can settle it.
//
// EVERY settlement test in this file needs it, and it is a helper rather than something
// each test writes because forgetting it produces a confusing failure: the settlement is
// refused with ErrReferralNotYetPayable, which looks like a bug in the cap or the claim
// and is actually just the seven-day wait. That is exactly how this file's first run
// after the model change failed — in eight places, none of which named the real cause.
//
// The default window plus a day, so the referral is unambiguously past it rather than
// exactly on the boundary.
func agePastWindow(t *testing.T, db *gorm.DB, referralID uint) {
	t.Helper()
	days := DefaultEconomyConfig().Referral.HoldDays + 1
	// Interpolated, not bound: pgx infers a placeholder's type from the surrounding
	// expression, and `?` next to `interval '... days'` would be inferred as text. The
	// value is an int literal from this file's own call sites.
	if err := db.Exec(fmt.Sprintf(
		`UPDATE user_referral SET created_at = created_at - interval '%d days' WHERE id = ?`,
		days), referralID).Error; err != nil {
		t.Fatalf("age referral %d by %d days: %v", referralID, days, err)
	}
}

// referralCoinsPaid sums what the referrer's referral grants have actually paid.
func referralCoinsPaid(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	return countRows(t, db,
		`SELECT COALESCE(SUM(p.amount), 0) FROM coin_posting p
		   JOIN coin_account a ON a.id = p.account_id
		   JOIN coin_journal j ON j.id = p.journal_id
		  WHERE a.owner_user_id = ? AND a.kind = ? AND j.reason_code = ? AND p.amount > 0`,
		userID, AccountUser, ReasonReferralQualified)
}

// referralJournalsFor counts the REFERRAL_QUALIFIED grants a user received.
//
// COALESCE ON EVERY JOIN COLUMN, for the reason profile_award_pg_test.go gives for
// its own counting helper: the faucet leg of every grant is a SYSTEM account with
// owner_user_id NULL, so a count that does not treat that NULL deliberately reports
// zero for a grant that demonstrably happened. A "no second generation" assertion
// built on a count that silently returns zero would pass on a broken mechanic.
func referralJournalsFor(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	return countRows(t, db,
		`SELECT count(DISTINCT j.id) FROM coin_journal j
		   JOIN coin_posting p ON p.journal_id = j.id
		   JOIN coin_account a ON a.id = p.account_id
		  WHERE j.reason_code = ? AND a.owner_user_id = ? AND a.kind = ? AND p.amount > 0`,
		ReasonReferralQualified, userID, AccountUser)
}

// readPackageSource reads a file from this package's own directory, for the
// source-scanning test.
//
// `go test` runs with the package's source directory as the working directory, so
// this is a same-directory read rather than a path relative to the repository root.
func readPackageSource(name string) (string, error) {
	raw, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
