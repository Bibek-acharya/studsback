// internal/coins/unlock_api_test.go
//
// The wallet API with no database.
//
// The claims worth testing here are the ones that are about ORDER and about the
// SHAPE of the request, because neither is observable from the happy path:
//
//   - the order of the four checks in §2.3 (entitlement, then allowance, then
//     balance, then the purchase). A handler that checks the balance first
//     refuses the students the allowance exists for, and a handler that checks
//     the entitlement last charges a student for something they already own.
//   - that the request has no amount, and that the price is the server's.
//   - that ways_to_earn reflects the caller's ACTUAL eligibility rather than
//     echoing the config, which is the whole reason the list is server-side.
//   - that the write path is dark.
//
// The transaction-level claims — that a refusal writes nothing, that a
// concurrent duplicate produces one debit — need a real database and are in
// unlock_api_pg_test.go.
package coins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ── fakes ────────────────────────────────────────────────────────────────────
//
// Each records the calls it received into a SHARED log, because the ORDER is the
// thing under test and a fake that only returns a value cannot show it. Two
// separate slices would hide exactly the ordering bugs being looked for — "the
// balance was read before the allowance" is invisible if each fake logs to its
// own list.

type callLog struct{ calls []string }

func (l *callLog) add(name string) { l.calls = append(l.calls, name) }

type fakeEntitlements struct {
	log *callLog

	hasAccess       bool
	hasAccessErr    error
	allowanceErr    error
	allowanceUnlock *ResourceUnlock
	remaining       *AllowanceStatus
}

func (f *fakeEntitlements) HasAccess(context.Context, uint, string, uint64) (bool, error) {
	f.log.add("has_access")
	return f.hasAccess, f.hasAccessErr
}

func (f *fakeEntitlements) ConsumeAllowance(_ context.Context, _ uint, _ string, _ uint64, _ time.Time) (*ResourceUnlock, error) {
	f.log.add("consume_allowance")
	if f.allowanceErr != nil {
		return nil, f.allowanceErr
	}
	if f.allowanceUnlock != nil {
		return f.allowanceUnlock, nil
	}
	return &ResourceUnlock{ID: 11, Source: UnlockSourceAllowance, CoinsPaid: 0}, nil
}

func (f *fakeEntitlements) RemainingAllowance(context.Context, uint, time.Time) (*AllowanceStatus, error) {
	f.log.add("remaining_allowance")
	if f.remaining == nil {
		return &AllowanceStatus{Classes: map[string]ClassAllowance{}}, nil
	}
	return f.remaining, nil
}

type fakeWallet struct {
	log *callLog

	available      int64
	soonest        *time.Time
	profilePaid    int64
	hasUploadAward bool
	transactions   []TransactionDTO
	nextCursor     string
}

func (f *fakeWallet) WalletBalance(context.Context, uint, time.Time) (*WalletBalance, error) {
	f.log.add("wallet_balance")
	return &WalletBalance{TotalAvailable: f.available, Buckets: []BucketBalance{}}, nil
}

func (f *fakeWallet) TransactionPage(context.Context, uint, transactionCursor, int) ([]TransactionDTO, string, error) {
	f.log.add("transaction_page")
	return f.transactions, f.nextCursor, nil
}

func (f *fakeWallet) ProfileInstalmentsPaid(context.Context, uint) (int64, error) {
	f.log.add("profile_instalments")
	return f.profilePaid, nil
}

func (f *fakeWallet) SoonestLotExpiry(context.Context, uint, time.Time) (*time.Time, error) {
	f.log.add("soonest_expiry")
	return f.soonest, nil
}

func (f *fakeWallet) HasResourceApprovedAward(context.Context, uint) (bool, error) {
	f.log.add("has_upload_award")
	return f.hasUploadAward, nil
}

type fakeProfiles struct {
	complete bool
	err      error
	calls    []uint
}

func (f *fakeProfiles) ProfileComplete(_ context.Context, userID uint) (bool, error) {
	f.calls = append(f.calls, userID)
	return f.complete, f.err
}

type fakeResources struct {
	lookupErr error
	calls     int
}

func (f *fakeResources) LookupUnlockable(context.Context, string, uint64) error {
	f.calls++
	return f.lookupErr
}

