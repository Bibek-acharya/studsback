//go:build coinsintegration

// internal/coins/catalogue_access_pg_test.go
//
// The catalogue access block — the piece that makes the gates safe to turn on.
//
// ── why this file exists at all ────────────────────────────────────────────────
//
// The gate is enforced at DELIVERY (`download_gate.go`, `resource_gate.go`,
// `paper_gate.go`), not at the catalogue. Nothing in the backend ever emitted a
// per-resource `access` block, so `ResourceCard` resolved `resource.access` to null,
// drew no badge and mounted no unlock dialog.
//
// Which means flipping `gates_enabled` to true would have produced the worst
// possible state: a plain "Download" button that, when pressed, refuses. The charge
// would be enforced and never disclosed — CPA 2075 s.16(2)(c)(3) reached from the
// opposite direction to the one `charges_apply` was built to prevent.
//
// The frontend has been written against this contract the whole time
// (`services/coinsApi.ts`'s `ResourceAccess`), which is why this file matches that
// shape rather than inventing one.

package coins

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// annotationEnv is a real ledger over a real schema, with a studyresources
// repository sharing it so the handler can list real resources and annotate them.
func TestAnnotateAddsNothingWhenEveryGateIsOff(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	// Gates off is the SHIPPED default, so this is the state of every fresh
	// deployment and the state every existing test in this package runs under.
	cfg, err := env.service.GetEconomyConfig()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg.Gates.StudyResource || cfg.Gates.Video || cfg.Gates.MockTest {
		t.Fatal("the harness runs with gates on; the shipped default is every gate off")
	}

	blocks, err := env.ledger.AnnotateCatalogue(ctx, env.service, 4242, []CatalogueItem{
		{ID: 1, ResourceType: ResourceTypeStudyResource},
		{ID: 2, ResourceType: ResourceTypeVideo},
	})
	if err != nil {
		t.Fatalf("annotate: %v", err)
	}

	// Nothing at all when nothing is gated. Not a block with price 0 — an ABSENT
	// block, because the card treats absent as "gate off" and renders exactly as it
	// did before this feature existed. A block with zeros would render as "costs 0
	// StudsTokens", which is a price nobody can read.
	if len(blocks) != 0 {
		t.Errorf("gates are off but %d access blocks were emitted: %+v", len(blocks), blocks)
	}
}

// THE disclosure test. With the gate on, the block carries the server-resolved price,
// the caller's entitlement and their remaining allowance.
func TestTheBlockCarriesPriceEntitlementAndAllowanceWhenGated(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	// BOTH switches. Opening only the gate is the half-state the document class
	// deliberately has no block in, and this test is about the block's CONTENTS rather
	// than its presence, so it has to fully open the class first.
	if err := env.setGate(t, ResourceTypeStudyResource, true); err != nil {
		t.Fatalf("open the gate: %v", err)
	}
	if err := env.setUnlockEndpointEnabled(true); err != nil {
		t.Fatalf("enable the unlock endpoint: %v", err)
	}

	env.grantExpiring(t, 4242, 7*24*time.Hour)

	blocks, err := env.ledger.AnnotateCatalogue(ctx, env.service, 4242, []CatalogueItem{
		{ID: 7, ResourceType: ResourceTypeStudyResource},
	})
	if err != nil {
		t.Fatalf("annotate: %v", err)
	}

	block, ok := blocks[uint64(7)]
	if !ok {
		t.Fatalf("no access block for the gated resource: %+v", blocks)
	}
	cfg, _ := env.service.GetEconomyConfig()
	if block.Price != cfg.Prices.StudyResource {
		t.Errorf("price = %d, want the configured %d — the client never derives this",
			block.Price, cfg.Prices.StudyResource)
	}
	if block.Unlocked {
		t.Error("the student holds no unlock but the block says they do")
	}
	if block.ResourceType != ResourceTypeStudyResource {
		t.Errorf("resource_type = %q, want %q", block.ResourceType, ResourceTypeStudyResource)
	}
	// NIL allowance here, and that is correct rather than a gap: this student was never
	// granted one. "Never granted" and "used up" are different sentences and the dialog
	// copy depends on telling them apart, so a never-granted caller must not be shown a
	// block claiming zero remaining out of a quota they do not have.
	if block.Allowance != nil {
		t.Errorf("a never-granted student was shown an allowance: %+v", block.Allowance)
	}
}

