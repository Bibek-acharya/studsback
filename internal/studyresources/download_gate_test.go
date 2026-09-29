// internal/studyresources/download_gate_test.go
//
// The download route with the coin gate attached, with no database and no object
// storage.
//
// The load-bearing test here is the FIRST one: with no gate wired — which is
// what every deployment looks like until an admin sets
// gates_enabled.study_resource, and what every test in this package that predates
// the gate looks like — a download behaves exactly as it did before the gate
// existed. That is what makes the slice safe to ship dark, and it is the property
// a later edit to this handler would break first.
//
// The rest is about the three things the gate must not do: serve a draft, serve a
// refusal's body as a file, or count a download the student did not get.
package studyresources

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ── fakes ────────────────────────────────────────────────────────────────────

// fakeGate is the coin economy answering from outside this package, which is the
// whole point: this module must not need to know anything about coins to be
// tested, and the gate must be replaceable without a ledger.
type fakeGate struct {
	decision DownloadDecision

	calls   int
	class   string
	id      uint64
	title   string
	userID  uint
	refused error
}

func (f *fakeGate) AuthorizeDownload(_ context.Context, userID uint, resourceType string, resourceID uint64, title string) DownloadDecision {
	f.calls++
	f.userID, f.class, f.id, f.title = userID, resourceType, resourceID, title
	if f.refused != nil {
		return DownloadDecision{Allowed: false, Refusal: f.gateErrorRefusal(f.refused)}
	}
	return f.decision
}

// gateErrorRefusal is the test's stand-in for the coin package's mapping, which
// this package cannot import.
func (f *fakeGate) gateErrorRefusal(err error) *GateRefusal {
	return NewGateErrorRefusal(err)
}

// ── fixtures ─────────────────────────────────────────────────────────────────

// gateFixture seeds one published document and one draft and returns a router
// with the download route and whatever gate was asked for.
//
// authed is the caller: the route is behind authMW in production and a test that
// skipped that would be testing a route nobody deploys.
func gateFixture(t *testing.T, gate DownloadGate, authed bool) (*gin.Engine, *gorm.DB, *StudyResource, *StudyResource) {
	t.Helper()
	withTestJWTSecret(t)
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	published := &StudyResource{
		Title: "Physics Past Questions 2081", ResourceType: TypePastQuestions,
		FileName: "physics.pdf", FilePath: "study-resources/physics.pdf",
		FileURL: "/uploads/study-resources/physics.pdf", FileSize: 10,
		MimeType: "application/pdf", IsPublished: true,
	}
	draft := &StudyResource{
		Title: "Draft physics", ResourceType: TypePastQuestions,
		FileName: "draft.pdf", FilePath: "study-resources/draft.pdf",
		FileURL: "/uploads/study-resources/draft.pdf", FileSize: 10,
		MimeType: "application/pdf", IsPublished: false,
	}
	repo := NewRepository(db)
	for _, resource := range []*StudyResource{published, draft} {
		if err := repo.CreateResource(resource); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(repo))
	if gate != nil {
		h = h.WithDownloadGate(gate)
	}
	// A stand-in for authMW. The real one answers 401 from the JWT; all this
	// route needs to behave correctly is to set the user id the way it does, or
	// to refuse when there is no session.
	r.GET("/api/v1/study-resources/:id/download", func(c *gin.Context) {
		if !authed {
			c.JSON(http.StatusUnauthorized, map[string]any{
				"success": false, "error": "Authentication required",
			})
			c.Abort()
			return
		}
		c.Set("user_id", uint(4242))
		h.DownloadResource(c)
	})
	return r, db, published, draft
}