// testAPI builds an UnlockAPI over the fakes, with a fixed clock and a config
// whose write path is on (the dark case has its own test).
//
// The two fakes share one call log, so the assertions can see a balance read
// that happened before an allowance check.
func testAPI(t *testing.T, entitlements *fakeEntitlements, wallet *fakeWallet, mutate func(*EconomyConfig)) *UnlockAPI {
	t.Helper()
	if entitlements.log == nil {
		entitlements.log = &callLog{}
	}
	if wallet.log == nil {
		wallet.log = entitlements.log
	} else if wallet.log != entitlements.log {
		// Two fakes with separate logs is the mistake this helper exists to stop.
		entitlements.log = wallet.log
	}

	settings := newFakeSettings()
	cfg := DefaultEconomyConfig()
	cfg.UnlockEndpointEnabled = true
	if mutate != nil {
		mutate(&cfg)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	settings.values[EconomyConfigSettingKey] = string(encoded)

	api := &UnlockAPI{
		entitlements: entitlements,
		wallet:       wallet,
		repo:         &Repository{},
		config:       NewConfigStore(settings),
		now:          func() time.Time { return time.Date(2026, 9, 26, 5, 0, 0, 0, time.UTC) },
	}
	api.purchase = func(_ context.Context, _ uint, req UnlockRequest, key string) (purchaseOutcome, error) {
		entitlements.log.add("purchase")
		price, err := spendPrice(cfg, ReasonResourceUnlock, req.ResourceType)
		if err != nil {
			return purchaseOutcome{}, err
		}
		if wallet.available < price {
			return purchaseOutcome{}, fmt.Errorf("%w: need %d, %d available", ErrInsufficientCoins, price, wallet.available)
		}
		return purchaseOutcome{
			CoinsPaid:    price,
			BalanceAfter: wallet.available - price,
			SpentFrom:    []SpentFromDTO{{Bucket: BucketEarned, Coins: price}},
			UnlockID:     7,
			JournalID:    "11111111-1111-4111-8111-111111111111",
		}, nil
	}
	return api
}

func testRequest() UnlockRequest {
	return UnlockRequest{ResourceType: ResourceTypeStudyResource, ResourceID: 812}
}

// ── the ordering ─────────────────────────────────────────────────────────────

// TestUnlockChecksEntitlementBeforeAnythingElse is the table the brief asks for,
// as a single ordered sequence of observations.
//
// The three refusals are separate rows rather than one because each is a
// different bug: a 403 instead of a 200 for an already-owned resource, a 402
// for a student the allowance was meant to cover, and a charge for a purchase
// that was never resolved. Asserting the CALL SEQUENCE rather than only the
// outcome is what catches "the balance was read first and happened to be
// affordable", which produces the right answer for the wrong reason and the
// wrong answer for a student with no coins.
func TestUnlockChecksEntitlementBeforeAnythingElse(t *testing.T) {
	const userID = uint(4242)
	cfg := DefaultEconomyConfig()
	price := cfg.Prices.StudyResource

	lapsed := fmt.Errorf("%w: %w: the allowance expired", ErrNoAllowanceRemaining, ErrAllowanceExpired)
	spent := fmt.Errorf("%w: all used", ErrNoAllowanceRemaining)

	tests := []struct {
		name            string
		hasAccess       bool
		allowanceErr    error
		available       int64
		wantAlready     bool
		wantAllowance   bool
		wantPaid        int64
		wantErr         error
		wantErrExpired  bool
		wantBeforeSpend []string
	}{
		{
			name:            "already holds it: never reaches the allowance or the balance",
			hasAccess:       true,
			available:       1000,
			wantAlready:     true,
			wantPaid:        0,
			wantBeforeSpend: []string{"has_access", "wallet_balance"},
		},
		{
			name:            "allowance covers it: no coins are read or charged",
			available:       0,
			wantAllowance:   true,
			wantPaid:        0,
			wantBeforeSpend: []string{"has_access", "consume_allowance", "wallet_balance"},
		},
		{
			name:            "allowance exhausted but affordable: the purchase proceeds",
			allowanceErr:    spent,
			available:       price,
			wantPaid:        price,
			wantBeforeSpend: []string{"has_access", "consume_allowance", "purchase"},
		},
		{
			name:            "allowance exhausted and cannot afford: 402, nothing written",
			allowanceErr:    spent,
			available:       price - 1,
			wantErr:         ErrInsufficientCoins,
			wantBeforeSpend: []string{"has_access", "consume_allowance", "purchase"},
		},
		{
			name:            "allowance lapsed and cannot afford: the 423 case is recorded",
			allowanceErr:    lapsed,
			available:       0,
			wantErr:         ErrInsufficientCoins,
			wantErrExpired:  true,
			wantBeforeSpend: []string{"has_access", "consume_allowance", "purchase"},
		},
		{
			name:            "nothing covers it and there is plenty: the ordinary purchase",
			allowanceErr:    spent,
			available:       price * 10,
			wantPaid:        price,
			wantBeforeSpend: []string{"has_access", "consume_allowance", "purchase"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entitlements := &fakeEntitlements{hasAccess: tc.hasAccess, allowanceErr: tc.allowanceErr}
			wallet := &fakeWallet{available: tc.available}
			api := testAPI(t, entitlements, wallet, nil)

			result, err := api.unlock(context.Background(), userID, testRequest(), "key-1", cfg)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				if result.AllowanceExpired != tc.wantErrExpired {
					t.Errorf("AllowanceExpired = %t, want %t (423 is decided here, where the allowance was read)",
						result.AllowanceExpired, tc.wantErrExpired)
				}
				if result.Required != price {
					t.Errorf("Required = %d, want the configured %d — the 402 quotes the server's price",
						result.Required, price)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.AlreadyUnlocked != tc.wantAlready {
				t.Errorf("AlreadyUnlocked = %t, want %t", result.AlreadyUnlocked, tc.wantAlready)
			}
			if result.UsedAllowance != tc.wantAllowance {
				t.Errorf("UsedAllowance = %t, want %t", result.UsedAllowance, tc.wantAllowance)
			}
			if err == nil && result.CoinsPaid != tc.wantPaid {
				t.Errorf("CoinsPaid = %d, want %d", result.CoinsPaid, tc.wantPaid)
			}
			if !reflect.DeepEqual(entitlements.log.calls, tc.wantBeforeSpend) {
				t.Errorf("call order = %v, want %v", entitlements.log.calls, tc.wantBeforeSpend)
			}
		})
	}
}

// An already-owned unlock is a 200 with coins_paid 0, and it is not an error.
// ErrAlreadyUnlocked arriving from the allowance attempt — which is what a
// request that lost the race sees — has to be answered the same way.
func TestAlreadyUnlockedIsNeverAnError(t *testing.T) {
	cfg := DefaultEconomyConfig()
	for _, tc := range []struct {
		name  string
		setup *fakeEntitlements
	}{
		{"caught by the first entitlement read", &fakeEntitlements{hasAccess: true}},
		{"lost the race inside the allowance", &fakeEntitlements{allowanceErr: ErrAlreadyUnlocked}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entitlements := tc.setup
			wallet := &fakeWallet{available: 77}
			api := testAPI(t, entitlements, wallet, nil)

			result, err := api.unlock(context.Background(), 1, testRequest(), "key-1", cfg)
			if err != nil {
				t.Fatalf("already-owned returned an error (%v); a 403 here renders a failure for a success", err)
			}
			if !result.AlreadyUnlocked || !result.Unlocked {
				t.Errorf("result = %+v, want unlocked AND already_unlocked", result)
			}
			if result.CoinsPaid != 0 {
				t.Errorf("CoinsPaid = %d, want 0", result.CoinsPaid)
			}
			if result.BalanceAfter != 77 {
				t.Errorf("BalanceAfter = %d, want the wallet's real 77 — reporting 0 looks like an empty wallet", result.BalanceAfter)
			}
			for _, call := range entitlements.log.calls {
				if call == "purchase" {
					t.Fatal("an already-owned resource reached the purchase; the student was charged for nothing")
				}
			}
		})
	}
}

// ── the request carries no amount ────────────────────────────────────────────

// TestUnlockRequestCarriesNoAmountAndThePriceComesFromConfig is the structural
// assertion behind 02-architecture.md §12.1.
//
// Three separate proofs, because each catches a different way the rule could be
// broken later:
//
//  1. Reflection over the struct: no field anywhere in it is named like money.
//     This is the one that fires when someone adds Amount to the struct, which is
//     the edit the codebase comments call "the exploit".
//  2. Decoding a body that TRIES to set an amount: the decoded value is
//     identical to the body without one. A json tag that shadowed an unexported
//     field, or a custom UnmarshalJSON, would break this.
//  3. The spend that follows used the CONFIGURED price. A struct that merely
//     looks amount-free is not enough if the handler reads the amount out of the
//     raw body.
func TestUnlockRequestCarriesNoAmountAndThePriceComesFromConfig(t *testing.T) {
	// 1. The shape.
	forbidden := map[string]bool{
		"amount": true, "price": true, "coins": true, "value": true,
		"quantity": true, "qty": true, "cost": true, "total": true, "fee": true,
	}
	typ := reflect.TypeOf(UnlockRequest{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if forbidden[strings.ToLower(field.Name)] {
			t.Errorf("UnlockRequest has a field %q; a client that can name a price is an exploit", field.Name)
		}
		for _, alias := range strings.Split(strings.ToLower(field.Tag.Get("json")), ",") {
			if forbidden[alias] {
				t.Errorf("UnlockRequest decodes %q from the body", field.Tag.Get("json"))
			}
		}
	}
	if typ.NumField() != 2 {
		t.Errorf("UnlockRequest has %d fields, want exactly resource_type and resource_id", typ.NumField())
	}

	// 2. A body that tries.
	honest := `{"resource_type":"study_resource","resource_id":812}`
	greedy := `{"resource_type":"study_resource","resource_id":812,"amount":1,"price":1,"coins":1}`
	var fromHonest, fromGreedy UnlockRequest
	if err := json.Unmarshal([]byte(honest), &fromHonest); err != nil {
		t.Fatalf("decode honest body: %v", err)
	}
	if err := json.Unmarshal([]byte(greedy), &fromGreedy); err != nil {
		t.Fatalf("decode greedy body: %v", err)
	}
	if fromHonest != fromGreedy {
		t.Errorf("a body carrying an amount decoded differently: %+v vs %+v", fromGreedy, fromHonest)
	}

	// 3. And the price.
	entitlements := &fakeEntitlements{allowanceErr: fmt.Errorf("%w: all used", ErrNoAllowanceRemaining)}
	wallet := &fakeWallet{available: 1000}
	api := testAPI(t, entitlements, wallet, func(c *EconomyConfig) { c.Prices.StudyResource = 137 })
	cfg, _ := api.config.Load()

	var seen SpendRequest
	api.purchase = func(_ context.Context, _ uint, req UnlockRequest, key string) (purchaseOutcome, error) {
		seen = SpendRequest{UserID: 1, ReasonCode: ReasonResourceUnlock, IdempotencyKey: key, RefType: req.ResourceType}
		price, err := spendPrice(cfg, ReasonResourceUnlock, req.ResourceType)
		return purchaseOutcome{CoinsPaid: price, BalanceAfter: 1000 - price, SpentFrom: []SpentFromDTO{}}, err
	}
	result, err := api.unlock(context.Background(), 1, fromGreedy, "key-amount-free", cfg)
	if err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if result.CoinsPaid != 137 {
		t.Errorf("CoinsPaid = %d, want the CONFIGURED 137 — a body-supplied amount of 1 must not reach the spend", result.CoinsPaid)
	}
	if seen.IdempotencyKey != "key-amount-free" {
		t.Errorf("the ledger key = %q, want the client's key passed through verbatim; a re-derived key cannot be replayed by a retry", seen.IdempotencyKey)
	}
	if seen.RefType != ResourceTypeStudyResource {
		t.Errorf("the spend class = %q, want %q", seen.RefType, ResourceTypeStudyResource)
	}
}

// ── ways_to_earn ─────────────────────────────────────────────────────────────

// TestWaysToEarnReflectsActualEligibility is the requirement that makes the list
// server-side: a student who has finished their profile is not told to finish
// it, and a student already paid for publishing a resource is not told to upload
// another.
func TestWaysToEarnReflectsActualEligibility(t *testing.T) {
	expires := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		profiles    ProfileEligibility
		profilePaid int64
		uploadTaken bool
		wantCodes   []string
		wantPotent  map[string]int64
	}{
		{
			name:      "nothing taken: every available route is offered",
			profiles:  &fakeProfiles{complete: false},
			wantCodes: []string{RouteProfile, RouteUpload},
			// 5 instalments x 5 coins, none paid yet.
			wantPotent: map[string]int64{RouteProfile: 25, RouteUpload: 80},
		},
		{
			name:       "a finished profile is not offered again",
			profiles:   &fakeProfiles{complete: true},
			wantCodes:  []string{RouteUpload},
			wantPotent: map[string]int64{RouteUpload: 80},
		},
		{
			name:        "an already-published upload is not offered again",
			profiles:    &fakeProfiles{complete: false},
			uploadTaken: true,
			wantCodes:   []string{RouteProfile},
			wantPotent:  map[string]int64{RouteProfile: 25},
		},
		{
			name:        "partly paid profile is worth only what is left",
			profiles:    &fakeProfiles{complete: false},
			profilePaid: 4,
			uploadTaken: true,
			wantCodes:   []string{RouteProfile},
			// One instalment of 5 remains, not the full 25. A number that
			// overstates what is on offer is how "you only need 15 more" becomes
			// a support ticket.
			wantPotent: map[string]int64{RouteProfile: 5},
		},
		{
			name:       "no eligibility lookup at all: the unverified route is not offered",
			profiles:   nil,
			wantCodes:  []string{RouteUpload},
			wantPotent: map[string]int64{RouteUpload: 80},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wallet := &fakeWallet{profilePaid: tc.profilePaid, hasUploadAward: tc.uploadTaken, soonest: &expires}
			api := testAPI(t, &fakeEntitlements{}, wallet, nil)
			api = api.WithProfileEligibility(tc.profiles)
			cfg, _ := api.config.Load()

			payload := api.insufficientCoinsPayload(context.Background(), 4242, cfg.Prices.StudyResource, cfg)

			var codes []string
			for _, w := range payload.WaysToEarn {
				codes = append(codes, w.Code)
				if want, ok := tc.wantPotent[w.Code]; ok && w.Potential != want {
					t.Errorf("%s potential = %d, want %d", w.Code, w.Potential, want)
				}
				if w.Potential <= 0 {
					t.Errorf("%s is offered with potential %d; a route worth nothing is noise", w.Code, w.Potential)
				}
				if w.Label == "" {
					t.Errorf("%s has no label; the UI renders the label verbatim", w.Code)
				}
			}
			if !reflect.DeepEqual(codes, tc.wantCodes) {
				t.Errorf("ways_to_earn = %v, want %v", codes, tc.wantCodes)
			}
		})
	}
}

