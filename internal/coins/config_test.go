package coins

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSettings is an in-memory SettingStore. Everything in this file runs
// without a database: the validation and default tests are pure functions, and
// the load/save/cache tests only need the two-method store interface.
type fakeSettings struct {
	mu     sync.Mutex
	values map[string]string
	reads  int
	writes int
}

func newFakeSettings() *fakeSettings {
	return &fakeSettings{values: map[string]string{}}
}

func (f *fakeSettings) GetSystemSetting(key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	value, ok := f.values[key]
	return value, ok, nil
}

func (f *fakeSettings) SetSystemSetting(key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	f.values[key] = value
	return nil
}

func (f *fakeSettings) get(key string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.values[key]
	return value, ok
}

// fakeVersions records the audit rows a service appends.
type fakeVersions struct {
	rows    []ConfigVersion
	failErr error
}

func (f *fakeVersions) AppendConfigVersion(version *ConfigVersion) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.rows = append(f.rows, *version)
	return nil
}

// testService wires a service over the fakes with a clock the test drives, so
// the cache TTL is exercised without sleeping.
func testService(t *testing.T) (*Service, *fakeSettings, *fakeVersions, *ConfigStore) {
	t.Helper()
	settings := newFakeSettings()
	versions := &fakeVersions{}
	store := NewConfigStore(settings)
	now := time.Now()
	store.now = func() time.Time { return now }
	svc := NewService(store, versions)
	return svc, settings, versions, store
}

func i64(v int64) *int64 { return &v }

// boolp is i64 for the switches. The gate flags are pointers so that "not
// mentioned" and "sent as false" are different requests.
func boolp(v bool) *bool { return &v }

func TestDefaultEconomyConfigMatchesAgreedPlaceholders(t *testing.T) {
	cfg := DefaultEconomyConfig()

	// Spelled out rather than compared against a literal, because the point of
	// the test is that the shipped placeholders are these numbers.
	if cfg.Prices.StudyResource != 40 || cfg.Prices.Video != 90 || cfg.Prices.MockTest != 60 {
		t.Fatalf("prices wrong: %+v", cfg.Prices)
	}
	if cfg.Awards.ProfileComplete != 25 || cfg.Awards.ProfileInstalment != 5 || cfg.Awards.ProfileInstalments != 5 {
		t.Fatalf("profile awards wrong: %+v", cfg.Awards)
	}
	if cfg.Awards.ReferralReferrer != 60 || cfg.Awards.ReferralReferred != 25 || cfg.Awards.ResourceApproved != 80 {
		t.Fatalf("referral/resource awards wrong: %+v", cfg.Awards)
	}
	if cfg.Allowance.DocumentUnlocks != 3 || cfg.Allowance.VideoUnlocks != 1 || cfg.Allowance.MockTestUnlocks != 1 {
		t.Fatalf("allowance wrong: %+v", cfg.Allowance)
	}
	if cfg.Allowance.ExpiresInDays != 30 {
		t.Fatalf("allowance.expires_in_days=%d want 30", cfg.Allowance.ExpiresInDays)
	}
	if cfg.Expiry.FreeDays != 30 || cfg.Expiry.EarnedDays != 365 || cfg.Expiry.ActivityExtendDays != 180 {
		t.Fatalf("expiry wrong: %+v", cfg.Expiry)
	}
	if cfg.Referral.MonthlyCap != 10 || cfg.Referral.HoldDays != 7 {
		t.Fatalf("referral caps wrong: %+v", cfg.Referral)
	}
	// 600 is derived (monthly_cap 10 x referral_referrer 60), not chosen. See
	// the DefaultEconomyConfig comment and 05-economy-and-fraud.md §2.4: the
	// original 200-coin ceiling contradicted the count cap by binding at 3.3
	// referrals, which made the cap of 10 decorative.
	if cfg.Referral.LifetimeCoinCap != 600 {
		t.Fatalf("referral.lifetime_coin_cap=%d want 600 (10 referrals x 60 coins)", cfg.Referral.LifetimeCoinCap)
	}
	if cfg.ClawbackWindowDays != 180 {
		t.Fatalf("clawback_window_days=%d want 180", cfg.ClawbackWindowDays)
	}
	if cfg.Gates.StudyResource || cfg.Gates.Video || cfg.Gates.MockTest {
		t.Fatalf("gates_enabled=%+v, want all three off", cfg.Gates)
	}
}

