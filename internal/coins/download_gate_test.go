// internal/coins/download_gate_test.go
//
// The download gate with no database.
//
// What is worth proving here is the ORDER and the SHIP-DARK property, because
// neither is visible from a happy path:
//
//   - with the gate off, a download is not merely free — it touches NOTHING. No
//     entitlement read, no allowance read, no balance read, no journal. That is
//     what makes this slice safe to deploy, and it is the one assertion in this
//     file that must never be relaxed to "it did not charge".
//   - with the gate on, the three refusals are distinguishable: 402 for a wallet
//     that cannot cover the price, 423 for a LAPSED allowance, 404 for a resource
//     this build cannot resolve. The frontend renders three different screens,
//     so collapsing any two of them is a defect, not a simplification.
//   - the gate reuses unlock(), so the ordering guarantees tested for the wallet
//     hold here. What is new is that an ALREADY-OWNED download is free and
//     silent, and that the 402's figures are the ones the refusal carried out
//     rather than a balance read afterwards.
//
// The transaction-level claims — that a refusal writes nothing, that concurrent
// downloads charge once — need a real database and are in
// download_gate_pg_test.go.
package coins

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"studsphere/backend/internal/studyresources"
)

// gateAPI is the gate over the same fakes the wallet tests use, with the gate
// switched on for documents. It exists so a gate test reads as a gate test and
// not as a wallet test with a different URL.
func gateAPI(t *testing.T, entitlements *fakeEntitlements, wallet *fakeWallet, resources *fakeResources, mutate func(*EconomyConfig)) *UnlockAPI {
	t.Helper()
	return testAPI(t, entitlements, wallet, func(cfg *EconomyConfig) {
		cfg.Gates.StudyResource = true
		if mutate != nil {
			mutate(cfg)
		}
	}).WithResourceLookup(resources)
}

const gateResourceTitle = "Physics Past Questions 2081"

// ── ship dark ────────────────────────────────────────────────────────────────

// The most important test in this slice. With the gate off — which is the
// shipped default, every class, everywhere — a download is answered without
// reading a single row of the economy. Not "reads and decides not to charge":
// READS NOTHING. A gate that consulted the entitlement or the balance while off
// would be a per-download cost paid for a feature nobody has turned on, and a
// database error in the coin service would take document downloads down for
// every student in the country.
func TestDownloadGateOffTouchesNothingInTheEconomy(t *testing.T) {
	entitlements := &fakeEntitlements{}
	wallet := &fakeWallet{available: 1000}
	resources := &fakeResources{title: gateResourceTitle}
	// testAPI's config has gates.StudyResource = true; turn it back off, which is
	// the state DefaultEconomyConfig() ships in.
	api := testAPI(t, entitlements, wallet, func(cfg *EconomyConfig) {
		cfg.Gates = GatesEnabledConfig{}
	}).WithResourceLookup(resources)

	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassStudyResource, 812, gateResourceTitle)

	if !decision.Allowed {
		t.Fatalf("an ungated download was refused: %+v", decision.Refusal)
	}
	if decision.Charged != 0 {
		t.Errorf("charged = %d, want 0 for an ungated download", decision.Charged)
	}
	if len(entitlements.log.calls) != 0 {
		t.Errorf("an ungated download called the economy: %v", entitlements.log.calls)
	}
	if resources.calls != 0 {
		t.Errorf("an ungated download resolved the resource %d times; the class was not even needed", resources.calls)
	}
}

// A gate switched off for one class must not be switched on by the others being
// on. The rollback is per class, and a merge that turned documents on because
// video was on would make the kill switch useless in exactly the incident it
// exists for.
func TestDownloadGateOffPerClassDoesNotFollowTheOtherClasses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		gates GatesEnabledConfig
	}{
		{"only video on", GatesEnabledConfig{Video: true}},
		{"only mock test on", GatesEnabledConfig{MockTest: true}},
		{"nothing on", GatesEnabledConfig{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entitlements := &fakeEntitlements{}
			wallet := &fakeWallet{available: 1000}
			api := testAPI(t, entitlements, wallet, func(cfg *EconomyConfig) {
				cfg.Gates = tc.gates
			})
			decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
				studyresources.GateClassStudyResource, 812, gateResourceTitle)
			if !decision.Allowed {
				t.Fatalf("a document download was gated while only another class was on: %+v", decision.Refusal)
			}
			if len(entitlements.log.calls) != 0 {
				t.Errorf("the economy was consulted: %v", entitlements.log.calls)
			}
		})
	}
}