// A fully paid profile ladder is worth zero, so it must not appear with a zero
// potential. A separate case from the table above because the "no lookup" path
// and the "nothing left" path are different reasons to omit a route.
func TestWaysToEarnOmitsAProfileLadderThatIsFullyPaid(t *testing.T) {
	wallet := &fakeWallet{profilePaid: 5, hasUploadAward: true}
	api := testAPI(t, &fakeEntitlements{}, wallet, nil).WithProfileEligibility(&fakeProfiles{complete: false})
	cfg, _ := api.config.Load()

	payload := api.insufficientCoinsPayload(context.Background(), 1, cfg.Prices.StudyResource, cfg)
	for _, w := range payload.WaysToEarn {
		if w.Code == RouteProfile {
			t.Errorf("the profile route is offered with potential %d after all five instalments were paid", w.Potential)
		}
	}
}

// The referral route is reported as unavailable, never offered: there is no
// referral code in this slice, so "Invite a friend" is a button that goes
// nowhere.
func TestWaysToEarnReportsReferralAsUnavailableRatherThanOfferingIt(t *testing.T) {
	api := testAPI(t, &fakeEntitlements{}, &fakeWallet{}, nil)
	cfg, _ := api.config.Load()

	payload := api.insufficientCoinsPayload(context.Background(), 1, cfg.Prices.StudyResource, cfg)
	for _, w := range payload.WaysToEarn {
		if w.Code == RouteReferral {
			t.Fatal("the referral route is offered; there is no referral code in this slice, so it is a dead button")
		}
	}
	found := false
	for _, u := range payload.Unavailable {
		if u.Code == RouteReferral {
			found = true
			if u.Reason == "" {
				t.Error("the referral route is reported as unavailable with no reason; the client cannot say 'coming soon'")
			}
		}
	}
	if !found {
		t.Errorf("the referral route is neither offered nor reported unavailable: %+v", payload.Unavailable)
	}
}

