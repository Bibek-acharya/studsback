// internal/coins/referral_test.go
//
// The properties of the attribution mechanic that are pure logic: code
// normalisation, the award-code shape, the month key, and the fraud-identifier
// hashing.
//
// Deliberately separate from referral_pg_test.go, which needs a real PostgreSQL
// instance. Everything here runs in the ordinary `go test ./...`, so a regression in
// the normalisation rules or the month key is caught before a deploy rather than in
// an integration run somebody has to remember to schedule.
package coins

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"studsphere/backend/internal/shared/utils"
)

// Normalisation happens on every code that arrives, from every one of the six
// user-creation paths. A bug here is a silently lost referral — the student typed
// their code, the lookup missed, nothing said so — which is the failure mode with
// no visible symptom anywhere.
func TestReferralCodeNormalisationCollapsesSpellings(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"canonical is unchanged", "K7M2QX9RT4", "K7M2QX9RT4"},
		{"lower case", "k7m2qx9rt4", "K7M2QX9RT4"},
		{"mixed case", "K7m2Qq9Rt4", "K7M2QQ9RT4"},
		{"a hyphen from a chat client", "K7M2-QX9RT4", "K7M2QX9RT4"},
		{"a space from reading it aloud", "K7M2 QX9RT4", "K7M2QX9RT4"},
		{"both", "k7m2 qx9-rt4", "K7M2QX9RT4"},
		{"punctuation", "<K7M2QX9RT4>", "K7M2QX9RT4"},
		{"Crockford O is zero", "KO2QX9RT4", "K02QX9RT4"},
		{"Crockford I is one", "K7I2QX9RT4", "K712QX9RT4"},
		{"Crockford L is one", "K7L2QX9RT4", "K712QX9RT4"},
		{"nothing recognisable", "%%%", ""},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := utils.NormalizeReferralCode(tc.in); got != tc.want {
				t.Errorf("NormalizeReferralCode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// The award code must be unique per referral, because reward_grant's UNIQUE is
// (user_id, award_code) and a per-user code would pay the first referral and never
// the other nine under the monthly cap.
func TestReferralAwardCodeIsPerReferral(t *testing.T) {
	seen := map[string]bool{}
	for id := uint(1); id <= 50; id++ {
		code := ReferralAwardCode(id)
		if seen[code] {
			t.Fatalf("two referrals share award_code %q", code)
		}
		seen[code] = true
		if !strings.HasPrefix(code, "REFERRAL_BONUS:") {
			t.Errorf("award_code = %q, want a REFERRAL_BONUS: prefix so it is greppable "+
				"alongside the profile ladder's PROFILE_STEP:", code)
		}
		if !strings.Contains(code, ":") {
			t.Errorf("award_code %q carries no referral id, so it cannot be unique per referral", code)
		}
	}
	// Stable for the same id: a code that changed between the claim and the grant
	// would create two reward_grant rows for one referral.
	if ReferralAwardCode(7) != ReferralAwardCode(7) {
		t.Error("ReferralAwardCode is not stable for the same referral")
	}
}

// The month key is UTC and calendar-shaped.
//
// UTC is the load-bearing decision, and the reasoning is in referral_cap_model.go:
// a cap that resets at local midnight in Asia/Kathmandu (UTC+5:45) would open the
// new month at 18:15 UTC the previous day, so two months' caps would overlap by six
// hours twice a year — at the exact moment the cap resets, which is when a fraudster
// wants it.
func TestReferralMonthKeyIsUTCCalendarMonth(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   time.Time
		want string
	}{
		{
			// Nepal is UTC+05:45 — a 45-minute offset, which is unusual and is the
			// whole reason this test exists. 2026-03-01 00:30 local is
			// 2026-02-28 18:45 UTC: under local time this is March, under UTC it is
			// still February. The cap belongs to February.
			//
			// time.FixedZone takes an offset in SECONDS, and 45 minutes is exactly
			// the sort of detail a test can silently get wrong by writing 5*60+45.
			// Nepal = 5h45m = 20700s.
			name: "Nepal's first minute of March is still February in UTC",
			in:   time.Date(2026, 3, 1, 0, 30, 0, 0, time.FixedZone("NPT", 20700)),
			want: "2026-02",
		},
		{
			// The other side of the same boundary, and the one that actually costs
			// money: 2026-02-28 23:30 local is 17:45 UTC the same day, so this
			// account's cap resets eight and a quarter hours before Nepal's does.
			// Under UTC both sides agree, which is the point.
			name: "Nepal's last minute of February is still February in UTC",
			in:   time.Date(2026, 2, 28, 23, 30, 0, 0, time.FixedZone("NPT", 20700)),
			want: "2026-02",
		},
		{
			// And the overlap window, which is what a LOCAL cap would produce: these
			// two instants are the same UTC hour but different local months. Under a
			// local cap, an account's March would open while a UTC cap's February
			// was still running.
			name: "Nepal's midnight on the first has already opened March locally but not in UTC",
			in:   time.Date(2026, 3, 1, 5, 0, 0, 0, time.FixedZone("NPT", 20700)),
			want: "2026-02",
		},
		{"mid-month UTC", time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC), "2026-05"},
		{"the last instant of a month", time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC), "2026-05"},
		{"the first instant of the next", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), "2026-06"},
		{"December rolls the year", time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC), "2026-12"},
		{"January after a year boundary", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), "2027-01"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReferralMonthKey(tc.in); got != tc.want {
				t.Errorf("ReferralMonthKey(%s) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// The month key must match the CHECK's regex, or a settlement would fail on its
// own state write.
func TestReferralMonthKeySatisfiesItsOwnCheckConstraint(t *testing.T) {
	// The expression in chk_referral_cap_slot: '^[0-9]{4}-[0-9]{2}$'.
	want := regexp.MustCompile(`^[0-9]{4}-[0-9]{2}$`)
	for i := 0; i < 400; i++ {
		at := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i*3)
		key := ReferralMonthKey(at)
		if !want.MatchString(key) {
			t.Fatalf("month key %q (from %s) does not match the CHECK's pattern, so a "+
				"settlement writing it would be refused by the database", key, at)
		}
		if len(key) != 7 {
			t.Fatalf("month key %q is %d chars, and the column is char(7)", key, len(key))
		}
	}
}

// The fraud hash is keyed, and keyed differently for different key ids.
//
// Keyed rather than a bare SHA-256, because the space of Nepali mobile numbers is
// small enough to enumerate: a plain hash of a phone number is reversible by anyone
// who has the table, so the "privacy control" would be no control at all. An HMAC
// under a key that never enters the database is not reversible that way.
//
// Different key ids must produce different hashes, or rotating the key would
// silently make old and new rows comparable — which defeats the point of rotation
// and lets a fraudster match an account across a rotation.
func TestHashFraudIdentifierIsKeyedAndKeyIDSeparated(t *testing.T) {
	key := []byte("a-key-that-never-enters-the-database")

	a := HashFraudIdentifier(key, "k1", "+9779812345678")
	b := HashFraudIdentifier(key, "k1", "+9779812345678")
	if string(a) != string(b) {
		t.Error("the same key, key id and value produced two different hashes; a fraud " +
			"match would fail for a reason that looks like a different person")
	}

	otherKeyID := HashFraudIdentifier(key, "k2", "+9779812345678")
	if string(a) == string(otherKeyID) {
		t.Error("two key ids produced the same hash for the same value, so rotating the " +
			"key would make old and new rows comparable")
	}

	otherKey := HashFraudIdentifier([]byte("a different key"), "k1", "+9779812345678")
	if string(a) == string(otherKey) {
		t.Error("two keys produced the same hash")
	}

	different := HashFraudIdentifier(key, "k1", "+9779899999999")
	if string(a) == string(different) {
		t.Error("two different phone numbers hashed to the same value")
	}

	// The hash must not be the value, obviously, and must be a plausible digest
	// length. A 32-byte SHA-256 HMAC is what the bytea column is sized for.
	if len(a) != 32 {
		t.Errorf("hash is %d bytes, want 32 (SHA-256)", len(a))
	}
	if strings.Contains(string(a), "9779") {
		t.Error("the hash contains the plaintext, so it is not a hash")
	}

	// No key means no hash. Returning nil rather than hashing with an empty key is
	// the safe direction: HMAC with an empty key is computable by anyone, so a
	// "protected" value would be reversible by anyone, and a caller that forgot to
	// configure the key should get nothing rather than something that looks
	// protected and is not.
	if got := HashFraudIdentifier(nil, "k1", "+9779812345678"); got != nil {
		t.Errorf("HashFraudIdentifier with no key returned %x, want nil: HMAC under an "+
			"empty key is computable by anyone, so a caller with no key must get "+
			"nothing rather than something that looks protected", got)
	}
}

// The HMAC input must be unambiguous: keyID and value are NUL-separated so no two
// different splits produce the same input. Without the separator, key "a" + value
// "b:c" and key "a:b" + value "c" collide.
func TestHashFraudIdentifierSeparatesItsInputs(t *testing.T) {
	key := []byte("separation-test-key")

	// Reuse the colon as the boundary so a naive concatenation would collide.
	x := HashFraudIdentifier(key, "scope", "user:1")
	y := HashFraudIdentifier(key, "scope:user", "1")
	if string(x) == string(y) {
		t.Error("key id and value are ambiguous: two different splits produced the " +
			"same hash, so a fraud match could be made across namespaces")
	}
}

// The referral fingerprint identifies a PAIR and is namespaced by kind, which is the
// same property referred_kind exists for: student 7 and institution 7 are different
// accounts.
func TestReferralFingerprintNamespacesByKind(t *testing.T) {
	student := ReferralFingerprint(7, 42, SubjectUser)
	again := ReferralFingerprint(7, 42, SubjectUser)
	if student != again {
		t.Error("the fingerprint is not stable for the same pair")
	}

	institution := ReferralFingerprint(7, 42, SubjectInstitution)
	if student == institution {
		t.Error("a student pair and an institution pair with the same ids fingerprint " +
			"identically, so matching on it would conflate two different accounts")
	}

	other := ReferralFingerprint(7, 43, SubjectUser)
	if student == other {
		t.Error("two different pairs fingerprint identically")
	}

	// And it must not be reversible to the ids, since it exists to be stored in a
	// table whose purpose is to be joined against.
	if strings.Contains(student, "42") {
		t.Error("the fingerprint contains the raw id in plain text")
	}
	if len(student) != 64 {
		t.Errorf("fingerprint is %d chars, want 64 (hex SHA-256)", len(student))
	}
}

// The referred kinds and statuses are the closed sets the CHECK constraints
// enforce. Asserted here because a kind or status added to a Go list but not to a
// CHECK is a write the database refuses — and the failure would surface as an
// INSERT error in the qualification pass rather than here.
func TestReferralVocabulariesAreClosed(t *testing.T) {
	if len(ReferredKinds) != 3 {
		t.Errorf("ReferredKinds has %d entries, want 3 (user, institution, provider). "+
			"See the file header: a fourth means a fourth account table, and "+
			"cmd/server/referral_wiring.go's mapping must be updated too", len(ReferredKinds))
	}
	for _, want := range []string{SubjectUser, SubjectInstitution, SubjectProvider} {
		if !validReferredKind(want) {
			t.Errorf("%q is not accepted by validReferredKind but is in ReferredKinds", want)
		}
	}
	for _, bad := range []string{"", "USER", "student", "admin", "superadmin", "institution_user", "users"} {
		if validReferredKind(bad) {
			t.Errorf("%q is accepted as a referred kind. An admin or student role here "+
				"would let an account be attributed as a class it is not, and the id would "+
				"point into the wrong table", bad)
		}
	}

	// FIVE states now: pending and qualified are written by the code, expired by the
	// qualification pass, and rejected/clawedback by an operator.
	//
	// The count grew from four to five with 'expired', and it is pinned because the
	// CHECK constraint in referral_indexes.go has to be widened in the same commit. A
	// state added here without widening the CHECK fails on the first write in
	// production rather than here, which is the worst place to find out — and this is
	// the only assertion that connects the Go vocabulary to that constraint's name.
	if len(ReferralStatuses) != 5 {
		t.Errorf("ReferralStatuses has %d entries, want 5 (pending, qualified, expired, "+
			"clawedback, rejected). If a state was added, chk_user_referral_status has to "+
			"list it too, or every write of the new state is refused by the database",
			len(ReferralStatuses))
	}
	// pending and rejected are distinct on purpose, and clawedback is spelled as one
	// word because that is what the CHECK says.
	if ReferralPending == ReferralRejected {
		t.Error("a rejection and a pending attribution are the same constant; a rejection " +
			"never paid anything and a clawback takes back something that was")
	}
	if ReferralClawedBack != "clawedback" {
		t.Errorf("ReferralClawedBack = %q, want \"clawedback\" to match chk_user_referral_status",
			ReferralClawedBack)
	}
	// 'expired' and 'clawedback' are BOTH terminal and must never be conflated, because
	// the only question anyone asks of them is whether this student ever got coins: a
	// clawback answers yes-then-no, an expiry answers never. Folding them together
	// loses that answer, which is the first thing a support or fraud review asks.
	if ReferralExpired == ReferralClawedBack {
		t.Error("expired and clawedback are the same constant. An expiry never paid and a " +
			"clawback takes back something that did; they are different support answers")
	}
	// And the terminal list is the three that never pay, so nothing that CAN pay is in
	// it — a qualified referral is terminal for further ATTEMPTS, not for payment.
	wantTerminal := []string{ReferralExpired, ReferralClawedBack, ReferralRejected}
	if len(ReferralTerminalStatuses) != len(wantTerminal) {
		t.Fatalf("ReferralTerminalStatuses has %d entries, want %d", len(ReferralTerminalStatuses), len(wantTerminal))
	}
	for _, want := range wantTerminal {
		if !IsTerminalReferralStatus(want) {
			t.Errorf("%q is not terminal, so settlement would not refuse it", want)
		}
	}
	for _, notTerminal := range []string{ReferralPending, ReferralQualified} {
		if IsTerminalReferralStatus(notTerminal) {
			t.Errorf("%q is reported as terminal; settlement refuses terminal statuses, so "+
				"this would make a referral that can still pay be unpayable", notTerminal)
		}
	}
	if IsTerminalReferralStatus("") || IsTerminalReferralStatus("PENDING") {
		t.Error("IsTerminalReferralStatus matched an empty or mis-cased status; it compares " +
			"exactly, and a fuzzy match would refuse a typo'd state for a new reason")
	}
}

// The schema ceiling and the config default must agree at launch, or the launch
// cap is a lie. Raising one without the other is a migration, and lowering the
// config is the supported way to tighten a cap mid-incident.
func TestReferralCapCeilingMatchesTheConfiguredDefault(t *testing.T) {
	cfg := DefaultEconomyConfig()
	if cfg.Referral.MonthlyCap != ReferralCapSlotCeiling {
		t.Errorf("config referral.monthly_cap is %d but the schema ceiling "+
			"chk_referral_cap_slot_ceiling is %d. Lowering the config to tighten a cap "+
			"is supported; raising it above the ceiling needs a migration and the two "+
			"disagreeing at launch means the documented cap is not the enforced one",
			cfg.Referral.MonthlyCap, ReferralCapSlotCeiling)
	}
}

// The cap figures from 05-economy-and-fraud.md §2.4: the count cap is primary and
// the coin ceiling is DERIVED from it. A config edit that made the coin ceiling
// bind first would recreate the contradiction that section resolves, and it would
// make the count cap decorative — the whole reason the resolution is written down.
func TestReferralCoinCeilingIsDerivedFromTheCountCap(t *testing.T) {
	cfg := DefaultEconomyConfig()
	want := ReferralCapSlotCeiling * cfg.Awards.ReferralReferrer
	if cfg.Referral.LifetimeCoinCap != want {
		t.Errorf("referral.lifetime_coin_cap is %d, want %d = monthly_cap x "+
			"referral_referrer. §2.4 resolves the D4 contradiction by making the COUNT "+
			"cap primary and deriving the coin ceiling; a ceiling that binds first makes "+
			"the count cap unreachable and decorative",
			cfg.Referral.LifetimeCoinCap, want)
	}
	// And the arithmetic that made the resolution necessary, restated so the
	// numbers stay honest.
	if ceiling := int64(200); cfg.Awards.ReferralReferrer*ceiling > cfg.Referral.LifetimeCoinCap {
		t.Logf("note: at %d coins per referral a %d-coin ceiling would bind at %.1f referrals, "+
			"so the count cap of %d would be unreachable. The config uses %d instead.",
			cfg.Awards.ReferralReferrer, ceiling,
			float64(cfg.Referral.LifetimeCoinCap)/float64(cfg.Awards.ReferralReferrer),
			ReferralCapSlotCeiling, cfg.Referral.LifetimeCoinCap)
	}
}

// The award value comes from config and is never a caller-supplied number, and the
// referral reward is the one that dominates issuance. That is the argument for the
// cap, and it is why the cap is structural rather than advisory.
func TestReferralRewardIsConfiguredAndLarge(t *testing.T) {
	cfg := DefaultEconomyConfig()
	if cfg.Awards.ReferralReferrer != 60 {
		t.Errorf("awards.referral_referrer is %d, want 60 (§2.1's launch value: equal to "+
			"one mock-test unlock, so the value is immediately legible to a student)",
			cfg.Awards.ReferralReferrer)
	}
	if cfg.Awards.ReferralReferred != 25 {
		t.Errorf("awards.referral_referred is %d, want 25", cfg.Awards.ReferralReferred)
	}
	// §2.1 prices a mock-test unlock at 60 and a referral at 60, and §2.3 calls a
	// referral "1.5x a mock-test unlock". Those two statements disagree, and the
	// disagreement is worth recording rather than papering over: the ratio that
	// matters for the cap is referral-to-FAUCET, not referral-to-sink, and a
	// referral at 60 alongside a 25-coin profile award and a 25-coin invitee award
	// issues 110 coins per successful referral against a 40-coin cheapest unlock —
	// 2.75x. So referrals dominate issuance regardless of which comparison §2.3
	// meant, which is why the cap is structural.
	//
	// Asserted as >= rather than >, because 60 vs 60 is equal and the doc's "1.5x"
	// does not survive its own price table. If somebody re-prices, this fires.
	if cfg.Awards.ReferralReferrer < cfg.Prices.MockTest {
		t.Errorf("a referral (%d) is now worth less than a mock test (%d). §2.3's finding "+
			"is that referrals dominate issuance, which is the argument for the cap — a "+
			"re-pricing that reverses it should be a deliberate decision with the doc "+
			"updated, not a config edit nobody checked",
			cfg.Awards.ReferralReferrer, cfg.Prices.MockTest)
	}
	// The figure that actually drives the faucet.
	perReferral := cfg.Awards.ReferralReferrer + cfg.Awards.ReferralReferred
	cheapestUnlock := cfg.Prices.StudyResource
	if perReferral <= cheapestUnlock {
		t.Errorf("one successful referral issues %d coins against a cheapest unlock of %d, "+
			"so a referral would not dominate issuance at all; §2.3's whole case for a "+
			"structural cap rests on it doing so",
			perReferral, cheapestUnlock)
	}
	// The reason code and ref type the whole mechanic hangs on must exist and be
	// distinct, because both are journal columns and a collision would write a
	// referral journal that a reconciliation reads as something else.
	if ReasonReferralQualified == ReasonReferralHold {
		t.Error("the grant reason and the hold reason are the same constant")
	}
	if !IsHoldReason(ReasonReferralHold) {
		t.Error("ReasonReferralHold is not a hold reason, so a hold journal would be " +
			"expected to carry postings it cannot have")
	}
	if IsHoldReason(ReasonReferralQualified) {
		t.Error("ReasonReferralQualified is treated as a hold reason; it posts two legs " +
			"and 'every posting-free journal is a hold' is the invariant reconcile.go checks")
	}
	if RefUserReferral != "user_referral" {
		t.Errorf("RefUserReferral = %q, want \"user_referral\"", RefUserReferral)
	}
}

// The credit-to-the-direct-referrer design is the whole single-level argument, and it
// rests on the referrer NOT BEING A PARAMETER. Asserted by reflection rather than by
// reading the source, because a source scan cannot survive a rename and this can:
// whoever adds the field gets a test failure naming it.
//
//	Attribution{ReferredKind, ReferredUserID, ReferralCode, SourcePath}
//
// Four fields, and none of them a referrer. If somebody adds ReferrerUserID, a caller
// can name whoever it likes and the mechanic becomes multi-level by a single field —
// which is the well-meaning request ("let B's invitees also thank A") arriving as a
// parameter instead of a code path, and this test is what makes that a failed build.
func TestAttributionCarriesNoReferrerParameter(t *testing.T) {
	typ := reflect.TypeOf(Attribution{})

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if strings.Contains(strings.ToLower(field.Name), "referrer") {
			t.Errorf("Attribution has a field named %q. The referrer must be resolved from "+
				"the code inside ApplyReferral: a caller-supplied referrer is a parameter "+
				"through which a multi-level referral tree could be built, which Consumer "+
				"Protection Act 2075 s.16(2)(p) prohibits", field.Name)
		}
	}

	// Pin the exact shape, so a REMOVED field is noticed too. Attribution is what four
	// call sites in internal/auth fill in, and losing a field means a path is quietly
	// no longer capturing something.
	want := map[string]bool{
		"ReferredKind":   true,
		"ReferredUserID": true,
		"ReferralCode":   true,
		"SourcePath":     true,
	}
	if typ.NumField() != len(want) {
		t.Errorf("Attribution has %d fields, want %d. A new field has to be argued for in "+
			"terms of what a CALLER can now say about a referral, not simply added",
			typ.NumField(), len(want))
	}
	for i := 0; i < typ.NumField(); i++ {
		if !want[typ.Field(i).Name] {
			t.Errorf("Attribution has an unexpected field %q", typ.Field(i).Name)
		}
	}

	// And the service's method takes no referrer either.
	m, ok := reflect.TypeOf(&ReferralService{}).MethodByName("ApplyReferral")
	if !ok {
		t.Fatal("ReferralService has no ApplyReferral method")
	}
	if got := m.Type.NumIn(); got != 3 { // receiver, ctx, Attribution
		t.Errorf("ApplyReferral takes %d arguments, want 3. Every extra parameter is a "+
			"way for a caller to say something the code should not be able to say", got)
	}
}