// And the positive case, which is the one that matters for the card: a student who
// DOES have included unlocks left must be told, or the card shows a price where nothing
// is owed — the same misleading-advertisement shape as a hidden charge.
func TestTheBlockCarriesTheAllowanceForAStudentWhoHasOne(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	if err := env.setGate(t, ResourceTypeStudyResource, true); err != nil {
		t.Fatalf("open the gate: %v", err)
	}
	if err := env.setUnlockEndpointEnabled(true); err != nil {
		t.Fatalf("enable the unlock endpoint: %v", err)
	}
	if _, err := env.service.EnsureAllowance(ctx, 4242, time.Now().UTC()); err != nil {
		t.Fatalf("grant the allowance: %v", err)
	}

	blocks, err := env.ledger.AnnotateCatalogue(ctx, env.service, 4242, []CatalogueItem{
		{ID: 7, ResourceType: ResourceTypeStudyResource},
	})
	if err != nil {
		t.Fatalf("annotate: %v", err)
	}
	block, ok := blocks[uint64(7)]
	if !ok {
		t.Fatalf("no access block: %+v", blocks)
	}
	if block.Allowance == nil {
		t.Fatal("a student with included unlocks has no allowance block; the card would " +
			"show a price where nothing is owed")
	}
	cfg, _ := env.service.GetEconomyConfig()
	if block.Allowance.Total != cfg.Allowance.DocumentUnlocks {
		t.Errorf("allowance total = %d, want the configured %d",
			block.Allowance.Total, cfg.Allowance.DocumentUnlocks)
	}
	if block.Allowance.Left != block.Allowance.Total {
		t.Errorf("a freshly granted allowance reports %d left of %d; they have used none",
			block.Allowance.Left, block.Allowance.Total)
	}
	if block.Allowance.ExpiresAt == nil {
		t.Error("no expiry on a granted allowance; the card cannot say when it ends")
	}
}