// A video lecture goes through the SAME handler but is a different class, and
// this slice must not have gated it. A video download with the video gate off
// touches nothing, which is the "do not gate video" requirement stated as a test
// rather than as a promise.
func TestVideoDownloadIsNotGatedByTheDocumentGate(t *testing.T) {
	entitlements := &fakeEntitlements{}
	wallet := &fakeWallet{available: 1000}
	api := testAPI(t, entitlements, wallet, func(cfg *EconomyConfig) {
		cfg.Gates.StudyResource = true // documents on, video off
	})
	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassVideo, 900, "Thermodynamics Lecture 3")
	if !decision.Allowed {
		t.Fatalf("a video download was gated by the document switch: %+v", decision.Refusal)
	}
	if len(entitlements.log.calls) != 0 {
		t.Errorf("a video download consulted the economy: %v", entitlements.log.calls)
	}
}

// An unreadable config is NOT "ungated". It is a 500. A student served a file
// because the kill switch could not be read is exactly the failure this gate
// exists to prevent, and failing closed on the CONFIG is the opposite of failing
// closed on the BALANCE: no coins move either way, so nothing is lost by
// refusing to answer.
func TestDownloadGateFailsClosedOnAnUnreadableConfig(t *testing.T) {
	entitlements := &fakeEntitlements{}
	wallet := &fakeWallet{available: 1000}
	// testAPI cannot build this one: it writes a VALID config, and a valid config
	// is the thing being tested against. The call log is attached by hand for the
	// same reason the API is — it is the evidence that the economy was not
	// consulted, not a convenience.
	entitlements.log = &callLog{}
	settings := newFakeSettings()
	settings.values[EconomyConfigSettingKey] = "{not json"
	api := &UnlockAPI{
		entitlements: entitlements,
		wallet:       wallet,
		repo:         &Repository{},
		config:       NewConfigStore(settings),
		now:          func() time.Time { return time.Date(2026, 9, 26, 5, 0, 0, 0, time.UTC) },
	}

	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassStudyResource, 812, gateResourceTitle)
	if decision.Allowed {
		t.Fatal("a download was served while the config could not be read")
	}
	if decision.Refusal == nil || decision.Refusal.Status != http.StatusInternalServerError {
		t.Fatalf("refusal = %+v, want a 500", decision.Refusal)
	}
	if len(entitlements.log.calls) != 0 {
		t.Errorf("the economy was consulted anyway: %v", entitlements.log.calls)
	}
}

// A gate wired without a repository answers 500 rather than panicking. The route
// is behind authMW in production, so this is about a wiring mistake surfacing as
// an error on the first download rather than as a stack trace.
func TestDownloadGateWithoutADatabaseAnswers500(t *testing.T) {
	decision := NewDownloadGate(&UnlockAPI{}).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassStudyResource, 812, gateResourceTitle)
	if decision.Allowed {
		t.Fatal("a gate with no database allowed a gated download")
	}
	if decision.Refusal == nil || decision.Refusal.Status != http.StatusInternalServerError {
		t.Fatalf("refusal = %+v, want a 500", decision.Refusal)
	}
	if decision.Refusal.Code != studyresources.CodeGateUnavailable {
		t.Errorf("code = %q, want %q", decision.Refusal.Code, studyresources.CodeGateUnavailable)
	}
}

// ── entitlement first ────────────────────────────────────────────────────────

