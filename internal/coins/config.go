// Package coins holds the StudsToken coin economy configuration: the typed,
// validated, admin-editable settings that the ledger will read once it is
// built.
//
// This unit is the configuration slice only. There are deliberately no
// accounts, journals, postings, lots or balances here — see
// docs/coin-system/02-architecture.md §7 (where config lives) and §8 (package
// layout) for the wider design, and docs/coin-system/03-api-contract.md §3.1
// for the JSON contract these types mirror.
//
// The product is called StudsToken; the Go identifiers, table names and the
// coin_economy settings key stay as they are until the rename is a deliberate
// single change rather than a scattering of half-migrated identifiers.
package coins

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// EconomyConfigSettingKey is the single system_settings key holding the
	// whole economy config as one JSON object. It follows the
	// find_college_ad_cards pattern (internal/system/service.go:1056-1109).
	EconomyConfigSettingKey = "coin_economy"
)

// maxAmount is the ceiling on any single configured amount.
//
// The value is a bigint, so int64 is already the storage range, but the config
// is arithmetic input: awards.profile_instalment * awards.profile_instalments
// is evaluated on write to keep the instalment ladder honest, and a pair of
// int64s near the type maximum multiplies into a wrapped (negative) result
// that would look like a valid ladder. 1e9 keeps every product below 1e18,
// comfortably inside int64, and is eight orders of magnitude above any price
// this catalogue will ever charge.
const maxAmount int64 = 1_000_000_000

// PriceConfig is the coin cost of unlocking one item of each class. The
// client never supplies a price; the server resolves it from here
// (03-api-contract.md §2.3).
type PriceConfig struct {
	StudyResource int64 `json:"study_resource"`
	Video         int64 `json:"video"`
	MockTest      int64 `json:"mock_test"`
}

// AwardConfig is what a student earns for each qualifying event. The profile
// award is paid as ProfileInstalments instalments of ProfileInstalment coins.
type AwardConfig struct {
	ProfileComplete    int64 `json:"profile_complete"`
	ProfileInstalment  int64 `json:"profile_instalment"`
	ProfileInstalments int64 `json:"profile_instalments"`
	ReferralReferrer   int64 `json:"referral_referrer"`
	ReferralReferred   int64 `json:"referral_referred"`
	ResourceApproved   int64 `json:"resource_approved"`
}

// AllowanceConfig is the free-allowance entitlement shape: three per-class
// counts and how long they last. An allowance is an entitlement, not a coin
// grant (02-architecture.md §5).
type AllowanceConfig struct {
	DocumentUnlocks int64 `json:"document_unlocks"`
	VideoUnlocks    int64 `json:"video_unlocks"`
	MockTestUnlocks int64 `json:"mock_test_unlocks"`
	ExpiresInDays   int64 `json:"expires_in_days"`
}

// ExpiryConfig is the per-bucket expiry policy. FreeDays may be extended by
// ActivityExtendDays of qualifying activity; earned coins live EarnedDays.
type ExpiryConfig struct {
	FreeDays           int64 `json:"free_days"`
	EarnedDays         int64 `json:"earned_days"`
	ActivityExtendDays int64 `json:"activity_extend_days"`
}

// ReferralConfig caps referral awards. MonthlyCap is the primary cap — a
// count of successful referrals per calendar month — and LifetimeCoinCap is
// derived from it. See DefaultEconomyConfig.
type ReferralConfig struct {
	MonthlyCap      int64 `json:"monthly_cap"`
	LifetimeCoinCap int64 `json:"lifetime_coin_cap"`
	HoldDays        int64 `json:"hold_days"`
}

