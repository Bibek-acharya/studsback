// internal/coins/unlock_api.go
//
// The student-facing wallet surface: the three reads of §2.1, §2.2 and §2.3
// plus the single write of §2.3.
//
// ── why the write path is dark ───────────────────────────────────────────────
//
// POST /api/v1/coins/unlock is mounted and answers 503 until an admin sets
// EconomyConfig.UnlockEndpointEnabled. Nothing consumes an unlock yet: the
// resource gates, which are what make a purchase worth making, land in the next
// slice. With the endpoint live today a student could spend real coins unlocking
// content they still have free access to, because the gate that would make the
// entitlement meaningful does not exist and nothing has been taken away. The
// ledger will not even refund it — Reverse explicitly refuses a spend, because
// a refund is a re-grant and not a clawback — so the coins would be gone for
// nothing. The slice that adds the first gate is the one that turns this on.
//
// The three reads are always available. They expose only the caller's own
// wallet, they move nothing, and they are exactly what the frontend needs to
// build the unlock affordance before the affordance can be paid for.
//
// ── the ordering inside unlock, and why each step is where it is ─────────────
//
//  1. The Idempotency-Key header is required and is checked before anything
//     else. It is the only thing that makes a mobile retry safe, and a retry is
//     the normal case rather than the exception. Doing it first also means the
//     request is refused before it can open a transaction or take a user lock.
//
//  2. Entitlement FIRST. A student who already holds a live unlock gets 200
//     with already_unlocked: true and coins_paid: 0, and NEVER a 403. A 403 is
//     a client-error status, so the frontend renders a failure for an outcome
//     that succeeded, and the retry the student just pressed looks broken.
//     ErrAlreadyUnlocked exists as a domain sentinel precisely so the handler
//     can answer it as a success; see errors.go.
//
//  3. The allowance BEFORE the balance. A student with zero coins and an unused
//     starter unlock must succeed with coins_paid: 0 and used_allowance: true.
//     Checking the balance first would block exactly the students the allowance
//     exists for, and would send them off to earn coins they did not need.
//
//  4. Otherwise spend through the ledger and record the entitlement, in ONE
//     transaction, with the journal id and the coins actually paid.
//
//  5. Any notification is emitted inside that same transaction. The one event a
//     purchase produces is coins.debited, announced to the student who paid; it
//     is written through the real registry over the OPEN transaction, so a
//     notification that cannot be written takes the purchase down with it rather
//     than leaving a debit nobody was told about. See unlockNotifier.
//
// ── the price ────────────────────────────────────────────────────────────────
//
// Resolved server-side from EconomyConfig.Prices by the ledger's own
// spendPrice, keyed on the resource class. The request body carries no amount
// and UnlockRequest has no field to carry one. The client's key is passed
// STRAIGHT THROUGH to the ledger journal — never re-derived, never prefixed,
// never salted with anything the client did not send — because a key the server
// rewrites is a key a retry cannot reproduce, and a retry that cannot reproduce
// its key double-charges.
package coins

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/httpx"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ── the seams ────────────────────────────────────────────────────────────────
//
// Narrow interfaces rather than *Service and *Ledger fields, for one reason: the
// ordering in unlock IS the contract of §2.3, and a table test over that
// ordering has to drive "already owned", "allowance exhausted", "cannot afford"
// and "succeeds" without a database. The concrete types satisfy these exactly —
// *Service has these three entitlement methods with these signatures and *Ledger
// has Spend — so production wiring passes them unchanged and there is no adapter
// to keep in step.

type unlockEntitlements interface {
	HasAccess(ctx context.Context, userID uint, resourceType string, resourceID uint64) (bool, error)
	ConsumeAllowance(ctx context.Context, userID uint, resourceType string, resourceID uint64, now time.Time) (*ResourceUnlock, error)
	RemainingAllowance(ctx context.Context, userID uint, now time.Time) (*AllowanceStatus, error)
}

type unlockWalletReader interface {
	WalletBalance(ctx context.Context, userID uint, now time.Time) (*WalletBalance, error)
	TransactionPage(ctx context.Context, userID uint, cursor transactionCursor, limit int) ([]TransactionDTO, string, error)
	ProfileInstalmentsPaid(ctx context.Context, userID uint) (int64, error)
	SoonestLotExpiry(ctx context.Context, userID uint, now time.Time) (*time.Time, error)
	HasResourceApprovedAward(ctx context.Context, userID uint) (bool, error)
}

type economyConfigReader interface {
	Load() (EconomyConfig, error)
}

// ProfileEligibility answers "may this student still take the profile route?".
//
// It is an interface because the answer belongs to another module. The twelve
// checks that produce it are studentdashboard's computeProfileCompletion and are
// evaluated THERE; this package must not grow a second copy, or the wallet and
// the profile page start disagreeing about the same student. main.go wires the
// adapter.
type ProfileEligibility interface {
	ProfileComplete(ctx context.Context, userID uint) (bool, error)
}

// ResourceLookup resolves a (class, id) pair to a resource the caller may buy,
// or returns ErrNotFound.
//
// It exists because 404 RESOURCE_NOT_FOUND means "unknown id, or not published,
// or wrong type", and none of those three questions can be answered by this
// package: the row lives in another module's table. The seam is implemented
// there — NewStudyResourceLookup in study_resource_lookup.go answers it from
// internal/studyresources for the two classes that module owns.
//
// The returned title is not decoration. It is what lets the debit receipt and
// the wallet history name the thing that was bought instead of rendering
// "study_resource 812" at the student, and a lookup that cannot produce one is
// allowed to return an empty Title: every caller falls back to the class words
// rather than failing an unlock over a label.
type ResourceLookup interface {
	LookupUnlockable(ctx context.Context, resourceType string, resourceID uint64) (UnlockableResource, error)
}

// UnlockableResource is what a ResourceLookup resolved: proof that the resource
// exists, is published and is of the class that was asked for.
type UnlockableResource struct {
	// Title is the resource's stored title, or "" when the lookup has none.
	Title string
}

// unlockNotifier is the §5 notification seam.
//
// It takes the transaction handle so the emission can be atomic with the unlock,
// and NewUnlockAPI wires the real implementation (registryNotifier) by default:
// notification.NotifyTx over the caller's tx. A caller that wants a different
// behaviour — a test asserting the rollback, a module that batches inbox writes —
// replaces it with WithNotifier, which is how the seam stays a seam without being
// an unlit one.
type unlockNotifier interface {
	NotifyUnlockTx(ctx context.Context, tx *gorm.DB, userID uint, event UnlockEvent) error
}

// UnlockEvent is what a notifier is told about a purchase that settled.
//
// It carries the journal id, the class and id that were unlocked, the coins
// actually paid and the balance after. It does NOT carry a title or a price
// string: this package cannot read the resource tables, so anything it invented
// about the resource would be a guess rendered into somebody's inbox.
type UnlockEvent struct {
	// Title is the resource's real name, resolved through ResourceLookup when one
	// is wired, and empty otherwise. It is the one field about the RESOURCE that
	// this type can carry, and it is what lets a receipt name what was bought
	// rather than naming its class. Every consumer falls back to the class words
	// when it is empty, so an unresolved title never changes an outcome — it
	// only changes how a sentence reads.
	Title        string
	JournalID    string
	ResourceType string
	ResourceID   uint64
	CoinsPaid    int64
	BalanceAfter int64
	Source       string
}

// registryNotifier is the one production implementation of the seam: the real
// registry, the real notification service, the caller's transaction.
//
// It does not look the resource up to name it — that is what the class in
// UnlockEvent is for. The gate slice, which can read studyresources / mocktests /
// pressmedia / downloadcenter, passes a better label through without changing
// this type.
type registryNotifier struct {
	notifier *notification.Service
}

// unlockItemLabel is the class in the words the copy deck uses for it.
//
// A resolved title REPLACES the class words rather than qualifying them, and
// that is the point of the ResourceLookup: "Physics Past Questions 2081" is
// what the student bought, while "study resource" is what this package knew
// about it when the class was all it had. An empty title falls back to the
// class, so a module with no lookup wired produces the old sentence rather than
// a blank one.
func unlockItemLabel(resourceType, title string) string {
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		return trimmed
	}
	switch resourceType {
	case ResourceTypeVideo:
		return "video lecture"
	case ResourceTypeMockTest:
		return "mock test"
	default:
		return "study resource"
	}
}

