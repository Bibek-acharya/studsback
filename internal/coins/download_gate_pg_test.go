//go:build coinsintegration

// internal/coins/download_gate_pg_test.go
//
// The download gate against a real PostgreSQL instance, through the real HTTP
// route.
//
// Everything in download_gate_test.go is about order and shape, and neither can
// be settled by a mock of the ledger. These claims are about what the DATABASE
// did:
//
//   - that a paid download really spends the configured price and really writes
//     the entitlement, in one transaction. A mocked Spend proves nothing about
//     whether the debit and the file survive or die together.
//   - that N concurrent downloads of the SAME unpurchased resource by the SAME
//     student produce exactly one charge and one unlock, and that all N are
//     served. This is the claim the reuse of coinPurchase exists for, and only a
//     real unique index, a real per-user advisory lock and N real connections can
//     produce the race that would break it.
//   - that a repeat download of an owned resource writes nothing at all.
//   - that a refusal writes nothing: no unlock, no journal, no posting, an
//     unchanged balance, and no download counted.
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// ── safety ───────────────────────────────────────────────────────────────────
//
// This file creates and drops its own schema, coins_gate_test, and nothing is
// ever created in the public schema. Every test skips when COINS_TEST_DSN is
// unset, so `go test ./...` needs no database. The search_path goes in the DSN
// rather than in a SET, because a SET applies to ONE pooled connection and the
// concurrency test would then have some of its goroutines writing to the public
// schema — the same trap unlock_api_pg_test.go documents for its own schema.
package coins

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/studyresources"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// gateSchema is the one schema this file owns.
const gateSchema = "coins_gate_test"