// EconomyConfig is the whole admin-editable economy. The json tags are the
// wire contract of 03-api-contract.md §3.1 and are used for both the
// system_settings payload and the admin response, so the two cannot drift.
type EconomyConfig struct {
	Prices             PriceConfig     `json:"prices"`
	Awards             AwardConfig     `json:"awards"`
	Allowance          AllowanceConfig `json:"allowance"`
	Expiry             ExpiryConfig    `json:"expiry"`
	Referral           ReferralConfig  `json:"referral"`
	ClawbackWindowDays int64           `json:"clawback_window_days"`
	// UnlockEndpointEnabled ships the write path, POST /api/v1/coins/unlock.
	//
	// It is DARK, and that is the decision this field exists to record. Nothing
	// consumes an unlock yet: the resource gates — the thing that would make an
	// unlock worth paying for — land in the next slice. Until they do, a live
	// unlock endpoint lets a student spend real coins unlocking content they
	// still have free access to, because the check that would make the purchase
	// meaningful does not exist yet and nothing has been taken away. Charging
	// for something that has not been gated is worse than charging for nothing:
	// the coins are gone and the entitlement buys nothing, and the refund path
	// (Reverse on a spend) is explicitly refused by the ledger.
	//
	// So the write endpoint is mounted but refuses with 503 until an admin turns
	// this on, and the slice that adds the first gate is the one that turns it
	// on. The three READ endpoints are unaffected and always available: they
	// expose only the caller's own wallet, they move nothing, and they are what
	// the frontend needs to build the unlock affordance before it can be paid
	// for.
	UnlockEndpointEnabled bool `json:"unlock_endpoint_enabled"`
}

// DefaultEconomyConfig is the contract when nothing is stored yet, and the
// base every stored value is unmarshalled on top of so a partial object still
// yields sane values for the keys it omits.
//
// The figures are the agreed launch placeholders from
// docs/coin-system/05-economy-and-fraud.md §2.1, so the admin screen shows
// real numbers instead of blanks. They are a starting hypothesis, not a
// specification — this is the only doc whose numbers are expected to move
// after launch.
func DefaultEconomyConfig() EconomyConfig {
	return EconomyConfig{
		Prices: PriceConfig{
			StudyResource: 40,
			Video:         90,
			MockTest:      60,
		},
		Awards: AwardConfig{
			ProfileComplete: 25,
			// 5 instalments x 5 coins = 25, matching ProfileComplete. The
			// instalment ladder is validated on every write, so editing one of
			// the three without the others is rejected rather than paid out
			// incorrectly.
			ProfileInstalment:  5,
			ProfileInstalments: 5,
			ReferralReferrer:   60,
			ReferralReferred:   25,
			ResourceApproved:   80,
		},
		Allowance: AllowanceConfig{
			DocumentUnlocks: 3,
			VideoUnlocks:    1,
			MockTestUnlocks: 1,
			ExpiresInDays:   30,
		},
		Expiry: ExpiryConfig{
			FreeDays:           30,
			EarnedDays:         365,
			ActivityExtendDays: 180,
		},
		Referral: ReferralConfig{
			MonthlyCap: 10,
			// LifetimeCoinCap is DERIVED, not chosen: 10 successful referrals
			// per month x 60 coins each = 600. Decision D4 originally said
			// "200 coins/month lifetime", which contradicts the count cap: at
			// 60 coins per referral a 200-coin ceiling binds at 3.3 referrals,
			// so the count cap of 10 would be unreachable and decorative
			// (05-economy-and-fraud.md §2.4). The count cap is primary and
			// the coin ceiling is derived from it. Do not "simplify" this back
			// to 200 — that re-creates the contradiction.
			LifetimeCoinCap: 600,
			HoldDays:        7,
		},
		ClawbackWindowDays: 180,
		// Explicit rather than left to the zero value, because "off" is a
		// decision here and not an accident of a struct literal. See the field
		// comment for why the write path ships dark.
		UnlockEndpointEnabled: false,
	}
}

// FieldError is one rejected field and the reason it was rejected.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError is the field-level rejection produced by
// ValidateEconomyConfig. It matches ErrInvalidConfig under errors.Is so the
// handler maps it to 400 without knowing the concrete type, per the house
// style at internal/mocktests/handler.go:247-258.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, f.Field+": "+f.Message)
	}
	return strings.Join(parts, "; ")
}