// unlockNotificationRequest is the request for one debit, split out so the test
// can render the registry's templates from the exact Data the emit uses — the
// contract templates have is missingkey=error, so "the key is in the map" is a
// fact worth asserting rather than a hope.
func unlockNotificationRequest(userID uint, event UnlockEvent) notification.NotifyRequest {
	return notification.NotifyRequest{
		EventKey:   notification.EventCoinsDebited,
		Recipients: []notification.Ref{{Type: "user", ID: userID}},
		Data: map[string]any{
			// The coin figures the copy deck allows, and nothing else: no
			// currency, no "free", no windfall-gain vocabulary.
			"coins":   event.CoinsPaid,
			"balance": event.BalanceAfter,
			"item":    unlockItemLabel(event.ResourceType, event.Title),
			"journal": event.JournalID,
		},
		// One emission per journal, and the journal id IS the purchase. It also
		// gives a retried emit a natural identity instead of a second inbox row
		// for one debit.
		OccurrenceKey: notification.EventCoinsDebited + ":" + event.JournalID,
		CorrelationID: event.JournalID,
	}
}

func (n *registryNotifier) NotifyUnlockTx(ctx context.Context, tx *gorm.DB, userID uint, event UnlockEvent) error {
	return n.notifier.NotifyTx(ctx, tx, unlockNotificationRequest(userID, event))
}

// purchaseOutcome is what step 4 decided.
type purchaseOutcome struct {
	CoinsPaid    int64
	BalanceAfter int64
	SpentFrom    []SpentFromDTO
	UnlockID     uint
	JournalID    string
	Replayed     bool
	// AlreadyUnlocked is set when the transaction found a live unlock it did
	// not create. See the concurrency argument on coinPurchase.
	AlreadyUnlocked bool
}

// UnlockAPI is the wallet surface. One object for the four routes so they share
// the config read, the clock and the eligibility lookups.
type UnlockAPI struct {
	entitlements unlockEntitlements
	wallet       unlockWalletReader
	repo         *Repository
	ledger       *Ledger
	config       economyConfigReader
	profiles     ProfileEligibility
	resources    ResourceLookup
	notifier     unlockNotifier
	now          func() time.Time

	// purchase is step 4 as a field, so the ordering test can drive the four
	// outcomes without a database. NewUnlockAPI points it at coinPurchase.
	purchase func(ctx context.Context, userID uint, req UnlockRequest, key string) (purchaseOutcome, error)
}

// NewUnlockAPI wires the wallet surface over the concrete service and ledger.
//
// A *Service built by NewService alone has no repository; the routes then answer
// 500 rather than panicking, which is what unlock.go's requireRepo documents for
// the domain methods and what ready below does for the handlers.
func NewUnlockAPI(service *Service, ledger *Ledger) *UnlockAPI {
	api := &UnlockAPI{
		entitlements: service,
		ledger:       ledger,
		now:          func() time.Time { return time.Now().UTC() },
	}
	if ledger != nil {
		api.wallet = ledger.repo
		api.repo = ledger.repo
	}
	if service != nil {
		api.repo = service.repo
		if service.repo != nil {
			api.wallet = service.repo
		}
		if service.config != nil {
			api.config = service.config
		}
	}
	// The notification seam is wired by DEFAULT, over the same handle the
	// purchase transaction runs on. It used to be left nil because the closed
	// registry had no coins.debited row and a key with no row stops the server
	// booting; the row exists now, so the debit is announced rather than
	// swallowed. WithNotifier still replaces it.
	if api.repo != nil && api.repo.db != nil {
		api.notifier = &registryNotifier{notifier: notification.NewService(api.repo.db)}
	}
	api.purchase = api.coinPurchase
	return api
}

// WithProfileEligibility wires the profile-completion lookup ways_to_earn needs.
// Optional: with no lookup the profile route is not offered, because telling a
// student to complete something this server cannot check is the drift the
// function exists to prevent.
func (a *UnlockAPI) WithProfileEligibility(p ProfileEligibility) *UnlockAPI {
	a.profiles = p
	return a
}

// WithResourceLookup wires the 404 check and the title that names a purchase.
// See ResourceLookup.
func (a *UnlockAPI) WithResourceLookup(r ResourceLookup) *UnlockAPI {
	a.resources = r
	return a
}

// titleFor is a resource's name, or "" when this build cannot name it.
//
// It never returns an error. The three callers are a receipt, a history line
// and a gate decision, and in all three a missing title is a worse sentence
// rather than a failed request: a notification about a purchase that did
// settle must not be dropped because a title could not be read, and a history
// list must not 500 over a label.
func (a *UnlockAPI) titleFor(ctx context.Context, resourceType string, resourceID uint64) string {
	resolved, err := a.lookupUnlockable(ctx, resourceType, resourceID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(resolved.Title)
}

// lookupUnlockable resolves a class and id through the wired ResourceLookup.
//
// With no lookup wired it is a no-op that resolves to an untitled resource
// rather than an error. That is not a hole: the gate path reads the resource
// from the module that owns it before it ever gets here, so the gate's 404 comes
// from that module and this is only the standalone unlock endpoint's own check.
// The fallback keeps the two answers from disagreeing about whether the
// endpoint is usable when the lookup is absent, and the "dark until wired"
// decision stays where it was made — on the flags.
func (a *UnlockAPI) lookupUnlockable(ctx context.Context, resourceType string, resourceID uint64) (UnlockableResource, error) {
	if a.resources == nil {
		return UnlockableResource{}, nil
	}
	return a.resources.LookupUnlockable(ctx, resourceType, resourceID)
}

// WithNotifier wires the notification seam. See unlockNotifier.
func (a *UnlockAPI) WithNotifier(n unlockNotifier) *UnlockAPI {
	a.notifier = n
	return a
}

// ── GET /api/v1/coins/balance ────────────────────────────────────────────────

// Balance handles GET /api/v1/coins/balance.
//
// The hot path: read on every wallet render and on every unlock attempt, which
// is why it serves the cached projection rather than SUM() over postings and why
// the allowance is one indexed count.
func (a *UnlockAPI) Balance(c *gin.Context) {
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !a.ready(c) {
		return
	}
	now := a.now()
	balance, err := a.wallet.WalletBalance(c.Request.Context(), userID, now)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Could not read the wallet")
		return
	}
	allowance, err := a.entitlements.RemainingAllowance(c.Request.Context(), userID, now)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Could not read the free allowance")
		return
	}
	response.Success(c, http.StatusOK, "Wallet balance", balanceResponseFrom(balance, allowance))
}

// ── GET /api/v1/coins/allowance ──────────────────────────────────────────────

// Allowance handles GET /api/v1/coins/allowance: the same object the balance
// body carries, on its own route, because the wallet render and the unlock
// affordance need it at different times and neither should ask for the whole
// wallet to get it.
func (a *UnlockAPI) Allowance(c *gin.Context) {
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !a.ready(c) {
		return
	}
	status, err := a.entitlements.RemainingAllowance(c.Request.Context(), userID, a.now())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Could not read the free allowance")
		return
	}
	response.Success(c, http.StatusOK, "Free allowance", allowanceDTOFrom(status))
}

// ── GET /api/v1/coins/transactions ───────────────────────────────────────────

// defaultTransactionPageSize is 20: small enough that a student's whole history
// is a handful of pages, large enough that the next page is not a keystroke.
const defaultTransactionPageSize = 20

// maxTransactionPageSize caps what a caller may ask for, so ?limit=1000000
// cannot turn a wallet render into a table scan.
const maxTransactionPageSize = 100

