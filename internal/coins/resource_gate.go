// internal/coins/resource_gate.go
//
// The gate core shared by the video-playback and mock-test-paper gates.
//
// ── why this file exists next to download_gate.go ────────────────────────────
//
// download_gate.go is committed, independently revertable, and must stay that
// way, so it was not refactored to share this. That leaves two gates that reach
// the SAME economy through the SAME unlock() and differ only in their refusal
// vocabulary — and two gates that differ only in their refusal vocabulary is
// exactly the situation where a second implementation of "entitlement, then
// allowance, then balance" would go quietly wrong while the wallet tests kept
// passing. So the ORDER lives here, once, and download_gate.go keeps its own
// copy. When the document gate is next touched by someone who is allowed to
// touch it, this is the one that should win and that file should call in.
//
// What IS shared for certain, and is the part that actually has to be single:
// a.unlock (unlock_api.go), coinPurchase, and spendPrice. A gate that reached
// past unlock() would be a second place for the ordering to be wrong and this
// file does not.
//
// ── the order, unchanged from the document gate ──────────────────────────────
//
//  1. THE SWITCH, read first and acted on immediately. Every gate defaults to
//     false, so a default deployment returns "allowed" here and nothing below
//     runs. An unreadable config is NOT "ungated": serving bytes because the
//     kill switch could not be read is the one failure a kill switch must not
//     produce by accident, so it is a 500.
//
//  2. IDENTITY. Unreachable in production — both routes sit behind authMW — and
//     here so a gate wired onto an unwrapped route answers "log in" rather than
//     spending against user 0.
//
//  3. READINESS, after the switch. An ungated class must be servable by an API
//     object that is not fully wired: that is the ungated path, and it does not
//     need the economy at all.
//
//  4. THE RESOURCE, resolved through the ResourceLookup seam. The caller has
//     already read the row — it had to, to know the resource is published — and
//     this read is not redundancy: it is what makes the 404 reachable from here,
//     so the gate is correct for any caller rather than only for the route that
//     checks publication first. A gate that trusted its caller would one day be
//     wired onto a route that does not make that check, and would sell a draft.
//
//  5. THE PRICE, server-resolved. A class priced at zero cannot be bought — the
//     ledger refuses a non-positive price — so serving it free is the only
//     outcome that is not a 500 on a published resource.
//
//  6. unlock(), which is entitlement FIRST, then allowance, then balance, then
//     the purchase. Unchanged and not reimplemented. The reasons are in
//     unlock_api.go's header and they are load-bearing: entitlement first is what
//     makes a repeat free, and allowance before balance is what stops a student
//     with no coins from being blocked by the mechanism meant to help them.
//
// ── the idempotency key a delivery point does not have ──────────────────────
//
// Neither route is a POST: a <video> element and a browser navigation both carry
// no Idempotency-Key and no body, and there is no way for the server to know a
// price exists. So the gate mints one. It is derived, not random, and derived from
// (user, class, id, price):
//
//   - derived, not random: a browser retries. A double-click, a flaky connection,
//     a refresh. A random key charges twice for one intent. A derived key makes
//     the retry REPLAY the original journal, which is what 02-architecture.md
//     §12.2 asks for, and the price is in the key so a price change cannot turn a
//     retry into a 409 on a key the client cannot see.
//   - scoped to the user: (scope, idempotency_key) is unique across EVERY user
//     journal in the table, so a key built from the resource alone would make one
//     student's purchase collide with another's — a 409 on a key neither of them
//     chose, or a replay of somebody else's spend.
//   - namespaced per route, after "gate:": so a gate key can never replay a
//     journal a client wrote through POST /coins/unlock, whose keys are the
//     client's own and whose namespace a client could guess. The per-route
//     namespace also means the video-playback gate and the video-DOWNLOAD gate
//     cannot replay one another's journals, so each of those slices stays
//     independently revertable and a rollback of one cannot strand a journal
//     written by the other. The cross-route double charge is prevented by
//     something stronger and simpler anyway: the entitlement check runs first and
//     one video is one entitlement, whichever route paid for it.
package coins

import (
	"context"
	"errors"
	"strconv"
)

