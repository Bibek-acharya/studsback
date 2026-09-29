// internal/coins/download_gate.go
//
// The coin gate on a study-resource download, and the only place a document is
// paid for at the moment it is delivered.
//
// ── the shape of the problem ─────────────────────────────────────────────────
//
// A download is not a POST. There is no request body, no Idempotency-Key header
// and no client that knows a price exists, because the client is a
// window.open() and a <a download>. Everything that 03-api-contract.md §2.3
// builds for the wallet — a key, a body, a 200 with coins_paid — has to be
// supplied by the server or abandoned. This file supplies it, and the only
// genuinely new thing about it is that the file must be handed over in the same
// breath as the debit.
//
// ── the order, and why each step is where it is ──────────────────────────────
//
// It is unlock() in unlock_api.go, unchanged, and the reasons are documented
// there. Reusing it is the point: a second implementation of "entitlement, then
// allowance, then balance" is a second place for the ordering to be wrong, and
// this one would be wrong silently — the wallet tests would still pass.
//
//  1. ENTITLEMENT FIRST. A live resource_unlock for this student and this
//     resource means the file is theirs. It is one indexed existence check
//     against resource_unlock_live_uniq, it writes nothing, and it is what makes
//     a repeat download free. It is checked before the price is even resolved
//     because a student who already paid must never see a charge, not even as a
//     figure they are then told they do not owe.
//
//  2. ALLOWANCE NEXT, before the balance. A student with zero coins and an
//     unused starter unlock downloads the file for nothing. Checking the balance
//     first would block exactly the students the allowance exists for, and would
//     send them off to earn coins they did not need — on the one route where the
//     alternative to paying is a wall rather than a button.
//
//  3. BALANCE LAST. If the wallet cannot cover the server-resolved price, the
//     bytes are NOT served. The answer is the 402 of §2.3 with the figures the
//     ledger refused on (InsufficientError), not a re-read of the balance.
//
// ── the charge, and why it is one transaction ────────────────────────────────
//
// A paid download runs the same coinPurchase the wallet endpoint runs: spend,
// entitlement and notification inside one InUserTx, under the per-user advisory
// lock, with the loser of a race re-reading the live unlock and writing nothing.
// A second purchase path here would be a second place for the atomicity to be
// wrong, and this one is the worse place to get it wrong: a charge that commits
// and then fails to deliver the file is a refund conversation, and Reverse
// refuses to refund a spend, so the coins would be gone and the file withheld.
//
// ── the idempotency key, which a download does not have ──────────────────────
//
// The ledger requires a non-empty key on every journal and the column is unique
// per scope, so the gate HAS to supply one. It is derived, not random, and
// derived from (user, class, id, price) for three reasons:
//
//   - derived, not random: a browser may retry a download — a flaky connection, a
//     double click, a refresh. A random key would charge twice for one intent.
//     The derived key makes the retry REPLAY the original journal, which is the
//     behaviour 02-architecture.md §12.2 asks for, and which is why the price is
//     in the key: a price change must not turn a retry into a 409 on a key the
//     client cannot see.
//   - scoped to the user: the key is unique across every user journal in the
//     table, so a key built from the resource alone would make one student's
//     download collide with another's and answer the second with a 409 or, worse,
//     a replay of somebody else's spend.
//   - namespaced with a "gate:" prefix: the standalone unlock endpoint's keys
//     come from clients and are their own namespace. A gate key must never be
//     able to replay a client's journal, or a client could forge a purchase by
//     guessing a key.
package coins

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"studsphere/backend/internal/studyresources"
)

// gateIdempotencyKeyPrefix namespaces every key this file mints. See the header.
const gateIdempotencyKeyPrefix = "gate:study-resource-download:"

// downloadGate is the *UnlockAPI answering studyresources' DownloadGate port.
//
// The same object answers the wallet, deliberately: the entitlement, the
// allowance, the price and the purchase are all one implementation, and a second
// one is the duplication this file exists to avoid.
type downloadGate struct {
	api *UnlockAPI
}

// NewDownloadGate wires the gate for the download route. A nil API is a gate
// that cannot decide, and the handler treats that as a 500 rather than as
// permission — see studyresources.ErrGateUnconfigured.
func NewDownloadGate(api *UnlockAPI) studyresources.DownloadGate {
	return &downloadGate{api: api}
}

// AuthorizeDownload implements studyresources.DownloadGate.
//
// It is called for EVERY download, including the ungated ones, and answering
// "allowed, nothing done" is a real answer rather than a shortcut: it is how the
// kill switch works. The switch is read from the cached config, so turning a
// class back to free is a config write and not a deploy, and an ungated download
// costs one cached read and no database work at all.
func (g *downloadGate) AuthorizeDownload(ctx context.Context, userID uint, resourceType string, resourceID uint64, title string) studyresources.DownloadDecision {
	if g == nil || g.api == nil {
		return refuse(studyresources.ErrGateUnconfigured)
	}
	return g.api.authorizeStudyResourceDownload(ctx, userID, resourceType, resourceID, title)
}