// ListTransactions handles GET /api/v1/coins/transactions.
//
// Scoped to the authenticated user IN SQL rather than filtered in Go: the query
// has to page with a keyset, and a keyset applied to "all journals, then drop
// the ones that are not yours" is not a keyset.
func (a *UnlockAPI) ListTransactions(c *gin.Context) {
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !a.ready(c) {
		return
	}
	cursor, err := decodeTransactionCursor(strings.TrimSpace(c.Query("cursor")))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "The cursor is not a page token this server issued")
		return
	}
	limit := defaultTransactionPageSize
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			response.Error(c, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > maxTransactionPageSize {
			parsed = maxTransactionPageSize
		}
		limit = parsed
	}

	items, next, err := a.wallet.TransactionPage(c.Request.Context(), userID, cursor, limit)
	if err != nil {
		if errors.Is(err, ErrInvalidArgument) {
			response.Error(c, http.StatusBadRequest, "The cursor is not a page token this server issued")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Could not read the transaction history")
		return
	}
	a.nameUnlockedItems(c.Request.Context(), items)
	response.Success(c, http.StatusOK, "Transaction history", TransactionsResponse{Items: items, NextCursor: next})
}

// nameUnlockedItems replaces the placeholder in an unlock's description with the
// resource's real title, so a student's history reads "Unlocked: Physics Past
// Questions 2081" rather than "Unlocked: study_resource 812".
//
// The title is resolved HERE, on the read, and not stored on the journal, for
// two reasons. A journal is immutable, so a title frozen at purchase time would
// keep naming a resource after an admin renames it; and the history is a read of
// a resource module this package only reaches through the lookup seam, so the
// name is as current as the page is.
//
// Cost is bounded by the page, not by the history: at most one indexed read per
// distinct (class, id) on this page, memoised because a student who bought
// several papers from one course would otherwise re-read the same row per row.
// It is best-effort — an unresolvable resource, an unwired lookup or a failed
// read leaves the existing description in place. A wallet that renders slightly
// worse is the right outcome for a history list; a history that 500s because a
// title could not be read is not.
func (a *UnlockAPI) nameUnlockedItems(ctx context.Context, items []TransactionDTO) {
	if a.resources == nil {
		return
	}
	type ref struct {
		class string
		id    uint64
	}
	seen := make(map[ref]string)
	for i := range items {
		if items[i].ReasonCode != ReasonResourceUnlock || items[i].Ref == nil {
			continue
		}
		key := ref{class: items[i].Ref.Type, id: items[i].Ref.ID}
		title, ok := seen[key]
		if !ok {
			resolved, err := a.lookupUnlockable(ctx, key.class, key.id)
			if err != nil {
				continue
			}
			title = resolved.Title
			seen[key] = title
		}
		items[i].Description = describeUnlocked(key.class, title, key.id)
	}
}

// ── POST /api/v1/coins/unlock ────────────────────────────────────────────────

// UnlockResource handles POST /api/v1/coins/unlock.
//
// The feature gate comes before the idempotency key and before the body. The
// ordering in the file header starts at the key because that is the first step
// of the UNLOCK PATH; whether the endpoint exists at all is a different
// question, and a 503 "this is switched off" is a truer answer than a 400 about
// a header on a request that was never going to run.
func (a *UnlockAPI) UnlockResource(c *gin.Context) {
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		respondUnlockError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required", nil)
		return
	}
	if !a.ready(c) {
		respondUnlockError(c, http.StatusInternalServerError, CodeInternal,
			"The coin service is not connected to a database.", nil)
		return
	}
	cfg, err := a.config.Load()
	if err != nil {
		respondUnlockError(c, http.StatusInternalServerError, CodeInternal,
			"Could not read the coin economy configuration.", nil)
		return
	}
	if !cfg.UnlockEndpointEnabled {
		respondUnlockError(c, http.StatusServiceUnavailable, CodeEndpointDisabled,
			"Unlocking is not switched on yet. Your wallet and free allowance are unaffected and nothing is charged.", nil)
		return
	}

	// Step 1. The key is required, and it is checked before the body is read: a
	// request with no key can never be safely retried, so there is no point
	// parsing what it wanted.
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		respondUnlockError(c, http.StatusBadRequest, CodeInvalidIdempotencyKey,
			"An Idempotency-Key header is required to unlock.", nil)
		return
	}
	if err := validateIdempotencyKey(key); err != nil {
		respondUnlockError(c, http.StatusBadRequest, CodeInvalidIdempotencyKey,
			"The Idempotency-Key header is malformed: it must be 1 to 255 bytes.", nil)
		return
	}

	var req UnlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondUnlockError(c, http.StatusBadRequest, CodeInvalidRequest,
			`The request body must be {"resource_type":...,"resource_id":...}.`, nil)
		return
	}
	if err := validateResourceType(req.ResourceType); err != nil {
		respondUnlockError(c, http.StatusBadRequest, CodeInvalidRequest,
			"resource_type must be one of "+strings.Join(ResourceTypes, ", ")+".", nil)
		return
	}
	if req.ResourceID == 0 {
		respondUnlockError(c, http.StatusBadRequest, CodeInvalidRequest,
			"resource_id must be a real resource id; 0 is not one.", nil)
		return
	}
	// 404 RESOURCE_NOT_FOUND — "unknown id, or not published, or wrong type".
	// Reachable only once a ResourceLookup is wired; see that interface.
	// 404 RESOURCE_NOT_FOUND — "unknown id, or not published, or wrong type".
	// Reachable because a ResourceLookup is wired; see that interface. The
	// resolved title is not used here: this endpoint's own response describes the
	// class, and the wallet history is where a name is wanted.
	if _, err := a.lookupUnlockable(c.Request.Context(), req.ResourceType, req.ResourceID); err != nil {
		respondUnlockError(c, walletStatusFor(err), walletErrorCode(err),
			"That resource does not exist, is not published, or is not of that type.", nil)
		return
	}

	result, err := a.unlock(c.Request.Context(), userID, req, key, cfg)
	if err != nil {
		a.respondUnlockFailure(c, err, result, cfg)
		return
	}
	response.Success(c, http.StatusOK, "Unlocked", UnlockResponse{
		Unlocked:        result.Unlocked,
		AlreadyUnlocked: result.AlreadyUnlocked,
		CoinsPaid:       result.CoinsPaid,
		BalanceAfter:    result.BalanceAfter,
		SpentFrom:       result.SpentFrom,
		UsedAllowance:   result.UsedAllowance,
	})
}

// UnlockResult is the domain-shaped outcome of one unlock attempt, before it
// becomes a response body. It exists so the ordering is testable without gin,
// and so the failure path can still say what it had learned (in particular that
// the allowance had lapsed) on its way to an error.
type UnlockResult struct {
	Unlocked        bool
	AlreadyUnlocked bool
	UsedAllowance   bool
	CoinsPaid       int64
	BalanceAfter    int64
	SpentFrom       []SpentFromDTO
	UnlockID        uint
	JournalID       string
	// UserID and Required travel with the result, including on the error path,
	// because the 402 body needs both and the failure has already rolled back by
	// the time the handler renders it. Required is the server-resolved price.
	UserID   uint
	Required int64
	// AllowanceExpired records that the starter allowance had LAPSED. It does
	// not change a successful purchase — a student whose free unlocks ran out
	// can still buy — but it changes a failed one, because §2.3 makes the 423
	// the answer for "the allowance lapsed and no purchase covers them".
	AllowanceExpired bool
	// Replayed marks a response reconstructed from an existing journal rather
	// than created by this request.
	Replayed bool
}