// Is reports ValidationError as ErrInvalidConfig. The whole update is
// rejected on any violation, so there is no partial-validity state to model.
func (e *ValidationError) Is(target error) bool { return target == ErrInvalidConfig }

// amountField pairs a configured amount with its JSON path, so the error
// message names the key the admin actually sent.
type amountField struct {
	path  string
	value int64
}

// economyAmountFields flattens every numeric leaf of the config. Keeping the
// list in one place is what makes "all amounts are non-negative bigint-safe
// values" a single rule instead of fifteen that can drift apart.
func economyAmountFields(cfg EconomyConfig) []amountField {
	return []amountField{
		{"prices.study_resource", cfg.Prices.StudyResource},
		{"prices.video", cfg.Prices.Video},
		{"prices.mock_test", cfg.Prices.MockTest},
		{"awards.profile_complete", cfg.Awards.ProfileComplete},
		{"awards.profile_instalment", cfg.Awards.ProfileInstalment},
		{"awards.profile_instalments", cfg.Awards.ProfileInstalments},
		{"awards.referral_referrer", cfg.Awards.ReferralReferrer},
		{"awards.referral_referred", cfg.Awards.ReferralReferred},
		{"awards.resource_approved", cfg.Awards.ResourceApproved},
		{"allowance.document_unlocks", cfg.Allowance.DocumentUnlocks},
		{"allowance.video_unlocks", cfg.Allowance.VideoUnlocks},
		{"allowance.mock_test_unlocks", cfg.Allowance.MockTestUnlocks},
		{"allowance.expires_in_days", cfg.Allowance.ExpiresInDays},
		{"expiry.free_days", cfg.Expiry.FreeDays},
		{"expiry.earned_days", cfg.Expiry.EarnedDays},
		{"expiry.activity_extend_days", cfg.Expiry.ActivityExtendDays},
		{"referral.monthly_cap", cfg.Referral.MonthlyCap},
		{"referral.lifetime_coin_cap", cfg.Referral.LifetimeCoinCap},
		{"referral.hold_days", cfg.Referral.HoldDays},
		{"clawback_window_days", cfg.ClawbackWindowDays},
	}
}

// instalmentProduct is profile_instalment * profile_instalments, reporting
// false when either factor is outside the ceiling. The factors are int64, so
// multiplying two of them near the type maximum wraps negative; a factor that
// big has already been rejected by economyAmountFields, and skipping the
// comparison avoids validating against a wrapped value.
func (a AwardConfig) instalmentProduct() (int64, bool) {
	if a.ProfileInstalment > maxAmount || a.ProfileInstalments > maxAmount {
		return 0, false
	}
	return a.ProfileInstalment * a.ProfileInstalments, true
}

