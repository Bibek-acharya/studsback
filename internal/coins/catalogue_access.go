// internal/coins/catalogue_access.go
//
// The catalogue access block — the disclosure half of the charge gate.
//
// ── why this file is the missing piece ─────────────────────────────────────────
//
// The gate is enforced at DELIVERY: download_gate.go, resource_gate.go and
// paper_gate.go all answer "no" once a class is gated. Nothing in the backend ever
// emitted a per-resource `access` block for the CATALOGUE, so `ResourceCard` resolved
// `resource.access` to null, drew no badge, and mounted no unlock dialog.
//
// Which means flipping `gates_enabled` to true would have produced the worst state
// available: a plain "Download" button that refuses when pressed. The charge enforced
// and was never disclosed — CPA 2075 s.16(2)(c)(3) arriving from the opposite
// direction to the one `charges_apply` in public_table.go exists to prevent.
//
// The frontend has been written against this contract throughout
// (`services/coinsApi.ts`'s `ResourceAccess`), so the shape here matches that rather
// than being invented.
//
// ── why it is a SESSION-SCOPED TWIN and not the public list ───────────────────
//
// The block carries `unlocked` (does this caller hold it) and a remaining allowance
// (how many included unlocks they have left). Both are per-user facts, and the public
// catalogue route has no session by design — 02's own route table calls it public, and
// `gate_reachability_test.go` pins that it discloses metadata only.
//
// Putting one per-user field on a public response is a leak whether or not the handler
// intends it, so this is a second route under the same path, behind authMW. Two tests
// hold the pair together: the anonymous caller is refused, and the public list carries
// no `access` key.
//
// ── the DOCUMENT class needs TWO switches ──────────────────────────────────────
//
// Same rule as the public coin table, and for the same reason: `gates_enabled` and
// `unlock_endpoint_enabled` are separate, and a document with only one of them has no
// single price — the site would charge at the download while POST /coins/unlock
// refuses, so the card would advertise something the server will not sell.
//
// ── ABSENT, never zero ────────────────────────────────────────────────────────
//
// An ungated class gets NO ENTRY in the map, not an entry with price 0. The frontend
// treats absent as "gate off" and renders the pre-feature card; a block of zeros would
// render as "costs 0 StudsTokens", which is a price nobody can read.

package coins

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"studsphere/backend/internal/shared/httpx"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