// unlock is the whole write path, in the order the file header sets out.
func (a *UnlockAPI) unlock(ctx context.Context, userID uint, req UnlockRequest, key string, cfg EconomyConfig) (UnlockResult, error) {
	now := a.now()

	// The price, resolved from config and nothing else, is computed before
	// anything can be refused so the 402 body can report it even when the
	// purchase never happens. It is the same spendPrice the ledger resolves for
	// itself, so a refusal cannot quote a different number from the one the
	// purchase would have charged. A resolution failure here is not fatal: the
	// purchase path will refuse it properly, and the 402 is only reachable once
	// a price exists.
	required, _ := spendPrice(cfg, ReasonResourceUnlock, req.ResourceType)
	base := UnlockResult{UserID: userID, Required: required}

	// ── step 2: entitlement first ────────────────────────────────────────────
	//
	// Before the allowance, before the price, before the balance. A student who
	// already holds this must be told so in the shape of a success and must not
	// be charged. It is one indexed existence check against
	// resource_unlock_live_uniq, and it is what makes a mobile retry cheap: the
	// retry lands here and stops.
	has, err := a.entitlements.HasAccess(ctx, userID, req.ResourceType, req.ResourceID)
	if err != nil {
		return base, err
	}
	if has {
		return a.alreadyUnlocked(ctx, userID, base)
	}

	// ── step 3: the allowance, before the balance ────────────────────────────
	//
	// A student with zero coins and an unused starter unlock must succeed here,
	// with coins_paid 0. Reading the balance first would refuse exactly the
	// students the allowance exists for.
	allowanceExpired := false
	unlock, err := a.entitlements.ConsumeAllowance(ctx, userID, req.ResourceType, req.ResourceID, now)
	switch {
	case err == nil:
		balance, balErr := a.balanceAfter(ctx, userID)
		if balErr != nil {
			return base, balErr
		}
		base.Unlocked = true
		base.UsedAllowance = true
		base.CoinsPaid = 0
		base.BalanceAfter = balance
		base.SpentFrom = []SpentFromDTO{}
		base.UnlockID = unlock.ID
		return base, nil

	case errors.Is(err, ErrAlreadyUnlocked):
		// Lost the race to a concurrent request that got there first. The other
		// request holds the entitlement, so this is the ALREADY_OWNED answer and
		// not an error — §2.3 is explicit that a 403 here would render a failure
		// for an outcome that succeeded.
		return a.alreadyUnlocked(ctx, userID, base)

	case errors.Is(err, ErrNoAllowanceRemaining):
		// Fall through to the purchase. An exhausted allowance is not a reason to
		// refuse: it is the reason to charge them. ErrAllowanceExpired rides
		// along with this error and is recorded, because it changes what a FAILED
		// purchase answers (423 rather than 402).
		allowanceExpired = errors.Is(err, ErrAllowanceExpired)

	case errors.Is(err, ErrInvalidResourceType), errors.Is(err, ErrInvalidArgument):
		return base, err

	default:
		return base, err
	}

	// ── step 4: the purchase ────────────────────────────────────────────────
	outcome, err := a.purchase(ctx, userID, req, key)
	if err != nil {
		// ErrAlreadyUnlocked escaping the purchase is the same answer as the one
		// step 2 gives, reached by a different route. The per-user advisory lock
		// means coinPurchase re-reads the live unlock under the same lock the
		// winner held, so it should never produce this — but the mapping is
		// total rather than conditional, because a 500 for "you already own it"
		// is the exact failure §2.3's ALREADY_OWNED row exists to prevent.
		if errors.Is(err, ErrAlreadyUnlocked) {
			return a.alreadyUnlocked(ctx, userID, base)
		}
		// The result travels with the error: an allowance that had lapsed turns
		// a 402 into a 423, and that has to be known here, where the allowance
		// was read.
		base.AllowanceExpired = allowanceExpired
		return base, err
	}
	if outcome.AlreadyUnlocked {
		balance, balErr := a.balanceAfter(ctx, userID)
		if balErr != nil {
			return base, balErr
		}
		base.Unlocked = true
		base.AlreadyUnlocked = true
		base.CoinsPaid = 0
		base.BalanceAfter = balance
		base.SpentFrom = []SpentFromDTO{}
		base.UnlockID = outcome.UnlockID
		return base, nil
	}
	base.Unlocked = true
	base.CoinsPaid = outcome.CoinsPaid
	base.BalanceAfter = outcome.BalanceAfter
	base.SpentFrom = outcome.SpentFrom
	base.UnlockID = outcome.UnlockID
	base.JournalID = outcome.JournalID
	base.Replayed = outcome.Replayed
	return base, nil
}

// coinPurchase is step 4: spend and record, in ONE transaction.
//
// Two transactions would be what the domain layer can express on its own —
// unlock.go documents Spend-then-RecordCoinUnlock as the seam a future gate
// would use. The gate that COMPOSES them is where the composition belongs, and
// it is what makes the concurrent duplicate safe:
//
// Two requests for the same resource, both of which passed the step-2 read before
// either wrote. Across two transactions the loser's spend COMMITS before its
// unlock insert is refused by resource_unlock_live_uniq, so it is charged for
// something it does not get — and Reverse will not give the coins back, because
// a spend is refunded by re-granting, not by clawing back. In one transaction
// the loser re-reads the live unlock under the same per-user advisory lock the
// winner took, sees it, and writes nothing at all.
//
// The spend itself is still Ledger.Spend, called over a Repository bound to the
// OPEN transaction handle. GORM runs a nested Transaction() as a SAVEPOINT on
// the same connection, so the journal, the postings, the lots, the cached
// balance, the unlock and any notification are one atomic unit, and the ledger's
// decision logic is not re-implemented here. The per-user advisory lock is taken
// once, by the outer InUserTx; re-entering it on the same session is a no-op,
// which is what makes the nesting safe rather than a self-deadlock.
func (a *UnlockAPI) coinPurchase(ctx context.Context, userID uint, req UnlockRequest, key string) (purchaseOutcome, error) {
	if a.ledger == nil {
		return purchaseOutcome{}, ErrNoDatabase
	}
	// The title is resolved ONCE, before the transaction opens, and not inside
	// it. The lookup reaches another module's table through its own handle rather
	// than through this transaction, so resolving it here keeps the money
	// transaction to the rows it owns and keeps the advisory lock held for the
	// shortest possible window. It cannot change the outcome: a failure yields
	// an empty title and the receipt falls back to the class words.
	title := a.titleFor(ctx, req.ResourceType, req.ResourceID)
	var outcome purchaseOutcome
	err := a.repo.InUserTx(ctx, userID, func(tx *TxContext) error {
		existing, err := tx.ReadUnlock(userID, req.ResourceType, req.ResourceID)
		if err != nil {
			return err
		}
		if existing != nil {
			// The winner committed while this request was between step 2 and
			// here. already_unlocked, nothing charged, and the transaction
			// commits having written nothing at all.
			outcome = purchaseOutcome{AlreadyUnlocked: true, UnlockID: existing.ID}
			return nil
		}

		spend, err := a.spendInTx(ctx, tx, userID, req, key)
		if err != nil {
			return err
		}
		journalID := spend.JournalID
		record := &ResourceUnlock{
			UserID:       userID,
			ResourceType: req.ResourceType,
			ResourceID:   req.ResourceID,
			JournalID:    &journalID,
			Source:       UnlockSourceCoins,
			// The snapshot, from the journal that actually settled — never
			// recomputed from today's config, which would retroactively re-price a
			// purchase on a later read.
			CoinsPaid:  spend.Amount,
			UnlockedAt: a.now(),
		}
		created, err := tx.InsertUnlock(record)
		if err != nil {
			return err
		}
		if !created {
			// A conflict here means a live row exists that the read above did
			// not see, which is only reachable by something bypassing the user
			// lock. The spend rolls back with the transaction rather than being
			// left behind unattached: a refusal must write nothing.
			return fmt.Errorf("%w: user %d already holds %s/%d and the spend was rolled back",
				ErrAlreadyUnlocked, userID, req.ResourceType, req.ResourceID)
		}

		outcome = purchaseOutcome{
			CoinsPaid:    spend.Amount,
			BalanceAfter: spend.Available,
			SpentFrom:    spentFromDTO(spend.SpentFrom),
			UnlockID:     record.ID,
			JournalID:    spend.JournalID,
			Replayed:     spend.Replayed,
		}

		// ── step 5: the notification, inside this transaction ────────────────
		// Wired by NewUnlockAPI to the real registry; a test or a caller that
		// wants a different implementation replaces it with WithNotifier. Nil is
		// the only unlit case, and it is a no-op rather than an error so an API
		// object assembled by hand in a test is not a purchase that fails.
		if a.notifier != nil {
			if err := a.notifier.NotifyUnlockTx(ctx, tx.DB(), userID, UnlockEvent{
				JournalID:    spend.JournalID,
				ResourceType: req.ResourceType,
				ResourceID:   req.ResourceID,
				// Resolved once before the transaction opened; see the comment
				// on that call. Empty is a valid answer and renders the class.
				Title:        title,
				CoinsPaid:    spend.Amount,
				BalanceAfter: spend.Available,
				Source:       UnlockSourceCoins,
			}); err != nil {
				// Inside the transaction on purpose. §5: "each is emitted inside
				// the same transaction as the state change that caused it — so a
				// rolled-back expiry never produces an 'your coins expired'
				// notification." The same holds in reverse: a failed notification
				// must not leave a purchase with no notification, and a
				// notification must not describe a purchase that rolled back.
				return err
			}
		}
		return nil
	})
	if err != nil {
		return purchaseOutcome{}, err
	}
	return outcome, nil
}