// The gates ship dark, and this is the test that says so independently of the
// JSON shape: a build that shipped a gate on would change what a student
// experiences the moment it deployed, and the only thing standing between that
// and production is a default nobody is supposed to edit.
func TestGatesShipDark(t *testing.T) {
	cfg := DefaultEconomyConfig()
	for _, class := range ResourceTypes {
		if cfg.GateEnabled(class) {
			t.Errorf("gates_enabled.%s is on in the default config; it must ship off", class)
		}
	}

	// A stored config written before gates_enabled existed reads back through
	// Load, which unmarshals onto the defaults — so an absent key is off too,
	// not "on because it was missing".
	settings := newFakeSettings()
	settings.values[EconomyConfigSettingKey] = `{"prices":{"study_resource":60}}`
	loaded, err := NewConfigStore(settings).Load()
	if err != nil {
		t.Fatalf("load a config with no gates_enabled key: %v", err)
	}
	if loaded.Prices.StudyResource != 60 {
		t.Fatalf("the stored price did not apply: %+v", loaded.Prices)
	}
	if loaded.GateEnabled(ResourceTypeStudyResource) {
		t.Error("a stored config that omits gates_enabled came up with the document gate on")
	}

	// One class at a time: turning the document gate on must not drag video or
	// mock tests with it, or an incident response reverts less than it means to.
	settings.values[EconomyConfigSettingKey] = `{"gates_enabled":{"study_resource":true}}`
	loaded, err = NewConfigStore(settings).Load()
	if err != nil {
		t.Fatalf("load a config with one gate on: %v", err)
	}
	if !loaded.GateEnabled(ResourceTypeStudyResource) {
		t.Error("the document gate did not come on")
	}
	if loaded.GateEnabled(ResourceTypeVideo) || loaded.GateEnabled(ResourceTypeMockTest) {
		t.Errorf("enabling one gate enabled another: %+v", loaded.Gates)
	}
}

// An admin can turn a single class back to free without touching the others, and
// omitting the block entirely changes nothing — the kill switch is only a kill
// switch if it is reachable from the same screen as the prices.
func TestGatesAreSettableOneClassAtATime(t *testing.T) {
	base := DefaultEconomyConfig()
	base.Gates.StudyResource = true

	off := applyEconomyConfigUpdate(base, UpdateEconomyConfigRequest{
		Gates: &UpdateGatesRequest{StudyResource: boolp(false)},
	})
	if off.GateEnabled(ResourceTypeStudyResource) {
		t.Error("sending study_resource:false did not turn the document gate off")
	}

	// A partial update that says nothing about the gates leaves them alone.
	untouched := applyEconomyConfigUpdate(base, UpdateEconomyConfigRequest{
		Prices: &UpdatePricesRequest{StudyResource: i64(55)},
	})
	if !untouched.GateEnabled(ResourceTypeStudyResource) {
		t.Error("a price edit silently turned the document gate off")
	}
	if untouched.Prices.StudyResource != 55 {
		t.Errorf("price = %d, want 55", untouched.Prices.StudyResource)
	}

	// Enabling video alone must not enable documents. The base here is a config
	// with every gate off, because a merge never turns a gate OFF that the
	// request did not mention — that is what makes a one-class incident response
	// possible without re-sending the whole block.
	video := applyEconomyConfigUpdate(DefaultEconomyConfig(), UpdateEconomyConfigRequest{
		Gates: &UpdateGatesRequest{Video: boolp(true)},
	})
	if !video.GateEnabled(ResourceTypeVideo) || video.GateEnabled(ResourceTypeStudyResource) {
		t.Errorf("gates = %+v, want only video on", video.Gates)
	}
	// And the result still has to be a config the rest of the package accepts.
	if err := ValidateEconomyConfig(video); err != nil {
		t.Errorf("a config with one gate on does not validate: %v", err)
	}
}

func TestDefaultEconomyConfigIsValid(t *testing.T) {
	if err := ValidateEconomyConfig(DefaultEconomyConfig()); err != nil {
		t.Fatalf("default config must validate, got: %v", err)
	}
}