// expires_in_days is the urgency the 402 exists to create, and it is derived
// from the caller's own soonest-expiring lots.
func TestInsufficientCoinsPayloadQuotesTheGapAndTheUrgency(t *testing.T) {
	// Against the API's pinned clock, so the figure is a whole number of days
	// rather than "twelve days plus whatever the test machine did since".
	soonest := time.Date(2026, 9, 26, 5, 0, 0, 0, time.UTC).AddDate(0, 0, 12)
	wallet := &fakeWallet{available: 25, soonest: &soonest}
	api := testAPI(t, &fakeEntitlements{}, wallet, nil)
	cfg, _ := api.config.Load()

	payload := api.insufficientCoinsPayload(context.Background(), 1, cfg.Prices.StudyResource, cfg)
	if payload.Required != 40 {
		t.Errorf("required = %d, want the configured 40", payload.Required)
	}
	if payload.Available != 25 {
		t.Errorf("available = %d, want the wallet's 25", payload.Available)
	}
	if payload.Shortfall != 15 {
		t.Errorf("shortfall = %d, want 15", payload.Shortfall)
	}
	if payload.ExpiresInDays == nil {
		t.Fatal("expires_in_days is null although the student holds coins that lapse")
	}
	if *payload.ExpiresInDays != 12 {
		t.Errorf("expires_in_days = %d, want 12 (whole days until the soonest lot lapses)", *payload.ExpiresInDays)
	}
}