// spendInTx runs the ledger's own Spend against the OPEN transaction.
//
// The whole trick is that a Repository is nothing but a *gorm.DB handle, so
// NewRepository(tx.DB()) is a repository whose transactions are SAVEPOINTs on
// the caller's connection rather than new top-level ones. Spend's own InUserTx
// therefore re-issues the per-user advisory lock on the session that already
// holds it — a no-op, because advisory locks are counted per session — and its
// COMMIT becomes a RELEASE SAVEPOINT.
//
// Nothing about Spend's behaviour changes: same validation, same server-side
// price resolution, same FEFO allocation, same journal insert, same replay
// handling. If it fails the error propagates out of the callback and the OUTER
// transaction rolls back, which is the behaviour the two-transaction version
// could not have had.
func (a *UnlockAPI) spendInTx(ctx context.Context, tx *TxContext, userID uint, req UnlockRequest, key string) (SpendResult, error) {
	resourceID := req.ResourceID
	spendReq := SpendRequest{
		UserID: userID,
		// RESOURCE_UNLOCK is the reason code, and the class in RefType is what
		// selects the price. No amount: there is nowhere for one to come from.
		ReasonCode: ReasonResourceUnlock,
		// The client's key goes through verbatim. A key this server rewrote
		// would be a key a retry cannot reproduce, and a retry that cannot
		// reproduce its key charges the student twice.
		IdempotencyKey: key,
		RefType:        req.ResourceType,
		RefID:          &resourceID,
		CreatedBy:      "user:" + strconv.FormatUint(uint64(userID), 10),
	}
	return NewLedger(NewRepository(tx.DB()), a.ledger.config).Spend(ctx, spendReq)
}

// alreadyUnlocked builds the ALREADY_OWNED answer: 200, coins_paid 0, nothing
// charged. It re-reads the balance rather than reporting zero, because a client
// rendering "0 coins" on an already-owned unlock looks like the wallet just
// emptied itself.
func (a *UnlockAPI) alreadyUnlocked(ctx context.Context, userID uint, base UnlockResult) (UnlockResult, error) {
	balance, err := a.balanceAfter(ctx, userID)
	if err != nil {
		return base, err
	}
	base.Unlocked = true
	base.AlreadyUnlocked = true
	base.CoinsPaid = 0
	base.BalanceAfter = balance
	base.SpentFrom = []SpentFromDTO{}
	return base, nil
}

func (a *UnlockAPI) balanceAfter(ctx context.Context, userID uint) (int64, error) {
	balance, err := a.wallet.WalletBalance(ctx, userID, a.now())
	if err != nil {
		return 0, err
	}
	return balance.TotalAvailable, nil
}

// ready reports whether this API object is wired to a database, answering 500
// when it is not. A route wired without a repository would otherwise panic on
// the first request, which is a worse first impression than an error.
func (a *UnlockAPI) ready(c *gin.Context) bool {
	if a.wallet == nil || a.repo == nil || a.config == nil || a.entitlements == nil {
		response.Error(c, http.StatusInternalServerError, "The coin service is not connected to a database")
		return false
	}
	return true
}

// ── the 402, and every other failure ─────────────────────────────────────────

// respondUnlockFailure turns a domain error into the §2.3 status and body.
//
// The designed object is built ONLY for the two states it was designed for — the
// 402 and the 423 that shares its payload. Attaching it to a 409 or a 500 would
// tell a client that "you need 15 more coins" about a key collision, which is
// how a UI ends up showing a money message on an unrelated error.
func (a *UnlockAPI) respondUnlockFailure(c *gin.Context, err error, result UnlockResult, cfg EconomyConfig) {
	if !errors.Is(err, ErrInsufficientCoins) {
		respondUnlockError(c, walletStatusFor(err), walletErrorCode(err), walletErrorMessage(err), nil)
		return
	}
	// The two figures the body is built from come out of the refusal itself.
	// Spend had them and had already rolled back by now, so re-reading the wallet
	// would have quoted a DIFFERENT moment from the one the decision was made on.
	required, available, carried := InsufficientFigures(err)
	if !carried {
		// Only reachable if something wrapped the bare sentinel. required is
		// still the server-resolved price, and available is reported as 0 rather
		// than invented — a 402 that says "you have none of what you actually
		// hold" is worse than one whose numbers are thin, and no caller in this
		// package produces that error.
		required = result.Required
	}
	payload := a.insufficientCoinsPayload(c.Request.Context(), result.UserID, required, available, cfg)

	// 423 ALLOWANCE_EXPIRED takes precedence over 402 for a student whose
	// allowance LAPSED rather than ran out: §2.3's row is "allowance lapsed and
	// the student is not covered by a purchase", and the two sentences need
	// different words. The designed payload travels either way, because the gap
	// and the earning routes are what the student needs in both cases.
	if result.AllowanceExpired {
		respondUnlockError(c, http.StatusLocked, CodeAllowanceExpired,
			"Your free unlocks have expired and this one is not covered by a purchase.", payload)
		return
	}
	respondUnlockError(c, http.StatusPaymentRequired, CodeInsufficientCoins,
		"You do not have enough coins for this yet.", payload)
}

// insufficientCoinsPayload builds the §2.3 object.
//
// required and available are passed in, not read here. They are the two figures
// the refusal was decided on: ErrInsufficient carries them out of Spend, and the
// 402 is rendered after that transaction rolled back. This function used to
// re-read the cached projection to recover `available`, which meant quoting a
// balance from a later moment than the decision — safe in both directions, and
// still a query whose answer nobody had asked for. There is no balance read left
// in this path, and the test that says so is
// TestInsufficientCoinsPayloadNeverReadsTheWallet.
//
// required is the server-resolved price for the class — the same spendPrice the
// ledger used, recomputed from the config rather than taken from the request,
// because there is no amount in the request to take.
//
// expires_in_days and ways_to_earn are still reads, and they have to be: they
// are about the student's wallet AFTER the refusal, and the wallet cannot change
// while a failed purchase is being rendered.
func (a *UnlockAPI) insufficientCoinsPayload(ctx context.Context, userID uint, required, available int64, cfg EconomyConfig) *InsufficientCoinsData {
	shortfall := required - available
	if shortfall < 0 {
		shortfall = 0
	}
	payload := &InsufficientCoinsData{
		Required:   required,
		Available:  available,
		Shortfall:  shortfall,
		WaysToEarn: []WayToEarnDTO{},
		Unavailable: []UnavailableDTO{
			// The referral route is REPORTED as unavailable rather than faked.
			// There is no referral code in this slice — 03-api-contract.md §2.4's
			// /referral/me is not built — so a student cannot act on "Invite a
			// friend" and must not be told to try. Listing it as unavailable
			// rather than dropping it lets the client say "coming soon" instead
			// of silently rendering one fewer route than the design, and it
			// disappears from this list the moment the referral slice lands.
			{Code: RouteReferral, Label: "Invite a friend", Reason: ReasonNotLaunched},
		},
	}
	// expires_in_days is how long the student's soonest-expiring coins survive.
	// It is the urgency the 402 exists to create: "spend these before they
	// lapse" is only actionable if they are told when they lapse. Null when
	// nothing they hold expires, because a made-up urgency is worse than none.
	if soonest, err := a.wallet.SoonestLotExpiry(ctx, userID, a.now()); err == nil && soonest != nil {
		days := int64(soonest.Sub(a.now()).Hours() / 24)
		if days < 0 {
			days = 0
		}
		payload.ExpiresInDays = &days
	}
	payload.WaysToEarn = a.waysToEarn(ctx, userID, cfg)
	return payload
}

