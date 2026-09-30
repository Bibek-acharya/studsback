// internal/studyresources/playback_gate_test.go
//
// The video playback route with the coin gate attached.
//
// THE FIRST TEST IS THE WHOLE SLICE. A playback token is a capability, not a
// hint: it carries its own storage access and the handler consults nothing
// afterwards. So a token minted and then ignored is not a gate, it is a bypass
// with extra steps, and a student refused for coins would walk away holding one.
// That the call site `return`s before `utils.IssuePlaybackToken` is therefore
// the entire difference between a gate and a decoration — and it is exactly the
// kind of thing a later edit moves without noticing. It is asserted here on the
// response body rather than on the position of the `return`, so it survives a
// refactor that hoists the mint.
//
// Everything here reuses the package's existing harness — playbackFixture,
// playbackRouter's shape, requestPlaybackToken — so the seed and the auth stand-in
// are the same ones the pre-gate tests use. A gate tested against a different
// fixture proves less than it looks like it proves.
package studyresources

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ── fakes ────────────────────────────────────────────────────────────────────

// fakePlaybackGate is the coin economy answering from outside this package. The
// point of the port is that this module can prove what it does with an answer
// without owning a ledger.
type fakePlaybackGate struct {
	decision DownloadDecision
	// refusal is built explicitly so a test controls status, code and body. A nil
	// here means "allowed"; a reasonless refusal needs silentRefusingGate.
	refusal *GateRefusal

	calls  int
	userID uint
	id     uint64
	title  string
}

func (f *fakePlaybackGate) AuthorizePlayback(_ context.Context, userID uint, resourceID uint64, title string) DownloadDecision {
	f.calls++
	f.userID, f.id, f.title = userID, resourceID, title
	if f.refusal != nil {
		return DownloadDecision{Allowed: false, Refusal: f.refusal}
	}
	return f.decision
}

// silentRefusingGate refuses with a nil Refusal, which is a bug in a gate rather
// than a decision. It exists to reach the substitution in writeGateRefusal, which
// fakePlaybackGate cannot: its own nil means "allowed".
type silentRefusingGate struct{ inner *fakePlaybackGate }

func (s *silentRefusingGate) AuthorizePlayback(ctx context.Context, userID uint, resourceID uint64, title string) DownloadDecision {
	s.inner.AuthorizePlayback(ctx, userID, resourceID, title)
	return DownloadDecision{Allowed: false, Refusal: nil}
}

func insufficientCoins() *GateRefusal {
	return &GateRefusal{
		Status:  http.StatusPaymentRequired,
		Code:    "INSUFFICIENT_COINS",
		Message: "You need 22 more StudsTokens",
		Data:    map[string]any{"required": 40, "available": 18, "shortfall": 22},
	}
}

// ── fixture ──────────────────────────────────────────────────────────────────

// gatedPlaybackRouter mirrors playbackRouter exactly, with the gate attached. It
// is a copy rather than a parameter on playbackRouter because that helper is used
// by the pre-gate tests, and their "no gate wired" shape is the ship-dark proof
// this file also needs — so the ungated router must keep existing untouched.
func gatedPlaybackRouter(t *testing.T, db *gorm.DB, gate PlaybackGate) *gin.Engine {
	t.Helper()
	withTestJWTSecret(t)
	gin.SetMode(gin.TestMode)

	r := gin.New()
	authMW := func(c *gin.Context) {
		c.Set("user_id", uint(11)) // the same session playbackRouter stands in for
		c.Set("user_role", "student")
		c.Next()
	}
	handler := NewHandler(NewService(NewRepository(db)))
	if gate != nil {
		handler = handler.WithPlaybackGate(gate)
	}
	RegisterRoutes(r, authMW, nil, handler)
	return r
}

// ── the load-bearing test ────────────────────────────────────────────────────

// A refused student receives the refusal and NO capability: no token, no
// stream_url, no expiry. If any of those appear, the gate minted a grant for
// someone it just refused.
func TestARefusedPlaybackGateMintsNoToken(t *testing.T) {
	db, video := playbackFixture(t)
	gate := &fakePlaybackGate{refusal: insufficientCoins()}
	r := gatedPlaybackRouter(t, db, gate)

	rec, _ := requestPlaybackToken(t, r, video.ID)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("playback-token = %d, want 402 (body %s)", rec.Code, rec.Body.String())
	}
	if gate.calls != 1 {
		t.Fatalf("gate consulted %d times, want exactly 1", gate.calls)
	}
	body := rec.Body.String()
	for _, leak := range []string{`"token"`, `"stream_url"`, `"expires_at"`, video.FilePath} {
		if strings.Contains(body, leak) {
			t.Errorf("refusal body carries %s — a capability was minted for a student who was refused:\n%s", leak, body)
		}
	}
}

// ── ship dark ────────────────────────────────────────────────────────────────

// With no gate attached the route is the handler it was before the coin economy
// existed: the same 200, the same token. This is the shape of every deployment
// until an admin sets gates_enabled.video, and the shape the pre-gate tests in
// playback_test.go already assume.
func TestPlaybackWithoutAGateIssuesTheTokenExactlyAsBefore(t *testing.T) {
	db, video := playbackFixture(t)
	r := gatedPlaybackRouter(t, db, nil) // no gate at all

	rec, envelope := requestPlaybackToken(t, r, video.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("ungated playback-token = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if envelope.Data.Token == "" {
		t.Error("ungated response carried no token — the gate changed behaviour while switched off")
	}
	if envelope.Data.StreamURL == "" {
		t.Error("ungated response carried no stream_url")
	}
}

// ── an allowed gate must not break the happy path ────────────────────────────

// A gate that allows is supposed to be invisible. If attaching one changed the
// 200, the gate would be charged with a regression it did not cause.
func TestAnAllowedPlaybackGateIssuesTheTokenAsUsual(t *testing.T) {
	db, video := playbackFixture(t)
	gate := &fakePlaybackGate{decision: DownloadDecision{Allowed: true}}
	r := gatedPlaybackRouter(t, db, gate)

	rec, envelope := requestPlaybackToken(t, r, video.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("allowed gate gave %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if envelope.Data.Token == "" {
		t.Error("an allowed gate suppressed the token")
	}
	if gate.userID != 11 {
		t.Errorf("gate asked about user %d, want the session's 11", gate.userID)
	}
	if gate.id != uint64(video.ID) {
		t.Errorf("gate asked about resource %d, want %d", gate.id, video.ID)
	}
	if gate.title != video.Title {
		t.Errorf("gate got title %q, want %q — the receipt names this", gate.title, video.Title)
	}
}

// ── a refusal without a reason ───────────────────────────────────────────────

// writeGateRefusal substitutes a 500 when the gate says no without saying why.
// Inventing a 402 there would tell a student with a full wallet that they cannot
// afford something, which is a different and much worse lie than a 500.
func TestARefusalWithNoReasonIsFiveHundredNotAPaymentDemand(t *testing.T) {
	db, video := playbackFixture(t)
	gate := &fakePlaybackGate{}
	r := gatedPlaybackRouter(t, db, &silentRefusingGate{inner: gate})

	rec, _ := requestPlaybackToken(t, r, video.ID)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("reasonless refusal = %d, want 500 (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "INSUFFICIENT_COINS") {
		t.Errorf("a reasonless refusal was rendered as a payment demand:\n%s", rec.Body.String())
	}
}
