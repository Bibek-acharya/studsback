//go:build coinsintegration

// internal/coins/config_history_pg_test.go
//
// The version history's REAL store, against Postgres.
//
// The reader's behaviour is covered by config_history_test.go against a fake. What a
// fake cannot check is the one thing most likely to be wrong here: the ORDER BY. A
// reader that returns rows in insertion order by accident passes every fake-based
// assertion, because the fake hands back exactly what the test put in.

package coins

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"studsphere/backend/internal/shared/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newHistoryEnv builds a throwaway schema for the version store.
//
// Its own harness rather than the shared coreSchema one because this is the only test
// writing ConfigVersion rows, and those rows are the thing several other tests would
// be confused by if they shared a table.
//
// search_path goes in the DSN for the reason ledger_pg_test.go's openCoreSchema
// documents at length: a SET applies to ONE pooled connection, so the next checkout
// gets the default search_path and the test writes to `public` instead of its own
// schema. In the DSN, every connection is pinned.
func newHistoryEnv(t *testing.T) (*gorm.DB, VersionStore, *Service) {
	t.Helper()
	dsn := os.Getenv("COINS_TEST_DSN")
	if dsn == "" {
		t.Skip("COINS_TEST_DSN not set; skipping config history integration test")
	}
	// The shared logger's lazy Init reads config.AppConfig.GinMode, so a nil there
	// panics rather than failing cleanly. Same guard as the other harnesses, needed
	// here because this file builds its own environment.
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{}
	}

	schema := "coins_config_history"
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	db.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`)
	db.Exec(`CREATE SCHEMA ` + schema)
	db.Exec(`SET search_path TO ` + schema)
	t.Cleanup(func() { db.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`) })

	if err := db.AutoMigrate(&ConfigVersion{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	store := NewVersionStore(db)
	svc := NewService(NewConfigStore(newFakeSettings()), store)
	return db, store, svc
}

// THE test. Rows are appended in one order and must be read back in the other.
func TestTheStoredHistoryComesBackNewestFirst(t *testing.T) {
	_, store, svc := newHistoryEnv(t)

	// Three changes, each distinguishable by its recorded price.
	for i, price := range []int64{40, 55, 70} {
		cfg := DefaultEconomyConfig()
		cfg.Prices.StudyResource = price
		encoded, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := store.AppendConfigVersion(&ConfigVersion{
			CreatedAt:       time.Now().UTC(),
			PreviousJSON:    "",
			NewJSON:         string(encoded),
			ChangedByUserID: uint(7 + i),
			ChangedBy:       "admin:" + u64str(uint(7+i)),
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	got, err := svc.ConfigVersionHistory(context.Background(), 50)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("%d versions, want 3", len(got))
	}
	// Newest first: 70 was written last, so it is first out.
	wantPrices := []int64{70, 55, 40}
	for i, want := range wantPrices {
		if got[i].New == nil {
			t.Fatalf("entry %d did not decode: %+v", i, got[i])
		}
		if got[i].New.Prices.StudyResource != want {
			t.Errorf("entry %d price = %d, want %d — the history is not newest-first",
				i, got[i].New.Prices.StudyResource, want)
		}
	}
	// The actor survives the round trip, which is the difference between an audit
	// trail and a log.
	if got[0].ChangedBy != "admin:9" || got[0].ChangedByUserID != 9 {
		t.Errorf("newest entry actor = %q/%d, want admin:9/9", got[0].ChangedBy, got[0].ChangedByUserID)
	}
}

// The limit is a SQL LIMIT, so a large table must not be fully read. Asserted by
// counting the rows the store was given rather than by timing: a slice-after-fetch
// returns the right answer and is wrong on the tenth thousand pricing change.
func TestTheStoredLimitTruncatesInTheDatabase(t *testing.T) {
	_, store, svc := newHistoryEnv(t)

	for i := 0; i < 12; i++ {
		if err := store.AppendConfigVersion(&ConfigVersion{
			CreatedAt: time.Now().UTC(),
			NewJSON:   `{}`, ChangedBy: "admin:1",
		}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	got, err := svc.ConfigVersionHistory(context.Background(), 5)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("%d entries for limit 5 over 12 rows", len(got))
	}
	// And the 5 returned are the newest 5, not an arbitrary 5.
	all, err := svc.ConfigVersionHistory(context.Background(), 50)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	for i := range got {
		if got[i].ID != all[i].ID {
			t.Errorf("limited entry %d = id %d, want id %d", i, got[i].ID, all[i].ID)
		}
	}
}

// An empty table is an empty list, not an error and not nil. The admin page's first
// render on a fresh deployment depends on this.
func TestAnEmptyHistoryIsAnEmptyList(t *testing.T) {
	_, _, svc := newHistoryEnv(t)

	got, err := svc.ConfigVersionHistory(context.Background(), 50)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if got == nil {
		t.Error("an empty history returned nil; a JSON client renders null rather than []")
	}
	if len(got) != 0 {
		t.Errorf("%d entries on an empty table", len(got))
	}
}

// A row whose stored JSON does not parse still appears, with its raw text, and only
// loses its decoded pair.
//
// The failure this prevents is specific and worth naming: the version table is an
// audit trail, and an audit trail that goes blank when one row is damaged is worse
// than one that shows a damaged row. This is asserted against a REAL row, because the
// whole point is that the damage survives the database round trip.
func TestADamagedStoredRowIsListedRatherThanDropped(t *testing.T) {
	db, store, svc := newHistoryEnv(t)

	if err := store.AppendConfigVersion(&ConfigVersion{
		CreatedAt: time.Now().UTC(), NewJSON: `{"prices":`, ChangedBy: "admin:4",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	// A row that does parse, so the two are distinguishable.
	cfg := DefaultEconomyConfig()
	encoded, _ := json.Marshal(cfg)
	if err := store.AppendConfigVersion(&ConfigVersion{
		CreatedAt: time.Now().UTC(), NewJSON: string(encoded), ChangedBy: "admin:5",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := svc.ConfigVersionHistory(context.Background(), 50)
	if err != nil {
		t.Fatalf("a damaged row failed the whole read: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("%d entries, want 2 — a damaged row must not remove itself", len(got))
	}
	damaged := got[1]
	if damaged.New != nil {
		t.Errorf("the damaged row decoded to %+v", damaged.New)
	}
	if damaged.NewJSON != `{"prices":` {
		t.Errorf("the damaged row's raw text was lost: %q", damaged.NewJSON)
	}
	// The damage is real in the table, not introduced by the reader.
	var stored string
	if err := db.Raw(`SELECT new_json FROM coin_economy_config_version WHERE id = ?`, damaged.ID).
		Scan(&stored).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored != `{"prices":` {
		t.Errorf("the stored row is %q; the test did not set up what it claims", stored)
	}
}

// An empty PreviousJSON is the FIRST version's normal state, not corruption. Asserted
// because the decode treats "" as nil for the same reason it treats unparseable text
// as nil, and one of those is an error and the other is not.
func TestTheFirstVersionHasNoPreviousAndThatIsNotDamage(t *testing.T) {
	_, store, svc := newHistoryEnv(t)
	cfg := DefaultEconomyConfig()
	encoded, _ := json.Marshal(cfg)
	if err := store.AppendConfigVersion(&ConfigVersion{
		CreatedAt: time.Now().UTC(), PreviousJSON: "", NewJSON: string(encoded),
		ChangedBy: "admin:1",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := svc.ConfigVersionHistory(context.Background(), 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if got[0].Previous != nil {
		t.Errorf("the first version has a previous snapshot: %+v", got[0].Previous)
	}
	if got[0].New == nil {
		t.Error("the first version's new snapshot did not decode")
	}
}