// A student who already owns the file is served with no charge. The assertion
// that matters is not just "allowed" but that the ALLOWANCE was never consulted:
// an already-owned download that burns a starter unlock is a student who paid for
// one document and lost one of their three free ones as well.
func TestDownloadOfAnOwnedResourceIsFreeAndDoesNotTouchTheAllowance(t *testing.T) {
	entitlements := &fakeEntitlements{hasAccess: true}
	wallet := &fakeWallet{available: 100}
	api := gateAPI(t, entitlements, wallet, &fakeResources{title: gateResourceTitle}, nil)

	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassStudyResource, 812, gateResourceTitle)

	if !decision.Allowed {
		t.Fatalf("an owned resource was refused: %+v", decision.Refusal)
	}
	if decision.Charged != 0 {
		t.Errorf("charged = %d, want 0 for a resource the student already owns", decision.Charged)
	}
	// has_access, then wallet_balance (to report what they have left) and
	// nothing else. consume_allowance or purchase appearing here is the bug.
	joined := strings.Join(entitlements.log.calls, ",")
	if strings.Contains(joined, "consume_allowance") {
		t.Errorf("an already-owned download consumed the allowance: %v", entitlements.log.calls)
	}
	if strings.Contains(joined, "purchase") {
		t.Errorf("an already-owned download reached the purchase: %v", entitlements.log.calls)
	}
	if entitlements.log.calls[0] != "has_access" {
		t.Errorf("the entitlement was not checked first: %v", entitlements.log.calls)
	}
}

// ── allowance before balance ─────────────────────────────────────────────────

// A student with no coins and an unused starter unlock downloads the file for
// nothing. The wallet is at zero deliberately: reading the balance first and
// refusing here would block exactly the students the allowance exists for.
func TestStarterAllowanceCoversAStudentWithNoCoins(t *testing.T) {
	entitlements := &fakeEntitlements{allowanceUnlock: &ResourceUnlock{ID: 55, Source: UnlockSourceAllowance}}
	wallet := &fakeWallet{available: 0}
	api := gateAPI(t, entitlements, wallet, &fakeResources{title: gateResourceTitle}, nil)

	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassStudyResource, 812, gateResourceTitle)

	if !decision.Allowed {
		t.Fatalf("a student with an unused allowance was refused: %+v", decision.Refusal)
	}
	if decision.Charged != 0 {
		t.Errorf("charged = %d, want 0 for an allowance unlock", decision.Charged)
	}
	joined := strings.Join(entitlements.log.calls, ",")
	if !strings.Contains(joined, "consume_allowance") {
		t.Errorf("the allowance was not consumed: %v", entitlements.log.calls)
	}
	if strings.Contains(joined, "purchase") {
		t.Errorf("an allowance unlock reached the purchase: %v", entitlements.log.calls)
	}
	// The order itself, as a sequence rather than as two index comparisons:
	// the entitlement, then the allowance, and only then a balance read — which
	// happens to report what is left, never to decide.
	if want := []string{"has_access", "consume_allowance", "wallet_balance"}; !reflect.DeepEqual(entitlements.log.calls, want) {
		t.Errorf("call order = %v, want %v", entitlements.log.calls, want)
	}
}

// ── the refusals ─────────────────────────────────────────────────────────────