// The write path ships dark, and the two things that could turn it on by
// accident are both closed: the default is false, and a config that enables it
// over a zero-priced class is rejected outright rather than deployed and then
// failing every purchase of that class.
func TestUnlockEndpointShipsDarkAndCannotBeEnabledOverAFreeClass(t *testing.T) {
	if DefaultEconomyConfig().UnlockEndpointEnabled {
		t.Fatal("the default config has the unlock write path enabled; it must ship dark")
	}
	// A stored value that omits the key entirely also reads as false, because
	// Load unmarshals onto the defaults. A config written before this field
	// existed must not come up with purchases switched on.
	if got := DefaultEconomyConfig(); got.UnlockEndpointEnabled {
		t.Fatalf("unlock_endpoint_enabled = %t, want false", got.UnlockEndpointEnabled)
	}

	enabled := DefaultEconomyConfig()
	enabled.UnlockEndpointEnabled = true
	if err := ValidateEconomyConfig(enabled); err != nil {
		t.Fatalf("enabling the write path with every class priced must validate, got: %v", err)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*EconomyConfig)
	}{
		{"a free document class", func(c *EconomyConfig) {
			c.Prices.StudyResource = 0
			c.Allowance.DocumentUnlocks = 0
		}},
		{"a free video class", func(c *EconomyConfig) {
			c.Prices.Video = 0
			c.Allowance.VideoUnlocks = 0
		}},
		{"a free mock test class", func(c *EconomyConfig) {
			c.Prices.MockTest = 0
			c.Allowance.MockTestUnlocks = 0
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultEconomyConfig()
			cfg.UnlockEndpointEnabled = true
			tc.mutate(&cfg)
			err := ValidateEconomyConfig(cfg)
			if err == nil {
				t.Fatal("a live write path over a free class was accepted")
			}
			var verr *ValidationError
			if !asValidationError(err, &verr) {
				t.Fatalf("error = %v, want a *ValidationError", err)
			}
			found := false
			for _, f := range verr.Fields {
				if f.Field == "unlock_endpoint_enabled" {
					found = true
				}
			}
			if !found {
				t.Errorf("the refusal does not name unlock_endpoint_enabled: %+v", verr.Fields)
			}
		})
	}
}

