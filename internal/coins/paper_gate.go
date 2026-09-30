// internal/coins/paper_gate.go
//
// The coin gate on a mock-test paper: internal/coins answering mocktests.PaperGate.
//
// Like playback_gate.go it is a thin adapter over authorizeDelivery
// (resource_gate.go), so the ordering has exactly one implementation across the
// three gates and only the vocabulary differs.
//
// ── WHAT AN UNLOCK BUYS, restated at the point of the charge ────────────────
//
// ONE PAPER, PERMANENTLY, WITH UNLIMITED ATTEMPTS ON THAT PAPER. Per paper and
// per student: it is not one attempt, and it is not the whole catalogue.
//
// The model says so. resource_unlock is keyed
// (user_id, resource_type, resource_id) under the partial UNIQUE
// resource_unlock_live_uniq, and for resource_type = 'mock_test' the resource_id
// IS the mock test's own id — so an unlock is scoped to a paper, and a student who
// unlocks paper 41 has paper 41 and nothing else. Nothing in the class has an
// attempt dimension: no counter in resource_unlock, no column in
// user_free_allowance, and no cap of any kind in internal/mocktests, where
// SubmitTest inserts one mock_attempts row per submission and never counts what is
// already there.
//
// The docs agree on the unit and disagree on the granularity. 05-economy-and-fraud
// §2.1 prices "Mock-test unlock | 60" and gives the allowance as "3 documents + 1
// video + 1 mock", and its sink table counts unlocks, not attempts;
// 02-architecture.md:475 records the submit path as "unlimited attempts, no cost,
// no per-user throttle". 06-ui-ux-spec §3.6/§10.3 write UI copy for the other
// reading ("1 attempt used of 1", "Uses 1 of your 1 mock test unlock") which the
// model cannot express without a change to the entitlement domain.
//
// So the reading chosen here is the one the model CAN express and the one that
// cannot be trivially abused. An unlock buys one paper forever; the abuse surface
// is sitting one paper indefinitely, which is the same surface documents and
// videos already have by design, and it does NOT reach a second paper for free
// because the uniqueness is per paper and the allowance counts unlocks. A
// per-attempt unlock is reported as the product change it is, not implemented
// here; see the report.
//
// ── why the gate is on the paper route and NOT on SubmitTest ────────────────
//
// Because by submit time the student has already been served the entire paper —
// every question and every option text, in PublicMockTestDetailDTO. A gate on
// submit would refuse the score for a resource already consumed, charge nothing
// for the privilege, and hand the student a worse version of the product: they
// have read every question and are told they get no mark for it. The gate belongs
// where the paper is RELEASED, which is mocktests/handler.go GetTest, and that is
// the only route in the module that returns a question or an option.
package coins

import (
	"context"

	"studsphere/backend/internal/mocktests"
)

// paperGateKeyPrefix namespaces every journal this gate writes. It is distinct
// from playbackGateKeyPrefix and from download_gate.go's own prefix so that
// neither of those can replay a paper purchase and vice versa, which keeps each
// class's slice independently revertable. The class is inside the key regardless,
// so a paper and a document with the same numeric id can never share one.
const paperGateKeyPrefix = "gate:mock-test-paper:"

// paperWording is the copy the paper route contributes.
var paperWording = gateWording{
	unauthenticated: "Sign in to open this mock test.",
	unavailable:     "This mock test is temporarily unavailable.",
	notFound:        "This mock test does not exist or is not published.",
	insufficient:    "You do not have enough StudsTokens for this mock test yet.",
	allowanceLapsed: "Your free unlocks have expired and this mock test is not covered by a purchase.",
}

// paperGate is the *UnlockAPI answering mocktests' PaperGate port.
type paperGate struct {
	api *UnlockAPI
}

// NewPaperGate wires the gate for the paper route. A nil API is a gate that cannot
// decide, and the handler treats that as a 500 rather than as permission.
func NewPaperGate(api *UnlockAPI) mocktests.PaperGate {
	return &paperGate{api: api}
}

// AuthorizePaper implements mocktests.PaperGate.
//
// The class is pinned to mock_test here, which is what makes gates_enabled.mock_test
// independent of the other two: this reads Gates.MockTest, Prices.MockTest and
// allowance.mock_test_unlocks, and there is no path by which turning the video or
// document gate on can charge for a paper.
//
// It is called for every public paper read, including for ungated classes, and
// "allowed, nothing done" is a real answer: it is how the kill switch works. With
// the switch off this returns before any database work at all, which is the
// ship-dark property — see resource_gate.go.
func (g *paperGate) AuthorizePaper(ctx context.Context, userID uint, paperID uint64, title string) mocktests.PaperDecision {
	if g == nil || g.api == nil {
		return mocktests.PaperDecision{
			Allowed: false,
			Refusal: mocktests.NewGateErrorRefusal(mocktests.ErrGateUnconfigured),
		}
	}
	return paperDecisionFrom(g.api.authorizeDelivery(ctx, userID, gateRequest{
		Class:      ResourceTypeMockTest,
		ResourceID: paperID,
		Title:      title,
	}, paperGateKeyPrefix, paperWording))
}