// CatalogueItem is the minimum a lister must return for the block to be computed,
// plus the catalogue fields a card renders.
//
// Deliberately not `studyresources.StudyResource`: this package imports
// `internal/studyresources`, not the reverse, and a lister adapter in main.go maps
// one to the other. Naming the minimal shape here means the annotation logic can be
// tested without the catalogue's whole model, and means a change to that model cannot
// silently change what is disclosed.
//
// The second half of that sentence is why the fields below are spelled out rather
// than embedded: THIS ROUTE IS THE CATALOGUE for every signed-in student, so an item
// that carried only id/title/type would render a card with no description, no
// course, no year, no download count and no mime type — the whole card, minus its
// price. What it deliberately does NOT carry is the moderation interior
// (approval_status, reviewed_by, reject_reason) or file_path: this endpoint exists to
// render a card, and a field added to the catalogue model later must not appear on a
// student-facing route without somebody deciding that it should.
//
// ResourceType is the CATALOGUE type ("past-questions", "video-lectures"), because
// the card renders it as a label and filters on it. The economy class the price was
// resolved against is derived — see catalogueClass — and reported separately on the
// block.
type CatalogueItem struct {
	ID           uint   `json:"id"`
	ResourceType string `json:"resource_type"`
	Title        string `json:"title,omitempty"`

	Description     string    `json:"description,omitempty"`
	Course          string    `json:"course,omitempty"`
	Year            string    `json:"year,omitempty"`
	FileName        string    `json:"file_name,omitempty"`
	FileURL         string    `json:"file_url,omitempty"`
	FileSize        int64     `json:"file_size,omitempty"`
	MimeType        string    `json:"mime_type,omitempty"`
	Downloads       int       `json:"downloads"`
	Views           int       `json:"views"`
	IsPublished     bool      `json:"is_published"`
	DurationSeconds int       `json:"duration_seconds,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// catalogueClass resolves the economy class an item is PRICED against.
//
// A lister can hand over either a catalogue type or a class name: the
// study-resources adapter has the former ("past-questions", "video-lectures")
// and nothing else, while a future mock-test lister would have the latter
// ("mock_test"). So the three class names are IDENTITY, and everything else is
// a study-resource catalogue type routed through studyResourceClass — which is
// the same mapping the download gate and the lookup already use, so the price a
// card shows, the class an unlock is recorded against, and the class the
// allowance is drawn from cannot disagree.
//
// This function is load-bearing in the way only a live deployment can show:
// without it, priceForClass hit its default branch for every document, and the
// gate being ON was indistinguishable from the gate being off — the price badge
// simply never appeared, on any card, for any student.
func catalogueClass(resourceType string) string {
	switch resourceType {
	case ResourceTypeStudyResource, ResourceTypeVideo, ResourceTypeMockTest:
		return resourceType
	default:
		return studyResourceClass(resourceType)
	}
}

// CatalogueAccess is the per-resource block the card renders from.
//
// Field-for-field with `services/coinsApi.ts`'s `ResourceAccess`. The comments are
// copied in spirit from there because that file is the contract and this one has to
// satisfy it.
type CatalogueAccess struct {
	// Price is the SERVER-RESOLVED cost. The client never derives it: a client that
	// can name the cost is an exploit (coinsApi.ts's own header).
	Price int64 `json:"price"`
	// Unlocked means the caller already holds this. Nothing is ever charged twice,
	// and 04 §4.4 records that a spend is not refundable — so this is what stops the
	// card asking a student for money they do not owe.
	Unlocked bool `json:"unlocked"`
	// Allowance is what is left of the included entitlement for this CLASS, and is
	// what turns "40 StudsTokens" into "you have 3 included unlocks". Nil when the
	// caller was never granted one, which is NOT the same as having used it — the
	// frontend distinguishes the two.
	Allowance *CatalogueAllowance `json:"allowance,omitempty"`
	// ResourceType is the class the price was resolved against, which differs from
	// the item's own type for the classes that price separately.
	ResourceType string `json:"resource_type,omitempty"`
}

// CatalogueAllowance is the remaining included-unlock entitlement for one class.
type CatalogueAllowance struct {
	Left      int64   `json:"left"`
	Total     int64   `json:"total"`
	ExpiresAt *string `json:"expires_at"`
}

// CatalogueEntitlements is what the annotation needs to know about ONE caller.
//
// A subset of `unlockEntitlements`, declared separately because a *Ledger does not
// hold a *Service and adding one would invert the dependency the two files currently
// have. `*Service` satisfies this unchanged, so main.go wires it with no adapter.
type CatalogueEntitlements interface {
	HasAccess(ctx context.Context, userID uint, resourceType string, resourceID uint64) (bool, error)
	RemainingAllowance(ctx context.Context, userID uint, now time.Time) (*AllowanceStatus, error)
}

// CatalogueLister returns the page of catalogue items to annotate.
//
// A PORT rather than a dependency on `*studyresources.Service`, for the direction
// reason above: this package may import studyresources, so it could take the concrete
// type — but a port keeps the annotation logic testable and lets main.go own the
// mapping, which is where the dependency between the two modules is visible.
type CatalogueLister interface {
	ListCatalogue(ctx context.Context, page, limit int) ([]CatalogueItem, error)
}

// AnnotatedCatalogueItem is one item with its block attached, or without.
type AnnotatedCatalogueItem struct {
	CatalogueItem
	// Access is OMITTED entirely when the class is ungated. `omitempty` on a pointer
	// is what makes that happen, and it is why the field is a pointer: a value type
	// would serialise as a zeroed block.
	Access *CatalogueAccess `json:"access,omitempty"`
}

// AnnotateCatalogue computes the access block for each item, keyed by resource id.
//
// Items whose class is ungated are ABSENT from the map. Callers attach blocks by
// lookup and leave the item alone on a miss, which is the "absent means ungated"
// contract the frontend depends on.
//
// The allowance is read ONCE for the whole page rather than per item: it is a
// per-CALLER entitlement, so every document on the page would otherwise issue the
// same query. A 20-item page is 20 identical reads of one row.
func (l *Ledger) AnnotateCatalogue(
	ctx context.Context, ent CatalogueEntitlements, userID uint, items []CatalogueItem,
) (map[uint64]*CatalogueAccess, error) {
	out := make(map[uint64]*CatalogueAccess, len(items))
	if l == nil || l.repo == nil {
		return out, ErrNoDatabase
	}
	// User 0 is not a student. The wallet endpoints refuse it, and annotating for it
	// would build an entitlement nobody holds.
	if userID == 0 || len(items) == 0 || ent == nil {
		return out, nil
	}

	cfg, err := l.config.Load()
	if err != nil {
		return out, err
	}
	// Nothing gated means nothing to say, and returning early avoids the allowance
	// read entirely on a deployment that has not turned anything on.
	if !anyClassChargeable(cfg) {
		return out, nil
	}

	now := l.now().UTC()
	status, err := ent.RemainingAllowance(ctx, userID, now)
	if err != nil {
		return out, err
	}

	for _, item := range items {
		// The CLASS, not the item's own type: it is what is priced, what an
		// entitlement is recorded against, and which allowance draws it down.
		class := catalogueClass(item.ResourceType)
		price, chargeable := priceForClass(cfg, class)
		if !chargeable {
			continue
		}
		unlocked, err := ent.HasAccess(ctx, userID, class, uint64(item.ID))
		if err != nil {
			return out, err
		}

		block := &CatalogueAccess{
			Price:        price,
			Unlocked:     unlocked,
			ResourceType: class,
		}
		if remaining := remainingForClass(status, class); remaining != nil {
			block.Allowance = remaining
		}
		out[uint64(item.ID)] = block
	}
	return out, nil
}

// anyClassChargeable reports whether ANY class currently has a real price.
//
// The document class needs both switches (see the file header); video and mock tests
// need only their gate, because neither has a second write path that could refuse a
// paid unlock the way the document download does.
func anyClassChargeable(cfg EconomyConfig) bool {
	documents := cfg.Gates.StudyResource && cfg.UnlockEndpointEnabled
	return documents || cfg.Gates.Video || cfg.Gates.MockTest
}

// priceForClass resolves the price for one class, and whether that class is
// CHARGEABLE — which is not the same as having a price.
//
// A configured price of zero is not a price; it is a statement that the class is not
// chargeable, so it never gets a block regardless of the gate.
func priceForClass(cfg EconomyConfig, class string) (int64, bool) {
	var (
		gate  bool
		price int64
	)
	switch class {
	case ResourceTypeStudyResource:
		gate, price = cfg.Gates.StudyResource && cfg.UnlockEndpointEnabled, cfg.Prices.StudyResource
	case ResourceTypeVideo:
		gate, price = cfg.Gates.Video, cfg.Prices.Video
	case ResourceTypeMockTest:
		gate, price = cfg.Gates.MockTest, cfg.Prices.MockTest
	default:
		// An unknown class is ungated, for the same reason EnabledFor returns false:
		// a gate that gated a class this build cannot price would charge a number it
		// cannot compute.
		return 0, false
	}
	if !gate || price <= 0 {
		return 0, false
	}
	return price, true
}

// remainingForClass pulls one class's remaining allowance out of the wallet's status.
//
// NIL when the caller has no allowance row, or none granted for THIS class. The
// frontend tells those two apart — "never granted" and "used up" are different
// sentences, and collapsing them would tell a student they have used up an entitlement
// they were never given.
func remainingForClass(status *AllowanceStatus, class string) *CatalogueAllowance {
	if status == nil || status.Classes == nil {
		return nil
	}
	perClass, ok := status.Classes[class]
	if !ok || perClass.Granted <= 0 {
		return nil
	}
	out := &CatalogueAllowance{
		Left:  perClass.Remaining,
		Total: perClass.Granted,
	}
	// ExpiresAt is a value on AllowanceStatus, not a pointer, so the zero time means
	// "no window" rather than "expired at midnight in year one".
	if !status.ExpiresAt.IsZero() {
		expires := status.ExpiresAt.UTC().Format("2006-01-02")
		out.ExpiresAt = &expires
	}
	return out
}

func expiresString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02")
	return &s
}

// CatalogueAccessAPI serves the session-scoped annotated catalogue.
type CatalogueAccessAPI struct {
	service *Service
	ledger  *Ledger
	lister  CatalogueLister
}

// NewCatalogueAccessAPI wires the endpoint. A nil lister answers 500 rather than an
// empty list, so a missing constructor call is visible instead of looking like a
// catalogue with nothing in it.
func NewCatalogueAccessAPI(service *Service, ledger *Ledger, lister CatalogueLister) *CatalogueAccessAPI {
	return &CatalogueAccessAPI{service: service, ledger: ledger, lister: lister}
}

// Access handles GET /api/v1/study-resources/access.
//
// SESSION REQUIRED, and the requirement is the whole reason this is a separate route.
func (a *CatalogueAccessAPI) Access(c *gin.Context) {
	if a == nil || a.service == nil || a.ledger == nil || a.lister == nil {
		response.Error(c, http.StatusInternalServerError, "The catalogue is not available")
		return
	}
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		// Refused before anything is read. A balance and an entitlement are per-user
		// facts and this response is the one place they are disclosed.
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 20)

	items, err := a.lister.ListCatalogue(c.Request.Context(), page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch study resources")
		return
	}
	blocks, err := a.ledger.AnnotateCatalogue(c.Request.Context(), a.service, userID, items)
	if err != nil {
		// 500 with no detail. An annotation failure must not fall back to "no block",
		// because that is indistinguishable from "gate off" and would show a student
		// a Download button that then refuses — the exact failure this file exists to
		// prevent. Failing loudly is the safer direction.
		response.Error(c, http.StatusInternalServerError, "Failed to resolve resource access")
		return
	}

	annotated := make([]AnnotatedCatalogueItem, 0, len(items))
	for _, item := range items {
		annotated = append(annotated, AnnotatedCatalogueItem{
			CatalogueItem: item,
			Access:        blocks[uint64(item.ID)],
		})
	}
	response.Success(c, http.StatusOK, "Study resources fetched successfully", gin.H{
		"page":            page,
		"limit":           limit,
		"study_resources": annotated,
	})
}

// RegisterCatalogueAccessRoutes mounts the annotated catalogue.
//
// A SEPARATE mount from `studyresources.RegisterRoutes` on purpose. That function owns
// the public catalogue, and the reachability tests assert that route discloses
// metadata only; putting a session-scoped twin inside it would put that assertion and
// this endpoint in the same group, where a future reader could not tell which rule
// applied where.
func RegisterCatalogueAccessRoutes(r *gin.Engine, authMW gin.HandlerFunc, api *CatalogueAccessAPI) {
	if r == nil || api == nil {
		return
	}
	v1 := r.Group("/api/v1")
	gated := v1.Group("/study-resources")
	gated.Use(authMW)
	{
		gated.GET("/access", api.Access)
	}
}

// queryInt reads a positive integer query parameter, falling back on a bad value.
//
// A malformed `?page=abc` falls back rather than 400-ing: the caller is a client that
// built the URL, and refusing to serve a catalogue over a typo in a pagination
// parameter is a worse failure than serving page 1.
func queryInt(c *gin.Context, key string, fallback int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