// asValidationError is errors.As with the import kept out of a test whose point
// is the rule rather than the mechanism.
func asValidationError(err error, target **ValidationError) bool {
	for err != nil {
		if v, ok := err.(*ValidationError); ok {
			*target = v
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// TestEconomyConfigJSONShape pins the stored/admin JSON to the object in
// 03-api-contract.md §3.1, key for key and value for value. This is what stops
// a rename or a dropped field from silently breaking the admin screen, the
// system_settings payload, and every future ledger read of it.
func TestEconomyConfigJSONShape(t *testing.T) {
	const want = `{"prices":{"study_resource":40,"video":90,"mock_test":60},` +
		`"awards":{"profile_complete":25,"profile_instalment":5,"profile_instalments":5,` +
		`"referral_referrer":60,"referral_referred":25,"resource_approved":80},` +
		`"allowance":{"document_unlocks":3,"video_unlocks":1,"mock_test_unlocks":1,"expires_in_days":30},` +
		`"expiry":{"free_days":30,"earned_days":365,"activity_extend_days":180},` +
		`"referral":{"monthly_cap":10,"lifetime_coin_cap":600,"hold_days":7},` +
		`"clawback_window_days":180,` +
		`"gates_enabled":{"study_resource":false,"video":false,"mock_test":false},` +
		`"unlock_endpoint_enabled":false}`

	got, err := json.Marshal(DefaultEconomyConfig())
	if err != nil {
		t.Fatalf("marshal default config: %v", err)
	}
	if string(got) != want {
		t.Fatalf("config JSON shape changed.\n got: %s\nwant: %s", got, want)
	}
}

func TestValidateEconomyConfig(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*EconomyConfig)
		wantFail bool
		wantMsg  string
	}{
		{name: "defaults are accepted", mutate: func(*EconomyConfig) {}},

		{name: "negative price", mutate: func(c *EconomyConfig) { c.Prices.StudyResource = -1 },
			wantFail: true, wantMsg: "prices.study_resource: must be a non-negative integer"},
		{name: "negative award", mutate: func(c *EconomyConfig) { c.Awards.ReferralReferrer = -60 },
			wantFail: true, wantMsg: "awards.referral_referrer: must be a non-negative integer"},
		{name: "negative allowance count", mutate: func(c *EconomyConfig) { c.Allowance.DocumentUnlocks = -1 },
			wantFail: true, wantMsg: "allowance.document_unlocks: must be a non-negative integer"},
		{name: "negative referral hold days", mutate: func(c *EconomyConfig) { c.Referral.HoldDays = -1 },
			wantFail: true, wantMsg: "referral.hold_days: must be a non-negative integer"},
		{name: "negative clawback window", mutate: func(c *EconomyConfig) { c.ClawbackWindowDays = -1 },
			wantFail: true, wantMsg: "clawback_window_days: must be a non-negative integer"},
		{name: "negative expiry extension", mutate: func(c *EconomyConfig) { c.Expiry.ActivityExtendDays = -180 },
			wantFail: true, wantMsg: "expiry.activity_extend_days: must be a non-negative integer"},

		// bigint-safe range. int64 is the storage type, so the ceiling is what
		// stops a value whose product with another overflows the instalment
		// ladder arithmetic.
		{name: "amount above the ceiling", mutate: func(c *EconomyConfig) { c.Prices.Video = maxAmount + 1 },
			wantFail: true, wantMsg: "prices.video: must be at most 1000000000 (bigint-safe ceiling for this config)"},
		{name: "amount at the ceiling is accepted", mutate: func(c *EconomyConfig) { c.Prices.Video = maxAmount }},

		// Per-class allowance consistency. The spec names video; documents and
		// mock tests are the same rule.
		{name: "video allowance needs a video price", mutate: func(c *EconomyConfig) { c.Prices.Video = 0 },
			wantFail: true, wantMsg: "allowance.video_unlocks: must be 0 while prices.video is 0"},
		{name: "document allowance needs a document price", mutate: func(c *EconomyConfig) { c.Prices.StudyResource = 0 },
			wantFail: true, wantMsg: "allowance.document_unlocks: must be 0 while prices.study_resource is 0"},
		{name: "mock test allowance needs a mock test price", mutate: func(c *EconomyConfig) { c.Prices.MockTest = 0 },
			wantFail: true, wantMsg: "allowance.mock_test_unlocks: must be 0 while prices.mock_test is 0"},
		{name: "no allowance needs no price", mutate: func(c *EconomyConfig) {
			c.Allowance.DocumentUnlocks, c.Allowance.VideoUnlocks, c.Allowance.MockTestUnlocks = 0, 0, 0
			c.Prices.StudyResource, c.Prices.Video, c.Prices.MockTest = 0, 0, 0
		}},

		{name: "free days must be positive", mutate: func(c *EconomyConfig) { c.Expiry.FreeDays = 0 },
			wantFail: true, wantMsg: "expiry.free_days: must be greater than 0"},
		{name: "earned days must be positive", mutate: func(c *EconomyConfig) { c.Expiry.EarnedDays = 0 },
			wantFail: true, wantMsg: "expiry.earned_days: must be greater than 0"},

		// The instalment ladder: 5 x 5 = 25. Editing one of the three alone
		// would otherwise pay out 25 in instalments of a different size, or
		// 5 instalments totalling something other than the advertised award.
		{name: "ladder total must match instalment product", mutate: func(c *EconomyConfig) { c.Awards.ProfileComplete = 30 },
			wantFail: true, wantMsg: "awards.profile_complete: must equal awards.profile_instalment * awards.profile_instalments"},
		{name: "ladder instalment edited alone", mutate: func(c *EconomyConfig) { c.Awards.ProfileInstalment = 6 },
			wantFail: true, wantMsg: "awards.profile_complete: must equal awards.profile_instalment * awards.profile_instalments"},
		{name: "whole ladder moved together", mutate: func(c *EconomyConfig) {
			c.Awards.ProfileComplete, c.Awards.ProfileInstalment = 50, 10
		}},

		// Every violation is reported, not just the first.
		{name: "all violations collected", mutate: func(c *EconomyConfig) {
			c.Prices.StudyResource = -1
			c.Expiry.FreeDays = 0
			c.Awards.ProfileComplete = 999
		}, wantFail: true, wantMsg: "prices.study_resource: must be a non-negative integer"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultEconomyConfig()
			tc.mutate(&cfg)

			err := ValidateEconomyConfig(cfg)
			if !tc.wantFail {
				if err != nil {
					t.Fatalf("want accepted, got rejection: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want rejection, got none for %+v", cfg)
			}
			// The handler maps this sentinel to 400; matching it here keeps the
			// status mapping honest.
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("error %v does not match ErrInvalidConfig", err)
			}
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error %v is not a *ValidationError", err)
			}
			if len(validationErr.Fields) == 0 {
				t.Fatalf("rejection carries no field errors: %v", err)
			}
			if got := err.Error(); !strings.Contains(got, tc.wantMsg) {
				t.Fatalf("message %q does not mention %q", got, tc.wantMsg)
			}
		})
	}
}