// ValidateEconomyConfig rejects the whole config on any violation, because a
// misconfigured economy is a support incident rather than a degraded request
// (03-api-contract.md §3.1). It is a pure function with no database, so the
// rules are testable on their own.
//
// The rules, in the order they are reported:
//
//   - every amount is a non-negative integer within the bigint-safe ceiling
//     (this covers the spec's explicit "allowance.document_unlocks >= 0",
//     "referral.hold_days >= 0" and "clawback_window_days >= 0" — they are the
//     same rule applied to those keys, and a duplicate message per field would
//     be noise);
//   - the per-class allowance values are consistent with the prices: a
//     non-zero allowance for a class requires that class to cost more than
//     zero, or the allowance grants a free unlock of something that is supposed
//     to be paid. The spec names video explicitly; the same rule is applied to
//     documents and mock tests because the reason is identical;
//   - unlock_endpoint_enabled requires every class to be priced above zero,
//     because a live purchase endpoint over a free class fails every request
//     for that class;
//   - expiry.free_days > 0 and expiry.earned_days > 0, since a zero lifetime
//     makes coins unusable the moment they are granted;
//   - awards.profile_instalment * awards.profile_instalments equals
//     awards.profile_complete, which keeps the 5 x 5 = 25 ladder honest if an
//     admin edits only one of the three.
//
// The result is a *ValidationError carrying one entry per offending field, or
// nil when the config is acceptable.
func ValidateEconomyConfig(cfg EconomyConfig) error {
	var fields []FieldError
	add := func(field, message string) {
		fields = append(fields, FieldError{Field: field, Message: message})
	}

	for _, f := range economyAmountFields(cfg) {
		switch {
		case f.value < 0:
			add(f.path, "must be a non-negative integer")
		case f.value > maxAmount:
			add(f.path, fmt.Sprintf("must be at most %d (bigint-safe ceiling for this config)", maxAmount))
		}
	}

	classes := []struct {
		allowanceField string
		allowance      int64
		priceField     string
		price          int64
	}{
		{"allowance.document_unlocks", cfg.Allowance.DocumentUnlocks, "prices.study_resource", cfg.Prices.StudyResource},
		{"allowance.video_unlocks", cfg.Allowance.VideoUnlocks, "prices.video", cfg.Prices.Video},
		{"allowance.mock_test_unlocks", cfg.Allowance.MockTestUnlocks, "prices.mock_test", cfg.Prices.MockTest},
	}
	for _, class := range classes {
		if class.allowance > 0 && class.price <= 0 {
			add(class.allowanceField, fmt.Sprintf("must be 0 while %s is 0", class.priceField))
		}
	}

	// Turning the write path on is validated against the prices, because a class
	// priced at zero is a free unlock and the ledger refuses it (Spend rejects a
	// non-positive price) rather than granting it — so the endpoint would answer
	// 400 for every purchase of that class while looking switched on. Failing the
	// whole config write says it at the moment an admin pressed save, which is
	// the only time anybody is looking.
	//
	// The refusal names the flag rather than the price, because the price is
	// legal on its own: a zero price is a legitimate way to make one class
	// free. What is refused is the COMBINATION of a free class and a live
	// purchase endpoint.
	if cfg.UnlockEndpointEnabled {
		for _, class := range classes {
			if class.price <= 0 {
				add("unlock_endpoint_enabled",
					fmt.Sprintf("must be false while %s is 0; a class priced at 0 cannot be bought and every unlock of it would fail", class.priceField))
			}
		}
	}

	if cfg.Expiry.FreeDays <= 0 {
		add("expiry.free_days", "must be greater than 0")
	}
	if cfg.Expiry.EarnedDays <= 0 {
		add("expiry.earned_days", "must be greater than 0")
	}

	if product, ok := cfg.Awards.instalmentProduct(); !ok || product != cfg.Awards.ProfileComplete {
		add("awards.profile_complete", "must equal awards.profile_instalment * awards.profile_instalments")
	}

	if len(fields) == 0 {
		return nil
	}
	return &ValidationError{Fields: fields}
}

// SettingStore is the narrow slice of system.Repository this package needs.
// Declaring it here keeps internal/system out of the signature and lets the
// tests drive load, save and cache behaviour with an in-memory fake.
type SettingStore interface {
	GetSystemSetting(key string) (value string, found bool, err error)
	SetSystemSetting(key, value string) error
}

// configCacheTTL bounds how long a cached economy config may be served without
// re-reading system_settings.
//
// Why the cache exists: internal/system has none — GetSystemSetting is a fresh
// SELECT on every call (internal/system/repository.go:1209-1219) — and the
// ledger reads pricing on every unlock attempt and every balance render.
// internal/system is not ours to change, because other modules depend on its
// current behaviour, so the caching lives in this package instead.
//
// Why 30s: the admin write path invalidates explicitly, so the TTL only
// bounds staleness for an edit made out of band — direct SQL, or a second
// server process writing the row. 30s is short enough that "I changed the
// price and it is not live yet" never becomes a support ticket, and long
// enough to collapse a burst of reads on the unlock hot path into one query.
// There is deliberately no singleflight: concurrent misses may issue more than
// one SELECT, which is harmless because they all read the same row.
const configCacheTTL = 30 * time.Second