// AuthorizeSubmission implements the second half of mocktests.PaperGate: the
// non-spending possession check the submit route makes.
//
// It deliberately does NOT call authorizeDelivery. That function's whole job is to
// end in a spend, and this one must not: the paper was paid for at the serve, and
// a second charge for the same entitlement is a double sale. So the order here is
// the same first three steps — switch, identity, readiness — and then the ONE
// question submit needs to ask, which is the entitlement check unlock() runs as
// its step 2 anyway.
//
// With the switch off this returns Allowed before touching the database, which is
// the ship-dark property and is what makes the check free for the deployment that
// has not turned mock tests on.
func (g *paperGate) AuthorizeSubmission(ctx context.Context, userID uint, paperID uint64) mocktests.PaperDecision {
	if g == nil || g.api == nil {
		return mocktests.PaperDecision{
			Allowed: false,
			Refusal: mocktests.NewGateErrorRefusal(mocktests.ErrGateUnconfigured),
		}
	}
	api := g.api
	if api.config == nil {
		return mocktests.PaperDecision{
			Allowed: false,
			Refusal: mocktests.NewGateErrorRefusal(mocktests.ErrGateUnconfigured),
		}
	}
	cfg, err := api.config.Load()
	if err != nil {
		return mocktests.PaperDecision{
			Allowed: false,
			Refusal: mocktests.NewGateErrorRefusal(mocktests.ErrGateUnconfigured),
		}
	}
	if !cfg.GateEnabled(ResourceTypeMockTest) {
		return mocktests.PaperDecision{Allowed: true}
	}
	if userID == 0 {
		return paperRefusalDecision(mocktests.GateRefusal{
			Status:  401,
			Code:    gateCodeUnauthenticated,
			Message: paperWording.unauthenticated,
		})
	}
	if !api.entitlementsReady() {
		return mocktests.PaperDecision{
			Allowed: false,
			Refusal: mocktests.NewGateErrorRefusal(mocktests.ErrGateUnconfigured),
		}
	}

	holds, err := api.entitlements.HasAccess(ctx, userID, ResourceTypeMockTest, paperID)
	if err != nil {
		return mocktests.PaperDecision{
			Allowed: false,
			Refusal: mocktests.NewGateErrorRefusal(mocktests.ErrGateFailed),
		}
	}
	if holds {
		return mocktests.PaperDecision{Allowed: true}
	}

	// The student was never served this paper. The 402 is §2.3's own answer for
	// "this costs coins you do not have", carrying the designed object, because
	// the one thing that makes it true is exactly the thing the gate exists to
	// sell: the paper.
	price, priceErr := spendPrice(cfg, ReasonResourceUnlock, ResourceTypeMockTest)
	if priceErr != nil || price <= 0 {
		// A class priced at zero is free, so the serve never created an unlock and
		// this check would refuse every submission forever. A free class is served
		// free at the gate too, and the two must agree.
		return mocktests.PaperDecision{Allowed: true}
	}
	balance, balErr := api.wallet.WalletBalance(ctx, userID, api.now())
	if balErr != nil {
		return mocktests.PaperDecision{
			Allowed: false,
			Refusal: mocktests.NewGateErrorRefusal(mocktests.ErrGateFailed),
		}
	}
	payload := api.insufficientCoinsPayload(ctx, userID, price, balance.TotalAvailable, cfg)
	return paperRefusalDecision(mocktests.GateRefusal{
		Status:  402,
		Code:    CodeInsufficientCoins,
		Message: paperWording.insufficient,
		Data:    payload,
	})
}

func paperRefusalDecision(refusal mocktests.GateRefusal) mocktests.PaperDecision {
	return mocktests.PaperDecision{Allowed: false, Refusal: &refusal}
}

// paperDecisionFrom maps the neutral outcome onto the type mocktests declares.
// A field copy: every status, code and payload was already chosen by
// authorizeDelivery.
func paperDecisionFrom(outcome gateOutcome) mocktests.PaperDecision {
	if outcome.Allowed {
		return mocktests.PaperDecision{Allowed: true, Charged: outcome.Charged}
	}
	refusal := outcome.Refusal
	if refusal == nil {
		return mocktests.PaperDecision{
			Allowed: false,
			Refusal: mocktests.NewGateErrorRefusal(mocktests.ErrGateFailed),
		}
	}
	return mocktests.PaperDecision{
		Allowed: false,
		Refusal: &mocktests.GateRefusal{
			Status:  refusal.Status,
			Code:    refusal.Code,
			Message: refusal.Message,
			Data:    refusal.Data,
		},
	}
}

// Compile-time proof that this is the port the paper route asks, and the reason a
// drift on either side is a build failure rather than a request that quietly
// serves a paper for free.
var _ mocktests.PaperGate = (*paperGate)(nil)