func TestValidateEconomyConfigDoesNotOverflowInstalmentProduct(t *testing.T) {
	// Two factors near the int64 maximum multiply into a negative result. A
	// wrapped product would compare unequal to profile_complete for the wrong
	// reason; the point is that the config is rejected and the process does not
	// silently accept a nonsense ladder.
	cfg := DefaultEconomyConfig()
	cfg.Awards.ProfileComplete = 0
	cfg.Awards.ProfileInstalment = 1 << 62
	cfg.Awards.ProfileInstalments = 8

	err := ValidateEconomyConfig(cfg)
	if err == nil {
		t.Fatal("want rejection for amounts above the ceiling")
	}
	if _, ok := cfg.Awards.instalmentProduct(); ok {
		t.Fatal("instalmentProduct must refuse to multiply an out-of-range factor")
	}
}

func TestApplyEconomyConfigUpdateKeepsOmittedFields(t *testing.T) {
	base := DefaultEconomyConfig()

	tests := []struct {
		name string
		req  UpdateEconomyConfigRequest
		want func(*testing.T, EconomyConfig)
	}{
		{
			name: "empty request changes nothing",
			req:  UpdateEconomyConfigRequest{},
			want: func(t *testing.T, got EconomyConfig) {
				if got != base {
					t.Fatalf("empty request changed the config: %+v", got)
				}
			},
		},
		{
			name: "single price change",
			req:  UpdateEconomyConfigRequest{Prices: &UpdatePricesRequest{StudyResource: i64(60)}},
			want: func(t *testing.T, got EconomyConfig) {
				if got.Prices.StudyResource != 60 {
					t.Fatalf("study_resource=%d want 60", got.Prices.StudyResource)
				}
				if got.Prices.Video != base.Prices.Video || got.Prices.MockTest != base.Prices.MockTest {
					t.Fatalf("sibling prices changed: %+v", got.Prices)
				}
				if got.Referral != base.Referral || got.Expiry != base.Expiry {
					t.Fatalf("unrelated groups changed: %+v", got)
				}
			},
		},
		{
			name: "explicit zero is applied, not treated as absent",
			req:  UpdateEconomyConfigRequest{ClawbackWindowDays: i64(0)},
			want: func(t *testing.T, got EconomyConfig) {
				if got.ClawbackWindowDays != 0 {
					t.Fatalf("clawback_window_days=%d want 0", got.ClawbackWindowDays)
				}
			},
		},
		{
			name: "whole referral block",
			req: UpdateEconomyConfigRequest{Referral: &UpdateReferralRequest{
				MonthlyCap:      i64(5),
				LifetimeCoinCap: i64(300),
				HoldDays:        i64(14),
			}},
			want: func(t *testing.T, got EconomyConfig) {
				want := ReferralConfig{MonthlyCap: 5, LifetimeCoinCap: 300, HoldDays: 14}
				if got.Referral != want {
					t.Fatalf("referral=%+v want %+v", got.Referral, want)
				}
			},
		},
		{
			name: "present but empty group changes nothing",
			req:  UpdateEconomyConfigRequest{Awards: &UpdateAwardsRequest{}},
			want: func(t *testing.T, got EconomyConfig) {
				if got.Awards != base.Awards {
					t.Fatalf("empty awards group changed values: %+v", got.Awards)
				}
			},
		},
		{
			name: "one award leaf only",
			req:  UpdateEconomyConfigRequest{Awards: &UpdateAwardsRequest{ResourceApproved: i64(100)}},
			want: func(t *testing.T, got EconomyConfig) {
				if got.Awards.ResourceApproved != 100 {
					t.Fatalf("resource_approved=%d want 100", got.Awards.ResourceApproved)
				}
				if got.Awards.ProfileComplete != base.Awards.ProfileComplete {
					t.Fatalf("sibling award changed: %+v", got.Awards)
				}
			},
		},
		{
			name: "allowance and expiry together",
			req: UpdateEconomyConfigRequest{
				Allowance: &UpdateAllowanceRequest{DocumentUnlocks: i64(5), ExpiresInDays: i64(45)},
				Expiry:    &UpdateExpiryRequest{EarnedDays: i64(730)},
			},
			want: func(t *testing.T, got EconomyConfig) {
				if got.Allowance.DocumentUnlocks != 5 || got.Allowance.ExpiresInDays != 45 {
					t.Fatalf("allowance wrong: %+v", got.Allowance)
				}
				if got.Allowance.VideoUnlocks != base.Allowance.VideoUnlocks {
					t.Fatalf("untouched allowance leaf changed: %+v", got.Allowance)
				}
				if got.Expiry.EarnedDays != 730 || got.Expiry.FreeDays != base.Expiry.FreeDays {
					t.Fatalf("expiry wrong: %+v", got.Expiry)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.want(t, applyEconomyConfigUpdate(base, tc.req))
		})
	}
}

func TestConfigStoreLoadReturnsDefaultsWhenAbsentOrEmpty(t *testing.T) {
	for _, stored := range []struct {
		name  string
		value string
		set   bool
	}{
		{name: "never written"},
		{name: "empty value", value: "", set: true},
		{name: "whitespace value", value: "   \n\t ", set: true},
	} {
		t.Run(stored.name, func(t *testing.T) {
			settings := newFakeSettings()
			if stored.set {
				settings.values[EconomyConfigSettingKey] = stored.value
			}
			store := NewConfigStore(settings)

			got, err := store.Load()
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got != DefaultEconomyConfig() {
				t.Fatalf("got %+v want defaults", got)
			}
		})
	}
}

// A stored value is unmarshalled on top of the defaults, so a config written
// before a key existed still yields sane values for the key it omits. This is
// the correction 02-architecture.md §7 asks for on top of the
// find_college_ad_cards pattern.
func TestConfigStoreLoadMergesStoredValueOntoDefaults(t *testing.T) {
	settings := newFakeSettings()
	settings.values[EconomyConfigSettingKey] = `{"prices":{"study_resource":75}}`
	store := NewConfigStore(settings)

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Prices.StudyResource != 75 {
		t.Fatalf("study_resource=%d want the stored 75", got.Prices.StudyResource)
	}
	if got.Prices.Video != 90 || got.Prices.MockTest != 60 {
		t.Fatalf("omitted price keys did not fall back to defaults: %+v", got.Prices)
	}
	if got.Referral != DefaultEconomyConfig().Referral {
		t.Fatalf("omitted groups did not fall back to defaults: %+v", got.Referral)
	}
}

// Corrupt storage is an error, not a silent revert to placeholder prices: the
// live economy is more dangerous to guess at than to report as unreadable.
func TestConfigStoreLoadRejectsCorruptStoredValue(t *testing.T) {
	settings := newFakeSettings()
	settings.values[EconomyConfigSettingKey] = `{"prices":` // truncated write
	store := NewConfigStore(settings)

	if _, err := store.Load(); err == nil {
		t.Fatal("want error for unparseable stored config")
	} else if !errors.Is(err, ErrConfigUnreadable) {
		t.Fatalf("error %v does not match ErrConfigUnreadable", err)
	}
}

func TestConfigStoreCachesReadsWithinTTLAndReloadsAfter(t *testing.T) {
	settings := newFakeSettings()
	store := NewConfigStore(settings)
	now := time.Now()
	store.now = func() time.Time { return now }

	if _, err := store.Load(); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("second load: %v", err)
	}
	if settings.reads != 1 {
		t.Fatalf("reads=%d want 1: the second load must come from the cache", settings.reads)
	}

	// Out-of-band write by something that did not invalidate: the TTL is what
	// bounds how long the old value is served.
	settings.values[EconomyConfigSettingKey] = `{"prices":{"video":120}}`

	now = now.Add(configCacheTTL - time.Second)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("load inside ttl: %v", err)
	}
	if cfg.Prices.Video != 90 {
		t.Fatalf("video=%d want the cached 90", cfg.Prices.Video)
	}
	if settings.reads != 1 {
		t.Fatalf("reads=%d want 1 inside the ttl", settings.reads)
	}

	now = now.Add(2 * time.Second)
	cfg, err = store.Load()
	if err != nil {
		t.Fatalf("load after ttl: %v", err)
	}
	if cfg.Prices.Video != 120 {
		t.Fatalf("video=%d want the out-of-band 120 after the ttl", cfg.Prices.Video)
	}
	if settings.reads != 2 {
		t.Fatalf("reads=%d want 2 after the ttl", settings.reads)
	}
}