// waysToEarn computes the §2.3 list from the caller's ACTUAL remaining
// eligibility.
//
// This is the whole point of the field. A list copied out of configuration tells
// a student with a finished profile to finish it, and tells one who has already
// been paid for a published resource to upload another; the UI and the data then
// disagree and the student is the one who looks wrong. The list is what the
// server decided, so the UI cannot drift from it.
func (a *UnlockAPI) waysToEarn(ctx context.Context, userID uint, cfg EconomyConfig) []WayToEarnDTO {
	ways := make([]WayToEarnDTO, 0, 2)

	// PROFILE — only while it is unfinished, and worth only what is left.
	//
	// The completion test is studentdashboard's, reached through the
	// ProfileEligibility adapter, so there is exactly one implementation of the
	// twelve checks. With no lookup wired the route is simply not offered: an
	// unverified "complete your profile" is the exact drift this function exists
	// to prevent.
	if a.profiles != nil {
		if complete, err := a.profiles.ProfileComplete(ctx, userID); err == nil && !complete {
			if potential := a.remainingProfileValue(ctx, userID, cfg); potential > 0 {
				ways = append(ways, WayToEarnDTO{
					Code:      RouteProfile,
					Label:     "Complete your profile",
					Potential: potential,
				})
			}
		}
	}

	// UPLOAD — awarded on publication, not on upload, and the award has no
	// producer in this slice: 03-api-contract.md §3.3 is admin-approve → grant,
	// and nothing calls that yet. So no student has already taken this route and
	// it is genuinely available to all of them. The eligibility check is what
	// stops it being offered forever: a student who has already been paid for a
	// published resource must not be told to upload another.
	if cfg.Awards.ResourceApproved > 0 {
		if taken, err := a.wallet.HasResourceApprovedAward(ctx, userID); err == nil && !taken {
			ways = append(ways, WayToEarnDTO{
				Code:      RouteUpload,
				Label:     "Upload a study resource",
				Potential: cfg.Awards.ResourceApproved,
			})
		}
	}
	return ways
}

// remainingProfileValue is what completing the profile is still worth to this
// student: the instalments they have not been paid, at the configured instalment
// size. Four instalments into a five-instalment ladder is worth ONE instalment,
// not the full award — a number that overstates what is on offer is how "you only
// need 15 more" becomes a support ticket.
func (a *UnlockAPI) remainingProfileValue(ctx context.Context, userID uint, cfg EconomyConfig) int64 {
	paid, err := a.wallet.ProfileInstalmentsPaid(ctx, userID)
	if err != nil {
		return 0
	}
	whole := cfg.Awards.ProfileComplete
	if cfg.Awards.ProfileInstalment > 0 && cfg.Awards.ProfileInstalments > 0 {
		whole = cfg.Awards.ProfileInstalment * cfg.Awards.ProfileInstalments
	}
	remaining := whole - paid*cfg.Awards.ProfileInstalment
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ── the status mapping, verbatim from 03-api-contract.md §4 ──────────────────

// Error codes. The §2.3 table names five of them; the rest exist because a
// handler that answers an unnamed code with an empty string teaches a frontend
// that the field might be empty.
const (
	CodeInvalidIdempotencyKey = "INVALID_IDEMPOTENCY_KEY"
	CodeIdempotencyKeyReuse   = "IDEMPOTENCY_KEY_REUSE"
	CodeInsufficientCoins     = "INSUFFICIENT_COINS"
	CodeResourceNotFound      = "RESOURCE_NOT_FOUND"
	CodeAllowanceExpired      = "ALLOWANCE_EXPIRED"
	CodeAccountFrozen         = "ACCOUNT_FROZEN"
	CodeInvalidRequest        = "INVALID_REQUEST"
	CodeEntryImmutable        = "ENTRY_IMMUTABLE"
	CodeEndpointDisabled      = "UNLOCK_ENDPOINT_DISABLED"
	CodeInternal              = "INTERNAL_ERROR"
	CodeNotConfigured         = "NOT_CONFIGURED"
	CodeUnauthenticated       = "UNAUTHENTICATED"
)

// walletStatusFor is §4's statusForError with the §2.3 table's additions.
//
// ErrAlreadyUnlocked is absent on purpose: it is not a failure, it is a 200 body.
// See the file header and errors.go.
func walletStatusFor(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrInsufficientCoins):
		return http.StatusPaymentRequired
	case errors.Is(err, ErrAllowanceExpired):
		// 423, and only when the allowance had LAPSED. An allowance that is
		// merely used up is not this: the student can still buy, and §2.3 says so
		// ("allowance lapsed and the student is not covered by a purchase").
		return http.StatusLocked
	case errors.Is(err, ErrIdempotencyKeyReuse):
		return http.StatusConflict
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrImmutable):
		return http.StatusConflict
	case errors.Is(err, ErrAccountFrozen):
		return http.StatusLocked
	case errors.Is(err, ErrInvalidResourceType), errors.Is(err, ErrInvalidArgument):
		return http.StatusBadRequest
	case errors.Is(err, ErrNoDatabase):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrNoAllowanceRemaining):
		// Reached only when a purchase did not cover them and the allowance was
		// spent rather than lapsed. The designed answer is the 402, which
		// respondUnlockFailure builds; this is the backstop for a path that
		// refuses here without one.
		return http.StatusPaymentRequired
	case errors.Is(err, ErrAlreadyUnlocked):
		// Listed only so the mapping is TOTAL. Nothing should reach here —
		// unlock() consumes ErrAlreadyUnlocked and answers it as a 200 body,
		// because §2.3 says it is not an error. If it ever did reach the handler,
		// 200 is still the right answer and a 500 is not.
		return http.StatusOK
	default:
		return http.StatusInternalServerError
	}
}

func walletErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrInsufficientCoins):
		return CodeInsufficientCoins
	case errors.Is(err, ErrAllowanceExpired):
		return CodeAllowanceExpired
	case errors.Is(err, ErrIdempotencyKeyReuse):
		return CodeIdempotencyKeyReuse
	case errors.Is(err, ErrNotFound):
		return CodeResourceNotFound
	case errors.Is(err, ErrImmutable):
		return CodeEntryImmutable
	case errors.Is(err, ErrAccountFrozen):
		return CodeAccountFrozen
	case errors.Is(err, ErrInvalidResourceType), errors.Is(err, ErrInvalidArgument):
		return CodeInvalidRequest
	case errors.Is(err, ErrNoDatabase):
		return CodeNotConfigured
	case errors.Is(err, ErrAlreadyUnlocked):
		// §2.3 names the outcome ALREADY_OWNED, and it is the only code here
		// that describes a success. It is never rendered, because unlock()
		// answers it as a 200 body, but naming it is better than an empty
		// string in a field the frontend switches on.
		return "ALREADY_OWNED"
	default:
		return CodeInternal
	}
}

// walletErrorMessage is what the student is shown.
//
// The domain error's own message names internal identifiers — a user id, a
// resource triple — which is what a log wants and not what a wallet wants. Only
// the 402 and the 423, whose text is part of the designed funnel, are rendered
// from the error at all.
func walletErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrInsufficientCoins):
		return "You do not have enough coins for this yet."
	case errors.Is(err, ErrAllowanceExpired):
		return "Your free unlocks have expired and this one is not covered by a purchase."
	case errors.Is(err, ErrIdempotencyKeyReuse):
		return "That Idempotency-Key was already used for a different unlock."
	default:
		return "That unlock could not be completed."
	}
}

// respondUnlockError writes the ErrorEnvelope: one shape for every failure of
// the write path, so a client parses one thing.
func respondUnlockError(c *gin.Context, status int, code, message string, data any) {
	c.JSON(status, ErrorEnvelope{
		Success: false,
		Message: message,
		Data:    data,
		Error:   ErrorDetail{Code: code, Message: message, Data: data},
	})
}

// ── response assembly ────────────────────────────────────────────────────────

// spentFromDTO converts the ledger's per-lot result into the §2.3 shape.
func spentFromDTO(from []SpentFrom) []SpentFromDTO {
	out := make([]SpentFromDTO, 0, len(from))
	for _, f := range from {
		out = append(out, SpentFromDTO{Bucket: f.Bucket, Coins: f.Amount, ExpiresAt: f.ExpiresAt})
	}
	return out
}