// gateRequest is one delivery point's question to the economy.
type gateRequest struct {
	// Class is the coin class the resource is priced and entitled under. It is
	// fixed by the route — a playback grant is always a video, a paper is always
	// a mock test — and it selects the kill switch, the price and the allowance
	// column. See playback_gate.go for why it is not a caller-chosen parameter on
	// the PORTS even though it is one here.
	Class string
	// ResourceID is the id in the class's own table.
	ResourceID uint64
	// Title is the caller's best knowledge of the resource's name. The lookup's
	// answer replaces it when it has one, and the class words are used when
	// neither does, so a receipt is never nameless.
	Title string
}

// gateWording is the student-facing copy one route contributes.
//
// Status, code and data are NOT per route: they are the §2.3 contract and they
// are identical across every gate, which is the point of reusing
// gateRefusalFor's mapping. Only the sentences name the resource, and a sentence
// that says "download" on the paper route is a wrong sentence a student reads.
type gateWording struct {
	unauthenticated string
	unavailable     string
	notFound        string
	insufficient    string
	allowanceLapsed string
}

// gateRefusal is the module-neutral refusal. Each asking module owns its own type
// and its own RefusalBody, and the adapters at the bottom of this file map
// between them; the shape is identical, so the mapping is a field copy and never a
// second decision.
type gateRefusal struct {
	Status  int
	Code    string
	Message string
	Data    any
}

// gateOutcome is the module-neutral answer.
type gateOutcome struct {
	Allowed bool
	Charged int64
	// Refusal is nil whenever Allowed is true.
	Refusal *gateRefusal
}

// authorizeDelivery is the gate proper. It lives on UnlockAPI rather than on a
// gate type because it needs the same unexported fields unlock() dereferences —
// entitlements, config, the purchase seam — and those are not reachable from a
// type that only holds an *UnlockAPI.
func (a *UnlockAPI) authorizeDelivery(ctx context.Context, userID uint, req gateRequest, keyPrefix string, words gateWording) gateOutcome {
	// ── 1. the switch ──────────────────────────────────────────────────────
	//
	// The nil check is here rather than in the readiness check because this is
	// the first thing that dereferences the API, and a gate attached to a
	// half-built object must answer 500 rather than panic on the first request.
	if a == nil || a.config == nil {
		return gateRefusalOutcome(gateRefusal{Status: 500, Code: gateCodeUnavailable, Message: words.unavailable})
	}
	cfg, err := a.config.Load()
	if err != nil {
		return gateRefusalOutcome(gateRefusal{Status: 500, Code: gateCodeUnavailable, Message: words.unavailable})
	}
	if !cfg.GateEnabled(req.Class) {
		// Ungated, or a class this build does not know. Either way nothing is read
		// or written in the economy: no entitlement check, no price, no balance,
		// no journal, and no resource lookup. This is the ship-dark property, and
		// the assertion that matters about it is that it READS NOTHING, not that
		// it did not charge.
		return gateOutcome{Allowed: true}
	}

	// ── 2. identity ────────────────────────────────────────────────────────
	if userID == 0 {
		return gateRefusalOutcome(gateRefusal{
			Status:  401,
			Code:    gateCodeUnauthenticated,
			Message: words.unauthenticated,
		})
	}

	// ── 3. readiness, after the switch on purpose ──────────────────────────
	if !a.entitlementsReady() {
		return gateRefusalOutcome(gateRefusal{Status: 500, Code: gateCodeUnavailable, Message: words.unavailable})
	}

	// ── 4. the resource, resolved ──────────────────────────────────────────
	//
	// The TITLE this returns is deliberately not threaded into the purchase, and
	// the reason is that unlock() cannot accept one: coinPurchase re-resolves it
	// through the same lookup for the debit receipt and the wallet history. Two
	// answers to the same question would be two chances to disagree about what a
	// student was charged for, so the gate's read exists for the 404 and the
	// receipt's read exists for the receipt.
	//
	// That makes this an indexed SELECT that is about to be followed by a money
	// transaction, and it is worth the one query: it is what makes the 404
	// reachable from HERE rather than only from the route that happened to check
	// publication first. A gate that trusted its caller's check would one day be
	// wired onto a route that does not make it, and would sell a draft.
	_, err = a.lookupUnlockable(ctx, req.Class, req.ResourceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// "Unknown id, or not published, or not of that type" — one answer for
			// all three, so the endpoint cannot be used to confirm that a draft
			// exists.
			return gateRefusalOutcome(gateRefusal{Status: 404, Code: CodeResourceNotFound, Message: words.notFound})
		}
		return gateRefusalOutcome(gateRefusal{Status: 500, Code: gateCodeUnavailable, Message: words.unavailable})
	}

	// ── 5. the price, server-resolved ──────────────────────────────────────
	price, err := spendPrice(cfg, ReasonResourceUnlock, req.Class)
	if err != nil || price <= 0 {
		// A class priced at zero cannot be bought. Serving it free is the only
		// outcome that is not a 500 on a published resource, and it is what a
		// price of zero actually says. Validation normally refuses this
		// combination on the config write, so this is the backstop for storage
		// that reached here another way.
		return gateOutcome{Allowed: true}
	}

	// ── 6. the decision, which is unlock() and nothing else ────────────────
	result, err := a.unlock(ctx, userID,
		UnlockRequest{ResourceType: req.Class, ResourceID: req.ResourceID},
		deliveryGateIdempotencyKey(keyPrefix, userID, req.Class, req.ResourceID, price), cfg)
	if err != nil {
		return a.deliveryRefusalFor(ctx, err, result, cfg, words)
	}
	return gateOutcome{Allowed: true, Charged: result.CoinsPaid}
}