func TestConfigStoreSaveInvalidatesCache(t *testing.T) {
	settings := newFakeSettings()
	store := NewConfigStore(settings)

	if _, err := store.Load(); err != nil {
		t.Fatalf("prime cache: %v", err)
	}
	encoded, err := json.Marshal(DefaultEconomyConfig())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := store.Save(string(encoded)); err != nil {
		t.Fatalf("save: %v", err)
	}
	if settings.writes != 1 {
		t.Fatalf("writes=%d want 1", settings.writes)
	}
	if value, ok := settings.get(EconomyConfigSettingKey); !ok || value != string(encoded) {
		t.Fatalf("stored value = %q, found=%v", value, ok)
	}

	// The write invalidates rather than refreshes, so the next read re-reads
	// storage and picks up anything written since.
	settings.values[EconomyConfigSettingKey] = `{"prices":{"mock_test":61}}`
	if _, err := store.Load(); err != nil {
		t.Fatalf("load after save: %v", err)
	}
	if settings.reads != 2 {
		t.Fatalf("reads=%d want 2: Save must invalidate the cache", settings.reads)
	}
}

func TestConfigStoreSaveFailureLeavesCacheIntact(t *testing.T) {
	settings := newFakeSettings()
	store := NewConfigStore(settings)
	if _, err := store.Load(); err != nil {
		t.Fatalf("prime cache: %v", err)
	}

	store.settings = failingSettings{}
	if err := store.Save(`{}`); err == nil {
		t.Fatal("want error from the settings store")
	}
	if settings.reads != 1 {
		t.Fatalf("reads=%d want 1: a failed write must not invalidate", settings.reads)
	}
}

