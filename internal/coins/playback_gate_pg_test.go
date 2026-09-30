//go:build coinsintegration

// internal/coins/playback_gate_pg_test.go
//
// The video-playback coin gate against a real PostgreSQL instance, through the
// real playback-token route.
//
// The document gate already has this file's shape (download_gate_pg_test.go) and
// the video gate is the same decision reached through a different class, a
// different switch, a different price, a different allowance column and a
// different journal-key namespace. That is exactly why it needs its own database
// proof rather than an argument that the shared core makes it automatic: the core
// fixes the ORDER, and the order is only half of the claim. The other half is
// that the video class really is the video class everywhere it can be got wrong
// — the switch that is read, the price that is charged, the allowance column
// that is drawn on and the entitlement that is written. A gate that read
// Gates.StudyResource, or charged Prices.StudyResource, or drew on
// allowance.document_unlocks, would produce every ordering this file asserts and
// still take 40 coins for a 90-coin video off the wrong quota.
//
// Everything in internal/studyresources/playback_gate_test.go is about the
// HANDLER: that a refusal mints no token, that a nil gate changes nothing, that a
// reasonless refusal is a 500. All of that is reachable with a fake gate and no
// database, and all of it stops at the money. None of it can reach the wallet, so
// none of it can tell a student who was charged 90 from one who was charged 40.
// This file is the other half: what the DATABASE did.
//
//   - that a paid play spends the VIDEO price and writes a video entitlement, in
//     one transaction, and that a repeat play of an owned video moves nothing.
//   - that the starter VIDEO allowance carries a student with no coins at all,
//     and is consumed exactly once however many times the video is played. This
//     is the case that matters most, because a zero-balance student is precisely
//     who the allowance exists for: a gate that read the balance first would
//     block the people the feature was built for.
//   - that the unaffordable case is the 402 of §2.3 with the designed object, and
//     that the video is NOT allowed.
//   - that the gate resolves the resource ITSELF, so an unknown or unpublished
//     video is a 404 from the gate rather than only from a route that happened to
//     check publication first.
//   - that the two switches are independent in both directions, and that both off
//     is the deployment as it ships: the same request that is a 402 with the
//     video gate on is a served video with an empty wallet when it is off.
//   - that N concurrent plays of the SAME unowned video by the SAME student
//     produce exactly one charge and exactly one unlock, and that all N are
//     served. Only a real unique index, a real per-user advisory lock and N real
//     connections can produce the race that would break it.
//
// Run with:
//
//	COINS_TEST_DSN='host=... user=... dbname=...' go test -tags coinsintegration ./internal/coins/...
//
// ── safety ───────────────────────────────────────────────────────────────────
//
// This file creates and drops its own schema, coins_playback_gate_test, and
// nothing is ever created in the public schema. Every test skips when
// COINS_TEST_DSN is unset, so `go test ./...` needs no database. The search_path
// goes in the DSN rather than in a SET, because a SET applies to ONE pooled
// connection and the concurrency test would then have some of its goroutines
// writing to the public schema — the trap unlock_api_pg_test.go and
// download_gate_pg_test.go both document for their own schemas.
package coins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"studsphere/backend/internal/mocktests"
	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/utils"
	"studsphere/backend/internal/studyresources"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// playbackGateSchema is the one schema this file owns.
const playbackGateSchema = "coins_playback_gate_test"

// openPlaybackGateSchema creates coins_playback_gate_test with the ledger, the
// entitlement, the notification and the study-resource models, plus every
// constraint AutoMigrate cannot build.
//
// study_resources is migrated in because the gate's ResourceLookup reads the real
// table. The 404 and the receipt title are claims about what that read does, and
// a fake of it would only prove the fake — which is the whole reason item 4 of
// this file exists.
func openPlaybackGateSchema(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("COINS_TEST_DSN")
	if dsn == "" {
		t.Skip("COINS_TEST_DSN not set; skipping StudsToken video playback gate integration test")
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
	if err := admin.Exec(`DROP SCHEMA IF EXISTS ` + playbackGateSchema + ` CASCADE`).Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := admin.Exec(`CREATE SCHEMA ` + playbackGateSchema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(`DROP SCHEMA IF EXISTS ` + playbackGateSchema + ` CASCADE`).Error
	})

	pool, err := gorm.Open(postgres.Open(dsn+" search_path="+playbackGateSchema), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres pinned to %s: %v", playbackGateSchema, err)
	}
	sqlPool, err := pool.DB()
	if err != nil {
		t.Fatalf("sql handle: %v", err)
	}
	// A connection is held for the whole purchase transaction, so a pool that is
	// too small silently serialises the concurrency test and it stops testing
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
	// It is last in the list so a failure in the money tables is not reported as
	// a studyresources problem.
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

	// A playback token is signed with the process-wide JWT secret, and a <video>
	// element is the caller that cannot attach a header. The mint is the last
	// thing the route does, so a missing secret turns every allowed play in this
	// file into a 500 and the tests would all be about that instead.
	previous := config.AppConfig
	t.Cleanup(func() { config.AppConfig = previous })
	config.AppConfig = &config.Config{
		JWTSecret:                          "playback-gate-test-secret",
		StudyResourceVideoMaxSizeMB:        200,
		StudyResourceVideoTranscodeTimeout: utils.DefaultVideoTranscodeTimeout,
	}
	return pool
}

// ── the fixture ──────────────────────────────────────────────────────────────

// playbackEnv is the whole path wired the way main.go wires it: one config
// store, one repository, one service, the resource lookup reading the real
// study-resources table, and the playback gate attached to the real route.
type playbackEnv struct {
	api     *UnlockAPI
	gate    studyresources.PlaybackGate
	paper   mocktests.PaperGate
	repo    *Repository
	service *Service
	ledger  *Ledger
	db      *gorm.DB

	// routers is keyed by user, so two students can play the same video
	// concurrently without sharing an identity. Building one router per request
	// would also work and costs nothing, but a per-user router makes a mistaken
	// cross-user assertion impossible to write.
	routers map[uint]*gin.Engine
}