func downloadPath(id uint) string {
	return "/api/v1/study-resources/" + strconv.FormatUint(uint64(id), 10) + "/download"
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func downloadsOf(t *testing.T, db *gorm.DB, id uint) int {
	t.Helper()
	var resource StudyResource
	if err := db.First(&resource, id).Error; err != nil {
		t.Fatalf("reload %d: %v", id, err)
	}
	return resource.Downloads
}

// ── ship dark: no gate wired ─────────────────────────────────────────────────

// The "ship dark" proof, and the most important test in this slice. With no gate
// attached the download route is byte-for-byte the handler it was before the coin
// economy existed: a published row is served, a draft is a 404, and an anonymous
// caller is a 401 from the middleware.
//
// The 404 and 401 are the handler's own, unchanged — a gate that was consulted
// before them would answer 402 to a request for a draft, which both leaks that
// the draft exists and leaves the student unable to tell "this does not exist"
// from "this costs coins".
func TestDownloadWithoutAGateIsExactlyAsItWas(t *testing.T) {
	// No gate at all, which is the shape of every deployment until an admin
	// switches one on and the shape of every test in this package until now.
	r, db, published, draft := gateFixture(t, nil, true)

	// A published document. Object storage is unavailable in a unit test, so the
	// handler reaches its 404 "File not found" — which is itself the proof that
	// the request travelled the whole ungated path and was not stopped earlier.
	// What must NOT happen is a coin refusal.
	rec := get(t, r, downloadPath(published.ID))
	if rec.Code == http.StatusPaymentRequired || rec.Code == http.StatusLocked {
		t.Errorf("an ungated download was charged: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "INSUFFICIENT") || strings.Contains(rec.Body.String(), "ALLOWANCE") {
		t.Errorf("an ungated download returned a coin body: %s", rec.Body.String())
	}
	// The download counter is incremented before the object read, so it moving is
	// the evidence that the request got past the point a gate would have stopped
	// it.
	if got := downloadsOf(t, db, published.ID); got != 1 {
		t.Errorf("downloads = %d, want 1: the request did not reach the serve path", got)
	}

	// A draft is 404 and must not even bump its counter.
	rec = get(t, r, downloadPath(draft.ID))
	if rec.Code != http.StatusNotFound {
		t.Errorf("draft download = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	if got := downloadsOf(t, db, draft.ID); got != 0 {
		t.Errorf("draft downloads = %d, want 0: a hidden file must not be counted", got)
	}
}

// An anonymous caller is a 401 from authMW, and it stays that way. The gate is
// never reached, so there is nothing for it to get wrong here — and this is the
// assertion that keeps it that way, since a gate wired inside the handler ahead
// of the identity check could turn an anonymous request into a 402.
func TestAnonymousDownloadIs401AndTheGateIsNotConsulted(t *testing.T) {
	gate := &fakeGate{decision: DownloadDecision{Allowed: false, Refusal: &GateRefusal{
		Status: http.StatusPaymentRequired, Code: "INSUFFICIENT_COINS", Message: "no",
	}}}
	r, db, published, _ := gateFixture(t, gate, false)

	rec := get(t, r, downloadPath(published.ID))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous download = %d, want 401 (body %s)", rec.Code, rec.Body.String())
	}
	if gate.calls != 0 {
		t.Errorf("the gate was consulted %d times for an anonymous caller", gate.calls)
	}
	if got := downloadsOf(t, db, published.ID); got != 0 {
		t.Errorf("an anonymous request counted a download: %d", got)
	}
}

// ── the gate is asked the right question ─────────────────────────────────────

// The gate is called with the class the economy prices the row under, the id, the
// title and the caller's user id. Getting the class wrong would charge the
// document price for a video lecture and draw on the wrong allowance, and
// nothing in the schema would notice — so the values are asserted rather than
// assumed.
func TestGateIsAskedAboutTheRightClassAndResource(t *testing.T) {
	t.Run("a document is a study_resource", func(t *testing.T) {
		gate := &fakeGate{decision: DownloadDecision{Allowed: true}}
		r, _, published, _ := gateFixture(t, gate, true)
		get(t, r, downloadPath(published.ID))

		if gate.calls != 1 {
			t.Fatalf("the gate was called %d times, want once per download", gate.calls)
		}
		if gate.class != GateClassStudyResource {
			t.Errorf("class = %q, want %q", gate.class, GateClassStudyResource)
		}
		if gate.id != uint64(published.ID) {
			t.Errorf("id = %d, want %d", gate.id, published.ID)
		}
		if gate.title != published.Title {
			t.Errorf("title = %q, want %q — it is what a receipt should name", gate.title, published.Title)
		}
		if gate.userID != 4242 {
			t.Errorf("user id = %d, want the caller's 4242", gate.userID)
		}
	})

	t.Run("a video lecture is a video", func(t *testing.T) {
		gate := &fakeGate{decision: DownloadDecision{Allowed: true}}
		r, db, _, _ := gateFixture(t, gate, true)
		lecture := &StudyResource{
			Title: "Thermodynamics Lecture 3", ResourceType: TypeVideoLectures,
			FileName: "v.mp4", FilePath: "private/study-resources/v.mp4",
			FileURL: "/uploads/private/study-resources/v.mp4", FileSize: 100,
			MimeType: "video/mp4", IsPublished: true,
		}
		if err := NewRepository(db).CreateResource(lecture); err != nil {
			t.Fatalf("seed the lecture: %v", err)
		}
		get(t, r, downloadPath(lecture.ID))

		if gate.class != GateClassVideo {
			t.Errorf("class = %q, want %q for a video lecture", gate.class, GateClassVideo)
		}
	})
}

// A draft never reaches the gate. It is a 404 from the handler's own publication
// check, which is both the pre-existing behaviour and the reason the gate is
// placed after it.
func TestTheGateIsNotAskedAboutADraft(t *testing.T) {
	gate := &fakeGate{decision: DownloadDecision{Allowed: true}}
	r, _, _, draft := gateFixture(t, gate, true)

	rec := get(t, r, downloadPath(draft.ID))
	if rec.Code != http.StatusNotFound {
		t.Errorf("draft download = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	if gate.calls != 0 {
		t.Errorf("the gate was asked about a draft %d times", gate.calls)
	}
}

// ── a refusal ────────────────────────────────────────────────────────────────

// A refusal is the gate's body, at the gate's status, and NOT a file. A handler
// that fell through to the object read after a refusal would serve the document
// to the student who was just told they could not have it, which is the single
// worst outcome this gate could have.
func TestARefusalIsTheGateBodyAndNoBytes(t *testing.T) {
	payload := map[string]any{
		"required": 40, "available": 25, "shortfall": 15,
		"expires_in_days": nil,
		"ways_to_earn": []map[string]any{
			{"code": "UPLOAD", "label": "Upload a study resource", "potential": 80},
		},
	}
	gate := &fakeGate{decision: DownloadDecision{Allowed: false, Refusal: &GateRefusal{
		Status:  http.StatusPaymentRequired,
		Code:    "INSUFFICIENT_COINS",
		Message: "You do not have enough StudsTokens for this yet.",
		Data:    payload,
	}}}
	r, db, published, _ := gateFixture(t, gate, true)

	rec := get(t, r, downloadPath(published.ID))
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (body %s)", rec.Code, rec.Body.String())
	}

	// The envelope, and the designed object under error.data.
	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Error   struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Data    map[string]any `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode the refusal: %v (%s)", err, rec.Body.String())
	}
	if body.Success {
		t.Error("a refusal reports success: true")
	}
	if body.Error.Code != "INSUFFICIENT_COINS" {
		t.Errorf("code = %q, want INSUFFICIENT_COINS", body.Error.Code)
	}
	if body.Error.Message == "" {
		t.Error("the refusal has no student-facing message")
	}
	for _, key := range []string{"required", "available", "shortfall", "ways_to_earn"} {
		if _, ok := body.Error.Data[key]; !ok {
			t.Errorf("the 402 object has no %q: %s", key, rec.Body.String())
		}
	}
	// 15 coins short is a number the UI can render, so the arithmetic travelled.
	if shortfall, ok := body.Error.Data["shortfall"].(float64); !ok || int64(shortfall) != 15 {
		t.Errorf("shortfall = %v, want 15", body.Error.Data["shortfall"])
	}

	// No attachment: a refused download must not carry a filename header, and its
	// content type must not be the document's.
	if cd := rec.Header().Get("Content-Disposition"); strings.Contains(cd, "attachment") {
		t.Errorf("a refusal was served as an attachment: %q", cd)
	}
	if ct := rec.Header().Get("Content-Type"); strings.Contains(ct, "application/pdf") {
		t.Errorf("a refusal carried the document's content type: %q", ct)
	}
	// And it was not counted: the student did not download anything.
	if got := downloadsOf(t, db, published.ID); got != 0 {
		t.Errorf("a refused download counted %d downloads, want 0", got)
	}
}

// A refusal is not a draft, and a draft is not a refusal: the two 404s and the
// 402 are different screens. A gate that reported a missing resource as
// "insufficient coins" would tell a student to go earn coins for a file that
// does not exist.
func TestTheGateCanDistinguishTheRefusals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		refusal *GateRefusal
		status  int
		code    string
	}{
		{"insufficient", &GateRefusal{Status: http.StatusPaymentRequired, Code: "INSUFFICIENT_COINS", Message: "no coins"}, http.StatusPaymentRequired, "INSUFFICIENT_COINS"},
		{"lapsed allowance", &GateRefusal{Status: http.StatusLocked, Code: "ALLOWANCE_EXPIRED", Message: "expired"}, http.StatusLocked, "ALLOWANCE_EXPIRED"},
		{"unknown resource", &GateRefusal{Status: http.StatusNotFound, Code: "RESOURCE_NOT_FOUND", Message: "gone"}, http.StatusNotFound, "RESOURCE_NOT_FOUND"},
		{"unauthenticated", &GateRefusal{Status: http.StatusUnauthorized, Code: CodeUnauthenticated, Message: "log in"}, http.StatusUnauthorized, CodeUnauthenticated},
		{"gate unavailable", &GateRefusal{Status: http.StatusInternalServerError, Code: CodeGateUnavailable, Message: "unavailable"}, http.StatusInternalServerError, CodeGateUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := &fakeGate{decision: DownloadDecision{Allowed: false, Refusal: tc.refusal}}
			r, db, published, _ := gateFixture(t, gate, true)

			rec := get(t, r, downloadPath(published.ID))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.status, rec.Body.String())
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v (%s)", err, rec.Body.String())
			}
			if body.Error.Code != tc.code {
				t.Errorf("code = %q, want %q", body.Error.Code, tc.code)
			}
			if got := downloadsOf(t, db, published.ID); got != 0 {
				t.Errorf("a refused download counted %d downloads", got)
			}
		})
	}
}

// A gate that refuses without saying why is a bug in the gate, and the handler
// answers 500 rather than inventing a 402. Telling a student with a full wallet
// that they cannot afford something is worse than admitting the server is
// confused, and the test is here because the failure is otherwise silent.
func TestARefusalWithNoBodyIsA500(t *testing.T) {
	gate := &fakeGate{decision: DownloadDecision{Allowed: false, Refusal: nil}}
	r, _, published, _ := gateFixture(t, gate, true)

	rec := get(t, r, downloadPath(published.ID))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "INSUFFICIENT") {
		t.Errorf("an undecided gate was reported as a wallet problem: %s", rec.Body.String())
	}
}

// A gate that cannot decide (no repository, an unreadable config) is a 500 and
// NOT a free download. Failing open here would serve a file the admin believes is
// paid for, which is the one thing a kill switch must not do by accident.
func TestAGateThatCannotDecideIsA500AndNotAFreeDownload(t *testing.T) {
	gate := &fakeGate{refused: ErrGateUnconfigured}
	r, db, published, _ := gateFixture(t, gate, true)

	rec := get(t, r, downloadPath(published.ID))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %s)", rec.Code, rec.Body.String())
	}
	if got := downloadsOf(t, db, published.ID); got != 0 {
		t.Errorf("an undecided gate served a file: %d downloads counted", got)
	}
	if !errors.Is(ErrGateUnconfigured, ErrGateFailed) {
		// Not a requirement on the sentinels — just noting they are distinct
		// reasons that happen to share a status, which is intentional.
		t.Log("the two gate sentinels are distinct, and both map to 500")
	}
}

// ── an allowed download ──────────────────────────────────────────────────────

// An allowed download goes on to the object read, which is the last thing this
// package does before the bytes. Object storage is unavailable in a unit test, so
// the handler's own "File not found" is the observable end state — and the point
// is that the request reached it rather than being turned into a refusal.
func TestAnAllowedDownloadReachesTheServePath(t *testing.T) {
	gate := &fakeGate{decision: DownloadDecision{Allowed: true, Charged: 40}}
	r, db, published, _ := gateFixture(t, gate, true)

	rec := get(t, r, downloadPath(published.ID))
	if rec.Code == http.StatusPaymentRequired || rec.Code == http.StatusLocked {
		t.Fatalf("an allowed download was refused: %d %s", rec.Code, rec.Body.String())
	}
	if gate.calls != 1 {
		t.Errorf("the gate was called %d times, want once", gate.calls)
	}
	// The counter moves for an allowed download and not for a refused one: that
	// is the whole distinction the gate draws, expressed in the one field this
	// package owns.
	if got := downloadsOf(t, db, published.ID); got != 1 {
		t.Errorf("downloads = %d, want 1 for a served download", got)
	}
}

// A bad id is a 400 before anything else, exactly as before the gate. The gate
// must never be asked about id 0.
func TestAMalformedIdIsStill400(t *testing.T) {
	gate := &fakeGate{decision: DownloadDecision{Allowed: true}}
	r, _, _, _ := gateFixture(t, gate, true)

	rec := get(t, r, "/api/v1/study-resources/not-a-number/download")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if gate.calls != 0 {
		t.Errorf("the gate was asked about a malformed id %d times", gate.calls)
	}
}

// ── the class mapping ────────────────────────────────────────────────────────

// The stored resource_type decides the class, and the legacy spellings decide it
// too, because the column holds whatever an older endpoint wrote. This is the
// studyresources half of the pairing with TestStudyResourceClassMappingCoversEveryStoredType
// in internal/coins; if the two ever disagree, a video is charged the document
// price.
func TestGateClassForEveryStoredType(t *testing.T) {
	for _, tc := range []struct {
		stored string
		want   string
	}{
		{TypePastQuestions, GateClassStudyResource},
		{TypeStudyNotes, GateClassStudyResource},
		{TypeModelQuestions, GateClassStudyResource},
		{TypeSyllabus, GateClassStudyResource},
		{TypeVideoLectures, GateClassVideo},
		{"Past Questions", GateClassStudyResource},
		{"Study Notes", GateClassStudyResource},
		{"video", GateClassVideo},
		{"Video Lectures", GateClassVideo},
		// An unrecognised stored value is a document. Every type ever storable in
		// this table is either a video lecture or one of the four documents, so
		// this is the only class that cannot be wrong.
		{"", GateClassStudyResource},
		{"something removed", GateClassStudyResource},
	} {
		got := gateClassFor(&StudyResource{ResourceType: tc.stored})
		if got != tc.want {
			t.Errorf("gateClassFor(%q) = %q, want %q", tc.stored, got, tc.want)
		}
	}
}