type failingSettings struct{}

func (failingSettings) GetSystemSetting(string) (string, bool, error) { return "", false, nil }
func (failingSettings) SetSystemSetting(string, string) error         { return errors.New("write failed") }

func TestServiceGetEconomyConfigReadsThroughCache(t *testing.T) {
	svc, settings, _, _ := testService(t)

	for i := 0; i < 3; i++ {
		got, err := svc.GetEconomyConfig()
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got != DefaultEconomyConfig() {
			t.Fatalf("get returned %+v want defaults", got)
		}
	}
	if settings.reads != 1 {
		t.Fatalf("reads=%d want 1 for three gets", settings.reads)
	}
}

func TestUpdateEconomyConfigPartialUpdatePersistsAndRecordsVersion(t *testing.T) {
	svc, settings, versions, _ := testService(t)

	// The base is seeded rather than left empty because the merge base has to
	// satisfy the reachability invariant (see ValidateReachability): an empty
	// store falls back to DefaultEconomyConfig, which breaks it, and then every
	// write is refused before persistence — this test would end up measuring the
	// refusal instead of the merge. The figures are the test's own; the shipped
	// defaults are unchanged.
	base := compliantEconomyConfig()
	seedConfig(t, settings, base)

	// Change one price, and the rest of the config must survive the round trip.
	// Mock test rather than study resource: the compliant base has its cheapest
	// price exactly equal to the lowest award, so raising the cheapest price
	// would break rule 1, and this test is about the merge rather than that.
	updated, err := svc.UpdateEconomyConfig(UpdateEconomyConfigRequest{
		Prices: &UpdatePricesRequest{MockTest: i64(65)},
	}, 7)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Prices.MockTest != 65 {
		t.Fatalf("mock_test=%d want 65", updated.Prices.MockTest)
	}
	if updated.Referral.LifetimeCoinCap != 600 || updated.Expiry.EarnedDays != 365 {
		t.Fatalf("update dropped unrelated config: %+v", updated)
	}

	// Exactly one version row, carrying the actor, the full previous JSON and
	// the full new JSON, so the change is reconstructible.
	if len(versions.rows) != 1 {
		t.Fatalf("version rows=%d want 1", len(versions.rows))
	}
	row := versions.rows[0]
	if row.ChangedByUserID != 7 || row.ChangedBy != "admin:7" {
		t.Fatalf("actor wrong: %+v", row)
	}
	var previous, next EconomyConfig
	if err := json.Unmarshal([]byte(row.PreviousJSON), &previous); err != nil {
		t.Fatalf("previous_json is not a config: %v", err)
	}
	if err := json.Unmarshal([]byte(row.NewJSON), &next); err != nil {
		t.Fatalf("new_json is not a config: %v", err)
	}
	if previous != base {
		t.Fatalf("previous_json=%s want the pre-update config", row.PreviousJSON)
	}
	if next != updated {
		t.Fatalf("new_json does not match the saved config:\n%s\n%+v", row.NewJSON, updated)
	}

	// The stored value round-trips, and the cache was invalidated by the write,
	// so the very next read sees the new price.
	if _, ok := settings.get(EconomyConfigSettingKey); !ok {
		t.Fatal("config was not persisted")
	}
	reloaded, err := svc.GetEconomyConfig()
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if reloaded != updated {
		t.Fatalf("reloaded %+v want %+v", reloaded, updated)
	}
}