// configCache holds one economy config with an expiry. A bool rather than a
// nil pointer marks "not loaded", so an empty value is never confused with a
// cached one.
type configCache struct {
	mu      sync.Mutex
	cfg     EconomyConfig
	loaded  bool
	expires time.Time
}

func (c *configCache) get(now time.Time) (EconomyConfig, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded || now.After(c.expires) {
		return EconomyConfig{}, false
	}
	return c.cfg, true
}

func (c *configCache) put(cfg EconomyConfig, expires time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
	c.loaded = true
	c.expires = expires
}

// invalidate is the explicit invalidation path. Every successful write calls
// it, so an admin edit is visible to the very next read in this process
// rather than after the TTL. Multi-process deployments rely on the TTL for
// propagation; that is the reason it is short and bounded.
func (c *configCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = EconomyConfig{}
	c.loaded = false
	c.expires = time.Time{}
}

// ConfigStore is the typed wrapper over the coin_economy system_settings
// value: it knows the key, the defaults and the JSON encoding, and it holds
// the cache so the hot read path is a map-free struct copy rather than a
// query.
type ConfigStore struct {
	settings SettingStore
	cache    configCache

	// now and ttl are injectable so the cache is testable without sleeping.
	now func() time.Time
	ttl time.Duration
}

func NewConfigStore(settings SettingStore) *ConfigStore {
	return &ConfigStore{settings: settings, now: time.Now, ttl: configCacheTTL}
}

// Load returns the current economy config, from the cache when it is fresh.
//
// Absent or empty storage yields the defaults. A stored value that is not
// valid JSON is an error rather than a silent fallback to defaults: quietly
// reverting the live economy to placeholder prices is worse than a failing
// read, and the value is only ever written through the validated path in this
// package, so unreadable storage means external tampering. The consequence is
// that recovery needs a database-level fix — noted rather than worked around
// with a heuristic that would hide the tamper.
func (s *ConfigStore) Load() (EconomyConfig, error) {
	now := s.now()
	if cfg, ok := s.cache.get(now); ok {
		return cfg, nil
	}
	value, found, err := s.settings.GetSystemSetting(EconomyConfigSettingKey)
	if err != nil {
		return EconomyConfig{}, err
	}
	// Unmarshal onto the defaults so a partial stored object still yields sane
	// values for the keys it omits.
	cfg := DefaultEconomyConfig()
	if found && strings.TrimSpace(value) != "" {
		if err := json.Unmarshal([]byte(value), &cfg); err != nil {
			return EconomyConfig{}, fmt.Errorf("%w: %s is not valid json: %v", ErrConfigUnreadable, EconomyConfigSettingKey, err)
		}
	}
	s.cache.put(cfg, now.Add(s.cacheTTL()))
	return cfg, nil
}

// Save persists the config and invalidates the cache, in that order: an
// invalidation before a successful write would leave a stale entry that the
// next reader trusts for the rest of its TTL.
//
// SetSystemSetting is read-then-create, not an upsert
// (internal/system/repository.go:1221-1233), so two concurrent admin writers
// can race on the unique key and one of them fails on the constraint. That is
// accepted for a value that changes a handful of times a year and is
// documented here, in this one place, rather than worked around by changing
// internal/system for every other module that depends on it.
func (s *ConfigStore) Save(encoded string) error {
	if err := s.settings.SetSystemSetting(EconomyConfigSettingKey, encoded); err != nil {
		return err
	}
	s.cache.invalidate()
	return nil
}

// Invalidate drops the cached config. Called after a write; exposed for tests
// and for an out-of-band change the process learns about another way.
func (s *ConfigStore) Invalidate() { s.cache.invalidate() }

func (s *ConfigStore) cacheTTL() time.Duration {
	if s.ttl > 0 {
		return s.ttl
	}
	return configCacheTTL
}