// A wallet that cannot cover the price gets the 402 of §2.3 with the designed
// object, and the figures are the ones the refusal carried out of the ledger
// rather than a balance read taken afterwards.
func TestDownloadRefusesWithTheDesigned402(t *testing.T) {
	// The allowance is explicitly NOT lapsed here: it is used up. The two answer
	// different sentences and the status must follow, so this is a bare
	// ErrNoAllowanceRemaining rather than the joined error a lapsed window
	// produces.
	entitlements := &fakeEntitlements{
		allowanceErr: &fakeAllowanceError{cause: ErrNoAllowanceRemaining},
	}
	wallet := &fakeWallet{available: 25}
	api := gateAPI(t, entitlements, wallet, &fakeResources{title: gateResourceTitle}, nil)

	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassStudyResource, 812, gateResourceTitle)

	if decision.Allowed {
		t.Fatal("a download the student cannot afford was allowed")
	}
	refusal := decision.Refusal
	if refusal == nil {
		t.Fatal("a refusal with no body")
	}
	if refusal.Status != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (%+v)", refusal.Status, refusal)
	}
	if refusal.Code != CodeInsufficientCoins {
		t.Errorf("code = %q, want %q", refusal.Code, CodeInsufficientCoins)
	}

	// The designed object, rendered as it will be written.
	encoded, err := json.Marshal(refusal.RefusalBody())
	if err != nil {
		t.Fatalf("marshal the refusal body: %v", err)
	}
	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Data    *struct {
				Required      int64  `json:"required"`
				Available     int64  `json:"available"`
				Shortfall     int64  `json:"shortfall"`
				ExpiresInDays *int64 `json:"expires_in_days"`
				WaysToEarn    []struct {
					Code      string `json:"code"`
					Label     string `json:"label"`
					Potential int64  `json:"potential"`
				} `json:"ways_to_earn"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatalf("decode the refusal body: %v (%s)", err, encoded)
	}
	if body.Success {
		t.Error("the refusal body reports success: true")
	}
	if body.Error.Code != CodeInsufficientCoins {
		t.Errorf("body code = %q, want %q", body.Error.Code, CodeInsufficientCoins)
	}
	data := body.Error.Data
	if data == nil {
		t.Fatalf("the 402 carries no designed object: %s", encoded)
	}
	// 40 is the configured document price and 25 is the wallet, so the figures
	// must be those and not zeroed or recomputed.
	if data.Required != 40 || data.Available != 25 || data.Shortfall != 15 {
		t.Errorf("required/available/shortfall = %d/%d/%d, want 40/25/15",
			data.Required, data.Available, data.Shortfall)
	}
	// ways_to_earn is the earning routes the student can still take. With no
	// profile adapter wired it is empty, and an empty list is the honest answer
	// rather than a copy of the config.
	if data.WaysToEarn == nil {
		t.Errorf("ways_to_earn is null, want []: %s", encoded)
	}
	// A gate that could not decide must not wear this code; the two are
	// different screens and different support paths.
	if body.Error.Code == studyresources.CodeGateUnavailable {
		t.Error("a refusal was reported as a gate outage")
	}
}

// A lapsed allowance that no purchase covers is the 423, and the two are kept
// apart deliberately: §2.3's row is "allowance lapsed and the student is not
// covered by a purchase", and the frontend shows a different screen for each.
func TestLapsedStarterAllowanceIsThe423NotThe402(t *testing.T) {
	entitlements := &fakeEntitlements{
		allowanceErr: errors.Join(ErrNoAllowanceRemaining, ErrAllowanceExpired),
	}
	wallet := &fakeWallet{available: 25}
	api := gateAPI(t, entitlements, wallet, &fakeResources{title: gateResourceTitle}, nil)

	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassStudyResource, 812, gateResourceTitle)

	if decision.Allowed {
		t.Fatal("an unaffordable download was allowed")
	}
	if decision.Refusal.Status != http.StatusLocked {
		t.Errorf("status = %d, want 423 for a lapsed allowance (%+v)", decision.Refusal.Status, decision.Refusal)
	}
	if decision.Refusal.Code != CodeAllowanceExpired {
		t.Errorf("code = %q, want %q", decision.Refusal.Code, CodeAllowanceExpired)
	}
	// The gap travels with the 423 too, because the student needs the same
	// earning routes whichever sentence they are shown.
	if decision.Refusal.Data == nil {
		t.Error("the 423 carries no designed object")
	}
}

// A resource this build cannot resolve is the 404, and it is the same 404 the
// unlock endpoint answers — one code, so a client has one case to handle.
func TestDownloadOfAnUnresolvableResourceIsThe404(t *testing.T) {
	entitlements := &fakeEntitlements{}
	wallet := &fakeWallet{available: 1000}
	resources := &fakeResources{lookupErr: ErrNotFound}
	api := gateAPI(t, entitlements, wallet, resources, nil)

	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassStudyResource, 999999, "Gone")

	if decision.Allowed {
		t.Fatal("a download of an unresolvable resource was allowed")
	}
	if decision.Refusal.Status != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (%+v)", decision.Refusal.Status, decision.Refusal)
	}
	if decision.Refusal.Code != CodeResourceNotFound {
		t.Errorf("code = %q, want %q", decision.Refusal.Code, CodeResourceNotFound)
	}
	if len(entitlements.log.calls) != 0 {
		t.Errorf("a 404 still reached the economy: %v", entitlements.log.calls)
	}
}

// ── the key a download does not have ─────────────────────────────────────────

// A download has no Idempotency-Key header, so the gate mints one — and the
// minted key must be stable for the same (user, class, id, price) and different
// for every other combination. Instability would charge twice for one browser
// retry; missing the user would make one student's download collide with
// another's, because the column is unique across every user journal.
func TestGateIdempotencyKeyIsDerivedAndScoped(t *testing.T) {
	base := gateIdempotencyKey(4242, ResourceTypeStudyResource, 812, 40)
	if again := gateIdempotencyKey(4242, ResourceTypeStudyResource, 812, 40); again != base {
		t.Errorf("the key is not stable across two identical downloads: %q vs %q", base, again)
	}
	for name, other := range map[string]string{
		"another user":     gateIdempotencyKey(4243, ResourceTypeStudyResource, 812, 40),
		"another class":    gateIdempotencyKey(4242, ResourceTypeVideo, 812, 40),
		"another resource": gateIdempotencyKey(4242, ResourceTypeStudyResource, 813, 40),
		"another price":    gateIdempotencyKey(4242, ResourceTypeStudyResource, 812, 55),
	} {
		if other == base {
			t.Errorf("%s produced the same key: %q", name, other)
		}
	}
	// Namespaced, so a gate key can never replay a journal a client wrote
	// through POST /coins/unlock, whose keys are the client's own.
	if !strings.HasPrefix(base, gateIdempotencyKeyPrefix) {
		t.Errorf("key %q is not in the gate namespace", base)
	}
	// And it must satisfy the ledger's own rule, since the journal insert would
	// refuse anything longer than the column.
	if err := validateIdempotencyKey(base); err != nil {
		t.Errorf("the minted key is not a valid ledger key: %v", err)
	}
}

// ── the classes agree ────────────────────────────────────────────────────────

// The two packages each declare the class vocabulary and each map a stored
// resource_type onto it. If those two mappings drift, a video lecture is charged
// the document price and the video allowance — with nothing in the schema to
// catch it, because the entitlement records the class the CALLER named. This
// test is from the coins side; gateClassFor in studyresources has its own.
func TestStudyResourceClassMappingCoversEveryStoredType(t *testing.T) {
	for _, tc := range []struct {
		stored string
		want   string
	}{
		{studyresources.TypePastQuestions, ResourceTypeStudyResource},
		{studyresources.TypeStudyNotes, ResourceTypeStudyResource},
		{studyresources.TypeModelQuestions, ResourceTypeStudyResource},
		{studyresources.TypeSyllabus, ResourceTypeStudyResource},
		{studyresources.TypeVideoLectures, ResourceTypeVideo},
		// Legacy spellings, because the column stores whatever an older endpoint
		// wrote and the gate reads those rows too.
		{"Past Questions", ResourceTypeStudyResource},
		{"video", ResourceTypeVideo},
		{"Video Lectures", ResourceTypeVideo},
		// A type that does not normalize at all. Every type ever storable here is
		// either a video lecture or one of the four documents, so a document is
		// the safe answer: it is the price and allowance a non-video row was
		// always meant to have.
		{"", ResourceTypeStudyResource},
		{"something removed", ResourceTypeStudyResource},
	} {
		if got := studyResourceClass(tc.stored); got != tc.want {
			t.Errorf("studyResourceClass(%q) = %q, want %q", tc.stored, got, tc.want)
		}
	}
}

// Every class the download route can name must be a class this package prices
// and gates. A class that looked switched on but was not in ResourceTypes would
// be ungated by EnabledFor's default and would quietly never charge.
func TestGateClassesAreRealUnlockableClasses(t *testing.T) {
	known := false
	for _, class := range ResourceTypes {
		if class == studyresources.GateClassStudyResource || class == studyresources.GateClassVideo {
			known = true
		}
	}
	if !known {
		t.Fatalf("neither %q nor %q is an unlockable class; the gate vocabulary and the class vocabulary have diverged",
			studyresources.GateClassStudyResource, studyresources.GateClassVideo)
	}
}

// A class with no configured price cannot be bought — the ledger refuses a
// non-positive price — so serving it free is the only answer that is not a 500
// on a published file, and it is what the price of zero actually says.
func TestDownloadOfAFreeClassIsServedRatherThan500(t *testing.T) {
	entitlements := &fakeEntitlements{}
	wallet := &fakeWallet{available: 0}
	api := gateAPI(t, entitlements, wallet, &fakeResources{title: gateResourceTitle}, func(cfg *EconomyConfig) {
		cfg.Prices.StudyResource = 0
	})

	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 4242,
		studyresources.GateClassStudyResource, 812, gateResourceTitle)
	if !decision.Allowed {
		t.Fatalf("a free class was refused: %+v", decision.Refusal)
	}
	if len(entitlements.log.calls) != 0 {
		t.Errorf("a free class consulted the economy: %v", entitlements.log.calls)
	}
}

// ── the title ────────────────────────────────────────────────────────────────

// The wired lookup makes the title real, and the two places a student reads it
// are the debit receipt and the wallet history. This is the "study_resource 812"
// becoming the resource's actual name.
func TestUnlockedDescriptionCarriesTheResourceTitle(t *testing.T) {
	// The lookup is wired so the description resolver has something to ask; the
	// sentences themselves are asserted on the helpers below, which is where the
	// rendering lives.
	api := testAPI(t, &fakeEntitlements{}, &fakeWallet{available: 1000}, nil).
		WithResourceLookup(&fakeResources{title: gateResourceTitle})
	if got := api.titleFor(context.Background(), ResourceTypeStudyResource, 812); got != gateResourceTitle {
		t.Errorf("titleFor = %q, want %q", got, gateResourceTitle)
	}

	if got := describeUnlocked(ResourceTypeStudyResource, gateResourceTitle, 812); got != "Unlocked: "+gateResourceTitle {
		t.Errorf("description = %q, want the resource title", got)
	}
	// A lookup that resolved nothing falls back to the class and the id rather
	// than to an empty sentence.
	if got := describeUnlocked(ResourceTypeStudyResource, "", 812); got != "Unlocked: study_resource 812" {
		t.Errorf("fallback description = %q, want the class and id", got)
	}
	// The receipt names the resource too.
	if got := unlockItemLabel(ResourceTypeStudyResource, gateResourceTitle); got != gateResourceTitle {
		t.Errorf("receipt item = %q, want the resource title", got)
	}
	if got := unlockItemLabel(ResourceTypeStudyResource, ""); got != "study resource" {
		t.Errorf("receipt item without a title = %q, want the class words", got)
	}
	// And the request the notification actually sends carries it, so this is not
	// a helper nothing calls.
	event := UnlockEvent{
		JournalID:    "j1",
		ResourceType: ResourceTypeStudyResource,
		ResourceID:   812,
		Title:        gateResourceTitle,
		CoinsPaid:    40,
		BalanceAfter: 105,
	}
	req := unlockNotificationRequest(4242, event)
	if req.Data["item"] != gateResourceTitle {
		t.Errorf("the coins.debited payload names %v, want the resource title", req.Data["item"])
	}
	// The other two classes keep their copy-deck words when nothing resolves
	// them, so a later slice does not have to re-derive the vocabulary.
	if got := unlockItemLabel(ResourceTypeVideo, ""); got != "video lecture" {
		t.Errorf("video label = %q, want %q", got, "video lecture")
	}
	if got := unlockItemLabel(ResourceTypeMockTest, ""); got != "mock test" {
		t.Errorf("mock test label = %q, want %q", got, "mock test")
	}
}

// The history page resolves titles itself, and it must not 500 or blank a row
// when a lookup fails. A wallet that renders slightly worse is the right
// outcome; a history that fails over a label is not.
func TestHistoryNamesUnlocksAndSurvivesAFailedLookup(t *testing.T) {
	wallet := &fakeWallet{transactions: []TransactionDTO{
		{ReasonCode: ReasonResourceUnlock, Ref: &RefDTO{Type: ResourceTypeStudyResource, ID: 812}, Description: "Unlocked: study_resource 812"},
		{ReasonCode: ReasonResourceUnlock, Ref: &RefDTO{Type: ResourceTypeStudyResource, ID: 812}, Description: "Unlocked: study_resource 812"},
		{ReasonCode: ReasonProfileComplete, Description: "Profile progress reward"},
	}}
	resources := &fakeResources{title: gateResourceTitle}
	api := testAPI(t, &fakeEntitlements{}, wallet, nil).WithResourceLookup(resources)

	api.nameUnlockedItems(context.Background(), wallet.transactions)

	for i, item := range wallet.transactions {
		switch i {
		case 0, 1:
			if item.Description != "Unlocked: "+gateResourceTitle {
				t.Errorf("item %d description = %q, want the title", i, item.Description)
			}
		case 2:
			if item.Description != "Profile progress reward" {
				t.Errorf("a profile award was renamed: %q", item.Description)
			}
		}
	}
	// Two rows for the same resource, one read: a student who bought several
	// papers from one course must not cost a query per row.
	if resources.calls != 1 {
		t.Errorf("the lookup was called %d times for one distinct resource, want 1", resources.calls)
	}

	// Now a lookup that fails. The rows keep whatever they had.
	failing := &fakeResources{lookupErr: errors.New("studyresources is down")}
	api2 := testAPI(t, &fakeEntitlements{}, &fakeWallet{}, nil).WithResourceLookup(failing)
	items := []TransactionDTO{
		{ReasonCode: ReasonResourceUnlock, Ref: &RefDTO{Type: ResourceTypeStudyResource, ID: 812}, Description: "Unlocked: study_resource 812"},
	}
	api2.nameUnlockedItems(context.Background(), items)
	if items[0].Description != "Unlocked: study_resource 812" {
		t.Errorf("a failed lookup changed the description to %q", items[0].Description)
	}

	// And with no lookup wired at all, nothing is even attempted.
	plain := testAPI(t, &fakeEntitlements{}, &fakeWallet{}, nil)
	plainItems := []TransactionDTO{
		{ReasonCode: ReasonResourceUnlock, Ref: &RefDTO{Type: ResourceTypeStudyResource, ID: 812}, Description: "Unlocked: study_resource 812"},
	}
	plain.nameUnlockedItems(context.Background(), plainItems)
	if plainItems[0].Description != "Unlocked: study_resource 812" {
		t.Errorf("an unwired lookup changed the description to %q", plainItems[0].Description)
	}
}

// ── no session ───────────────────────────────────────────────────────────────

// The route is behind authMW and answers 401 there, before this module runs. The
// gate still refuses rather than spending against user 0, so a gate wired onto an
// unwrapped route cannot buy something for nobody.
func TestDownloadWithoutASessionIsRefused(t *testing.T) {
	entitlements := &fakeEntitlements{}
	wallet := &fakeWallet{available: 1000}
	api := gateAPI(t, entitlements, wallet, &fakeResources{title: gateResourceTitle}, nil)

	decision := NewDownloadGate(api).AuthorizeDownload(context.Background(), 0,
		studyresources.GateClassStudyResource, 812, gateResourceTitle)

	if decision.Allowed {
		t.Fatal("a download with no session was allowed")
	}
	if decision.Refusal.Status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (%+v)", decision.Refusal.Status, decision.Refusal)
	}
	if len(entitlements.log.calls) != 0 {
		t.Errorf("an anonymous download reached the economy: %v", entitlements.log.calls)
	}
}

// fakeAllowanceError is an ErrNoAllowanceRemaining WITHOUT ErrAllowanceExpired,
// which is how a USED-UP allowance is reported and how a LAPSED one is not. The
// distinction is the 402/423 split, so it needs a value that can express it.
type fakeAllowanceError struct{ cause error }

func (e *fakeAllowanceError) Error() string {
	return "no starter allowance remaining: " + e.cause.Error()
}

func (e *fakeAllowanceError) Unwrap() error { return e.cause }