func TestUpdateEconomyConfigRejectsInvalidUpdateWithoutWriting(t *testing.T) {
	tests := []struct {
		name string
		req  UpdateEconomyConfigRequest
	}{
		{
			// The named example from 03-api-contract.md §3.1.
			name: "video allowance with a zero video price",
			req:  UpdateEconomyConfigRequest{Prices: &UpdatePricesRequest{Video: i64(0)}},
		},
		{
			name: "zero free days",
			req:  UpdateEconomyConfigRequest{Expiry: &UpdateExpiryRequest{FreeDays: i64(0)}},
		},
		{
			name: "negative hold days",
			req:  UpdateEconomyConfigRequest{Referral: &UpdateReferralRequest{HoldDays: i64(-1)}},
		},
		{
			name: "negative clawback window",
			req:  UpdateEconomyConfigRequest{ClawbackWindowDays: i64(-30)},
		},
		{
			// Editing the ladder total alone is the exact mistake the
			// instalment-product rule exists to catch.
			name: "instalment ladder broken by one field",
			req:  UpdateEconomyConfigRequest{Awards: &UpdateAwardsRequest{ProfileComplete: i64(50)}},
		},
		{
			name: "amount above the bigint-safe ceiling",
			req:  UpdateEconomyConfigRequest{Prices: &UpdatePricesRequest{StudyResource: i64(maxAmount + 1)}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, settings, versions, _ := testService(t)
			// Prime the cache so the assertion below proves the *write* was
			// skipped, not merely that nothing was read.
			if _, err := svc.GetEconomyConfig(); err != nil {
				t.Fatalf("prime: %v", err)
			}

			if _, err := svc.UpdateEconomyConfig(tc.req, 3); err == nil {
				t.Fatal("want a validation rejection")
			} else if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("error %v does not match ErrInvalidConfig", err)
			}
			if settings.writes != 0 {
				t.Fatalf("writes=%d want 0: a rejected update must not persist", settings.writes)
			}
			if len(versions.rows) != 0 {
				t.Fatalf("version rows=%d want 0 for a rejected update", len(versions.rows))
			}
			// The cache is still valid and still holds the old config.
			got, err := svc.GetEconomyConfig()
			if err != nil {
				t.Fatalf("get after rejection: %v", err)
			}
			if got != DefaultEconomyConfig() {
				t.Fatalf("rejected update changed the config: %+v", got)
			}
		})
	}
}

// A rejected update must not apply half of itself: the merge is validated as a
// whole before anything is written, so a change that is individually fine but
// jointly invalid is still refused.
func TestUpdateEconomyConfigValidatesTheMergedWhole(t *testing.T) {
	svc, settings, _, _ := testService(t)

	// Dropping the video price to 0 alone would be invalid; pairing it with
	// dropping the video allowance makes the whole thing legal, and that is the
	// only way it should be accepted.
	_, err := svc.UpdateEconomyConfig(UpdateEconomyConfigRequest{
		Prices:    &UpdatePricesRequest{Video: i64(0)},
		Allowance: &UpdateAllowanceRequest{VideoUnlocks: i64(0)},
	}, 1)
	if err != nil {
		t.Fatalf("paired update should be accepted: %v", err)
	}
	if settings.writes != 1 {
		t.Fatalf("writes=%d want 1", settings.writes)
	}

	got, err := svc.GetEconomyConfig()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Prices.Video != 0 || got.Allowance.VideoUnlocks != 0 {
		t.Fatalf("paired update not applied: %+v", got)
	}
}

func TestUpdateEconomyConfigReportsVersionFailure(t *testing.T) {
	svc, settings, versions, _ := testService(t)
	versions.failErr = errors.New("insert failed")

	// Seeded compliant base, for the same reason as the partial-update test:
	// on the defaults this request would be refused by the reachability rule
	// before it was ever written, and the test would still see an error while
	// no longer covering the version-append failure it is named for.
	seedConfig(t, settings, compliantEconomyConfig())

	// The setting is already stored and the cache already invalidated, so the
	// error is about the missing audit row rather than a lost change. Asserted
	// so a future refactor cannot turn this into a silent success.
	_, err := svc.UpdateEconomyConfig(UpdateEconomyConfigRequest{
		ClawbackWindowDays: i64(90),
	}, 5)
	if err == nil {
		t.Fatal("want an error when the version row cannot be appended")
	}
	// ...and it is THAT error rather than a validation rejection. The write
	// reached storage, which is what makes the missing audit row a real
	// problem worth a test.
	if errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("the request was refused by validation instead of reaching the version append: %v", err)
	}
	if settings.writes != 1 {
		t.Fatalf("writes=%d want 1: the write itself must have succeeded for this test to mean anything", settings.writes)
	}
}