// newPlaybackEnv wires the video gate ON, which is the point of this file: the
// dark case is a config default and is asserted here too, by building a second
// env with every switch off. The unlock endpoint's own flag is left OFF
// throughout, deliberately — the gate is what makes a purchase meaningful, and
// the two are separate switches, which is why the document gate can be reverted
// on its own.
func newPlaybackEnv(t *testing.T, db *gorm.DB, mutate func(*EconomyConfig)) *playbackEnv {
	t.Helper()
	cfg := DefaultEconomyConfig()
	cfg.Gates.Video = true
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
	gate := NewPlaybackGate(api)
	resourcesHandler := studyresources.NewHandler(studyresources.NewService(studyresources.NewRepository(db)))
	playbackHandler := resourcesHandler.WithPlaybackGate(gate).WithDownloadGate(NewDownloadGate(api))

	env := &playbackEnv{
		api: api, gate: gate, paper: NewPaperGate(api),
		repo: repo, service: service, ledger: ledger, db: db,
		routers: map[uint]*gin.Engine{},
	}
	gin.SetMode(gin.TestMode)
	// One id per user this file uses. The two-student test and the eight-way
	// concurrency test both need several, and a fixed list means a typo in a user
	// id is a test failure rather than a silent second identity.
	for _, userID := range []uint{700, 701, 702, 703, 704, 705, 706, 707, 708, 709, 710} {
		playbackHandler := playbackHandler
		userID := userID
		router := gin.New()
		// A stand-in for authMW, which is what the route really sits behind. It
		// is the only thing that answers 401, so the gate never sees an anonymous
		// request — the same arrangement as production, and the reason a 401 is
		// absent from the cases here.
		router.GET("/api/v1/study-resources/:id/playback-token", func(c *gin.Context) {
			c.Set("user_id", userID)
			playbackHandler.IssuePlaybackToken(c)
		})
		// The download route is mounted too, because the switch-independence
		// claims are questions about the two gates at once: a video switch that
		// charged for a document download would be found here and nowhere else.
		router.GET("/api/v1/study-resources/:id/download", func(c *gin.Context) {
			c.Set("user_id", userID)
			playbackHandler.DownloadResource(c)
		})
		env.routers[userID] = router
	}
	return env
}

// ungatedPlaybackRouter is the same route with NO gate attached at all: the
// handler this slice had before internal/coins existed. It exists so the
// both-flags-off claim can be stated as what it is — indistinguishable from no
// gate — rather than only as "nothing was charged".
func ungatedPlaybackRouter(t *testing.T, db *gorm.DB, userID uint) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := studyresources.NewHandler(studyresources.NewService(studyresources.NewRepository(db)))
	router := gin.New()
	router.GET("/api/v1/study-resources/:id/playback-token", func(c *gin.Context) {
		c.Set("user_id", userID)
		handler.IssuePlaybackToken(c)
	})
	return router
}

// seedVideo inserts one published or draft video lecture and returns it.
//
// It goes through the module's own CreateResource rather than db.Create, and that
// is not a style choice. IsPublished carries a `default:true` tag, and GORM never
// writes a zero-valued field that has a database default — so a raw Create of a
// draft silently produces a PUBLISHED row, and every draft case in this file
// would be asserting against a published video.
func (e *playbackEnv) seedVideo(t *testing.T, title string, published bool) *studyresources.StudyResource {
	t.Helper()
	video := &studyresources.StudyResource{
		Title: title, ResourceType: studyresources.TypeVideoLectures,
		FileName: "kinematics.mp4", FilePath: "private/study-resources/kinematics.mp4",
		FileURL:  "/uploads/private/study-resources/kinematics.mp4",
		FileSize: 4096, MimeType: studyresources.NormalizedVideoContentType,
		DurationSeconds: 900, IsPublished: published,
	}
	if err := studyresources.NewRepository(e.db).CreateResource(video); err != nil {
		t.Fatalf("seed video %q: %v", title, err)
	}
	return video
}

// seedDocument inserts one document. Only the switch-independence cases need one,
// and they need it precisely because it is NOT a video.
func (e *playbackEnv) seedDocument(t *testing.T, title, storedType string) *studyresources.StudyResource {
	t.Helper()
	doc := &studyresources.StudyResource{
		Title: title, ResourceType: storedType,
		FileName: "physics.pdf", FilePath: "study-resources/physics.pdf",
		FileURL: "/uploads/study-resources/physics.pdf", FileSize: 10,
		MimeType: "application/pdf", IsPublished: true,
	}
	if err := studyresources.NewRepository(e.db).CreateResource(doc); err != nil {
		t.Fatalf("seed document %q: %v", title, err)
	}
	return doc
}

// playbackResponse is one request's outcome, whichever route it went to.
type playbackResponse struct {
	Status    int
	Message   string
	ErrorCode string
	ErrorData *InsufficientCoinsData
	Token     string
	StreamURL string
	// DataKeys is the sorted key set of the 200 body's `data` object. It is what
	// makes "the same response as no gate" checkable: the token and its expiry
	// differ on every mint by construction, so the claim is about the shape.
	DataKeys []string
	Raw      string
}

// played reports whether the request got past the gate AND the token was minted.
//
// Both halves are required. A gate that allowed and then a mint that failed is a
// 500, which is not a served video; and a 200 carrying no token is a grant
// missing, which is the failure internal/studyresources/playback_gate_test.go
// exists to catch on the refusal side.
func (r playbackResponse) played() bool {
	return r.Status == http.StatusOK && r.Token != ""
}

// refused reports whether the PLAYBACK route refused the request, whatever the
// reason.
//
// 402 and 423 are the coin refusals. 404 and 400 are included too: a draft video,
// an unknown id and a document asked for as a video are all refusals of the
// REQUEST rather than of the wallet, and counting them as "not refused" would let
// a broken 404 pass as a served video.
//
// It is deliberately NOT the same question as coinRefused. On the download route
// a 404 is the handler's own "File not found" from object storage, which arrives
// after the gate has already allowed the request, so a download helper cannot
// reuse this and claim the gate let it through.
func (r playbackResponse) refused() bool {
	switch r.Status {
	case http.StatusPaymentRequired, http.StatusLocked, http.StatusNotFound, http.StatusBadRequest:
		return true
	default:
		return false
	}
}

// coinRefused reports whether the coin economy refused. It is the only status
// question the download route can be asked, because that route's other failures
// (the 404 from a missing object, in particular) are the handler's and not the
// gate's. The witness that the gate allowed the request is the download counter,
// which the handler increments before it touches object storage — that ordering is
// the reason it can be trusted, and it is the same witness
// download_gate_pg_test.go uses.
func (r playbackResponse) coinRefused() bool {
	switch r.Status {
	case http.StatusPaymentRequired, http.StatusLocked:
		return true
	default:
		return false
	}
}