// balanceResponseFrom assembles §2.1 from the two reads.
//
// SpendOrder is built from the same rows as Buckets rather than re-queried:
// "what will be spent" and "what is there" describing different wallets is the
// failure this avoids.
func balanceResponseFrom(balance *WalletBalance, allowance *AllowanceStatus) BalanceResponse {
	out := BalanceResponse{
		TotalAvailable: balance.TotalAvailable,
		TotalReserved:  balance.TotalReserved,
		Buckets:        []BucketDTO{},
		SpendOrder:     []SpendOrderDTO{},
	}
	for _, b := range balance.Buckets {
		out.Buckets = append(out.Buckets, BucketDTO{
			Bucket:    b.Bucket,
			Balance:   b.Balance,
			ExpiresAt: b.ExpiresAt,
			LotCount:  b.LotCount,
		})
		out.SpendOrder = append(out.SpendOrder, SpendOrderDTO{
			Bucket:    b.Bucket,
			Coins:     b.Balance,
			ExpiresAt: b.ExpiresAt,
		})
	}
	if allowance != nil {
		dto := allowanceDTOFrom(allowance)
		out.Allowance = &dto
	}
	return out
}

// allowanceDTOFrom renders one AllowanceStatus as §2.1's allowance block.
//
// GrantedAt and ExpiresAt are null for a user who was never granted one. That is
// not the same as an allowance that expired: an allowance never issued has not
// lapsed, and reporting a timestamp in the past would tell a student their free
// unlocks ran out when they were never given any.
func allowanceDTOFrom(status *AllowanceStatus) AllowanceDTO {
	dto := AllowanceDTO{}
	if status == nil {
		return dto
	}
	if !status.GrantedAt.IsZero() {
		granted := status.GrantedAt
		expires := status.ExpiresAt
		dto.GrantedAt = &granted
		dto.ExpiresAt = &expires
	}
	if c, ok := status.Classes[ResourceTypeStudyResource]; ok {
		dto.DocumentUnlocks, dto.DocumentUsed = c.Granted, c.Used
	}
	if c, ok := status.Classes[ResourceTypeVideo]; ok {
		dto.VideoUnlocks, dto.VideoUsed = c.Granted, c.Used
	}
	if c, ok := status.Classes[ResourceTypeMockTest]; ok {
		dto.MockTestUnlocks, dto.MockTestUsed = c.Granted, c.Used
	}
	return dto
}

// sortBucketsFEFO is the spend order: soonest expiry first, buckets that never
// expire last, name as the tie-break so the order is total and therefore stable
// between two reads of the same wallet.
func sortBucketsFEFO(buckets []BucketBalance) {
	sort.Slice(buckets, func(i, j int) bool {
		li, lj := buckets[i].ExpiresAt, buckets[j].ExpiresAt
		switch {
		case li == nil && lj == nil:
			return buckets[i].Bucket < buckets[j].Bucket
		case li == nil:
			return false
		case lj == nil:
			return true
		case li.Equal(*lj):
			return buckets[i].Bucket < buckets[j].Bucket
		default:
			return li.Before(*lj)
		}
	})
}

// ── the wallet reads ─────────────────────────────────────────────────────────
//
// The SQL behind §2.1, §2.2 and the two eligibility questions the 402 body asks.
//
// They live here rather than in repository.go because repository.go is not this
// slice's file: the brief for this work excludes it, and 02-architecture.md §8
// puts repository SQL in repository.go for tidiness rather than because the file
// boundary is load-bearing. They move to repository.go the next time it is
// opened. What is load-bearing — and is honoured here — is that no decision
// happens in this section: each of these is a read, and the decisions all live
// in spendPrice, AllocateFEFO and the handler.

// WalletBalance is the §2.1 source: the cached projection for the totals, the
// open lots for the per-bucket view and the spend order.
type WalletBalance struct {
	// TotalAvailable is SUM(posted_balance - reserved) over the caller's
	// accounts, which is exactly what Spend gates on. It is the CACHED
	// projection, never SUM() over postings: §2.1 says the hot path serves the
	// projection, and a wallet render is not the place to find out that it and
	// the postings disagree.
	TotalAvailable int64
	// TotalReserved is coins promised but not yet issued. It is ALWAYS ZERO from the
	// earn path: a referral payout credits the referrer directly rather than
	// reserving against their balance, so nothing in coins writes `reserved`. The
	// field survives because Reserve/ReleaseReserved are general ledger primitives
	// and a reservation is the right shape for a mechanic that promises coins before
	// issuing them. It is NOT a referral figure, and 03-api-contract.md §2.4's
	// coins_pending was removed rather than redefined for exactly this reason.
	TotalReserved int64
	// Buckets is the per-bucket view, already in FEFO order.
	Buckets []BucketBalance
}

// BucketBalance is one bucket: what is in it, how many lots it is spread over,
// and when the soonest of them lapses.
type BucketBalance struct {
	Bucket    string
	Balance   int64
	LotCount  int64
	ExpiresAt *time.Time
}