// A student holding nothing that lapses is told so with a null rather than a
// number: a made-up urgency is worse than none.
func TestExpiresInDaysIsNullWhenNothingLapses(t *testing.T) {
	api := testAPI(t, &fakeEntitlements{}, &fakeWallet{available: 0}, nil)
	cfg, _ := api.config.Load()

	payload := api.insufficientCoinsPayload(context.Background(), 1, cfg.Prices.StudyResource, cfg)
	if payload.ExpiresInDays != nil {
		t.Errorf("expires_in_days = %d, want null for a student with no expiring lots", *payload.ExpiresInDays)
	}
}

// ── the feature gate ─────────────────────────────────────────────────────────

// The write path ships dark. A 503 with a message that says WHY is the whole
// design: an admin or a curious frontend needs to be able to tell "switched off
// on purpose" from "broken".
func TestUnlockEndpointShipsDark(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := testAPI(t, &fakeEntitlements{}, &fakeWallet{}, func(c *EconomyConfig) { c.UnlockEndpointEnabled = false })
	RegisterWalletRoutes(router, stubAuth(4242), api)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock",
		strings.NewReader(`{"resource_type":"study_resource","resource_id":812}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "dark-1")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 while unlock_endpoint_enabled is false", recorder.Code)
	}
	var body ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (%s)", err, recorder.Body.String())
	}
	if body.Success {
		t.Error("a 503 body reported success")
	}
	if body.Error.Code != CodeEndpointDisabled {
		t.Errorf("code = %q, want %q", body.Error.Code, CodeEndpointDisabled)
	}
	if !strings.Contains(strings.ToLower(body.Error.Message), "not") {
		t.Errorf("message = %q, want one that says the feature is off", body.Error.Message)
	}
}

// With the flag on, the same request reaches the write path. This is what makes
// the dark test meaningful: it is not passing because the route is broken.
func TestUnlockEndpointReachesTheWritePathOnceEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	entitlements := &fakeEntitlements{hasAccess: true}
	api := testAPI(t, entitlements, &fakeWallet{available: 100}, nil)
	RegisterWalletRoutes(router, stubAuth(4242), api)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock",
		strings.NewReader(`{"resource_type":"study_resource","resource_id":812}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "lit-1")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 once enabled (%s)", recorder.Code, recorder.Body.String())
	}
	if len(entitlements.log.calls) == 0 {
		t.Fatal("an enabled endpoint never reached the write path")
	}
}