// decode parses whichever of the two envelopes this is. The 200 is the coin
// module's own Success shape; the 402/423/404 is studyresources' refusal
// envelope, which is the same {success, message, error{code, data}} contract.
func (r *playbackResponse) decode() {
	var envelope struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
		Error   struct {
			Code string                 `json:"code"`
			Data *InsufficientCoinsData `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(r.Raw), &envelope); err != nil {
		return
	}
	r.Message = envelope.Message
	r.ErrorCode = envelope.Error.Code
	r.ErrorData = envelope.Error.Data
	if envelope.Data != nil {
		for key := range envelope.Data {
			r.DataKeys = append(r.DataKeys, key)
		}
		sort.Strings(r.DataKeys)
		if token, ok := envelope.Data["token"].(string); ok {
			r.Token = token
		}
		if stream, ok := envelope.Data["stream_url"].(string); ok {
			r.StreamURL = stream
		}
	}
}

// play performs one GET of the real playback-token route as userID.
//
// It takes no *testing.T on purpose: it is called from the concurrency test's
// goroutines, and a t.Fatalf there would Goexit a goroutine the test is waiting
// on rather than fail it cleanly. A missing router comes back as status 0 with
// the reason in Raw, which every assertion below reports as a failure with that
// reason attached.
func (e *playbackEnv) play(userID uint, resourceID uint) playbackResponse {
	router, ok := e.routers[userID]
	if !ok {
		return playbackResponse{
			Raw: "no router was wired for user " + strconv.FormatUint(uint64(userID), 10),
		}
	}
	rec := httptest.NewRecorder()
	path := "/api/v1/study-resources/" + strconv.FormatUint(uint64(resourceID), 10) + "/playback-token"
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	out := playbackResponse{Status: rec.Code, Raw: rec.Body.String()}
	out.decode()
	return out
}

// download performs one GET of the real download route as userID, for the
// independence cases that need the other gate.
func (e *playbackEnv) download(userID uint, resourceID uint) playbackResponse {
	router, ok := e.routers[userID]
	if !ok {
		return playbackResponse{
			Raw: "no router was wired for user " + strconv.FormatUint(uint64(userID), 10),
		}
	}
	rec := httptest.NewRecorder()
	path := "/api/v1/study-resources/" + strconv.FormatUint(uint64(resourceID), 10) + "/download"
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	out := playbackResponse{Status: rec.Code, Raw: rec.Body.String()}
	out.decode()
	return out
}

// ── row helpers ─────────────────────────────────────────────────────────────

func playbackSpendJournals(t *testing.T, db *gorm.DB, userID uint) int64 {
	t.Helper()
	return apiCount(t, db,
		`SELECT count(*) FROM coin_journal j
		   WHERE j.scope = 'user' AND j.entry_type = 'SPEND'
		     AND EXISTS (SELECT 1 FROM coin_posting p
		                  JOIN coin_account a ON a.id = p.account_id
		                 WHERE p.journal_id = j.id AND a.owner_user_id = ? AND a.kind = 'USER')`, userID)
}

func playbackUnlocksFor(t *testing.T, db *gorm.DB, userID uint, resourceID uint64) []ResourceUnlock {
	t.Helper()
	var rows []ResourceUnlock
	if err := db.Raw(
		`SELECT `+unlockColumns+` FROM resource_unlock
		  WHERE user_id = ? AND resource_type = ? AND resource_id = ? ORDER BY id`,
		userID, ResourceTypeVideo, resourceID,
	).Scan(&rows).Error; err != nil {
		t.Fatalf("read video unlocks for user %d %d: %v", userID, resourceID, err)
	}
	return rows
}

// playbackGrant pays this student through the real ledger, so the balance, the
// lots and the expiry the 402 reports are all the ones a real award produces.
func (e *playbackEnv) grant(t *testing.T, userID uint, reason, key string) int64 {
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

// exhaustVideos burns the whole VIDEO allowance, so a test that wants the COIN
// path is testing it. ConsumeAllowance creates the allowance when there is none,
// so without this a test named "a video play spends coins" could be silently
// testing the video allowance and would pass with the purchase path broken.
func (e *playbackEnv) exhaustVideos(t *testing.T, userID uint, firstID uint64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := e.service.EnsureAllowance(context.Background(), userID, now); err != nil {
		t.Fatalf("EnsureAllowance: %v", err)
	}
	status, err := e.service.RemainingAllowance(context.Background(), userID, now)
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	for i := int64(0); i < status.Remaining(ResourceTypeVideo); i++ {
		if _, err := e.service.ConsumeAllowance(context.Background(), userID,
			ResourceTypeVideo, firstID+uint64(i), now); err != nil {
			t.Fatalf("burning video allowance %d: %v", i+1, err)
		}
	}
}

func remainingVideos(t *testing.T, e *playbackEnv, userID uint) int64 {
	t.Helper()
	status, err := e.service.RemainingAllowance(context.Background(), userID, time.Now().UTC())
	if err != nil {
		t.Fatalf("RemainingAllowance: %v", err)
	}
	return status.Remaining(ResourceTypeVideo)
}

// assertVideoWroteNothing is the refusal and dark-path invariant in one place,
// so every test in this file that says "and nothing was written" says it
// identically: no unlock, no journal, no coins moved.
func assertVideoWroteNothing(t *testing.T, e *playbackEnv, userID uint, resourceID uint64, wantAvailable int64) {
	t.Helper()
	if rows := playbackUnlocksFor(t, e.db, userID, resourceID); len(rows) != 0 {
		t.Errorf("the request wrote %d video unlock rows, want 0", len(rows))
	}
	if n := playbackSpendJournals(t, e.db, userID); n != 0 {
		t.Errorf("the request wrote %d spend journals, want 0", n)
	}
	if got := apiUserAvailable(t, e.db, userID); got != wantAvailable {
		t.Errorf("wallet = %d, want the unchanged %d", got, wantAvailable)
	}
}

// ── 1. an owned video ────────────────────────────────────────────────────────

// A video the student already holds is theirs: the play is allowed, and it costs
// nothing. No journal, no second unlock row, no movement in the balance.
//
// The entitlement is bought through the real gate rather than inserted as a row,
// so the row under test is the one the money path actually wrote. The first play
// is a funded purchase that exhausts the starter video allowance, which is the
// only way to get to the coin path here — and which means the second play would
// be a 402 if the entitlement were not checked first.
func TestAnOwnedVideoIsPlayedForFreeAndMovesNoCoins(t *testing.T) {
	const userID = 700
	db := openPlaybackGateSchema(t)
	e := newPlaybackEnv(t, db, nil)
	video := e.seedVideo(t, "Kinematics lecture 3", true)
	price := DefaultEconomyConfig().Prices.Video

	// The video price is 90 and the upload award is 80, so ONE grant does not
	// cover a video. Stating that rather than discovering it as a 402 is the
	// point: a fixture that quietly underfunds the student is a test of the
	// refusal path wearing the costume of a test of the purchase path.
	granted := e.grant(t, userID, ReasonResourceApproved, "playback-owned-grant-1")
	granted += e.grant(t, userID, ReasonResourceApproved, "playback-owned-grant-2")
	if granted < price {
		t.Fatalf("the fixture raised %d, want more than the video price %d", granted, price)
	}
	e.exhaustVideos(t, userID, 7001)

	// The purchase that makes them an owner.
	first := e.play(userID, video.ID)
	if !first.played() {
		t.Fatalf("a funded play was refused: %d %s", first.Status, first.Raw)
	}
	rows := playbackUnlocksFor(t, db, userID, uint64(video.ID))
	if len(rows) != 1 {
		t.Fatalf("unlock rows = %d, want exactly 1", len(rows))
	}
	if rows[0].CoinsPaid != price {
		t.Errorf("coins_paid = %d, want the VIDEO price %d — the wrong class was priced",
			rows[0].CoinsPaid, price)
	}
	if rows[0].ResourceType != ResourceTypeVideo {
		t.Errorf("the entitlement is classed %q, want %q: the entitlement must follow the row, not the route",
			rows[0].ResourceType, ResourceTypeVideo)
	}
	if rows[0].JournalID == nil || *rows[0].JournalID == "" {
		t.Error("the unlock is not joined to a journal: the debit is not attributable")
	}
	before := apiUserAvailable(t, db, userID)
	if before != granted-price {
		t.Fatalf("wallet = %d after the purchase, want %d; the fixture is wrong", before, granted-price)
	}

	// The play under test. Every assertion after this one is about nothing having
	// happened, which is the whole point of the entitlement check.
	second := e.play(userID, video.ID)
	if !second.played() {
		t.Fatalf("the owner was refused their own video: %d %s", second.Status, second.Raw)
	}
	if got := playbackUnlocksFor(t, db, userID, uint64(video.ID)); len(got) != 1 {
		t.Errorf("unlock rows = %d after a repeat play, want 1", len(got))
	}
	if n := playbackSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want 1: a second play of an owned video charged again", n)
	}
	if after := apiUserAvailable(t, db, userID); after != before {
		t.Errorf("wallet = %d, want the unchanged %d: a second play took coins", after, before)
	}

	// The decision itself reports nothing charged, which is what the download
	// counter and any future logging read. Asserted on the gate rather than on
	// the body because the 200 body has no coin fields at all — the 200 is
	// deliberately the same 200 a free video produced.
	decision := e.gate.AuthorizePlayback(context.Background(), userID, uint64(video.ID), video.Title)
	if !decision.Allowed {
		t.Fatalf("the gate refused an owner: %+v", decision.Refusal)
	}
	if decision.Charged != 0 {
		t.Errorf("the gate charged %d for a video the student already owns, want 0", decision.Charged)
	}
}

// ── 2. the starter video allowance ───────────────────────────────────────────

// The starter allowance covers a student with no coins at all, and it is consumed
// exactly once however many times the video is played.
//
// This is the ordering requirement as a database fact, and it is the case in this
// file that would hurt a real student most. A zero-balance student is exactly who
// the allowance exists for; read the balance first and this person is refused by
// the very mechanism meant to help them, and the video is never played. The
// wallet is asserted at 0 before, during and after, so a gate that quietly found
// some other source of coins cannot pass by luck.
func TestTheStarterVideoAllowanceCoversAStudentWithNoCoinsAndIsConsumedOnce(t *testing.T) {
	const userID = 701
	db := openPlaybackGateSchema(t)
	e := newPlaybackEnv(t, db, nil)
	video := e.seedVideo(t, "Thermodynamics lecture 2", true)
	// No grant at all: the wallet is empty and the allowance has to carry it.
	granted := DefaultEconomyConfig().Allowance.VideoUnlocks
	if granted < 1 {
		t.Fatalf("the configured video allowance is %d; the fixture is wrong", granted)
	}
	if got := apiUserAvailable(t, db, userID); got != 0 {
		t.Fatalf("wallet = %d, want an empty one; the fixture is wrong", got)
	}

	res := e.play(userID, video.ID)
	if !res.played() {
		t.Fatalf("a student with an unused video allowance was refused: %d %s", res.Status, res.Raw)
	}
	rows := playbackUnlocksFor(t, db, userID, uint64(video.ID))
	if len(rows) != 1 {
		t.Fatalf("unlock rows = %d, want 1", len(rows))
	}
	if rows[0].Source != UnlockSourceAllowance {
		t.Errorf("source = %q, want %q: the allowance did not cover this", rows[0].Source, UnlockSourceAllowance)
	}
	if rows[0].CoinsPaid != 0 {
		t.Errorf("coins_paid = %d, want 0 for an allowance unlock", rows[0].CoinsPaid)
	}
	if rows[0].JournalID != nil {
		t.Error("an allowance unlock is joined to a journal; it spends nothing")
	}
	if n := playbackSpendJournals(t, db, userID); n != 0 {
		t.Errorf("spend journals = %d, want 0 for an allowance unlock", n)
	}
	if got := apiUserAvailable(t, db, userID); got != 0 {
		t.Errorf("wallet = %d, want 0: an allowance unlock moved coins", got)
	}
	if got := remainingVideos(t, e, userID); got != granted-1 {
		t.Errorf("remaining video allowance = %d, want %d (one consumed by one play)", got, granted-1)
	}

	// The second play of the SAME video is free and must not burn a second
	// starter unlock: the entitlement from the first answers it, and the
	// entitlement is checked BEFORE the allowance. With a quota of one this is
	// the only place a double-burn could hide.
	if res := e.play(userID, video.ID); !res.played() {
		t.Fatalf("the second play was refused: %d %s", res.Status, res.Raw)
	}
	if n := playbackSpendJournals(t, db, userID); n != 0 {
		t.Errorf("spend journals = %d, want 0: a second play charged a coin-less student", n)
	}
	if got := apiUserAvailable(t, db, userID); got != 0 {
		t.Errorf("wallet = %d, want 0 after a repeat play", got)
	}
	if got := remainingVideos(t, e, userID); got != granted-1 {
		t.Errorf("remaining video allowance = %d after a repeat play, want %d: a second starter unlock was burned",
			got, granted-1)
	}
	if rows := playbackUnlocksFor(t, db, userID, uint64(video.ID)); len(rows) != 1 {
		t.Errorf("unlock rows = %d after a repeat play, want 1", len(rows))
	}

	// A DIFFERENT video is a different entitlement, and this student has no
	// coins left and no allowance left. The refusal is the point: it proves the
	// allowance above was the thing that carried the first play, and that it was
	// drawn on the video class rather than on some shared pool.
	other := e.seedVideo(t, "Thermodynamics lecture 3", true)
	second := e.play(userID, other.ID)
	if second.played() {
		t.Fatalf("a second video was played for free with an empty wallet and no allowance: %d %s",
			second.Status, second.Raw)
	}
	assertVideoWroteNothing(t, e, userID, uint64(other.ID), 0)
}

// ── 3. the 402 ───────────────────────────────────────────────────────────────

// A wallet that cannot cover the video price is the 402 of §2.3 with the designed
// object — required, available, shortfall, expires_in_days, ways_to_earn — and the
// video is NOT allowed.
//
// The figures are the ones the refusal carried out of the rolled-back spend, so
// they are asserted against what this student was actually granted and never
// against a re-read of the wallet. That distinction is not pedantry: a re-read
// happens after the rollback, so it describes a later moment than the one the
// decision was made on, and a test that used the re-read as its expectation could
// not tell the two apart. The wallet read below is there to prove nothing moved,
// not to produce the expectation.
func TestAnUnaffordableVideoIsThe402WithTheDesignedBodyAndNoPlayback(t *testing.T) {
	const userID = 702
	db := openPlaybackGateSchema(t)
	e := newPlaybackEnv(t, db, nil)
	video := e.seedVideo(t, "Kinematics lecture 3", true)
	// One profile instalment, so "available" is a real figure rather than a zero.
	granted := e.grant(t, userID, ReasonProfileComplete, "playback-402-grant")
	e.exhaustVideos(t, userID, 7021)
	price := DefaultEconomyConfig().Prices.Video
	if price <= granted {
		t.Fatalf("the fixture is wrong: the price %d is affordable with %d", price, granted)
	}

	res := e.play(userID, video.ID)
	if res.played() {
		t.Fatalf("a video the student cannot afford was played: %d %s", res.Status, res.Raw)
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
	if res.ErrorData.Required != price {
		t.Errorf("required = %d, want the VIDEO price %d", res.ErrorData.Required, price)
	}
	if res.ErrorData.Available != granted {
		t.Errorf("available = %d, want the %d the refusal saw; this is the typed InsufficientError's figure, not a re-read",
			res.ErrorData.Available, granted)
	}
	if res.ErrorData.Shortfall != price-granted {
		t.Errorf("shortfall = %d, want %d", res.ErrorData.Shortfall, price-granted)
	}
	// expires_in_days is the urgency the 402 exists to create. It is a pointer
	// precisely so "nothing you hold expires" is null rather than a made-up
	// number, so the assertion is that a real lot with a real expiry produced a
	// real figure rather than that the field is present.
	if res.ErrorData.ExpiresInDays == nil {
		t.Error("expires_in_days is null, but this student holds an awarded lot with a 365-day life")
	} else if *res.ErrorData.ExpiresInDays <= 0 {
		t.Errorf("expires_in_days = %d, want the days left on the lot", *res.ErrorData.ExpiresInDays)
	}
	// ways_to_earn is computed from this student's ACTUAL remaining eligibility,
	// so it is not null and it is not a copy of the config. With no profile
	// adapter wired the only offered route is the upload award, which this
	// student has not taken.
	if res.ErrorData.WaysToEarn == nil {
		t.Error("ways_to_earn is null, want []")
	} else if len(res.ErrorData.WaysToEarn) == 0 {
		t.Error("ways_to_earn is empty for a student who can still earn the upload award")
	} else if res.ErrorData.WaysToEarn[0].Potential <= 0 {
		t.Errorf("ways_to_earn[0].potential = %d, want what the route is still worth",
			res.ErrorData.WaysToEarn[0].Potential)
	}
	// A refusal mints nothing. This is the half that only the database can show:
	// the handler tests already prove no token is issued on a refusal, and they
	// prove it with a fake gate, so they cannot know whether a balance moved.
	assertVideoWroteNothing(t, e, userID, uint64(video.ID), granted)
	if res.Token != "" || res.StreamURL != "" {
		t.Errorf("a refused play still carried a capability: token=%q stream_url=%q", res.Token, res.StreamURL)
	}
}

// ── 4. the 404, from the gate ───────────────────────────────────────────────

// The gate resolves the resource ITSELF, so an unknown id, an unpublished video
// and a row that is not a video lecture are all 404 from HERE, not only from a
// route that happened to check publication first.
//
// This is a claim about authorizeDelivery step 4, and the reason it matters is in
// that file: a gate that trusted its caller would one day be wired onto a route
// that does not make the publication check, and would sell a draft. The playback
// route does check — playback.go reaches the gate only after
// GetPlayableVideoResource — so the route-level assertion below passes either
// way and proves nothing about the gate. The gate-level assertion is the one that
// bites, and it is asserted against the real lookup over the real table: a
// missing id and a draft answer identically, and a published document asked for
// as a video is a 404 too rather than a video at the video price.
func TestTheGateRefusesAnUnknownOrUnpublishableVideoWithThe404(t *testing.T) {
	const userID = 703
	db := openPlaybackGateSchema(t)
	e := newPlaybackEnv(t, db, nil)
	draft := e.seedVideo(t, "Unpublished lecture", false)
	document := e.seedDocument(t, "Syllabus", studyresources.TypeSyllabus)
	published := e.seedVideo(t, "Kinematics lecture 3", true)
	e.grant(t, userID, ReasonResourceApproved, "playback-404-grant")
	before := apiUserAvailable(t, db, userID)

	for name, resourceID := range map[string]uint{
		"an id that does not exist":        999999,
		"an unpublished video":             draft.ID,
		"a published document":             document.ID,
		"the id zero":                      0,
		"an id past the addressable range": 1 << 40,
	} {
		t.Run(name+" is 404 from the gate", func(t *testing.T) {
			decision := e.gate.AuthorizePlayback(context.Background(), userID, uint64(resourceID), "")
			if decision.Allowed {
				t.Fatal("the gate allowed a video it should not have resolved")
			}
			if decision.Refusal == nil {
				t.Fatal("the gate refused without saying why")
			}
			if decision.Refusal.Status != http.StatusNotFound {
				t.Errorf("status = %d, want 404", decision.Refusal.Status)
			}
			if decision.Refusal.Code != CodeResourceNotFound {
				t.Errorf("code = %q, want %q", decision.Refusal.Code, CodeResourceNotFound)
			}
			// The three cases must be indistinguishable, because §2.3 asks for ONE
			// 404 so the endpoint cannot be used to confirm that a draft exists.
			if decision.Refusal.Message != playbackWording.notFound {
				t.Errorf("message = %q, want the one sentence every case shares", decision.Refusal.Message)
			}
			if decision.Refusal.Data != nil {
				t.Errorf("a 404 carries a money object: %+v", decision.Refusal.Data)
			}
		})
	}

	// Nothing above moved anything, which is asserted once over all of it: a 404
	// that took a coin would be a worse bug than a 404 that did not happen.
	assertVideoWroteNothing(t, e, userID, uint64(draft.ID), before)
	if rows := playbackUnlocksFor(t, db, userID, uint64(published.ID)); len(rows) != 0 {
		t.Errorf("the refusals wrote %d unlock rows against the published video", len(rows))
	}

	// The route answers the same way for the two cases it can see, so a caller
	// holding only the route is not left with a different answer. The document is
	// a 400 rather than a 404 because the route knows it exists and is refusing to
	// play it, which is a different sentence and a different frontend screen.
	for _, tc := range []struct {
		what string
		id   uint
		want int
	}{
		{"an unpublished video", draft.ID, http.StatusNotFound},
		{"an id that does not exist", 999999, http.StatusNotFound},
		{"a document", document.ID, http.StatusBadRequest},
	} {
		t.Run("the route answers 404 for "+tc.what, func(t *testing.T) {
			res := e.play(userID, tc.id)
			if res.Status != tc.want {
				t.Fatalf("status = %d, want %d (%s)", res.Status, tc.want, res.Raw)
			}
			if res.ErrorCode == CodeInsufficientCoins || res.ErrorCode == CodeAllowanceExpired {
				t.Errorf("a %d was reported as a wallet problem: %q", tc.want, res.ErrorCode)
			}
			if res.Token != "" {
				t.Error("a refused route minted a playback token")
			}
		})
	}
}

// ── 5. both switches off ─────────────────────────────────────────────────────

// With the video gate off — the shipped default — this request is a served video,
// with an EMPTY wallet, and writes nothing at all.
//
// The empty wallet is the load-bearing detail and it is why this is one test
// rather than a pair of assertions in a fixture. The same published video, the
// same student, the same request: with gates_enabled.video true it is a 402
// (asserted here, in the same function, from the same seed), and with the switch
// false it is a token. So the entire difference between the two deployments is
// the switch and nothing else — not a balance, not an allowance, not a
// registration that happened to pay a student.
//
// It is also the claim that a kill switch works by being OFF, so the test builds
// a third router with no gate attached at all and compares the 200's shape to it.
// The token and its expiry differ on every mint by construction, so what is
// compared is the message and the key set.
func TestWithTheVideoSwitchOffTheSameRequestIsServedWithAnEmptyWallet(t *testing.T) {
	const userID = 704
	db := openPlaybackGateSchema(t)
	// Two independent environments over one database: each has its own config
	// store, so this is two deployments rather than one deployment mutated under
	// a cache. A cached store is a real thing to get wrong, and folding the switch
	// mid-test would be testing the cache rather than the switch.
	on := newPlaybackEnv(t, db, nil)
	off := newPlaybackEnv(t, db, func(cfg *EconomyConfig) {
		cfg.Gates = GatesEnabledConfig{} // the shipped default: every switch off
	})
	video := on.seedVideo(t, "Kinematics lecture 3", true)

	// No grant. This student has never been paid a coin in this database.
	if got := apiUserAvailable(t, db, userID); got != 0 {
		t.Fatalf("wallet = %d, want an empty one; the fixture is wrong", got)
	}
	// The starter allowance is burned, and this is load-bearing for the control
	// below rather than tidiness. Left in place, the first play is answered by the
	// allowance at zero coins in BOTH environments, the "gate on" control would be
	// a 200 instead of a 402, and the whole comparison would collapse into "a
	// student with a free unlock may watch a video". The claim under test is
	// narrower and sharper than that: with the allowance spent, the switch alone
	// decides between a 402 and a served video.
	on.exhaustVideos(t, userID, 7041)

	// The same request, gate on. This is the control: it proves the 402 below is
	// about the switch and not about a video that happens to be free.
	refused := on.play(userID, video.ID)
	if refused.played() {
		t.Fatalf("with the video gate on and an empty wallet the play was served: %d %s",
			refused.Status, refused.Raw)
	}
	if refused.Status != http.StatusPaymentRequired {
		t.Fatalf("with the gate on: status = %d, want 402 (%s)", refused.Status, refused.Raw)
	}
	if refused.ErrorData == nil || refused.ErrorData.Required != DefaultEconomyConfig().Prices.Video {
		t.Errorf("with the gate on, the 402 does not quote the video price: %+v", refused.ErrorData)
	}
	assertVideoWroteNothing(t, on, userID, uint64(video.ID), 0)

	// The same request, gate off.
	served := off.play(userID, video.ID)
	if !served.played() {
		t.Fatalf("with the video switch off an empty-wallet play was refused: %d %s", served.Status, served.Raw)
	}
	if served.StreamURL == "" {
		t.Error("the 200 carried no stream_url")
	}
	assertVideoWroteNothing(t, off, userID, uint64(video.ID), 0)

	// Both of the requests above are for the same video, so the invariant helper
	// below is scoped to that video: the starter unlock burned by the fixture is a
	// row against resource 7041, and this test's subject is the video. The wallet
	// read it also makes is the one that would move if either environment charged.

	// And it is the handler this slice had before the economy existed. The claim
	// is about the shape, because the token's own bytes and expiry are different
	// on every mint and cannot be.
	bare := ungatedPlaybackRouter(t, db, userID)
	rec := httptest.NewRecorder()
	path := "/api/v1/study-resources/" + strconv.FormatUint(uint64(video.ID), 10) + "/playback-token"
	bare.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var ungated playbackResponse
	ungated.Status = rec.Code
	ungated.Raw = rec.Body.String()
	ungated.decode()

	if ungated.Status != served.Status {
		t.Errorf("with the switch off: %d, with no gate at all: %d", served.Status, ungated.Status)
	}
	if served.Message != ungated.Message {
		t.Errorf("message with the switch off = %q, with no gate = %q", served.Message, ungated.Message)
	}
	if strings.Join(served.DataKeys, ",") != strings.Join(ungated.DataKeys, ",") {
		t.Errorf("the 200 body changed shape: switch off %v, no gate %v", served.DataKeys, ungated.DataKeys)
	}
	assertVideoWroteNothing(t, off, userID, uint64(video.ID), 0)
}

// ── 6. the three switches are independent ────────────────────────────────────

// Each class has its own switch, and this says so in both directions rather than
// by assertion about a config struct. Each case is an empty wallet, so a switch
// that leaked would turn it into a 402, and each asserts that nothing was written
// so a leak that somehow succeeded would be caught too.
//
// The mock-test case is the one that needs an argument rather than a route. A
// paper is not in this database and the paper gate is deliberately not asked to
// resolve it: with its switch off, authorizeDelivery returns allowed before the
// lookup, before the price and before the balance. That makes the assertion
// falsifiable in the useful direction — had gates_enabled.mock_test been on for
// this fixture, the same call would have been a 404, not an allow, because there
// is no mocktests lookup wired here at all.
func TestEachGateSwitchGatesOnlyItsOwnClass(t *testing.T) {
	db := openPlaybackGateSchema(t)

	t.Run("the video switch does not gate a mock test paper", func(t *testing.T) {
		const userID = 705
		e := newPlaybackEnv(t, db, nil) // video ON, mock_test OFF
		cfg, err := e.api.config.Load()
		if err != nil {
			t.Fatalf("load the config: %v", err)
		}
		if !cfg.GateEnabled(ResourceTypeVideo) {
			t.Fatal("the video gate is off in this fixture; the independence claim is not being tested")
		}
		if cfg.GateEnabled(ResourceTypeMockTest) {
			t.Fatal("the mock-test gate is on in this fixture; the independence claim is not being tested")
		}

		// Paper 41 exists in no table this fixture has. An allow here can only
		// come from the switch being read as off, because every other step of the
		// decision would have produced a 404.
		decision := e.paper.AuthorizePaper(context.Background(), userID, 41, "Physics mock 2081")
		if !decision.Allowed {
			t.Fatalf("the video switch gated a mock test: %+v", decision.Refusal)
		}
		if decision.Charged != 0 {
			t.Errorf("the paper gate charged %d while its switch is off", decision.Charged)
		}
		if n := playbackSpendJournals(t, db, userID); n != 0 {
			t.Errorf("an ungated paper wrote %d spend journals", n)
		}
		if n := apiCount(t, db,
			`SELECT count(*) FROM resource_unlock WHERE user_id = ? AND resource_type = ?`,
			userID, ResourceTypeMockTest); n != 0 {
			t.Errorf("an ungated paper wrote %d mock_test unlock rows", n)
		}
	})

	t.Run("the document switch does not gate a video", func(t *testing.T) {
		const userID = 706
		e := newPlaybackEnv(t, db, func(cfg *EconomyConfig) {
			cfg.Gates.Video = false
			cfg.Gates.StudyResource = true
		})
		video := e.seedVideo(t, "Kinematics lecture 3", true)
		if got := apiUserAvailable(t, db, userID); got != 0 {
			t.Fatalf("wallet = %d, want an empty one; the fixture is wrong", got)
		}

		res := e.play(userID, video.ID)
		if !res.played() {
			t.Fatalf("the document switch refused a video: %d %s", res.Status, res.Raw)
		}
		assertVideoWroteNothing(t, e, userID, uint64(video.ID), 0)
	})

	t.Run("the video switch does not gate a document download", func(t *testing.T) {
		const userID = 707
		e := newPlaybackEnv(t, db, nil) // video ON, study_resource OFF
		doc := e.seedDocument(t, "Physics Past Questions 2081", studyresources.TypePastQuestions)
		if got := apiUserAvailable(t, db, userID); got != 0 {
			t.Fatalf("wallet = %d, want an empty one; the fixture is wrong", got)
		}

		// The document price is 40 and the video price is 90, so a class mix-up is
		// not a rounding difference: a 40-coin 402 here would be obvious, and a
		// silent charge would be caught by the wallet read.
		//
		// Only a coin refusal counts as gated. This route's 404 is the handler's own
		// "File not found" from object storage, which arrives after the gate has
		// already allowed the request — so the status cannot tell the two apart and
		// the download counter below is the witness.
		res := e.download(userID, doc.ID)
		if res.coinRefused() {
			t.Fatalf("the video switch gated a document download: %d %s", res.Status, res.Raw)
		}
		// A served download is identified by NOT being a coin refusal and by the
		// resource's own counter having moved. Object storage is a global client
		// with no local mode, so the bytes themselves are not what this asserts.
		var reloaded studyresources.StudyResource
		if err := db.First(&reloaded, doc.ID).Error; err != nil {
			t.Fatalf("reload %d: %v", doc.ID, err)
		}
		if reloaded.Downloads != 1 {
			t.Errorf("downloads = %d, want 1: the request did not get past the gate", reloaded.Downloads)
		}
		if rows := playbackUnlocksFor(t, db, userID, uint64(doc.ID)); len(rows) != 0 {
			t.Errorf("an ungated document download wrote %d VIDEO unlock rows", len(rows))
		}
		if n := apiCount(t, db,
			`SELECT count(*) FROM resource_unlock WHERE user_id = ?`, userID); n != 0 {
			t.Errorf("an ungated document download wrote %d unlock rows of any class", n)
		}
		if n := playbackSpendJournals(t, db, userID); n != 0 {
			t.Errorf("an ungated document download wrote %d spend journals", n)
		}
		if got := apiUserAvailable(t, db, userID); got != 0 {
			t.Errorf("wallet = %d, want 0", got)
		}
	})
}

// ── 7. concurrency ───────────────────────────────────────────────────────────

// The load-bearing test of this slice. N concurrent plays of the SAME unowned
// video by the SAME student must produce exactly one charge and exactly one
// unlock — and every one of them must be served.
//
// This is the double-tap, not the retry: a <video> element issues a second
// identical GET and nothing tells the server about it, so there is no idempotency
// key for the client to send. What stands between the student and a double charge
// is that the gate reuses coinPurchase, which spends and records inside ONE
// transaction under the per-user advisory lock, re-reading the live unlock on the
// way in. Across two transactions the loser's spend would commit before its
// unlock insert was refused, so it would be charged for a video it does not get —
// and Reverse refuses to refund a spend, so the coins would be gone for nothing.
//
// A NOTE ON THE KEYS, because it is a property of the route and not a choice the
// test makes: all eight requests mint the SAME derived key, because the key is
// derived from (user, class, id, price) and there is no client to send one. That
// is the design — a derived key is what makes a browser retry a REPLAY rather
// than a second charge — and it is why the losers are answered ALREADY_OWNED
// rather than charged. The "same resource, DIFFERENT keys" race is a claim about
// coinPurchase, and it is covered where the keys can actually differ:
// TestConcurrentDuplicateUnlocksProduceOneUnlockAndOneDebit in
// unlock_api_pg_test.go drives the unlock endpoint, which does take a client's
// key. Asserting eight different keys here would be asserting something the
// playback route cannot express.
//
// ── what this test was falsified against, and what that found ────────────────
//
// A concurrency test that cannot fail proves nothing, so each of the two
// protections was removed in turn and the test re-run. The result is worth
// recording, because it is not the same as the file header above assumes:
//
//   - BREAKING THE ADVISORY LOCK (dropping the pg_advisory_xact_lock in
//     Repository.InUserTx) DID NOT FAIL THIS TEST. Eight simultaneous transactions
//     still produced one unlock and one debit. What still stopped the duplicates
//     is the PARTIAL UNIQUE INDEX resource_unlock_live_uniq: the loser's insert
//     is refused, coinPurchase turns that into ErrAlreadyUnlocked, and
//     unlock() maps it to the already-owned answer with nothing charged.
//
//   - BREAKING THE IN-TRANSACTION RE-READ (the tx.ReadUnlock in coinPurchase)
//     ALSO DID NOT FAIL THIS TEST, for the same reason and by the same route.
//
//   - WHAT DOES FAIL IT, and is the only thing this test is really about, is the
//     ONE-TRANSACTION COMPOSITION. Replacing spendInTx's
//     NewLedger(NewRepository(tx.DB()), ...) with a spend on the outer repository
//     — so the debit commits on its own — fails immediately and in every
//     assertion: all eight plays answer 500, there are zero unlock rows and zero
//     spend journals, and the wallet is UNCHANGED at 720. The notification
//     emission fails with the rolled-back unlock, the errors surface as
//     GATE_UNAVAILABLE, and the coins are correctly not taken — which is the
//     point: a student pressing play eight times gets eight errors and no
//     charge, rather than a double charge.
//
// So the honest reading is that the advisory lock and the re-read are each
// individually REDUNDANT against the unique index for this workload, and the
// composition is the load-bearing part. That is a finding about the code, not
// about the test, and it is reported rather than fixed: removing either of the
// two would not change behaviour here, but both are cheap and one of them is what
// makes the loser ANSWER "already yours" rather than "we failed", which is the
// difference between a retry that works and a retry that surfaces a 500 to a
// student.
func TestConcurrentVideoPlaysByOneStudentChargeExactlyOnce(t *testing.T) {
	const (
		userID = 708
		fan    = 8
	)
	db := openPlaybackGateSchema(t)
	e := newPlaybackEnv(t, db, nil)
	video := e.seedVideo(t, "Kinematics lecture 3", true)
	// Enough for every request to be affordable, so the only thing stopping a
	// second debit is the system and not the balance.
	// NINE awards of 80 is 720, which is exactly eight prices of 90. The fixture
	// is built on that arithmetic deliberately: a wallet that could not cover every
	// request would let the ledger refuse a duplicate debit for lack of funds, and
	// the test would pass with a double charge behind it. The balance must never be
	// the thing that stops a second debit — that is the partial unique index and
	// the per-user advisory lock, and the point is to watch them do it.
	const awards = 9
	for i := 1; i <= awards; i++ {
		e.grant(t, userID, ReasonResourceApproved, "playback-race-grant-"+strconv.Itoa(i))
	}
	// Exhaust the video allowance so all eight are forced onto the coin path.
	// Left in place, the winner would be decided by the allowance machinery
	// instead of by the purchase composition, and this test would stop testing
	// the thing it exists for.
	e.exhaustVideos(t, userID, 7081)
	before := apiUserAvailable(t, db, userID)
	price := DefaultEconomyConfig().Prices.Video
	// The check is every price, not one: the balance assertion below is only
	// meaningful if all eight requests were AFFORDABLE, and if seven duplicate
	// charges were affordable too. A fixture that left the student with 90 coins
	// would let the ledger refuse the duplicate for lack of funds, and the test
	// would pass with the bug still in place.
	if before < fan*price {
		t.Fatalf("wallet = %d, want the %d that funds all %d requests at %d each; "+
			"a poorer fixture would let the balance hide a double charge", before, fan*price, fan, price)
	}

	results := make([]playbackResponse, fan)
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
			results[i] = e.play(userID, video.ID)
		}(i)
	}
	start.Done()
	done.Wait()

	// Every request is served. A 402 or a 500 here would mean the race produced a
	// failure rather than a duplicate answer, which is a different and also
	// unacceptable outcome: the student pressed play eight times and should have
	// eight videos and one bill.
	for i, got := range results {
		if !got.played() {
			t.Errorf("play %d = %d, want a served video (%s)", i, got.Status, got.Raw)
		}
	}

	// Exactly one unlock and exactly one debit.
	if rows := playbackUnlocksFor(t, db, userID, uint64(video.ID)); len(rows) != 1 {
		t.Errorf("live video unlocks for user %d = %d, want exactly 1", userID, len(rows))
	}
	if n := playbackSpendJournals(t, db, userID); n != 1 {
		t.Errorf("spend journals = %d, want exactly 1 — a concurrent play was charged twice", n)
	}
	if after := apiUserAvailable(t, db, userID); after != before-price {
		t.Errorf("wallet = %d, want %d (one debit of %d), so %d coins went missing or were taken twice",
			after, before-price, price, before-after)
	}
}

// The per-user half of the derived key, and the other direction of the lock:
// "once" means once per student, not once per video. The advisory lock is per user
// and the key is namespaced per student, so two students buying the same video
// each pay once and neither is charged for the other's purchase.
//
// The two purchases succeeding at all is the evidence about the key.
// (scope, idempotency_key) is unique across EVERY user journal in the table, so a
// key built from the resource alone would have made the second insert fail with a
// replay or a 409 — and a key built from the user alone would have made the
// second student replay the FIRST student's spend, which is a different and much
// worse answer: the video would be theirs and the coins would already be gone.
func TestTwoStudentsEachPayOnceForTheSameVideo(t *testing.T) {
	const (
		firstUser  = 709
		secondUser = 710
	)
	db := openPlaybackGateSchema(t)
	e := newPlaybackEnv(t, db, nil)
	video := e.seedVideo(t, "Kinematics lecture 3", true)
	// Two awards each, because the video price is 90 and the upload award is 80 —
	// a student who could not afford the video would be refused for the right
	// reason and this test would prove nothing about key collision.
	e.grant(t, firstUser, ReasonResourceApproved, "playback-two-a-1")
	e.grant(t, firstUser, ReasonResourceApproved, "playback-two-a-2")
	e.grant(t, secondUser, ReasonResourceApproved, "playback-two-b-1")
	e.grant(t, secondUser, ReasonResourceApproved, "playback-two-b-2")
	e.exhaustVideos(t, firstUser, 7091)
	e.exhaustVideos(t, secondUser, 7101)

	beforeFirst := apiUserAvailable(t, db, firstUser)
	beforeSecond := apiUserAvailable(t, db, secondUser)
	price := DefaultEconomyConfig().Prices.Video

	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for _, user := range []uint{firstUser, secondUser} {
		for i := 0; i < 2; i++ {
			done.Add(1)
			go func(user uint, i int) {
				defer done.Done()
				start.Wait()
				if res := e.play(user, video.ID); !res.played() {
					t.Errorf("user %d play %d was refused: %d %s", user, i, res.Status, res.Raw)
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
		if rows := playbackUnlocksFor(t, db, tc.user, uint64(video.ID)); len(rows) != 1 {
			t.Errorf("user %d holds %d video unlocks, want 1", tc.user, len(rows))
		}
		if n := playbackSpendJournals(t, db, tc.user); n != 1 {
			t.Errorf("user %d has %d spend journals, want 1: a key collision made one of them pay for the other",
				tc.user, n)
		}
		if after := apiUserAvailable(t, db, tc.user); after != tc.before-price {
			t.Errorf("user %d wallet = %d, want %d", tc.user, after, tc.before-price)
		}
	}
}

// The key itself, asserted directly. Everything above it — the losers replaying
// rather than charging, the second student not colliding with the first — is
// downstream of these three properties, and each of them is invisible from the
// outside: a random key per request would also produce one unlock, by luck of the
// entitlement check, and would double-charge on a retry where the entitlement is
// not there to catch it.
func TestThePlaybackGateMintsOneKeyPerStudentAndResource(t *testing.T) {
	const (
		userID  = 700
		otherID = 701
		videoID = 812
		price   = 90
	)
	first := deliveryGateIdempotencyKey(playbackGateKeyPrefix, userID, ResourceTypeVideo, videoID, price)

	if again := deliveryGateIdempotencyKey(playbackGateKeyPrefix, userID, ResourceTypeVideo, videoID, price); again != first {
		t.Errorf("two concurrent plays minted different keys: %q and %q", first, again)
	}
	if !strings.HasPrefix(first, playbackGateKeyPrefix) {
		t.Errorf("the minted key is not namespaced to this route: %q", first)
	}
	// The namespace is per route, not just per class, so the video-PLAYBACK gate
	// and the video-DOWNLOAD gate cannot replay one another's journals and each
	// slice stays independently revertable.
	if same := deliveryGateIdempotencyKey(gateIdempotencyKeyPrefix, userID, ResourceTypeVideo, videoID, price); same == first {
		t.Error("the playback gate and the download gate minted the same key for one video")
	}
	if same := deliveryGateIdempotencyKey(paperGateKeyPrefix, userID, ResourceTypeVideo, videoID, price); same == first {
		t.Error("the playback gate and the paper gate minted the same key")
	}
	// A key with no user in it would make the second student replay the first
	// student's spend, and (scope, idempotency_key) is unique across every user
	// journal in the table rather than per student.
	if other := deliveryGateIdempotencyKey(playbackGateKeyPrefix, otherID, ResourceTypeVideo, videoID, price); other == first {
		t.Error("two students minted the same key for one video, so one would pay for the other")
	}
	if other := deliveryGateIdempotencyKey(playbackGateKeyPrefix, userID, ResourceTypeVideo, videoID+1, price); other == first {
		t.Error("two videos minted the same key for one student")
	}
	// The price is in the key so a price change cannot turn a browser retry into a
	// 409 on a key the client has never seen and cannot reproduce.
	if other := deliveryGateIdempotencyKey(playbackGateKeyPrefix, userID, ResourceTypeVideo, videoID, price+1); other == first {
		t.Error("a price change reused the key, so a retry would be a 409 rather than a replay")
	}
}