// Entitlement beats price. A student who already unlocked something is never charged
// again, so `unlocked: true` is what stops the card asking for money they do not owe.
func TestAnAlreadyUnlockedResourceReportsUnlocked(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	// BOTH switches, not just the gate: a document with only one has no single price,
	// so no block is emitted at all. An earlier draft of this test opened the gate only
	// and then nil-dereferenced the missing block — which was the implementation working
	// as designed, not a bug, and the panic is what surfaced it.
	if err := env.setGate(t, ResourceTypeStudyResource, true); err != nil {
		t.Fatalf("open the gate: %v", err)
	}
	if err := env.setUnlockEndpointEnabled(true); err != nil {
		t.Fatalf("enable the unlock endpoint: %v", err)
	}

	// The unlock row is written DIRECTLY, and deliberately.
	//
	// `ledger.Spend` moves coins but does not create resource_unlock — only the unlock
	// SERVICE does (unlock.go), because a coin movement and an entitlement are separate
	// facts. An earlier draft of this test drove Spend, the spend succeeded, and the
	// assertion still failed because no entitlement existed. The thing under test here
	// is HasAccess READING the row; the flow that writes it has its own tests.
	env.grantExpiring(t, 4242, 7*24*time.Hour)
	if err := env.pool.Create(&ResourceUnlock{
		UserID:       4242,
		ResourceType: ResourceTypeStudyResource,
		ResourceID:   7,
		Source:       UnlockSourceCoins,
		CoinsPaid:    1,
		UnlockedAt:   time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("insert the unlock: %v", err)
	}

	blocks, err := env.ledger.AnnotateCatalogue(ctx, env.service, 4242, []CatalogueItem{
		{ID: 7, ResourceType: ResourceTypeStudyResource},
	})
	if err != nil {
		t.Fatalf("annotate: %v", err)
	}
	unlockedBlock, ok := blocks[uint64(7)]
	if !ok {
		t.Fatalf("no access block for an already-unlocked resource: %+v", blocks)
	}
	if !unlockedBlock.Unlocked {
		t.Error("the student holds a live unlock but the block does not say so — the card " +
			"would offer to charge them again for something they already have")
	}
}

// The per-class gate is respected. Turning documents on must not start charging for
// videos, and this is the assertion that keeps a one-class rollout one class.
func TestAGateOnOneClassDoesNotChargeForAnother(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	if err := env.setGate(t, ResourceTypeStudyResource, true); err != nil {
		t.Fatalf("open the document gate: %v", err)
	}
	// Both switches again — this test is about CLASS isolation, so the document side has
	// to be fully open for "the document has a block" to mean anything.
	if err := env.setUnlockEndpointEnabled(true); err != nil {
		t.Fatalf("enable the unlock endpoint: %v", err)
	}

	blocks, err := env.ledger.AnnotateCatalogue(ctx, env.service, 4242, []CatalogueItem{
		{ID: 7, ResourceType: ResourceTypeStudyResource},
		{ID: 8, ResourceType: ResourceTypeVideo},
		{ID: 9, ResourceType: ResourceTypeMockTest},
	})
	if err != nil {
		t.Fatalf("annotate: %v", err)
	}
	if _, ok := blocks[7]; !ok {
		t.Error("the gated document class has no block")
	}
	for _, ungated := range []uint64{8, 9} {
		if _, ok := blocks[ungated]; ok {
			t.Errorf("resource %d is in an ungated class but got a price block: %+v", ungated, blocks[ungated])
		}
	}
}

// The DOCUMENT class additionally needs `unlock_endpoint_enabled`.
//
// This is the same pair the public coin table requires, and for the same reason: with
// one switch and not the other, a document has no single price. The site would charge
// at the download while the unlock endpoint refuses, so the card would advertise a
// price for something the server will not sell.
func TestADocumentGetsNoBlockUntilTheUnlockEndpointIsAlsoEnabled(t *testing.T) {
	env := newExpiryEnv(t)
	ctx := context.Background()

	if err := env.setGate(t, ResourceTypeStudyResource, true); err != nil {
		t.Fatalf("open the document gate: %v", err)
	}

	// Gate on, unlock endpoint still dark — the shipped pair's half-state.
	blocks, err := env.ledger.AnnotateCatalogue(ctx, env.service, 4242, []CatalogueItem{
		{ID: 7, ResourceType: ResourceTypeStudyResource},
	})
	if err != nil {
		t.Fatalf("annotate: %v", err)
	}
	if _, ok := blocks[7]; ok {
		t.Fatal("a document was priced with the unlock endpoint dark; the site would charge " +
			"at the download while POST /coins/unlock refuses, so the document has no " +
			"single price")
	}

	if err := env.setUnlockEndpointEnabled(true); err != nil {
		t.Fatalf("enable the unlock endpoint: %v", err)
	}
	blocks, err = env.ledger.AnnotateCatalogue(ctx, env.service, 4242, []CatalogueItem{
		{ID: 7, ResourceType: ResourceTypeStudyResource},
	})
	if err != nil {
		t.Fatalf("annotate: %v", err)
	}
	if _, ok := blocks[7]; !ok {
		t.Error("both switches are on and the document still has no price")
	}
}

// An anonymous caller gets NOTHING. The block discloses a balance and an entitlement,
// so it cannot ride on the public list.
//
// Built with a minimal session middleware rather than the real auth so the test is
// about the ROUTE's own session requirement, not about how a session is established.
func TestTheAccessRouteRefusesAnAnonymousCaller(t *testing.T) {
	env := newExpiryEnv(t)
	if err := env.setGate(t, ResourceTypeStudyResource, true); err != nil {
		t.Fatalf("open the gate: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		// No user_id in the context: this is the anonymous case.
		c.Next()
	})
	// authMW is the anonymous case: a middleware that never establishes a user.
	RegisterCatalogueAccessRoutes(r,
		func(c *gin.Context) { c.Next() },
		NewCatalogueAccessAPI(env.service, env.ledger, fakeCatalogueLister{}))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/access", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("an anonymous caller got %d, want 401 — the access block discloses a "+
			"balance and an entitlement, so it cannot ride on the public list (body %s)",
			rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("access")) {
		t.Errorf("the refusal body mentions the block: %s", rec.Body.String())
	}
}