// WalletBalance reads the caller's wallet.
//
// Two queries, deliberately. The projection and the lots are two different
// tables and they can disagree — by exactly the value of lots that have expired
// but have not yet been swept to expired_burn — and collapsing them into one
// join would hide that instead of surfacing it. The reconciliation job is what
// closes the gap; this endpoint is what shows the student their side of it.
func (r *Repository) WalletBalance(ctx context.Context, userID uint, now time.Time) (*WalletBalance, error) {
	if r == nil || r.db == nil {
		return nil, ErrNoDatabase
	}
	if userID == 0 {
		return nil, fmt.Errorf("%w: a wallet read needs a user id", ErrInvalidArgument)
	}
	var totals []struct {
		Posted   int64
		Reserved int64
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT COALESCE(SUM(b.posted_balance), 0) AS posted,
		        COALESCE(SUM(b.reserved), 0)      AS reserved
		   FROM coin_account_balance b
		   JOIN coin_account a ON a.id = b.account_id
		  WHERE a.owner_user_id = ? AND a.kind = 'USER'`,
		userID,
	).Scan(&totals).Error; err != nil {
		return nil, fmt.Errorf("read wallet totals for user %d: %w", userID, err)
	}

	var rows []BucketBalance
	if err := r.db.WithContext(ctx).Raw(
		`SELECT l.bucket,
		        COALESCE(SUM(l.granted - l.consumed), 0) AS balance,
		        count(*)                                AS lot_count,
		        min(l.expires_at)                       AS expires_at
		   FROM coin_lot l
		   JOIN coin_account a ON a.id = l.account_id
		  WHERE a.owner_user_id = ? AND a.kind = 'USER'
		    AND l.consumed < l.granted
		    AND (l.expires_at IS NULL OR l.expires_at > ?)
		  GROUP BY l.bucket`,
		userID, now,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read wallet lots for user %d: %w", userID, err)
	}
	sortBucketsFEFO(rows)
	if rows == nil {
		rows = []BucketBalance{}
	}

	out := &WalletBalance{Buckets: rows}
	if len(totals) > 0 {
		out.TotalAvailable = totals[0].Posted - totals[0].Reserved
		out.TotalReserved = totals[0].Reserved
	}
	return out, nil
}

// SoonestLotExpiry is the soonest expiry among the caller's OPEN lots, or nil
// when they hold nothing that lapses. It is the `expires_in_days` figure of the
// 402 body: the only number that makes "you are 15 coins short" actionable is
// the one that says whether the coins they already have are about to stop
// existing.
//
// Expired lots are excluded, not clamped: a lot whose expiry has passed is
// already unspendable (OpenLots refuses it and the sweep will take it), and
// reporting it would tell a student their coins expire today when in fact they
// stopped being spendable today.
func (r *Repository) SoonestLotExpiry(ctx context.Context, userID uint, now time.Time) (*time.Time, error) {
	if r == nil || r.db == nil {
		return nil, ErrNoDatabase
	}
	if userID == 0 {
		return nil, fmt.Errorf("%w: a wallet read needs a user id", ErrInvalidArgument)
	}
	var soonest *time.Time
	if err := r.db.WithContext(ctx).Raw(
		`SELECT min(l.expires_at) AS soonest
		   FROM coin_lot l
		   JOIN coin_account a ON a.id = l.account_id
		  WHERE a.owner_user_id = ? AND a.kind = 'USER'
		    AND l.consumed < l.granted
		    AND l.expires_at IS NOT NULL
		    AND l.expires_at > ?`,
		userID, now,
	).Scan(&soonest).Error; err != nil {
		return nil, fmt.Errorf("read soonest lot expiry for user %d: %w", userID, err)
	}
	return soonest, nil
}

// ProfileInstalmentsPaid counts the PROFILE_COMPLETE journals already written
// for this user, which is how many instalments of the profile award they have
// been paid.
//
// Derived from the journal rather than stored on the user, for the same reason
// the allowance's `used` counter does not exist: a stored copy is a second truth
// that disagrees the first time something is reversed.
func (r *Repository) ProfileInstalmentsPaid(ctx context.Context, userID uint) (int64, error) {
	if r == nil || r.db == nil {
		return 0, ErrNoDatabase
	}
	if userID == 0 {
		return 0, fmt.Errorf("%w: a wallet read needs a user id", ErrInvalidArgument)
	}
	var paid int64
	if err := r.db.WithContext(ctx).Raw(
		`SELECT count(*) FROM coin_posting p
		   JOIN coin_account a ON a.id = p.account_id
		   JOIN coin_journal j ON j.id = p.journal_id
		  WHERE a.owner_user_id = ? AND a.kind = 'USER'
		    AND j.reason_code = ? AND p.amount > 0`,
		userID, ReasonProfileComplete,
	).Scan(&paid).Error; err != nil {
		return 0, fmt.Errorf("read profile instalments paid for user %d: %w", userID, err)
	}
	return paid, nil
}

// HasResourceApprovedAward reports whether this user has already been paid for
// publishing a resource.
//
// It asks the LEDGER rather than the resource tables on purpose: RESOURCE_APPROVED
// is the reason code the award is journalled under, and a journal that exists IS
// the record of an award having been made. Reading studyresources for the same
// question would couple the wallet to a module whose publication semantics
// change underneath it, and would be a second source of truth for a fact the
// ledger already owns.
//
// It counts the user's own positive legs rather than journals, so a journal with
// no user leg (there cannot be one for this reason code, but the shape allows it)
// does not read as a payment.
func (r *Repository) HasResourceApprovedAward(ctx context.Context, userID uint) (bool, error) {
	if r == nil || r.db == nil {
		return false, ErrNoDatabase
	}
	if userID == 0 {
		return false, fmt.Errorf("%w: a wallet read needs a user id", ErrInvalidArgument)
	}
	var n int64
	if err := r.db.WithContext(ctx).Raw(
		`SELECT count(*) FROM coin_posting p
		   JOIN coin_account a ON a.id = p.account_id
		   JOIN coin_journal j ON j.id = p.journal_id
		  WHERE a.owner_user_id = ? AND a.kind = 'USER'
		    AND j.reason_code = ? AND p.amount > 0`,
		userID, ReasonResourceApproved,
	).Scan(&n).Error; err != nil {
		return false, fmt.Errorf("read approved-resource awards for user %d: %w", userID, err)
	}
	return n > 0, nil
}

// TransactionPage is one page of §2.2, newest first, plus the cursor for the
// next page (empty on the last page).
//
// The shape of the query is the interesting part:
//
//   - It joins the user's postings, so "scoped to the authenticated user" is a
//     WHERE clause on the join rather than a filter applied in Go. A Go filter
//     over "all journals" would make the keyset wrong, not just slow.
//   - `amount` is the signed sum of the user's own legs: negative for a spend,
//     positive for a grant, negative again for the clawback of a grant. Nothing
//     is negated or re-signed anywhere.
//   - `balance_after` is derived with a window over the WHOLE history, computed
//     BEFORE the cursor filter, and then read as (grand total − everything newer
//     than this row). Filtering first and windowing after would compute a
//     "balance" that only counts the current page, which is wrong on every page
//     but the first and wrong in a way nobody would notice.
//   - The window is O(history) per page. At launch scale a wallet history is
//     hundreds of rows and this is a fraction of a millisecond; if it ever is
//     not, the fix is a per-user running-balance column maintained by the
//     ledger, not a cleverer query here.
func (r *Repository) TransactionPage(ctx context.Context, userID uint, cursor transactionCursor, limit int) ([]TransactionDTO, string, error) {
	if r == nil || r.db == nil {
		return nil, "", ErrNoDatabase
	}
	if userID == 0 {
		return nil, "", fmt.Errorf("%w: a wallet read needs a user id", ErrInvalidArgument)
	}
	if limit <= 0 {
		limit = defaultTransactionPageSize
	}

	// The cursor filter is applied in the OUTER select, never inside the CTE.
	// The window in `ranked` has to see the whole history for the running total
	// to be a running total; filtering before the window would make
	// balance_after count only the rows on this page, which is right on page one
	// and wrong on every page after it, in a way no assertion would catch.
	query := `WITH user_legs AS (
	    SELECT p.journal_id, SUM(p.amount) AS amount
	      FROM coin_posting p
	      JOIN coin_account a ON a.id = p.account_id
	     WHERE a.owner_user_id = ? AND a.kind = 'USER'
	     GROUP BY p.journal_id
	), ranked AS (
	    SELECT j.id, j.entry_type, j.reason_code, j.ref_type, j.ref_id,
	           j.created_at, j.reversal_of,
	           COALESCE(u.amount, 0) AS amount,
	           EXISTS (SELECT 1 FROM coin_journal x WHERE x.reversal_of = j.id) AS reversed,
	           SUM(COALESCE(u.amount, 0)) OVER (
	               ORDER BY j.created_at DESC, j.id DESC
	               ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS cumulative,
	           (SELECT COALESCE(SUM(amount), 0) FROM user_legs) AS grand_total
	      FROM coin_journal j
	      JOIN user_legs u ON u.journal_id = j.id
	     WHERE j.scope = 'user'
	)
	SELECT id AS journal_id, entry_type, reason_code, ref_type, ref_id, created_at, reversal_of,
	       reversed, amount, (grand_total - cumulative + amount) AS balance_after
	  FROM ranked`
	args := []any{userID}
	if cursor.ID != "" {
		query += ` WHERE (created_at, id) < (?, ?::uuid)`
		args = append(args, cursor.CreatedAt, cursor.ID)
	}
	// One more row than asked for: the extra row is how "is there another page"
	// is answered without a COUNT, and it is dropped from the response.
	// Every column is aliased to the field name it lands in. GORM's Scan maps
	// by COLUMN name, and `id` would not reach a field called JournalID — which
	// is how a history list ends up with an empty journal id on every row.
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit+1)

	var rows []struct {
		JournalID    string
		EntryType    string
		ReasonCode   string
		RefType      *string
		RefID        *uint64
		CreatedAt    time.Time
		ReversalOf   *string
		Reversed     bool
		Amount       int64
		BalanceAfter int64
	}
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, "", fmt.Errorf("read transaction page for user %d: %w", userID, err)
	}

	items := make([]TransactionDTO, 0, len(rows))
	next := ""
	for i, row := range rows {
		if i == limit {
			// The extra row: there IS another page, and its cursor starts at this
			// row's key. The row itself is not in this response.
			last := items[len(items)-1]
			next = encodeTransactionCursor(transactionCursor{CreatedAt: last.CreatedAt, ID: last.JournalID})
			break
		}
		ref := refFromPointers(row.RefType, row.RefID)
		items = append(items, TransactionDTO{
			JournalID:    row.JournalID,
			EntryType:    row.EntryType,
			ReasonCode:   row.ReasonCode,
			Amount:       row.Amount,
			BalanceAfter: row.BalanceAfter,
			Ref:          ref,
			Description:  describeReason(row.ReasonCode, ref),
			Reversed:     row.Reversed,
			CreatedAt:    row.CreatedAt.UTC(),
		})
	}
	return items, next, nil
}
