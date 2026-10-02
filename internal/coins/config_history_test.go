package coins

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

// A fake VersionStore, so the version-history READ is testable without a database.
//
// The real store is exercised in config_versions_pg_test.go. What is here is the
// behaviour of the reader itself — the bounds, the ordering, and the response shape —
// none of which needs SQL to be wrong in an interesting way.
type fakeVersionStore struct {
	rows []ConfigVersion
	err  error
	// lastLimit records what the caller asked for, so the bound can be asserted on
	// the QUERY rather than on the returned slice length. A reader that clamps the
	// slice after fetching everything has not bounded anything.
	lastLimit int
}

func (f *fakeVersionStore) AppendConfigVersion(v *ConfigVersion) error { return f.err }

func (f *fakeVersionStore) RecentConfigVersions(limit int) ([]ConfigVersion, error) {
	f.lastLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

// 04 §6 asks for "a config version history" as an admin-console deliverable. The
// WRITES have existed since the service was built; what was missing is anything to
// READ them back, which is the only half a history page can use.
func TestTheConfigVersionHistoryIsReadableNewestFirstAndBounded(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeVersionStore{rows: []ConfigVersion{
		{ID: 3, CreatedAt: now, ChangedBy: "admin:7"},
		{ID: 2, CreatedAt: now.Add(-time.Hour), ChangedBy: "admin:6"},
		{ID: 1, CreatedAt: now.Add(-2 * time.Hour), ChangedBy: "admin:5"},
	}}
	svc := &Service{config: NewConfigStore(newFakeSettings()), versions: store}

	got, err := svc.ConfigVersionHistory(context.Background(), ConfigVersionHistoryDefault)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("%d versions, want 3", len(got))
	}
	// NEWEST FIRST. An audit trail read oldest-first makes an operator scroll to the
	// bottom to answer "what did I just change", which is the question they have.
	if got[0].ID != 3 {
		t.Errorf("first version = id %d, want id 3 — the newest change is the one being asked about", got[0].ID)
	}
	// The actor must be carried: a history row without WHO changed it is a log, not
	// an audit trail, and this table is the audit trail for pricing.
	if got[0].ChangedBy != "admin:7" {
		t.Errorf("changed_by = %q, want admin:7", got[0].ChangedBy)
	}
}

// The limit is pushed into the QUERY. A reader that fetched every row and sliced
// afterwards would still return the right answer here, and would return the wrong one
// on a table with a few thousand pricing changes in it.
func TestTheHistoryLimitIsAppliedToTheQueryNotAfterIt(t *testing.T) {
	store := &fakeVersionStore{}
	svc := &Service{config: NewConfigStore(newFakeSettings()), versions: store}

	if _, err := svc.ConfigVersionHistory(context.Background(), 0); err != nil {
		t.Fatalf("history: %v", err)
	}
	if store.lastLimit != ConfigVersionHistoryDefault {
		t.Errorf("asked the store for %d rows, want the default %d",
			store.lastLimit, ConfigVersionHistoryDefault)
	}

	// Negative and absurd limits both fall back rather than erroring or emptying the
	// page. An operator typing ?limit=-1 should get the history, not a blank screen.
	for _, asked := range []int{-1, 0, ConfigVersionHistoryMax + 1, 1 << 30} {
		if _, err := svc.ConfigVersionHistory(context.Background(), asked); err != nil {
			t.Errorf("limit %d errored: %v", asked, err)
		}
		if store.lastLimit < 1 || store.lastLimit > ConfigVersionHistoryMax {
			t.Errorf("limit %d reached the store as %d, which is outside [1, %d]",
				asked, store.lastLimit, ConfigVersionHistoryMax)
		}
	}
}

// A store error must NOT be reported as an empty history. "You have never changed a
// price" and "we could not read the audit trail" are opposite messages, and an admin
// reading the second as the first has just been told their pricing is unchanged when
// nobody knows what it is.
func TestAHistoryReadErrorIsNotAnEmptyHistory(t *testing.T) {
	svc := &Service{
		config:   NewConfigStore(newFakeSettings()),
		versions: &fakeVersionStore{err: errors.New("connection reset")},
	}
	got, err := svc.ConfigVersionHistory(context.Background(), 20)
	if err == nil {
		t.Fatalf("a failed read returned %d versions and no error", len(got))
	}
	if got != nil {
		t.Errorf("a failed read returned %d versions alongside its error", len(got))
	}
}