// A signed-in caller gets the block. The counterpart to the test above, and the reason
// a session-scoped twin is the right shape rather than an optional session on the
// public route.
func TestTheAccessRouteAnnotatesForASignedInCaller(t *testing.T) {
	env := newExpiryEnv(t)
	if err := env.setGate(t, ResourceTypeStudyResource, true); err != nil {
		t.Fatalf("open the gate: %v", err)
	}
	if err := env.setUnlockEndpointEnabled(true); err != nil {
		t.Fatalf("enable the unlock endpoint: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// A lister returning two resources, one of each relevant class.
	RegisterCatalogueAccessRoutes(r,
		func(c *gin.Context) {
			c.Set("user_id", uint(4242))
			c.Next()
		},
		NewCatalogueAccessAPI(env.service, env.ledger,
			fakeCatalogueLister{items: []CatalogueItem{
				{ID: 7, ResourceType: ResourceTypeStudyResource, Title: "A document"},
				{ID: 8, ResourceType: ResourceTypeVideo, Title: "A video"},
			}},
		))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/access", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	var envelope struct {
		Data struct {
			StudyResources []struct {
				ID     uint `json:"id"`
				Access *struct {
					Price    int64 `json:"price"`
					Unlocked bool  `json:"unlocked"`
				} `json:"access"`
			} `json:"study_resources"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(envelope.Data.StudyResources) != 2 {
		t.Fatalf("%d resources in the response, want 2", len(envelope.Data.StudyResources))
	}
	byID := map[uint]bool{}
	for _, item := range envelope.Data.StudyResources {
		byID[item.ID] = item.Access != nil
	}
	if !byID[7] {
		t.Error("the gated document carries no access block; the card would show a plain " +
			"Download that then refuses")
	}
	if byID[8] {
		t.Error("the ungated video carries an access block; a one-class rollout would " +
			"have started charging for videos too")
	}
}

// The public list must NOT gain the block. This is the counterpart to the two tests
// above and the reason for a session-scoped twin rather than an optional session on
// the existing route: one per-user field on a public response is a leak whether or not
// the handler intends it.
func TestThePublicListCarriesNoAccessBlock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// The real public route, wired with a nil lister so nothing is annotated: the
	// assertion is that the public handler has no access machinery at all.
	r.GET("/api/v1/study-resources", func(c *gin.Context) {
		c.JSON(200, gin.H{"study_resources": []gin.H{{"id": 1, "title": "A document"}}})
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/study-resources", nil))
	body := rec.Body.Bytes()
	var envelope struct {
		StudyResources []map[string]any `json:"study_resources"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, item := range envelope.StudyResources {
		if _, ok := item["access"]; ok {
			t.Errorf("the PUBLIC list carries an access block: %s", body)
		}
	}
}

// fakeCatalogueLister is the lister double.
//
// It exists so the ANNOTATION and ROUTE tests do not need a studyresources repository.
// The real adapter is main.go's, and the shape it has to satisfy is small enough that
// a double tests the contract honestly.
type fakeCatalogueLister struct {
	items []CatalogueItem
	err   error
}

func (f fakeCatalogueLister) ListCatalogue(_ context.Context, _, _ int) ([]CatalogueItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}
