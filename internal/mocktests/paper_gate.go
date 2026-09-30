// internal/mocktests/paper_gate.go
//
// The coin gate on a mock-test paper, declared on the side that OWNS THE PAPER.
//
// ── why the port lives here and not in internal/coins ────────────────────────
//
// The same direction the download gate uses, for the same reason. This module
// knows whether a test is published, what it is called and what its question graph
// is; it cannot answer "may this student have it" because spending is in
// internal/coins. So the question is asked through a port declared by the asking
// side, and internal/coins implements it without importing this module's types.
// Somebody has to own the interface and the owner must be the side that does not
// need the other's types — that is the whole of the rule, and it is why the two
// modules never import each other.
//
// ── what a mock-test unlock BUYS, and how that was established ───────────────
//
// This is the question the design leaves under-specified, so here is the evidence
// rather than an invented rule. The code answers it, and the answer is:
//
//	AN UNLOCK BUYS ONE PAPER, PERMANENTLY, WITH UNLIMITED ATTEMPTS ON THAT
//	PAPER. It is per paper and per student. It is NOT one attempt, and it is
//	NOT unlimited papers.
//
// From the model:
//
//   - resource_unlock is keyed (user_id, resource_type, resource_id) with the
//     partial UNIQUE resource_unlock_live_uniq (unlock_model.go:33-48). For
//     resource_type = 'mock_test', resource_id is the mock test's OWN id. So an
//     unlock is scoped to a paper, not to a student: a student who unlocks paper
//     41 has paper 41 and nothing else.
//   - The class has no attempt dimension at all. There is no attempt counter in
//     resource_unlock, no attempt column in user_free_allowance, and no
//     per-attempt cap anywhere in this module. MockAttempt (model.go:60-71) is
//     append-only history: SubmitTest inserts one row per submission and there is
//     no check of any kind against how many already exist.
//
// From the docs:
//
//   - 05-economy-and-fraud.md §2.1 prices "Mock-test unlock | 60" and gives the
//     free allowance as "3 documents + 1 video + 1 mock". The unit of the economy
//     is the UNLOCK, and the faucet/sink table counts "Mock-test unlocks 100 x 60".
//     One mock in the allowance is one paper.
//   - 02-architecture.md:475 records the same thing about the current code: the
//     submit path is "Auth-gated but unlimited attempts, no cost, no per-user
//     throttle", and the change asked for is a coin check.
//
// ── the reading that cannot be trivially abused, and why it is this one ─────
//
// The alternative reading — an unlock buys ONE ATTEMPT — is the one 06-ui-ux-spec
// §3.6 and §10.3 write copy for ("1 attempt used of 1", "Uses 1 of your 1 mock
// test unlock"). That reading is NOT implementable against the model this slice
// must not change: an attempt is a row in mock_attempts, and an entitlement
// counted in resource_unlock cannot be decremented per attempt without a second
// counter that the existing partial-unique index knows nothing about. Choosing it
// would mean changing the entitlement domain, which is out of scope here.
//
// So the choice is between two readings that the model CAN express, and the one
// above is the safe one:
//
//   - ONE UNLOCK = ONE PAPER, unlimited attempts. The abuse surface is "take one
//     paper and sit it indefinitely". The same surface already exists, by design,
//     for documents and videos, and the economy is priced per resource: the sink
//     table in 05 §2.3 counts unlocks, not attempts. Critically, it does not let a
//     student reach a SECOND paper for free — the unique index is per paper, and
//     the allowance is a count of unlocks, so the starter allowance buys exactly
//     one paper. That is the cap that matters, and it is structural.
//   - ONE UNLOCK = UNLIMITED PAPERS would be the trivially abusable one, and it is
//     not what resource_unlock_live_uniq permits.
//
// The divergence from the UI copy is real and is reported rather than papered
// over: metering attempts is a product change that needs an entitlement-domain
// change, and the copy in 06 would have to change with it.
//
// ── what the gate may decide ─────────────────────────────────────────────────
//
// Only whether to release the paper. It never grades, never writes a mock_attempt
// row and never counts a view — the handler owns all three, so a refusal leaves
// the module exactly as it found it.
//
// A NOTE ON WHAT THIS GATE IS NOT. It is not attached to SubmitTest. By the time a
// submission arrives the student has already been served the whole paper — every
// question and every option, in PublicMockTestDetailDTO — so refusing the
// submission would take away the score for a resource they have already consumed
// and would charge nothing for the privilege. The gate is where the paper is
// SERVED. See the call site in handler.go, which says so at the point of use.
package mocktests

import (
	"context"
	"errors"
	"net/http"
)