func refuse(err error) studyresources.DownloadDecision {
	refusal := studyresources.NewGateErrorRefusal(err)
	return studyresources.DownloadDecision{Allowed: false, Refusal: refusal}
}

// authorizeStudyResourceDownload is the gate proper. It lives on UnlockAPI rather
// than on downloadGate because it needs the same unexported fields the wallet
// path uses — entitlements, config, the purchase seam — and those are not
// reachable from a type that only holds an *UnlockAPI.
func (a *UnlockAPI) authorizeStudyResourceDownload(ctx context.Context, userID uint, resourceType string, resourceID uint64, title string) studyresources.DownloadDecision {
	// ── the switch ─────────────────────────────────────────────────────────
	//
	// Read first and act on it immediately. Every gate defaults to false, so this
	// returns "allowed" for every class in a default deployment and the download
	// path below is not reached at all. A config that cannot be read is NOT
	// treated as "ungated": a student would be served a file the admin believes
	// is paid for, which is the one thing a kill switch must not do by accident.
	//
	// The nil check is here rather than in the readiness check further down
	// because this is the first thing that dereferences the API, and a gate
	// attached to a half-built object must answer 500 rather than panic on the
	// first download.
	if a.config == nil {
		return refuse(studyresources.ErrGateUnconfigured)
	}
	cfg, err := a.config.Load()
	if err != nil {
		return refuse(studyresources.ErrGateUnconfigured)
	}
	if !cfg.GateEnabled(resourceType) {
		// Ungated, or gated for a class this build does not know. Either way the
		// file is served and nothing is read or written in the economy: no
		// entitlement check, no price, no balance, no journal.
		return studyresources.DownloadDecision{Allowed: true}
	}
	if userID == 0 {
		// Not reachable in production — the route is behind authMW — and the
		// handler does not re-implement that check. It is here so a gate wired
		// onto an unwrapped route answers "log in" rather than spending against
		// user 0.
		return studyresources.DownloadDecision{
			Allowed: false,
			Refusal: &studyresources.GateRefusal{
				Status:  http.StatusUnauthorized,
				Code:    studyresources.CodeUnauthenticated,
				Message: "Log in to download this.",
			},
		}
	}
	// The readiness check comes AFTER the switch, deliberately. An ungated class
	// must be served even by an API object that is not fully wired: that is the
	// ungated path, and the ungated path does not need the economy at all.
	if !a.entitlementsReady() {
		return refuse(studyresources.ErrGateUnconfigured)
	}

	// ── the resource, resolved ──────────────────────────────────────────────
	//
	// The caller has already read the row — it had to, to know the resource is
	// published — and it passes the title in. The gate still asks its own lookup
	// when one is wired, and the reason is not redundancy: it is what makes the
	// 404 reachable from here, so the gate is correct for any caller rather than
	// only for the route that happens to check publication first. A gate that
	// trusted its caller's check would one day be wired onto a route that does
	// not make it, and would sell a draft.
	//
	// The cost is one indexed SELECT on a download that is about to open a money
	// transaction anyway, and it is skipped entirely when no lookup is wired.
	resolved, err := a.lookupUnlockable(ctx, resourceType, resourceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// "Unknown id, or not published, or wrong type" — one answer for all
			// three, so the endpoint cannot be used to confirm that a draft exists.
			return studyresources.DownloadDecision{
				Allowed: false,
				Refusal: &studyresources.GateRefusal{
					Status:  http.StatusNotFound,
					Code:    CodeResourceNotFound,
					Message: "This resource does not exist, is not published, or is not of that type.",
				},
			}
		}
		return refuse(err)
	}
	if strings.TrimSpace(resolved.Title) != "" {
		title = resolved.Title
	}

	price, err := spendPrice(cfg, ReasonResourceUnlock, resourceType)
	if err != nil || price <= 0 {
		// A class priced at zero cannot be bought: Spend refuses a non-positive
		// price, so charging would fail every download of a class an admin has
		// both ungated in the price table and gated here. Serving it free is the
		// only outcome that is not a 500 on a published file, and it is what a
		// price of zero actually says. Validation normally refuses a live
		// endpoint over a free class, so this is the backstop for a config that
		// reached storage another way.
		return studyresources.DownloadDecision{Allowed: true}
	}

	result, err := a.unlock(ctx, userID, UnlockRequest{ResourceType: resourceType, ResourceID: resourceID},
		gateIdempotencyKey(userID, resourceType, resourceID, price), cfg)
	if err != nil {
		return a.gateRefusalFor(ctx, err, result, cfg)
	}
	return studyresources.DownloadDecision{Allowed: true, Charged: result.CoinsPaid}
}

