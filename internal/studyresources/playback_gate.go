// internal/studyresources/playback_gate.go
//
// The coin gate on video playback, and why it is a SEPARATE port from
// DownloadGate rather than another call through it.
//
// ── why a second port, in the same module ────────────────────────────────────
//
// The download gate's method takes a class:
//
//	AuthorizeDownload(ctx, userID, resourceType, resourceID, title)
//
// That parameter is a hazard, not a convenience. The class is what selects the
// PRICE (ledger.go spendPrice), the ALLOWANCE COLUMN (unlock.go allowanceClasses)
// and the kill switch (config.go EnabledFor), and the entitlement it writes
// records the class the CALLER named rather than the class the row actually is —
// study_resource_lookup.go exists partly because a mismatch there is invisible to
// the schema. So a caller that names the wrong class is charged the wrong price
// and given a durable entitlement for the wrong thing, and nothing downstream can
// see it.
//
// A playback token is only ever minted for a video lecture: playback.go reaches
// the gate only after GetPlayableVideoResource has refused drafts, documents and
// rows with no stored object. So the class is a CONSTANT of this route, and the
// port therefore does not accept one:
//
//	AuthorizePlayback(ctx, userID, resourceID, title)
//
// A caller cannot ask the wrong question, because the question has no class
// parameter to get wrong. The decision and refusal TYPES are still the ones
// download_gate.go declares — same package, same wire envelope, one refusal body
// renderer — because duplicating them would be a second spelling of a contract the
// frontend already parses.
//
// ── the direction of the dependency is unchanged ─────────────────────────────
//
// Declared here, implemented in internal/coins, for exactly the reason
// download_gate.go sets out: this module owns the bytes and the row, coins owns
// the money, and the question is asked through a port the asking side owns. Both
// gates are satisfied by the SAME object in main.go, which is the point — the
// entitlement/allowance/balance ordering is one implementation, reached twice.
//
// ── what the gate is allowed to decide ───────────────────────────────────────
//
// Only whether to mint a grant. It does not issue the token, and the handler
// does not issue one when the gate refuses: a token that is minted and then
// ignored is a bypass, not a gate. See the call site in playback.go.
package studyresources

import (
	"context"

	"github.com/gin-gonic/gin"
)

// PlaybackGate authorizes a student to receive a playback grant for a video
// lecture, or explains why not.
//
// The class is absent by design; see the file header. The title is passed
// alongside the id because the caller has it already and it is what a receipt
// should name — the gate is not asked to look it up again.
type PlaybackGate interface {
	AuthorizePlayback(ctx context.Context, userID uint, resourceID uint64, title string) DownloadDecision
}

// PlaybackDecision is DownloadDecision under the name this route uses.
//
// It is an ALIAS, not a defined type, so the two gates cannot drift: a refusal
// built by one is a refusal the other can write, and there is exactly one
// RefusalBody implementation behind both.
//
// The compile-time proof that internal/coins satisfies this port is on the
// implementing side — `var _ studyresources.PlaybackGate = (*playbackGate)(nil)`
// in internal/coins/playback_gate.go — so a signature change breaks the build
// there rather than at runtime on the first play.
type PlaybackDecision = DownloadDecision

// writeGateRefusal writes a refusal and stops the handler.
//
// It lives here rather than in download_gate.go because that file is committed and
// independently revertable and must not be touched. DownloadResource still carries
// its own inline copy of these four lines; the day that file is next edited by
// someone entitled to edit it, this is what should replace that copy.
func writeGateRefusal(c *gin.Context, refusal *GateRefusal) {
	if refusal == nil {
		// A gate that says no without saying why is a bug in the gate, and 500 is
		// the only honest response: inventing a 402 here would tell a student with a
		// full wallet that they cannot afford something.
		refusal = NewGateErrorRefusal(ErrGateFailed)
	}
	c.JSON(refusal.Status, refusal.RefusalBody())
}