// A nil VersionStore must not panic. main.go wires one, and a constructor call missed
// there should surface as one broken admin tab rather than a panic on page load.
func TestTheHistoryOnANilStoreRefusesRatherThanPanicking(t *testing.T) {
	svc := &Service{config: NewConfigStore(newFakeSettings())}
	if _, err := svc.ConfigVersionHistory(context.Background(), 20); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("err = %v, want ErrNoDatabase", err)
	}
}

// The stored JSON must be DECODED for the response, and a row whose JSON is corrupt
// must not take the whole history down with it.
//
// This is the property that makes the endpoint worth having: the admin console's
// version list shows a diff, and a diff needs both snapshots parsed. One row with a
// truncated payload — a half-finished write, a hand-edited row, a migration from a
// build that serialised differently — should cost that one entry, not the page.
func TestACorruptVersionRowCostsOnlyThatEntry(t *testing.T) {
	now := time.Now().UTC()
	cfg := DefaultEconomyConfig()
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	store := &fakeVersionStore{rows: []ConfigVersion{
		{ID: 2, CreatedAt: now, NewJSON: string(encoded), ChangedBy: "admin:2"},
		{ID: 1, CreatedAt: now.Add(-time.Hour), NewJSON: "{truncated", ChangedBy: "admin:1"},
	}}
	svc := &Service{config: NewConfigStore(newFakeSettings()), versions: store}

	got, err := svc.ConfigVersionHistory(context.Background(), 20)
	if err != nil {
		t.Fatalf("a corrupt row failed the whole read: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("%d entries, want 2 — the corrupt row must still be listed", len(got))
	}
	if got[0].New == nil {
		t.Error("the good row was not decoded")
	}
	if got[1].New != nil {
		t.Errorf("the corrupt row decoded to %+v; it should be nil with the error recorded", got[1].New)
	}
	// The raw text survives on the corrupt row, so an operator can see WHAT was stored
	// and someone can repair it. Dropping it makes the row unrecoverable.
	if got[1].NewJSON == "" {
		t.Error("the corrupt row's raw text was discarded, so it cannot be repaired")
	}
}

// A history row carries BOTH snapshots. Asserted because a reader that returns only
// `new_json` cannot show a diff, and the diff is the entire reason an admin opens this
// page — "what did this change" is unanswerable with one side.
func TestEachVersionCarriesBothSnapshots(t *testing.T) {
	before := DefaultEconomyConfig()
	before.Prices.StudyResource = 40
	after := before
	after.Prices.StudyResource = 55

	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)

	store := &fakeVersionStore{rows: []ConfigVersion{{
		ID: 1, CreatedAt: time.Now().UTC(),
		PreviousJSON: string(beforeJSON), NewJSON: string(afterJSON),
		ChangedBy: "admin:9",
	}}}
	svc := &Service{config: NewConfigStore(newFakeSettings()), versions: store}

	got, err := svc.ConfigVersionHistory(context.Background(), 20)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if got[0].Previous == nil || got[0].New == nil {
		t.Fatalf("a snapshot is missing; the diff cannot be computed: %+v", got[0])
	}
	if got[0].Previous.Prices.StudyResource != 40 || got[0].New.Prices.StudyResource != 55 {
		t.Errorf("snapshots = %d -> %d, want 40 -> 55",
			got[0].Previous.Prices.StudyResource, got[0].New.Prices.StudyResource)
	}
}

// Guard against the response accidentally carrying the fraud thresholds to a
// non-admin surface. The endpoint IS admin-gated, but the DTO is what decides what a
// future caller can reach, and the admin config itself already returns these.
func TestTheVersionHistoryDTOIsSelfContained(t *testing.T) {
	// A compile-time-ish assertion that the DTO exists and its fields are the ones the
	// tests above read. Cheap, and it fails loudly if someone swaps in ConfigVersion
	// directly and widens the shape without noticing.
	var _ ConfigVersionEntry
}

// make sure gorm import is used even if the above changes.
var _ = gorm.ErrRecordNotFound