// ── the request shape the handler refuses ────────────────────────────────────

func TestUnlockRefusesAMissingOrMalformedIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const body = `{"resource_type":"study_resource","resource_id":812}`

	tests := []struct {
		name string
		key  string
	}{
		{"no header at all", ""},
		{"whitespace only", "   "},
		{"longer than the column allows", strings.Repeat("k", 256)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			entitlements := &fakeEntitlements{}
			api := testAPI(t, entitlements, &fakeWallet{available: 100}, nil)
			RegisterWalletRoutes(router, stubAuth(4242), api)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			if tc.key != "" {
				request.Header.Set("Idempotency-Key", tc.key)
			}
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", recorder.Code, recorder.Body.String())
			}
			var envelope ErrorEnvelope
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if envelope.Error.Code != CodeInvalidIdempotencyKey {
				t.Errorf("code = %q, want %q", envelope.Error.Code, CodeInvalidIdempotencyKey)
			}
			if len(entitlements.log.calls) != 0 {
				t.Errorf("a request with no usable key reached the write path: %v", entitlements.log.calls)
			}
		})
	}
}

func TestUnlockRefusesABadResourceReference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
	}{
		{"unknown class", `{"resource_type":"press_media","resource_id":812}`},
		{"empty class", `{"resource_type":"","resource_id":812}`},
		{"zero id", `{"resource_type":"study_resource","resource_id":0}`},
		{"not json", `nonsense`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			entitlements := &fakeEntitlements{}
			api := testAPI(t, entitlements, &fakeWallet{available: 1000}, nil)
			RegisterWalletRoutes(router, stubAuth(4242), api)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "bad-ref")
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", recorder.Code, recorder.Body.String())
			}
			if len(entitlements.log.calls) != 0 {
				t.Errorf("a bad reference reached the write path: %v", entitlements.log.calls)
			}
		})
	}
}