// openGateSchema creates coins_gate_test with the ledger, the entitlement, the
// notification and the study-resource models, plus every constraint
// AutoMigrate cannot build.
//
// study_resources is migrated in because the gate's ResourceLookup reads the real
// table: the 404 and the receipt title are claims about what that read does, and
// a fake of it would only prove the fake.
func openGateSchema(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("COINS_TEST_DSN")
	if dsn == "" {
		t.Skip("COINS_TEST_DSN not set; skipping StudsToken download gate integration test")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if sqlDB, err := admin.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := admin.Exec(`DROP SCHEMA IF EXISTS ` + gateSchema + ` CASCADE`).Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := admin.Exec(`CREATE SCHEMA ` + gateSchema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(`DROP SCHEMA IF EXISTS ` + gateSchema + ` CASCADE`).Error
	})

	pool, err := gorm.Open(postgres.Open(dsn+" search_path="+gateSchema), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres pinned to %s: %v", gateSchema, err)
	}
	sqlPool, err := pool.DB()
	if err != nil {
		t.Fatalf("sql handle: %v", err)
	}
	// A connection is held for the whole purchase transaction, so a pool that is
	// too small silently serialises the concurrency tests and they stop testing
	// anything. 24 covers the widest fan-out here with headroom.
	sqlPool.SetMaxOpenConns(24)
	sqlPool.SetMaxIdleConns(2)
	sqlPool.SetConnMaxLifetime(2 * time.Minute)
	t.Cleanup(func() { _ = sqlPool.Close() })

	models := append(append([]any{}, LedgerModels...), EntitlementModels...)
	models = append(models,
		notification.AccountNotification{}, notification.NotificationOutbox{},
		notification.NotificationBroadcast{}, notification.NotificationDedupeLease{},
		notification.NotificationPreference{}, notification.NotificationDelivery{})
	if err := pool.AutoMigrate(models...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	// study_resources is a plain GORM model, so AutoMigrate is all it needs. It is
	// last in the list so a failure in the money tables is not reported as a
	// studyresources problem.
	if err := pool.AutoMigrate(&studyresources.StudyResource{}); err != nil {
		t.Fatalf("automigrate study_resources: %v", err)
	}
	if err := EnsurePostgresIndexes(pool); err != nil {
		t.Fatalf("ensure ledger indexes: %v", err)
	}
	if err := EnsureEntitlementIndexes(pool); err != nil {
		t.Fatalf("ensure entitlement indexes: %v", err)
	}
	// NewUnlockAPI wires the real notification service by default, so every
	// purchase in this file emits coins.debited inside its transaction. Without
	// the tables the emission would fail and — correctly — take the purchase down
	// with it, which would make every test here a test of a missing table.
	if err := notification.EnsurePostgresIndexes(pool); err != nil {
		t.Fatalf("ensure notification indexes: %v", err)
	}
	indexes, err := os.ReadFile("../../migrations/20260903-02-notification-indexes.sql")
	if err != nil {
		t.Fatalf("read the notification index migration: %v", err)
	}
	if err := pool.Exec(string(indexes)).Error; err != nil {
		t.Fatalf("apply the notification index migration: %v", err)
	}
	return pool
}

// ── the fixture ──────────────────────────────────────────────────────────────

// gateEnv is the whole path wired the way main.go wires it: one config store,
// one repository, one service, the resource lookup reading the real
// study-resources table, and the gate attached to the real download route.
type gateEnv struct {
	api     *UnlockAPI
	repo    *Repository
	service *Service
	ledger  *Ledger
	db      *gorm.DB

	// routers is keyed by user so two students can download the same resource
	// concurrently without sharing an identity. Building one router per request
	// would also work and costs nothing, but a per-user router makes a mistaken
	// cross-user assertion impossible to write.
	routers map[uint]*gin.Engine
}

// newGateEnv wires the document gate ON, which is the point of this file: the
// dark case is a config default and is asserted in download_gate_test.go. The
// unlock endpoint's own flag is left OFF throughout, deliberately — the gate is
// what makes a purchase meaningful, and turning the document path live needs BOTH
// flags set. If these tests only passed with both on, that coupling would be
// invisible.
func newGateEnv(t *testing.T, db *gorm.DB, mutate func(*EconomyConfig)) *gateEnv {
	t.Helper()
	cfg := DefaultEconomyConfig()
	cfg.Gates.StudyResource = true
	if mutate != nil {
		mutate(&cfg)
	}
	if err := ValidateEconomyConfig(cfg); err != nil {
		t.Fatalf("the test config does not validate: %v", err)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal the test config: %v", err)
	}
	settings := newFakeSettings()
	settings.values[EconomyConfigSettingKey] = string(encoded)

	store := NewConfigStore(settings)
	repo := NewRepository(db)
	service := NewServiceWithRepository(repo, store, &fakeVersions{})
	ledger := NewLedger(repo, store)
	resources := studyresources.NewService(studyresources.NewRepository(db))

	api := NewUnlockAPI(service, ledger).WithResourceLookup(NewStudyResourceLookup(resources))
	handler := studyresources.NewHandler(studyresources.NewService(studyresources.NewRepository(db))).
		WithDownloadGate(NewDownloadGate(api))

	env := &gateEnv{api: api, repo: repo, service: service, ledger: ledger, db: db, routers: map[uint]*gin.Engine{}}
	gin.SetMode(gin.TestMode)
	for _, userID := range []uint{950, 951, 952, 953, 954, 955, 956, 957, 958, 959, 960, 961, 962, 963, 964} {
		router := gin.New()
		// A stand-in for authMW, which is what the route really sits behind. It
		// is the only thing that answers 401, so the gate never sees an anonymous
		// request — the same arrangement as production, and the reason a 401 is
		// absent from the cases here.
		userID := userID
		router.GET("/api/v1/study-resources/:id/download", func(c *gin.Context) {
			c.Set("user_id", userID)
			handler.DownloadResource(c)
		})
		env.routers[userID] = router
	}
	return env
}

// seedDocument inserts one study resource and returns it.
//
// It goes through the module's own CreateResource rather than db.Create, and that
// is not a style choice. IsPublished carries a `default:true` tag, and GORM never
// writes a zero-valued field that has a database default — so a raw Create of a
// draft silently produces a PUBLISHED row, and every draft test in this file
// would be asserting against a published resource. CreateResource is where the
// module handles exactly that.
func (e *gateEnv) seedDocument(t *testing.T, title, storedType string, published bool) *studyresources.StudyResource {
	t.Helper()
	resource := &studyresources.StudyResource{
		Title: title, ResourceType: storedType,
		FileName: "physics.pdf", FilePath: "study-resources/physics.pdf",
		FileURL: "/uploads/study-resources/physics.pdf", FileSize: 10,
		MimeType: "application/pdf", IsPublished: published,
	}
	if err := studyresources.NewRepository(e.db).CreateResource(resource); err != nil {
		t.Fatalf("seed %q: %v", title, err)
	}
	return resource
}

// gateResponse is one download's outcome.
type gateResponse struct {
	Status    int
	ErrorCode string
	ErrorData *InsufficientCoinsData
	Raw       string
}

// served reports whether the request got past the gate and down the serve path.
//
// Object storage is a global MinIO client with no local mode, so the object read
// itself cannot run here. That is not a gap in what these tests claim: a served
// download is identified by NOT being a coin refusal AND by the resource's own
// download counter having moved, and that counter is incremented before the object
// read precisely so it is a reliable witness. The bytes are MinIO's business and
// are covered by the module's own storage tests.
func (r gateResponse) served() bool {
	switch r.Status {
	case http.StatusPaymentRequired, http.StatusLocked:
		return false
	default:
		return true
	}
}

// download performs one GET of the real route as userID.
func (e *gateEnv) download(t *testing.T, userID uint, resourceID uint) gateResponse {
	t.Helper()
	router, ok := e.routers[userID]
	if !ok {
		t.Fatalf("no router was wired for user %d", userID)
	}
	rec := httptest.NewRecorder()
	path := "/api/v1/study-resources/" + strconv.FormatUint(uint64(resourceID), 10) + "/download"
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	out := gateResponse{Status: rec.Code, Raw: rec.Body.String()}
	var envelope struct {
		Success bool `json:"success"`
		Error   struct {
			Code string                 `json:"code"`
			Data *InsufficientCoinsData `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err == nil {
		out.ErrorCode = envelope.Error.Code
		out.ErrorData = envelope.Error.Data
	}
	return out
}

// ── row helpers ─────────────────────────────────────────────────────────────

func gateSpendJournals(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	return apiCount(t, db,
		`SELECT count(*) FROM coin_journal j
		   WHERE j.scope = 'user' AND j.entry_type = 'SPEND'
		     AND EXISTS (SELECT 1 FROM coin_posting p
		                  JOIN coin_account a ON a.id = p.account_id
		                 WHERE p.journal_id = j.id AND a.owner_user_id = ? AND a.kind = 'USER')`, userID)
}

func gateUnlocksFor(t *testing.T, db *gorm.DB, userID uint, resourceID uint64) []ResourceUnlock {
	t.Helper()
	var rows []ResourceUnlock
	if err := db.Raw(
		`SELECT `+unlockColumns+` FROM resource_unlock
		  WHERE user_id = ? AND resource_type = ? AND resource_id = ? ORDER BY id`,
		userID, ResourceTypeStudyResource, resourceID,
	).Scan(&rows).Error; err != nil {
		t.Fatalf("read unlocks for user %d %d: %v", userID, resourceID, err)
	}
	return rows
}

func gateGrant(t *testing.T, e *gateEnv, userID uint, reason, key string) int64 {
	t.Helper()
	res, err := e.ledger.Grant(context.Background(), GrantRequest{
		UserID:         userID,
		ReasonCode:     reason,
		IdempotencyKey: key,
		CreatedBy:      "test",
	})
	if err != nil {
		t.Fatalf("grant %s to user %d: %v", reason, userID, err)
	}
	return res.Amount
}

// gateExhaustDocuments burns the whole document allowance, so a test that wants
// the COIN path is testing it. ConsumeAllowance creates the allowance when there
// is none, so without this a test named "a download spends coins" could be
// silently testing the allowance and would pass with the purchase path broken.
func gateExhaustDocuments(t *testing.T, e *gateEnv, userID uint, firstID uint64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := e.service.EnsureAllowance(context.Background(), userID, now); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}
	status, err := e.service.RemainingAllowance(context.Background(), userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	for i := int64(0); i < status.Remaining(ResourceTypeStudyResource); i++ {
		if _, err := e.service.ConsumeAllowance(context.Background(), userID,
			ResourceTypeStudyResource, firstID+uint64(i), now); err != nil {
			t.Fatalf("burning document allowance %d: %v", i+1, err)
		}
	}
}

func gateResourceDownloads(t *testing.T, db *gorm.DB, resourceID uint) int {
	t.Helper()
	var resource studyresources.StudyResource
	if err := db.First(&resource, resourceID).Error; err != nil {
		t.Fatalf("reload %d: %v", resourceID, err)
	}
	return resource.Downloads
}

// assertNothingWritten is the refusal invariant, in one place so every refusal
// test states it identically: a refusal moves nothing in the economy and is not
// counted as a download.
func assertNothingWritten(t *testing.T, e *gateEnv, userID uint, resourceID uint64, wantAvailable int64) {
	t.Helper()
	if rows := gateUnlocksFor(t, e.db, userID, resourceID); len(rows) != 0 {
		t.Errorf("a refusal wrote %d unlock rows", len(rows))
	}
	if n := gateSpendJournals(t, e.db, userID); n != 0 {
		t.Errorf("a refusal wrote %d spend journals", n)
	}
	if got := apiUserAvailable(t, e.db, userID); got != wantAvailable {
		t.Errorf("wallet = %d, want the unchanged %d", got, wantAvailable)
	}
	if got := gateResourceDownloads(t, e.db, uint(resourceID)); got != 0 {
		t.Errorf("a refusal counted %d downloads, want 0 — the student got no file", got)
	}
}

// ── the paid download ────────────────────────────────────────────────────────

// A paid download spends the configured price, writes the entitlement, and the
// two are joined by a journal id — one transaction, so a student is never charged
// for a file they did not get or given a file they did not pay for.
func TestGatedDownloadSpendsThePriceAndRecordsTheEntitlement(t *testing.T) {
	const userID = 950
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions, true)

	granted := gateGrant(t, e, userID, ReasonResourceApproved, "gate-happy-grant")
	if granted != 80 {
		t.Fatalf("the grant was %d, want the configured 80; the fixture is wrong", granted)
	}
	gateExhaustDocuments(t, e, userID, 9501)
	before := apiUserAvailable(t, db, userID)
	price := DefaultEconomyConfig().Prices.StudyResource

	res := e.download(t, userID, doc.ID)
	if !res.served() {
		t.Fatalf("a funded download was refused: %d %s", res.Status, res.Raw)
	}

	rows := gateUnlocksFor(t, db, userID, uint64(doc.ID))
	if len(rows) != 1 {
		t.Fatalf("unlock rows = %d, want exactly 1", len(rows))
	}
	if rows[0].CoinsPaid != price {
		t.Errorf("coins_paid = %d, want the configured %d", rows[0].CoinsPaid, price)
	}
	if rows[0].JournalID == nil || *rows[0].JournalID == "" {
		t.Error("the unlock is not joined to a journal: the debit is not attributable")
	}
	if rows[0].Source != UnlockSourceCoins {
		t.Errorf("source = %q, want %q", rows[0].Source, UnlockSourceCoins)
	}
	if after := apiUserAvailable(t, db, userID); after != before-price {
		t.Errorf("wallet = %d, want %d (one debit of %d)", after, before-price, price)
	}
	if n := gateSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want exactly 1", n)
	}
	if got := gateResourceDownloads(t, db, doc.ID); got != 1 {
		t.Errorf("downloads = %d, want 1 for a served download", got)
	}
}

// The first download charges; the second is free. A browser may retry a download
// and nothing tells the server about it, so "not charged again" is the property
// that makes a retry safe rather than expensive.
func TestSecondDownloadOfTheSameResourceIsFree(t *testing.T) {
	const userID = 951
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	doc := e.seedDocument(t, "Math Study Notes", studyresources.TypeStudyNotes, true)
	gateGrant(t, e, userID, ReasonResourceApproved, "gate-second-grant")
	gateExhaustDocuments(t, e, userID, 9511)
	before := apiUserAvailable(t, db, userID)
	price := DefaultEconomyConfig().Prices.StudyResource

	for attempt := 1; attempt <= 2; attempt++ {
		if res := e.download(t, userID, doc.ID); !res.served() {
			t.Fatalf("download %d was refused: %d %s", attempt, res.Status, res.Raw)
		}
	}

	if n := gateSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want exactly 1 across two downloads of one resource", n)
	}
	if rows := gateUnlocksFor(t, db, userID, uint64(doc.ID)); len(rows) != 1 {
		t.Errorf("unlock rows = %d, want 1", len(rows))
	}
	if after := apiUserAvailable(t, db, userID); after != before-price {
		t.Errorf("wallet = %d, want %d: a repeat download charged again", after, before-price)
	}
	// Both attempts were real downloads, so both counted. That is the difference
	// between "not charged" and "not served", and the counter is how the two are
	// told apart here.
	if got := gateResourceDownloads(t, db, doc.ID); got != 2 {
		t.Errorf("downloads = %d, want 2: both were served", got)
	}
}

// The starter allowance covers a student with no coins at all, and it is consumed
// exactly once however many times the file is fetched. This is the ordering
// requirement as a database fact: read the balance first and this student is
// refused by the very mechanism meant to help them.
func TestStarterAllowanceCoversADownloadAndIsConsumedOnce(t *testing.T) {
	const userID = 952
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	doc := e.seedDocument(t, "Entrance Model Questions", studyresources.TypeModelQuestions, true)
	// No grant at all: the wallet is empty and the allowance has to carry it.
	granted := DefaultEconomyConfig().Allowance.DocumentUnlocks

	res := e.download(t, userID, doc.ID)
	if !res.served() {
		t.Fatalf("a student with an unused allowance was refused: %d %s", res.Status, res.Raw)
	}
	rows := gateUnlocksFor(t, db, userID, uint64(doc.ID))
	if len(rows) != 1 {
		t.Fatalf("unlock rows = %d, want 1", len(rows))
	}
	if rows[0].Source != UnlockSourceAllowance {
		t.Errorf("source = %q, want %q", rows[0].Source, UnlockSourceAllowance)
	}
	if rows[0].CoinsPaid != 0 {
		t.Errorf("coins_paid = %d, want 0 for an allowance unlock", rows[0].CoinsPaid)
	}
	if rows[0].JournalID != nil {
		t.Error("an allowance unlock is joined to a journal; it spends nothing")
	}
	if n := gateSpendJournals(t, db, userID); n != 0 {
		t.Errorf("spend journals = %d, want 0 for an allowance unlock", n)
	}
	if got := gateResourceDownloads(t, db, doc.ID); got != 1 {
		t.Errorf("downloads = %d, want 1", got)
	}

	// The second download of the SAME resource is free, and does not burn a
	// second starter unlock: the entitlement from the first one answers it.
	status, err := e.service.RemainingAllowance(context.Background(), userID, time.Now().UTC())
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	if remaining := status.Remaining(ResourceTypeStudyResource); remaining != granted-1 {
		t.Errorf("remaining document allowance = %d, want %d (one consumed by one download)", remaining, granted-1)
	}

	if res := e.download(t, userID, doc.ID); !res.served() {
		t.Fatalf("the second download was refused: %d %s", res.Status, res.Raw)
	}
	if n := gateSpendJournals(t, db, userID); n != 0 {
		t.Errorf("spend journals = %d, want 0: a second download charged a coin-less student", n)
	}
	status, err = e.service.RemainingAllowance(context.Background(), userID, time.Now().UTC())
	if err != nil {
		t.Fatalf("RemainingAllowance after the second download: %v", err)
	}
	if remaining := status.Remaining(ResourceTypeStudyResource); remaining != granted-1 {
		t.Errorf("remaining document allowance = %d after a repeat download, want %d",
			remaining, granted-1)
	}
	if got := gateResourceDownloads(t, db, doc.ID); got != 2 {
		t.Errorf("downloads = %d, want 2", got)
	}
}

// ── the refusals ─────────────────────────────────────────────────────────────

// A wallet that cannot cover the price is the 402 of §2.3, with the designed
// object, and no bytes served.
func TestUnaffordableDownloadIsThe402AndServesNothing(t *testing.T) {
	const userID = 953
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions, true)
	// One profile instalment, so "available" is a real figure rather than a zero.
	granted := gateGrant(t, e, userID, ReasonProfileComplete, "gate-402-grant")
	gateExhaustDocuments(t, e, userID, 9531)

	res := e.download(t, userID, doc.ID)
	if res.served() {
		t.Fatalf("a download the student cannot afford was served: %d %s", res.Status, res.Raw)
	}
	if res.Status != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (%s)", res.Status, res.Raw)
	}
	if res.ErrorCode != CodeInsufficientCoins {
		t.Errorf("code = %q, want %q", res.ErrorCode, CodeInsufficientCoins)
	}
	if res.ErrorData == nil {
		t.Fatalf("the 402 carries no designed object: %s", res.Raw)
	}
	price := DefaultEconomyConfig().Prices.StudyResource
	if res.ErrorData.Required != price {
		t.Errorf("required = %d, want the configured %d", res.ErrorData.Required, price)
	}
	if res.ErrorData.Available != granted {
		t.Errorf("available = %d, want the wallet's real %d", res.ErrorData.Available, granted)
	}
	if res.ErrorData.Shortfall != price-granted {
		t.Errorf("shortfall = %d, want %d", res.ErrorData.Shortfall, price-granted)
	}
	if res.ErrorData.WaysToEarn == nil {
		t.Error("ways_to_earn is null, want []")
	}
	assertNothingWritten(t, e, userID, uint64(doc.ID), granted)
}

// A lapsed allowance that no purchase covers is the 423, not the 402. The two
// sentences are different and the frontend renders two different screens, so
// collapsing them would tell a student to earn coins for an expiry they cannot
// fix.
func TestLapsedAllowanceAndNoCoinsIsThe423(t *testing.T) {
	const userID = 954
	db := openGateSchema(t)
	e := newGateEnv(t, db, func(cfg *EconomyConfig) { cfg.Allowance.ExpiresInDays = 1 })
	doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions, true)
	// The window closed two days ago and the wallet is empty.
	if _, err := e.service.EnsureAllowance(context.Background(), userID,
		time.Now().UTC().AddDate(0, 0, -2)); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}

	res := e.download(t, userID, doc.ID)
	if res.served() {
		t.Fatalf("a lapsed-and-unaffordable download was served: %d %s", res.Status, res.Raw)
	}
	if res.Status != http.StatusLocked {
		t.Fatalf("status = %d, want 423 (%s)", res.Status, res.Raw)
	}
	if res.ErrorCode != CodeAllowanceExpired {
		t.Errorf("code = %q, want %q", res.ErrorCode, CodeAllowanceExpired)
	}
	// The gap travels with the 423 too: the student needs the same earning
	// routes whichever sentence they are shown.
	if res.ErrorData == nil || res.ErrorData.Required != DefaultEconomyConfig().Prices.StudyResource {
		t.Errorf("the 423 carries no gap: %+v", res.ErrorData)
	}
	assertNothingWritten(t, e, userID, uint64(doc.ID), 0)
}

// A draft is a 404 and never reaches the gate, because the handler's own
// publication check answers first. A gate consulted before it would answer 402 to
// a request for a file the public is not allowed to know exists, which both leaks
// the draft and leaves the student unable to tell "this does not exist" from
// "this costs coins".
func TestDraftDownloadIsThe404AndCostsNothing(t *testing.T) {
	const userID = 955
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	draft := e.seedDocument(t, "Unpublished draft", studyresources.TypePastQuestions, false)
	gateGrant(t, e, userID, ReasonResourceApproved, "gate-draft-grant")

	res := e.download(t, userID, draft.ID)
	if res.Status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", res.Status, res.Raw)
	}
	// The 404 comes from the handler's own response shape, not the gate's coin
	// envelope, and it is not a money message.
	if res.ErrorCode == CodeInsufficientCoins || res.ErrorCode == CodeAllowanceExpired {
		t.Errorf("a draft was reported as a wallet problem: %q", res.ErrorCode)
	}
	assertNothingWritten(t, e, userID, uint64(draft.ID), 80)
}

// An id that does not exist is a 404 too, and it is the handler's publication
// check that answers it. The gate is never reached, because the gate is only ever
// asked about rows the public can see.
func TestUnknownResourceIdIsThe404(t *testing.T) {
	const userID = 956
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)

	res := e.download(t, userID, 999999)
	if res.Status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", res.Status, res.Raw)
	}
	if n := gateSpendJournals(t, db, userID); n != 0 {
		t.Errorf("an unknown id wrote %d spend journals", n)
	}
}

// ── video is not gated ───────────────────────────────────────────────────────

// A video lecture goes through the same handler and is NOT gated by the document
// switch. The gates are per class precisely so the video slice can be enabled,
// reviewed and reverted on its own — and this is the test that says enabling
// documents did not quietly enable video.
func TestVideoDownloadIsNotGatedByTheDocumentSwitch(t *testing.T) {
	const userID = 957
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	lecture := e.seedDocument(t, "Thermodynamics Lecture 3", studyresources.TypeVideoLectures, true)
	gateGrant(t, e, userID, ReasonResourceApproved, "gate-video-grant")

	res := e.download(t, userID, lecture.ID)
	if !res.served() {
		t.Fatalf("a video download was refused by the document gate: %d %s", res.Status, res.Raw)
	}
	// No unlock and no journal: the video class is ungated, so nothing about this
	// student or this resource reached the economy at all.
	if n := len(gateUnlocksFor(t, db, userID, uint64(lecture.ID))); n != 0 {
		t.Errorf("a video download wrote %d unlock rows", n)
	}
	if n := gateSpendJournals(t, db, userID); n != 0 {
		t.Errorf("a video download wrote %d spend journals", n)
	}
	if got := apiUserAvailable(t, db, userID); got != 80 {
		t.Errorf("wallet = %d, want the unchanged 80", got)
	}
	// The class is what makes that true: asked about the VIDEO class the gate is
	// off, and asked about the DOCUMENT class it is on. Asserting it directly
	// pins the switch to the class rather than to the route.
	if decision := e.api.authorizeStudyResourceDownload(context.Background(), userID,
		studyresources.GateClassVideo, uint64(lecture.ID), lecture.Title); !decision.Allowed || decision.Charged != 0 {
		t.Errorf("the video class was charged while the video gate is off: %+v", decision)
	}
}

// ── the wired lookup ─────────────────────────────────────────────────────────

// The wired ResourceLookup is what makes 404 reachable and what puts a real title
// on the purchase. Both are asserted here over the real table, because the
// publication rule and the title live in another module's row and a fake would
// only prove the fake.
func TestTheWiredLookupResolvesTheTitleAndRefusesWhatItMust(t *testing.T) {
	db := openGateSchema(t)
	published := &studyresources.StudyResource{
		Title: "Physics Past Questions 2081", ResourceType: studyresources.TypePastQuestions,
		FileName: "p.pdf", FilePath: "study-resources/p.pdf",
		FileURL: "/uploads/study-resources/p.pdf", IsPublished: true,
	}
	draft := &studyresources.StudyResource{
		Title: "Hidden draft", ResourceType: studyresources.TypePastQuestions,
		FileName: "d.pdf", FilePath: "study-resources/d.pdf",
		FileURL: "/uploads/study-resources/d.pdf", IsPublished: false,
	}
	lecture := &studyresources.StudyResource{
		Title: "Thermodynamics Lecture 3", ResourceType: studyresources.TypeVideoLectures,
		FileName: "v.mp4", FilePath: "private/study-resources/v.mp4",
		FileURL: "/uploads/private/study-resources/v.mp4", IsPublished: true,
	}
	// The module's CreateResource, not db.Create: a raw Create of a draft writes
	// the column's `default:true` instead of the requested false, and a "draft"
	// fixture that is actually published would make the 404 assertions below pass
	// for the wrong reason.
	repo := studyresources.NewRepository(db)
	for _, resource := range []*studyresources.StudyResource{published, draft, lecture} {
		if err := repo.CreateResource(resource); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	lookup := NewStudyResourceLookup(studyresources.NewService(repo))

	t.Run("a published document resolves with its title", func(t *testing.T) {
		got, err := lookup.LookupUnlockable(context.Background(), ResourceTypeStudyResource, uint64(published.ID))
		if err != nil {
			t.Fatalf("resolve %d: %v", published.ID, err)
		}
		if got.Title != published.Title {
			t.Errorf("title = %q, want %q", got.Title, published.Title)
		}
	})

	t.Run("a published video lecture resolves under the video class", func(t *testing.T) {
		got, err := lookup.LookupUnlockable(context.Background(), ResourceTypeVideo, uint64(lecture.ID))
		if err != nil {
			t.Fatalf("resolve the lecture: %v", err)
		}
		if got.Title != lecture.Title {
			t.Errorf("title = %q, want %q", got.Title, lecture.Title)
		}
	})

	// The refusals are indistinguishable from each other, and that IS the
	// requirement: §2.3 asks for ONE 404 so the endpoint cannot be used to confirm
	// that a draft exists. The class mismatch is here for the same reason — a
	// video asked for as a document would be charged the document price.
	for name, tc := range map[string]struct {
		class    string
		resource uint
	}{
		"an unpublished row":               {ResourceTypeStudyResource, draft.ID},
		"an unknown id":                    {ResourceTypeStudyResource, 999999},
		"a video asked for as a document":  {ResourceTypeStudyResource, lecture.ID},
		"a class this lookup does not own": {ResourceTypeMockTest, published.ID},
		"the id zero":                      {ResourceTypeStudyResource, 0},
		"an id past the addressable range": {ResourceTypeStudyResource, 1 << 40},
	} {
		t.Run(name+" is ErrNotFound", func(t *testing.T) {
			_, err := lookup.LookupUnlockable(context.Background(), tc.class, uint64(tc.resource))
			if err == nil {
				t.Fatal("the lookup resolved a resource it should not have")
			}
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("error = %v, want one matching ErrNotFound so the caller maps it to 404", err)
			}
		})
	}

	// A lookup with no service resolves nothing, which is the same answer as not
	// having wired one — and not a crash.
	if _, err := NewStudyResourceLookup(nil).LookupUnlockable(context.Background(),
		ResourceTypeStudyResource, uint64(published.ID)); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unwired lookup returned %v, want ErrNotFound", err)
	}
}

// The description on the unlock now names the resource, resolved through the real
// lookup over the real table. The string the unwired build produced was
// "study_resource <id>"; this asserts the real title is what reaches the wallet
// history and the debit receipt.
func TestTheUnlockedDescriptionNamesTheResourceNotTheClass(t *testing.T) {
	const userID = 958
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions, true)
	gateGrant(t, e, userID, ReasonResourceApproved, "gate-title-grant")
	gateExhaustDocuments(t, e, userID, 9581)

	if res := e.download(t, userID, doc.ID); !res.served() {
		t.Fatalf("the download was refused: %d %s", res.Status, res.Raw)
	}

	// What the wallet history renders, from the real resolved title.
	title := e.api.titleFor(context.Background(), ResourceTypeStudyResource, uint64(doc.ID))
	if title != doc.Title {
		t.Fatalf("the wired lookup resolved %q, want %q", title, doc.Title)
	}
	titled := describeUnlocked(ResourceTypeStudyResource, title, uint64(doc.ID))
	if titled != "Unlocked: "+doc.Title {
		t.Errorf("description = %q, want the resource title", titled)
	}
	untitled := describeUnlocked(ResourceTypeStudyResource, "", uint64(doc.ID))
	if untitled != "Unlocked: study_resource "+strconv.FormatUint(uint64(doc.ID), 10) {
		t.Errorf("the fallback description = %q, want the class and id", untitled)
	}
	if titled == untitled {
		t.Error("the titled and untitled descriptions are identical, so the title changed nothing")
	}
	// And the receipt names it too, which is where a student looks when a balance
	// does not match what they expected.
	if got := unlockItemLabel(ResourceTypeStudyResource, title); got != doc.Title {
		t.Errorf("the debit receipt names %q, want %q", got, doc.Title)
	}
}

// ── concurrency ──────────────────────────────────────────────────────────────

// The load-bearing test of this slice. N concurrent downloads of the SAME
// unpurchased resource by the SAME student must produce exactly one charge and
// exactly one unlock — and every one of them must be served.
//
// This is the double-tap, not the retry: a browser issues a second identical GET
// and there is no idempotency key to tell the two apart. What stands between the
// student and a double charge is that the gate reuses coinPurchase, which spends
// and records inside one transaction under the per-user advisory lock. Across two
// transactions the loser's spend would commit before its unlock insert was
// refused, so it would be charged for something it does not get — and Reverse
// refuses to refund a spend, so the coins would be gone for nothing.
func TestConcurrentDownloadsOfTheSameResourceChargeOnce(t *testing.T) {
	const (
		userID = 959
		fan    = 8
	)
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions, true)
	// Enough for every request to be affordable, so the only thing stopping a
	// second debit is the system and not the balance.
	gateGrant(t, e, userID, ReasonResourceApproved, "gate-race-grant-1")
	gateGrant(t, e, userID, ReasonResourceApproved, "gate-race-grant-2")
	// Exhaust the allowance so all eight are forced onto the coin path. Left in
	// place, the winner would be decided by the allowance machinery instead of by
	// the purchase composition, and this test would stop testing the thing it
	// exists for.
	gateExhaustDocuments(t, e, userID, 9591)
	before := apiUserAvailable(t, db, userID)
	price := DefaultEconomyConfig().Prices.StudyResource

	results := make([]gateResponse, fan)
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for i := 0; i < fan; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			// Every goroutine blocks on the same WaitGroup, so they collide as
			// tightly as the pool allows rather than trickling in.
			start.Wait()
			results[i] = e.download(t, userID, doc.ID)
		}(i)
	}
	start.Done()
	done.Wait()

	// Every request is served. A 402 or a 500 here would mean the race produced a
	// failure rather than a duplicate answer, which is a different and also
	// unacceptable outcome: the student pressed download eight times and should
	// have eight files and one bill.
	for i, got := range results {
		if !got.served() {
			t.Errorf("download %d = %d, want a served file (%s)", i, got.Status, got.Raw)
		}
	}

	// Exactly one unlock and exactly one debit.
	if rows := gateUnlocksFor(t, db, userID, uint64(doc.ID)); len(rows) != 1 {
		t.Errorf("live unlocks for %s/%d = %d, want exactly 1",
			ResourceTypeStudyResource, doc.ID, len(rows))
	}
	if n := gateSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want exactly 1 — a concurrent download was charged twice", n)
	}
	if after := apiUserAvailable(t, db, userID); after != before-price {
		t.Errorf("wallet = %d, want %d (one debit of %d), so %d coins went missing or were taken twice",
			after, before-price, price, before-after)
	}
	// All eight were served, so all eight counted. The counter is the witness that
	// the losers got their file rather than a duplicate answer about their file.
	if got := gateResourceDownloads(t, db, doc.ID); got != fan {
		t.Errorf("downloads = %d, want %d: every concurrent request was served", got, fan)
	}
}

// A student with no coins racing against themselves: the allowance is the only
// thing that can serve them, and the race must not burn two starter unlocks or
// produce two unlocks.
func TestConcurrentDownloadsAgainstAnEmptyWalletServeEveryRequest(t *testing.T) {
	const (
		userID = 960
		fan    = 6
	)
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions, true)
	// No grant: the allowance is the only path, and it covers exactly one.
	granted := DefaultEconomyConfig().Allowance.DocumentUnlocks

	results := make([]gateResponse, fan)
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for i := 0; i < fan; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			start.Wait()
			results[i] = e.download(t, userID, doc.ID)
		}(i)
	}
	start.Done()
	done.Wait()

	for i, got := range results {
		if !got.served() {
			t.Errorf("download %d = %d, want a served file — a student with a starter unlock must not be blocked by it (%s)",
				i, got.Status, got.Raw)
		}
	}
	rows := gateUnlocksFor(t, db, userID, uint64(doc.ID))
	if len(rows) != 1 {
		t.Errorf("unlock rows = %d, want exactly 1", len(rows))
	} else if rows[0].CoinsPaid != 0 {
		t.Errorf("coins_paid = %d, want 0", rows[0].CoinsPaid)
	}
	if n := gateSpendJournals(t, db, userID); n != 0 {
		t.Errorf("spend journals = %d, want 0: a coin-less student was charged", n)
	}
	status, err := e.service.RemainingAllowance(context.Background(), userID, time.Now().UTC())
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	if remaining := status.Remaining(ResourceTypeStudyResource); remaining != granted-1 {
		t.Errorf("remaining document allowance = %d, want %d: the race burned more than one starter unlock",
			remaining, granted-1)
	}
}

// Two students racing for the same resource is a different claim and is stated as
// one: the advisory lock is per user, so "once" means once per student, not once
// per resource. Each is charged once and each ends up with their own unlock.
//
// The two purchases succeeding at all is itself the evidence that the derived
// idempotency key is per user: (scope, idempotency_key) is unique across every
// user journal, so a key built from the resource alone would have made the second
// insert fail with a replay or a 409.
func TestTwoStudentsEachPayOnceForTheSameResource(t *testing.T) {
	const (
		firstUser  = 961
		secondUser = 962
	)
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil)
	doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions, true)
	gateGrant(t, e, firstUser, ReasonResourceApproved, "gate-two-a")
	gateGrant(t, e, secondUser, ReasonResourceApproved, "gate-two-b")
	gateExhaustDocuments(t, e, firstUser, 9611)
	gateExhaustDocuments(t, e, secondUser, 9621)

	beforeFirst := apiUserAvailable(t, db, firstUser)
	beforeSecond := apiUserAvailable(t, db, secondUser)
	price := DefaultEconomyConfig().Prices.StudyResource

	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for _, user := range []uint{firstUser, secondUser} {
		for i := 0; i < 2; i++ {
			done.Add(1)
			go func(user uint, i int) {
				defer done.Done()
				start.Wait()
				if res := e.download(t, user, doc.ID); !res.served() {
					t.Errorf("user %d download %d was refused: %d %s", user, i, res.Status, res.Raw)
				}
			}(user, i)
		}
	}
	start.Done()
	done.Wait()

	for _, tc := range []struct {
		user   uint
		before int64
	}{{firstUser, beforeFirst}, {secondUser, beforeSecond}} {
		if rows := gateUnlocksFor(t, db, tc.user, uint64(doc.ID)); len(rows) != 1 {
			t.Errorf("user %d holds %d unlocks, want 1", tc.user, len(rows))
		}
		if n := gateSpendJournals(t, db, tc.user); n != 1 {
			t.Errorf("user %d has %d spend journals, want 1", tc.user, n)
		}
		if after := apiUserAvailable(t, db, tc.user); after != tc.before-price {
			t.Errorf("user %d wallet = %d, want %d", tc.user, after, tc.before-price)
		}
	}
}

// ── the switch is off ────────────────────────────────────────────────────────

// With the document gate off — the shipped default — a real database is not
// consulted at all. This is the ship-dark proof over the real route: the same
// published document, an EMPTY wallet, and no unlock, no journal and no charge.
//
// An empty wallet is the load-bearing detail. With the gate on, this exact request
// is a 402; with it off, it is a served file. So the difference between the two
// deployments is the switch and nothing else.
func TestGatedDownloadWithTheSwitchOffIsFreeAndWritesNothing(t *testing.T) {
	const userID = 963
	db := openGateSchema(t)
	e := newGateEnv(t, db, func(cfg *EconomyConfig) {
		cfg.Gates = GatesEnabledConfig{} // the shipped default
	})
	doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions, true)

	res := e.download(t, userID, doc.ID)
	if !res.served() {
		t.Fatalf("an ungated download was refused: %d %s", res.Status, res.Raw)
	}
	if n := len(gateUnlocksFor(t, db, userID, uint64(doc.ID))); n != 0 {
		t.Errorf("an ungated download wrote %d unlock rows", n)
	}
	if n := gateSpendJournals(t, db, userID); n != 0 {
		t.Errorf("an ungated download wrote %d spend journals", n)
	}
	if got := apiUserAvailable(t, db, userID); got != 0 {
		t.Errorf("wallet = %d, want 0: an ungated download moved coins", got)
	}
	if got := gateResourceDownloads(t, db, doc.ID); got != 1 {
		t.Errorf("downloads = %d, want 1: an ungated download must behave exactly as it always has", got)
	}
}

// The unlock endpoint's own flag is independent of the gate, and that
// independence is a decision with a reason: the gate is what makes a purchase
// meaningful, and the endpoint is what stops a student spending coins through a
// route that is cheaper than the one they are charged on. Turning the document
// path live needs BOTH.
func TestTheGateAndTheUnlockEndpointAreSeparateSwitches(t *testing.T) {
	const userID = 964
	db := openGateSchema(t)
	e := newGateEnv(t, db, nil) // document gate ON, unlock endpoint OFF
	doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions, true)
	gateGrant(t, e, userID, ReasonResourceApproved, "gate-switch-grant")
	gateExhaustDocuments(t, e, userID, 9641)

	if res := e.download(t, userID, doc.ID); !res.served() {
		t.Fatalf("the gated download was refused: %d %s", res.Status, res.Raw)
	}
	if n := gateSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want 1: the gate must not need the unlock endpoint", n)
	}

	cfg, err := e.api.config.Load()
	if err != nil {
		t.Fatalf("load the config: %v", err)
	}
	if cfg.UnlockEndpointEnabled {
		t.Error("the unlock endpoint is enabled in this fixture; the independence claim is not being tested")
	}
	if !cfg.GateEnabled(ResourceTypeStudyResource) {
		t.Error("the document gate is off in this fixture; the independence claim is not being tested")
	}
}

// The keys two concurrent downloads mint are the same key, which is what makes
// the losers REPLAY rather than charge. Asserted directly because it is the
// mechanism behind TestConcurrentDownloadsOfTheSameResourceChargeOnce and it
// would otherwise be invisible: a random key per request would also produce one
// unlock, by luck of the entitlement check, and would double-charge on a retry
// where the entitlement is not there to catch it.
func TestTheGateReusesOneKeyPerStudentAndResource(t *testing.T) {
	first := gateIdempotencyKey(959, ResourceTypeStudyResource, 812, 40)
	second := gateIdempotencyKey(959, ResourceTypeStudyResource, 812, 40)
	if first != second {
		t.Errorf("two concurrent downloads minted different keys: %q and %q", first, second)
	}
	if strings.Contains(first, " ") || !strings.HasPrefix(first, gateIdempotencyKeyPrefix) {
		t.Errorf("the minted key is not namespaced: %q", first)
	}
}