// The service's exported surface is three entry points plus two wiring seams, on
// purpose.
//
// ApplyReferral records a relationship. SettleReferral pays one, as a single
// transaction. QualifyPendingReferrals is the pass that decides which referrals
// SettleReferral is called for. claimCapSlot and readReferralForUpdate are steps of
// the state machine and are unexported so that a caller cannot run one out of order —
// a cap slot claimed without a grant, or a settlement's status read without its lock.
//
// THIS LIST GREW FROM TWO TO THREE, and the reason is worth recording rather than
// absorbing into an allow-list: the qualification pass cannot be a method on
// something else, because the decision "is this referral payable" has to be the SAME
// code that SettleReferral uses to refuse an early payment. A second implementation
// of that question is how a sweep and a settlement come to disagree about one row.
// Splitting them for tidiness would have bought nothing and cost the invariant.
//
// WithQualifiers and WithNotifier are also in the list, and they are different in
// kind: neither moves a coin or reads a row. They are wiring seams, shaped as builder
// methods because the qualification ports must be OPTIONAL at construction — attribution
// needs neither, and a deployment missing the phone-verification flow must still record
// every referral from day one.
//
// This is an assertion on names, which is coarse. The point is not that nothing can
// be added; it is that a fourth exported METHOD that touches the state machine has to
// be argued for in review rather than added in passing.
func TestReferralServiceExposesOnlyItsEntryPoints(t *testing.T) {
	typ := reflect.TypeOf(&ReferralService{})
	allowed := map[string]bool{
		"ApplyReferral":           true,
		"SettleReferral":          true,
		"QualifyPendingReferrals": true,
		"WithQualifiers":          true,
		"WithNotifier":            true,
	}
	for i := 0; i < typ.NumMethod(); i++ {
		if name := typ.Method(i).Name; !allowed[name] {
			t.Errorf("ReferralService exports %q. The state machine has three entry points — "+
				"record a referral, settle one, run the qualification pass — and a fourth "+
				"exported method that touches the state machine can be called out of order. "+
				"WithQualifiers and WithNotifier are in the allow-list because they only "+
				"attach wiring and move no coins", name)
		}
	}
}