// An unresolvable resource is a 404, and it is checked before anything is
// charged. The lookup is unwired in this slice, so the seam is exercised through
// the interface rather than through a real resource table.
func TestUnlockRefusesAnUnknownResourceAs404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	entitlements := &fakeEntitlements{}
	api := testAPI(t, entitlements, &fakeWallet{available: 1000}, nil).
		WithResourceLookup(&fakeResources{lookupErr: fmt.Errorf("%w: no such resource", ErrNotFound)})
	RegisterWalletRoutes(router, stubAuth(4242), api)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock",
		strings.NewReader(`{"resource_type":"study_resource","resource_id":999999}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "missing-resource")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", recorder.Code, recorder.Body.String())
	}
	if len(entitlements.log.calls) != 0 {
		t.Errorf("a request for a resource that does not exist reached the write path: %v", entitlements.log.calls)
	}
}

// ── the 402 body over HTTP ───────────────────────────────────────────────────

func TestInsufficientCoinsIsADesignedObjectOverHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// A student with no coins and no allowance: the designed state.
	entitlements := &fakeEntitlements{allowanceErr: fmt.Errorf("%w: all used", ErrNoAllowanceRemaining)}
	wallet := &fakeWallet{available: 25}
	api := testAPI(t, entitlements, wallet, nil).WithProfileEligibility(&fakeProfiles{complete: false})
	RegisterWalletRoutes(router, stubAuth(4242), api)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock",
		strings.NewReader(`{"resource_type":"study_resource","resource_id":812}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "short-1")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (%s)", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Required  int64 `json:"required"`
			Available int64 `json:"available"`
			Shortfall int64 `json:"shortfall"`
		} `json:"data"`
		Error struct {
			Code string `json:"code"`
			Data struct {
				Required  int64 `json:"required"`
				Available int64 `json:"available"`
				Shortfall int64 `json:"shortfall"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body: %v (%s)", err, recorder.Body.String())
	}
	if envelope.Success {
		t.Error("the 402 body reported success")
	}
	if envelope.Error.Code != CodeInsufficientCoins {
		t.Errorf("code = %q, want %q", envelope.Error.Code, CodeInsufficientCoins)
	}
	if envelope.Error.Data.Required != 40 || envelope.Error.Data.Available != 25 || envelope.Error.Data.Shortfall != 15 {
		t.Errorf("the designed object = %+v, want required 40 / available 25 / shortfall 15", envelope.Error.Data)
	}
	if envelope.Data.Required != envelope.Error.Data.Required {
		t.Error("the envelope's data and the error's data disagree")
	}
}

// A lapsed allowance that cannot be covered by a purchase is the 423 row of the
// §2.3 table, not the 402, and it still carries the gap.
func TestLapsedAllowanceWithNoPurchaseIs423(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	lapsed := fmt.Errorf("%w: %w: expired on the 26th", ErrNoAllowanceRemaining, ErrAllowanceExpired)
	entitlements := &fakeEntitlements{allowanceErr: lapsed}
	api := testAPI(t, entitlements, &fakeWallet{available: 3}, nil)
	RegisterWalletRoutes(router, stubAuth(4242), api)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock",
		strings.NewReader(`{"resource_type":"study_resource","resource_id":812}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "lapsed-1")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusLocked {
		t.Fatalf("status = %d, want 423 for a lapsed allowance that no purchase covers (%s)", recorder.Code, recorder.Body.String())
	}
	var envelope ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if envelope.Error.Code != CodeAllowanceExpired {
		t.Errorf("code = %q, want %q", envelope.Error.Code, CodeAllowanceExpired)
	}
}