// entitlementsReady mirrors ready() for a caller that has no gin context: it
// checks the fields unlock() dereferences directly, so a gate wired without a
// repository answers 500 rather than panicking on the first download.
//
// The ledger is deliberately NOT in this list. It is reached only through the
// purchase seam, and coinPurchase already refuses a nil ledger with
// ErrNoDatabase — a refusal that is part of the purchase's own contract rather
// than a second thing to keep in step. Requiring it here would make the
// readiness check stricter than the path it guards, and would fail the gate on
// an object that can in fact decide an entitlement, an allowance and a 402
// without ever spending.
func (a *UnlockAPI) entitlementsReady() bool {
	return a.entitlements != nil && a.wallet != nil && a.repo != nil && a.config != nil
}

// gateRefusalFor turns the unlock path's error into the refusal the download
// handler writes, reusing the §2.3 status and code mapping verbatim so a
// download and a wallet call cannot answer the same situation differently.
//
// The 402 and the 423 share the designed object, and the figures in it come from
// the typed InsufficientError the refusal carries out of the rolled-back
// transaction. The balance is deliberately NOT re-read: the numbers would then
// describe a different moment from the one the decision was made on, which is
// the whole reason the typed error exists.
func (a *UnlockAPI) gateRefusalFor(ctx context.Context, err error, result UnlockResult, cfg EconomyConfig) studyresources.DownloadDecision {
	if !errors.Is(err, ErrInsufficientCoins) {
		// Everything else — a 404 from the resource lookup, a 409 on a key, an
		// outage — is mapped by the same walletStatusFor the unlock endpoint
		// uses. An unrecognised error is a 500 and says so, rather than being
		// reported as "you cannot afford this".
		if errors.Is(err, ErrNotFound) {
			return studyresources.DownloadDecision{
				Allowed: false,
				Refusal: &studyresources.GateRefusal{
					Status:  http.StatusNotFound,
					Code:    CodeResourceNotFound,
					Message: "This resource does not exist, is not published, or is not of that type.",
				},
			}
		}
		return refuse(err)
	}

	required, available, carried := InsufficientFigures(err)
	if !carried {
		// Only reachable if something wrapped the bare sentinel. required is still
		// the server-resolved price; available is reported as 0 rather than
		// invented, because a 402 claiming a student has none of what they
		// actually hold is worse than one whose numbers are thin.
		required = result.Required
	}
	payload := a.insufficientCoinsPayload(ctx, result.UserID, required, available, cfg)

	// 423 ALLOWANCE_EXPIRED takes precedence over the 402 for a student whose
	// starter unlocks LAPSED rather than ran out. §2.3's row is "allowance lapsed
	// and the student is not covered by a purchase", and the frontend renders
	// two different screens for the two sentences: one says earn coins, the other
	// says the free ones have gone. The designed object travels either way.
	if result.AllowanceExpired {
		return studyresources.DownloadDecision{
			Allowed: false,
			Refusal: &studyresources.GateRefusal{
				Status:  http.StatusLocked,
				Code:    CodeAllowanceExpired,
				Message: "Your free unlocks have expired and this one is not covered by a purchase.",
				Data:    payload,
			},
		}
	}
	return studyresources.DownloadDecision{
		Allowed: false,
		Refusal: &studyresources.GateRefusal{
			Status:  http.StatusPaymentRequired,
			Code:    CodeInsufficientCoins,
			Message: "You do not have enough StudsTokens for this yet.",
			Data:    payload,
		},
	}
}

// gateIdempotencyKey mints the key a download's purchase journal is written
// under. See the header for why it is derived and why it carries the user and
// the price.
//
// The user id is included because (scope, idempotency_key) is unique across every
// user journal in the table, so a key built from the resource alone would make
// one student's download collide with another's — answering the second with a 409
// on a key neither of them chose, or replaying the first's spend.
func gateIdempotencyKey(userID uint, resourceType string, resourceID uint64, price int64) string {
	return gateIdempotencyKeyPrefix +
		strconv.FormatUint(uint64(userID), 10) + ":" +
		resourceType + ":" +
		strconv.FormatUint(resourceID, 10) + ":" +
		strconv.FormatInt(price, 10)
}

// Compile-time proof that the gate is the port the download route asks. A
// signature change on either side breaks the build here rather than at runtime on
// the first download.
var _ studyresources.DownloadGate = (*downloadGate)(nil)
