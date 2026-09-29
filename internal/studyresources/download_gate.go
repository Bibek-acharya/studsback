// internal/studyresources/download_gate.go
//
// The coin gate on document downloads.
//
// ── why the port is declared HERE ────────────────────────────────────────────
//
// This file declares an interface that internal/coins implements, and it is
// declared in the module that OWNS THE BYTES rather than in the module that owns
// the money. That direction is the whole reason there is no import cycle:
//
//   - this module already knows the resource (it read the row two lines earlier,
//     including whether it is published and what it is called), so the gate only
//     has to answer a question about a class and an id;
//   - it cannot answer that question itself, because spending is in internal/coins;
//   - so the question is asked through a port, and the port is declared on the
//     asking side. That is the rule Go forces once both sides need each other:
//     somebody has to own the interface, and the owner must be the side that
//     does not need the other's types.
//
// The alternative — importing coins here and calling its API directly — is the
// other half of the cycle, and it is not merely ugly: coins needs to read the
// study-resources table for its 404 check and its receipt title
// (study_resource_lookup.go), so the two would import each other and the build
// would stop.
//
// This is the same shape as the profile-completion adapter in main.go, applied
// in the other direction because here the CONSUMER of the answer is in the other
// module, not the implementer.
//
// ── what the gate is allowed to decide ───────────────────────────────────────
//
// Only whether to release bytes. It does not serve the file, count the download
// or translate a status: the handler owns all of that, so a refusal is the same
// body whether it came from a gate or from the publication check. And it never
// answers 401 — the route is already behind authMW, and re-implementing that
// check here would be a second source of truth about who is logged in.
package studyresources

import (
	"context"
	"errors"
	"net/http"
)

// DownloadGate authorizes a student to receive a stored file, or explains why
// not.
//
// The class and the id are passed rather than the resource itself so the
// interface stays a question about the economy rather than a handle onto this
// module's row. The title is passed alongside them because the caller has it
// already and it is what a receipt should name; the gate is not asked to look
// it up again.
type DownloadGate interface {
	AuthorizeDownload(ctx context.Context, userID uint, resourceType string, resourceID uint64, title string) DownloadDecision
}

// DownloadDecision is the gate's answer, and it is a value rather than an error
// because a refusal is a designed outcome, not a failure. The three situations
// the frontend has to tell apart — not enough coins, lapsed starter unlocks,
// and an unknown or unpublished resource — are all refusals with different
// statuses, and modelling them as Go errors would put a documented product state
// behind the same path as a database outage.
type DownloadDecision struct {
	// Allowed is true when the bytes may be served. When it is true nothing
	// else on this struct is read: a student who already owns the file is
	// served without a charge, and a student whose allowance covered it is
	// served without one too.
	Allowed bool
	// Charged is the coins actually spent to reach this decision. It is zero for
	// an already-owned file, an allowance unlock and an ungated class, and it is
	// only ever read for logging and for the download's own counter. It is not a
	// bill: the student has already been told what a download costs.
	Charged int64
	// Refusal carries the status and body of a refusal, and is nil whenever
	// Allowed is true.
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
	// expires_in_days, ways_to_earn — for a 402 and a 423, and nil for the rest.
	// It is typed as any because it is rendered verbatim into the error envelope
	// this module cannot name: the shape is the coin economy's, and duplicating
	// it here would be a second contract to keep in step.
	Data any
}

// Gate errors, distinct from ErrNotAVideoResource and from the service's
// "resource not found". They exist so a caller can tell a gate that could not
// run from a gate that said no.
//
// ErrGateUnconfigured is the answer when the gate is wired but the coin service
// is not: no repository, no config reader, or a config row this build cannot
// parse. It maps to 500, because nothing the student did is wrong and the file
// they are entitled to exists. Serving the bytes anyway would be a silent bypass
// of a gate an admin believes is on, and that is the one outcome a money gate
// must never produce by accident.
var (
	ErrGateUnconfigured = errors.New("the download gate is not connected to a database")
	ErrGateFailed       = errors.New("the download gate could not decide")
)

// gateRefusalPayload is the body the handler writes for a refusal: the same
// {success, message, error} envelope the coin endpoints use, so a client parses
// one shape whether the refusal came from a wallet call or from a download.
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
// It is a method rather than a bare function because the handler must never
// assemble this itself: the envelope is the contract the frontend parses, and a
// second spelling of it is a second thing to keep in step with the coin
// endpoints' ErrorEnvelope.
func (r *GateRefusal) RefusalBody() any {
	payload := gateRefusalPayload{Success: false, Message: r.Message}
	payload.Error.Code = r.Code
	payload.Error.Message = r.Message
	payload.Error.Data = r.Data
	return payload
}

// CodeGateUnavailable is the code for a gate that could not decide. It is
// deliberately not one of the §2.3 codes: §2.3 enumerates refusals about the
// student's wallet, and naming an outage with INSUFFICIENT_COINS would put a
// money message on an error with no money in it.
const CodeGateUnavailable = "GATE_UNAVAILABLE"

// CodeUnauthenticated is the code for a request with no session. The production
// route answers 401 from authMW before this module is reached, so this value is
// only reachable from a gate wired onto a route that is not wrapped — it exists
// so that case still parses as the login screen rather than as a generic error.
const CodeUnauthenticated = "UNAUTHENTICATED"

// NewGateErrorRefusal maps an error the gate could not decide on into the
// refusal the handler writes.
//
// Both known errors answer 500, and the difference between "not configured" and
// "failed" is for the log rather than the response: a client cannot act on
// either, and inventing a distinction in the status would only teach the
// frontend to branch on something meaningless. The err parameter is taken rather
// than ignored so the call site reads as a mapping and a reader looking for
// where a sentinel becomes a status finds it in one place.
func NewGateErrorRefusal(err error) *GateRefusal {
	_ = err
	return &GateRefusal{
		Status:  http.StatusInternalServerError,
		Code:    CodeGateUnavailable,
		Message: "This download is temporarily unavailable.",
	}
}

// GateClassVideo and GateClassStudyResource are the classes the coin economy
// prices a study-resource row under. They are declared here as the values the
// gate is asked about, and studyResourceClass in internal/coins maps the stored
// resource_type onto one of them, so the vocabulary has one definition on each
// side and the mapping is tested from both.
const (
	GateClassStudyResource = "study_resource"
	GateClassVideo         = "video"
)