// deliveryRefusalFor maps unlock()'s error onto the §2.3 status, code and body.
// It is the same mapping the wallet handler and the document gate use, and it is
// written out here rather than shared so that download_gate.go stays untouched —
// see the file header.
func (a *UnlockAPI) deliveryRefusalFor(ctx context.Context, err error, result UnlockResult, cfg EconomyConfig, words gateWording) gateOutcome {
	if !errors.Is(err, ErrInsufficientCoins) {
		if errors.Is(err, ErrNotFound) {
			return gateRefusalOutcome(gateRefusal{Status: 404, Code: CodeResourceNotFound, Message: words.notFound})
		}
		// Everything else — a 409 on a key, an outage — is a 500 and says so, rather
		// than being reported as "you cannot afford this".
		return gateRefusalOutcome(gateRefusal{Status: 500, Code: gateCodeUnavailable, Message: words.unavailable})
	}

	required, available, carried := InsufficientFigures(err)
	if !carried {
		// Only reachable if something wrapped the bare sentinel. required is still
		// the server-resolved price; available is reported as 0 rather than
		// invented, because a 402 claiming a student holds none of what they
		// actually hold is worse than one whose numbers are thin.
		required = result.Required
	}
	payload := a.insufficientCoinsPayload(ctx, result.UserID, required, available, cfg)

	// 423 ALLOWANCE_EXPIRED takes precedence over the 402 for a student whose
	// starter unlocks LAPSED rather than ran out. §2.3's row is "allowance lapsed
	// and the student is not covered by a purchase", and the frontend renders two
	// different screens for the two sentences. The designed object travels either
	// way, because the gap and the earning routes are what the student needs in
	// both cases.
	if result.AllowanceExpired {
		return gateRefusalOutcome(gateRefusal{
			Status:  423,
			Code:    CodeAllowanceExpired,
			Message: words.allowanceLapsed,
			Data:    payload,
		})
	}
	return gateRefusalOutcome(gateRefusal{
		Status:  402,
		Code:    CodeInsufficientCoins,
		Message: words.insufficient,
		Data:    payload,
	})
}

func gateRefusalOutcome(refusal gateRefusal) gateOutcome {
	return gateOutcome{Allowed: false, Refusal: &refusal}
}

// deliveryGateIdempotencyKey mints the journal key for one delivery point. See the
// file header for why it is derived, why it carries the user, and why the price is
// in it.
func deliveryGateIdempotencyKey(prefix string, userID uint, class string, resourceID uint64, price int64) string {
	return prefix +
		strconv.FormatUint(uint64(userID), 10) + ":" +
		class + ":" +
		strconv.FormatUint(resourceID, 10) + ":" +
		strconv.FormatInt(price, 10)
}

// The two codes that are not §2.3 codes, redeclared rather than imported from the
// asking modules. A gate on the coins side must not depend on studyresources or
// mocktests for a wire constant: the import direction is the whole reason the port
// exists, and a string constant is not worth breaking it for. Each asking module
// declares the same value and each pair is asserted equal from the coins side, so
// a rename on one side fails a test rather than a request.
const (
	gateCodeUnavailable     = "GATE_UNAVAILABLE"
	gateCodeUnauthenticated = "UNAUTHENTICATED"
)
