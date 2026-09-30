// internal/coins/playback_gate.go
//
// The coin gate on video playback: internal/coins answering
// studyresources.PlaybackGate.
//
// It is a thin adapter over authorizeDelivery (resource_gate.go) rather than a
// second implementation of the decision. The whole of the ordering — switch,
// identity, readiness, resource resolution, server-resolved price, then
// entitlement/allowance/balance/purchase — lives there and is not restated here.
//
// ── the one thing this file decides that the core does not ──────────────────
//
// The class. A playback grant is always the video class, and the video class is a
// different switch, a different price and a different allowance column from a
// document. Pinning it here rather than taking it as an argument is what makes
// gates_enabled.video and gates_enabled.study_resource genuinely independent: a
// request here consults Gates.Video and Prices.Video and
// allowance.video_unlocks, and there is no path by which turning the document gate
// on could charge a video at the document price.
//
// ── why the key namespace is this route's own ───────────────────────────────
//
// A video DOWNLOAD (handler.go DownloadResource, class video) and a video PLAYBACK
// (here) are the same purchase — one entitlement, keyed (user, video, id) — but
// they are separate slices with separate kill switches, and a rollback of one must
// not be able to replay or orphan a journal written by the other. So the two mint
// keys in two namespaces. The cross-route double charge is prevented by something
// stronger than the key: the entitlement check runs first, so the second route to
// be asked about an already-owned video is answered ALREADY_OWNED with no charge.
package coins

import (
	"context"

	"studsphere/backend/internal/studyresources"
)

// playbackGateKeyPrefix namespaces every journal this gate writes.
const playbackGateKeyPrefix = "gate:video-playback:"

// playbackWording is the copy the playback route contributes. The statuses, codes
// and the §2.3 data object are NOT here: they are the same for every gate, which
// is why a client parses one shape whatever refused it.
var playbackWording = gateWording{
	unauthenticated: "Sign in to play this video.",
	unavailable:     "This video is temporarily unavailable.",
	notFound:        "This video does not exist, is not published, or is not a video lecture.",
	insufficient:    "You do not have enough StudsTokens for this video yet.",
	allowanceLapsed: "Your free unlocks have expired and this video is not covered by a purchase.",
}

// playbackGate is the *UnlockAPI answering studyresources' PlaybackGate port.
//
// The same object answers the wallet, the document gate and the paper gate,
// deliberately: the entitlement, the allowance, the price and the purchase are all
// one implementation, and a second one is the duplication resource_gate.go exists
// to avoid.
type playbackGate struct {
	api *UnlockAPI
}

// NewPlaybackGate wires the gate for the playback-token route. A nil API is a gate
// that cannot decide, and the handler treats that as a 500 rather than as
// permission — see studyresources.ErrGateUnconfigured.
func NewPlaybackGate(api *UnlockAPI) studyresources.PlaybackGate {
	return &playbackGate{api: api}
}

// AuthorizePlayback implements studyresources.PlaybackGate.
//
// It is called for every playback-token request, including for ungated videos, and
// answering "allowed, nothing done" is a real answer rather than a shortcut: it is
// how the kill switch works. The switch is read from the cached config, so turning
// the video class back to free is a config write and not a deploy, and an ungated
// play costs one cached read and no database work at all.
func (g *playbackGate) AuthorizePlayback(ctx context.Context, userID uint, resourceID uint64, title string) studyresources.DownloadDecision {
	if g == nil || g.api == nil {
		return studyresources.DownloadDecision{
			Allowed: false,
			Refusal: studyresources.NewGateErrorRefusal(studyresources.ErrGateUnconfigured),
		}
	}
	return playbackDecisionFrom(g.api.authorizeDelivery(ctx, userID, gateRequest{
		Class:      ResourceTypeVideo,
		ResourceID: resourceID,
		Title:      title,
	}, playbackGateKeyPrefix, playbackWording))
}

// playbackDecisionFrom maps the neutral outcome onto the type studyresources
// declares. It is a field copy, not a second decision: every status, code and
// payload was already chosen by authorizeDelivery.
func playbackDecisionFrom(outcome gateOutcome) studyresources.DownloadDecision {
	if outcome.Allowed {
		return studyresources.DownloadDecision{Allowed: true, Charged: outcome.Charged}
	}
	refusal := outcome.Refusal
	if refusal == nil {
		// Unreachable — authorizeDelivery always attaches a refusal when it refuses
		// — and handled rather than dereferenced anyway, because a nil here would
		// be a nil dereference in a money path.
		return studyresources.DownloadDecision{
			Allowed: false,
			Refusal: studyresources.NewGateErrorRefusal(studyresources.ErrGateFailed),
		}
	}
	return studyresources.DownloadDecision{
		Allowed: false,
		Refusal: &studyresources.GateRefusal{
			Status:  refusal.Status,
			Code:    refusal.Code,
			Message: refusal.Message,
			Data:    refusal.Data,
		},
	}
}

// Compile-time proof that this is the port the playback route asks. A signature
// change on either side breaks the build here rather than at runtime on the first
// play — the same reason download_gate.go carries the same assertion for the
// download route.
var _ studyresources.PlaybackGate = (*playbackGate)(nil)