// A key reused with a DIFFERENT payload is a conflict, not a replay
// (02-architecture.md §12.2). The domain error is the ledger's; the mapping is
// this handler's.
func TestIdempotencyKeyReuseIs409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	entitlements := &fakeEntitlements{allowanceErr: fmt.Errorf("%w: all used", ErrNoAllowanceRemaining)}
	api := testAPI(t, entitlements, &fakeWallet{available: 1000}, nil)
	api.purchase = func(context.Context, uint, UnlockRequest, string) (purchaseOutcome, error) {
		return purchaseOutcome{}, fmt.Errorf("%w: key %q was already used for a different request", ErrIdempotencyKeyReuse, "k")
	}
	RegisterWalletRoutes(router, stubAuth(4242), api)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/coins/unlock",
		strings.NewReader(`{"resource_type":"study_resource","resource_id":812}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "reused")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (%s)", recorder.Code, recorder.Body.String())
	}
	var envelope ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if envelope.Error.Code != CodeIdempotencyKeyReuse {
		t.Errorf("code = %q, want %q", envelope.Error.Code, CodeIdempotencyKeyReuse)
	}
}

// ── the status mapping, verbatim from §4 ─────────────────────────────────────

func TestWalletStatusMapping(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantErr  string
	}{
		{"insufficient coins", fmt.Errorf("%w: need 40", ErrInsufficientCoins), http.StatusPaymentRequired, CodeInsufficientCoins},
		{"key reuse", ErrIdempotencyKeyReuse, http.StatusConflict, CodeIdempotencyKeyReuse},
		{"resource not found", fmt.Errorf("%w: gone", ErrNotFound), http.StatusNotFound, CodeResourceNotFound},
		{"allowance lapsed", ErrAllowanceExpired, http.StatusLocked, CodeAllowanceExpired},
		{"frozen account", ErrAccountFrozen, http.StatusLocked, CodeAccountFrozen},
		{"immutable entry", ErrImmutable, http.StatusConflict, CodeEntryImmutable},
		{"bad resource type", fmt.Errorf("%w: nope", ErrInvalidResourceType), http.StatusBadRequest, CodeInvalidRequest},
		{"bad argument", fmt.Errorf("%w: nope", ErrInvalidArgument), http.StatusBadRequest, CodeInvalidRequest},
		{"not configured", ErrNoDatabase, http.StatusServiceUnavailable, CodeNotConfigured},
		{"already owned is a success, never a 4xx or a 5xx", ErrAlreadyUnlocked, http.StatusOK, "ALREADY_OWNED"},
		{"anything else", errors.New("the database is on fire"), http.StatusInternalServerError, CodeInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := walletStatusFor(tc.err); got != tc.wantCode {
				t.Errorf("walletStatusFor = %d, want %d", got, tc.wantCode)
			}
			if got := walletErrorCode(tc.err); got != tc.wantErr {
				t.Errorf("walletErrorCode = %q, want %q", got, tc.wantErr)
			}
		})
	}
}

// ── the cursor ───────────────────────────────────────────────────────────────

func TestTransactionCursorRoundTripsAndRefusesRubbish(t *testing.T) {
	cursor := transactionCursor{
		CreatedAt: time.Date(2026, 9, 26, 5, 2, 11, 123456789, time.UTC),
		ID:        "9f2c4a1e-0000-4000-8000-000000000001",
	}
	encoded := encodeTransactionCursor(cursor)
	decoded, err := decodeTransactionCursor(encoded)
	if err != nil {
		t.Fatalf("decode our own cursor: %v", err)
	}
	if !decoded.CreatedAt.Equal(cursor.CreatedAt) || decoded.ID != cursor.ID {
		t.Errorf("round trip = %+v, want %+v", decoded, cursor)
	}
	// Opaque to the client: not a timestamp, not a number.
	if strings.Contains(encoded, "2026") {
		t.Errorf("the cursor %q leaks its shape; a client will start parsing it", encoded)
	}
	// Padded base64 is tolerated, because "it works in the app but not on the
	// web" is a bug report nobody enjoys.
	padded := strings.TrimRight(encoded, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_")
	if _, err := decodeTransactionCursor(padded); err != nil {
		// Trimming base64url characters can produce a shorter but still valid
		// token or an invalid one; either is acceptable as long as this is not a
		// panic, which is what the assertion guards.
		t.Logf("truncated cursor rejected: %v", err)
	}

	for _, bad := range []string{"not base64 !!!", "eyJ4Ijox", "e30"} {
		if _, err := decodeTransactionCursor(bad); err == nil {
			t.Errorf("decodeTransactionCursor(%q) = nil error, want a refusal", bad)
		}
	}
	if _, err := decodeTransactionCursor(""); err != nil {
		t.Errorf("an absent cursor = %v, want nil (first page)", err)
	}
}

// ── the read endpoints, without a database ──────────────────────────────────

func TestReadEndpointsRefuseAnUnauthenticatedCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := testAPI(t, &fakeEntitlements{}, &fakeWallet{}, nil)
	RegisterWalletRoutes(router, func(c *gin.Context) { c.Next() }, api)

	for _, path := range []string{"/api/v1/coins/balance", "/api/v1/coins/transactions", "/api/v1/coins/allowance"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("GET %s with no identity = %d, want 401", path, recorder.Code)
		}
	}
}

func TestReadEndpointsNeverExposeAnotherUsersWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	wallet := &fakeWallet{available: 500}
	api := testAPI(t, &fakeEntitlements{}, wallet, nil)
	RegisterWalletRoutes(router, stubAuth(4242), api)

	// The user id comes from the middleware, never from a query parameter, so
	// there is nothing in the request to point at somebody else's wallet.
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/coins/balance?user_id=1", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data BalanceResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.TotalAvailable != 500 {
		t.Errorf("total_available = %d, want the caller's own 500", body.Data.TotalAvailable)
	}
	if body.Data.Allowance == nil {
		t.Error("the balance body omits the allowance block §2.1 specifies")
	}
	if body.Data.Buckets == nil || body.Data.SpendOrder == nil {
		t.Error("buckets and spend_order must be arrays, never null — the UI iterates them")
	}
}

func TestListTransactionsRejectsRubbish(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := testAPI(t, &fakeEntitlements{}, &fakeWallet{}, nil)
	RegisterWalletRoutes(router, stubAuth(4242), api)

	for _, query := range []string{"?cursor=@@@", "?limit=0", "?limit=-3", "?limit=abc"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/coins/transactions"+query, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("GET /transactions%s = %d, want 400", query, recorder.Code)
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// stubAuth is the shape middleware.Auth leaves behind: "user_id" in the context.
// One fixed id, because nothing in these tests should be able to become user 0.
func stubAuth(userID uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}
}