// PaperGate authorizes a student to receive a mock test's questions, or explains
// why not.
//
// The class is NOT a parameter, for the same reason it is not one on the playback
// port: a mock test is only ever a mock test, and a class parameter on a gate is a
// way for a caller to be charged the wrong price and handed a durable entitlement
// for the wrong resource with nothing downstream able to see it. The title is
// passed because the caller has it and it is what a receipt should name.
type PaperGate interface {
	// AuthorizePaper decides whether this paper may be RELEASED, and may charge
	// for it. It is the gate, and it is the only method that moves money.
	AuthorizePaper(ctx context.Context, userID uint, paperID uint64, title string) PaperDecision

	// AuthorizeSubmission decides whether this student may be GRADED on this
	// paper, and MUST NOT charge, burn an allowance or write a journal.
	//
	// It exists to close one specific hole, not to gate the product: question and
	// option ids are small sequential integers, so a student who was refused the
	// paper could otherwise enumerate them against the submit route and read the
	// answer key off is_correct. A student who was actually served the paper
	// already holds the entitlement, because the serve wrote it, so this cannot
	// refuse anyone who did the thing it checks. See handler.go SubmitTest.
	//
	// Returning a PaperDecision rather than a bool is so the 402 body is the same
	// designed §2.3 object the wallet, the document route and the playback route
	// write, instead of a second one assembled here.
	AuthorizeSubmission(ctx context.Context, userID uint, paperID uint64) PaperDecision
}

// PaperDecision is the gate's answer.
//
// It is a value rather than an error for the reason DownloadDecision is: a refusal
// is a designed product state with its own status, code and body, and modelling it
// as a Go error would put "you cannot afford this yet" on the same path as a
// database outage.
type PaperDecision struct {
	// Allowed is true when the paper may be served. When it is true nothing else
	// here is read: an already-owned paper and a starter-allowance paper are both
	// served with no charge, and an ungated class is served having consulted
	// nothing at all.
	Allowed bool
	// Charged is the coins actually spent to reach this decision. It is zero for an
	// owned paper, an allowance unlock and an ungated class, and it is read only
	// for logging. It is not a bill: the student was told the price before this.
	Charged int64
	// Refusal carries the status and body of a refusal and is nil whenever Allowed
	// is true.
	Refusal *GateRefusal
}

// GateRefusal is one refusal, in the shape the handler writes it.
type GateRefusal struct {
	// Status is the HTTP status to answer with.
	Status int
	// Code is the machine-readable reason from 03-api-contract.md §2.3
	// (INSUFFICIENT_COINS, ALLOWANCE_EXPIRED, RESOURCE_NOT_FOUND). It travels in
	// the body rather than only in a log, because the frontend switches on it.
	Code string
	// Message is the student-facing sentence.
	Message string
	// Data is the designed object of §2.3 — required, available, shortfall,
	// expires_in_days, ways_to_earn — for a 402 and a 423, and nil for the rest. It
	// is typed as any because it is rendered verbatim into an envelope this module
	// cannot name: the shape belongs to the coin economy, and restating it here
	// would be a second contract to keep in step.
	Data any
}

// Gate errors, so a caller can tell a gate that could not run from a gate that said
// no. Both answer 500: nothing the student did is wrong, and the paper they are
// entitled to exists. Serving it anyway would be a silent bypass of a gate an
// admin believes is on, which is the one outcome a money gate must never produce by
// accident.
var (
	ErrGateUnconfigured = errors.New("the mock test gate is not connected to a database")
	ErrGateFailed       = errors.New("the mock test gate could not decide")
)

// gateRefusalPayload is the body written for a refusal: the same
// {success, message, error} envelope the coin endpoints and the download gate use,
// so one client parses one shape whether the refusal came from a wallet call, a
// document download, a video playback or a paper.
type gateRefusalPayload struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data,omitempty"`
	} `json:"error"`
}

// RefusalBody renders a refusal as the JSON body to write.
//
// It is a method rather than a free function because the handler must never
// assemble this itself: the envelope is the contract the frontend parses.
func (r *GateRefusal) RefusalBody() any {
	payload := gateRefusalPayload{Success: false, Message: r.Message}
	payload.Error.Code = r.Code
	payload.Error.Message = r.Message
	payload.Error.Data = r.Data
	return payload
}

// CodeGateUnavailable is the code for a gate that could not decide. It is
// deliberately not one of the §2.3 codes: §2.3 enumerates refusals about the
// student's wallet, and naming an outage INSUFFICIENT_COINS would put a money
// message on an error with no money in it.
const CodeGateUnavailable = "GATE_UNAVAILABLE"

// CodeUnauthenticated is the code for a request with no session. The production
// paper route sits behind OptionalAuth rather than Auth so that an ungated class
// stays browsable, so this is genuinely reachable — a logged-out visitor asking
// for a gated paper is answered with the login screen rather than a generic error.
const CodeUnauthenticated = "UNAUTHENTICATED"

// NewGateErrorRefusal maps an error the gate could not decide on into the refusal
// the handler writes. The err parameter is taken rather than ignored so the call
// site reads as a mapping and a reader looking for where a sentinel becomes a
// status finds it in one place.
func NewGateErrorRefusal(err error) *GateRefusal {
	_ = err
	return &GateRefusal{
		Status:  http.StatusInternalServerError,
		Code:    CodeGateUnavailable,
		Message: "This mock test is temporarily unavailable.",
	}
}

// GateClassMockTest is the class the coin economy prices a paper under. It is
// declared here as the value the gate is asked about, and internal/coins maps onto
// its own ResourceTypeMockTest constant, so the vocabulary has one definition on
// each side and the mapping is tested from both. See
// TestGateClassMockTestIsTheClassCoinsPrices.
const GateClassMockTest = "mock_test"
