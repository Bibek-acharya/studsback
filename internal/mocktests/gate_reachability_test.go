package mocktests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/middleware"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

// The companion to internal/studyresources/gate_reachability_test.go: the
// executable form of "is there a second way to a paper's questions?".
//
// A mock test has no file, so there is no object-storage path to check the way
// there is for a study resource. The surface here is entirely the HTTP routes,
// which makes the route inventory and the per-route refusal checks the whole of
// the argument.

// stubGate records what the handler asked and answers with a fixed verdict.
type stubGate struct {
	allowPaper       bool
	allowSubmission  bool
	refusal          *GateRefusal
	paperCalls       []uint64
	submissionCalls  []uint64
	submissionUsers  []uint
	chargeOnServeErr bool
}

func (g *stubGate) AuthorizePaper(_ context.Context, _ uint, paperID uint64, _ string) PaperDecision {
	g.paperCalls = append(g.paperCalls, paperID)
	if g.allowPaper {
		return PaperDecision{Allowed: true}
	}
	if g.refusal != nil {
		return PaperDecision{Allowed: false, Refusal: g.refusal}
	}
	return PaperDecision{Allowed: false, Refusal: NewGateErrorRefusal(ErrGateFailed)}
}

func (g *stubGate) AuthorizeSubmission(_ context.Context, userID uint, paperID uint64) PaperDecision {
	g.submissionCalls = append(g.submissionCalls, paperID)
	g.submissionUsers = append(g.submissionUsers, userID)
	if g.allowSubmission {
		return PaperDecision{Allowed: true}
	}
	if g.refusal != nil {
		return PaperDecision{Allowed: false, Refusal: g.refusal}
	}
	return PaperDecision{Allowed: false, Refusal: NewGateErrorRefusal(ErrGateFailed)}
}

func insufficientCoins() *GateRefusal {
	return &GateRefusal{
		Status:  http.StatusPaymentRequired,
		Code:    "INSUFFICIENT_COINS",
		Message: "You do not have enough StudsTokens for this mock test yet.",
	}
}

func signedInAs(userID uint, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", role)
		c.Next()
	}
}

func mockTestRouteSurface(t *testing.T) []string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, func(c *gin.Context) { c.Next() }, func(c *gin.Context) { c.Next() },
		NewHandler(NewService(NewRepository(setupTestDB(t)))))
	var out []string
	for _, route := range r.Routes() {
		out = append(out, route.Method+" "+route.Path)
	}
	sort.Strings(out)
	return out
}

// The complete route surface, as a literal. There is exactly one route that
// returns a question or an option (GET /:id) and exactly one that returns the
// answer key (GET /admin/mock-tests/:id); if either of those facts changes, or
// a second copy of either appears, this fails.
func TestMockTestRouteSurfaceIsExactlyThis(t *testing.T) {
	got := mockTestRouteSurface(t)
	want := []string{
		// Superadmin CRUD. AdminGetTest returns the full answer key with no
		// coin check; it is reachable only through RequireRole.
		"DELETE /api/v1/admin/mock-tests/:id",
		"GET /api/v1/admin/mock-tests",
		"GET /api/v1/admin/mock-tests/:id",
		"POST /api/v1/admin/mock-tests",
		"PUT /api/v1/admin/mock-tests/:id",
		// Public browsing: the list carries summaries only.
		"GET /api/v1/mock-tests",
		// THE gated route: the only one that releases a paper.
		"GET /api/v1/mock-tests/:id",
		// Owner-scoped attempt results. An attempt only exists if the owner was
		// already served the paper, so this cannot be a way in.
		"GET /api/v1/mock-tests/:id/attempts/:attempt_id",
		// Grading. Not the gate; it asks the gate whether the student HOLDS
		// the paper, so the answer key cannot be enumerated through it.
		"POST /api/v1/mock-tests/:id/submit",
	}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("route count = %d, want %d\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("route[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// With the paper gate refusing everything, no NON-admin route may hand out a
// question or an answer key. The admin routes are deliberately not in this list:
// AdminGetTest returns the full answer key with no coin check by design, and is
// held by the superadmin role check instead — see
// TestAdminAnswerKeyIsRefusedToANonSuperadminSession.
//
// The role middleware here refuses too, so an accidental unwiring of the admin
// group would also show up below rather than silently serving the key.
func TestNoNonAdminRouteServesAPaperWhileTheGateRefuses(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Gated paper")

	gate := &stubGate{allowPaper: false, refusal: insufficientCoins()}
	h := NewHandler(svc).WithPaperGate(gate)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	roleGuard := func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) }
	RegisterRoutes(r, signedInAs(7, "student"), roleGuard, h)

	probes := []struct {
		name   string
		method string
		path   string
	}{
		{"paper detail", http.MethodGet, "/api/v1/mock-tests/" + itoa(dto.ID)},
		{"public list", http.MethodGet, "/api/v1/mock-tests"},
		{"submit", http.MethodPost, "/api/v1/mock-tests/" + itoa(dto.ID) + "/submit"},
		{"attempt result", http.MethodGet, "/api/v1/mock-tests/" + itoa(dto.ID) + "/attempts/1"},
		{"admin list", http.MethodGet, "/api/v1/admin/mock-tests"},
		{"admin detail (the answer key)", http.MethodGet, "/api/v1/admin/mock-tests/" + itoa(dto.ID)},
	}

	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			body := strings.NewReader("")
			if probe.method == http.MethodPost {
				body = strings.NewReader(`{"answers":[{"question_id":1,"option_id":1}]}`)
			}
			r.ServeHTTP(rec, httptest.NewRequest(probe.method, probe.path, body))

			for _, leak := range []string{"correct_option_id", `"explanation"`, "question_text", "option_text"} {
				if strings.Contains(rec.Body.String(), leak) {
					t.Errorf("%s %s leaked %s while the gate refused: %s",
						probe.method, probe.path, leak, rec.Body.String())
				}
			}
		})
	}
}

// The gate's release path is reachable from exactly one route. If a second
// route ever calls AuthorizePaper, the paper has two release points and this
// fails; the route inventory above says which one is allowed to.
func TestOnlyThePaperRouteConsultsTheReleaseGate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Gated paper")

	gate := &stubGate{allowPaper: true}
	h := NewHandler(svc).WithPaperGate(gate)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	roleGuard := func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) }
	RegisterRoutes(r, signedInAs(7, "student"), roleGuard, h)

	before := len(gate.paperCalls)

	// Browse, submit and fetch an attempt. None of these releases a paper.
	for _, probe := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/mock-tests"},
		{http.MethodPost, "/api/v1/mock-tests/" + itoa(dto.ID) + "/submit"},
		{http.MethodGet, "/api/v1/mock-tests/" + itoa(dto.ID) + "/attempts/1"},
		{http.MethodGet, "/api/v1/admin/mock-tests"},
		{http.MethodGet, "/api/v1/admin/mock-tests/" + itoa(dto.ID)},
	} {
		body := strings.NewReader("")
		if probe.method == http.MethodPost {
			body = strings.NewReader(`{"answers":[{"question_id":1,"option_id":1}]}`)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(probe.method, probe.path, body))
	}
	if len(gate.paperCalls) != before {
		t.Errorf("gate release calls = %v, want none from browsing/submit/attempt/admin", gate.paperCalls[before:])
	}

	// The paper route is the one that asks.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/mock-tests/"+itoa(dto.ID), nil))
	if len(gate.paperCalls) != before+1 || gate.paperCalls[before] != uint64(dto.ID) {
		t.Fatalf("gate release calls = %v, want exactly one for paper %d", gate.paperCalls, dto.ID)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("an allowed paper = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "question_text") {
		t.Errorf("an allowed paper should carry its questions: %s", rec.Body.String())
	}
}

// The gate's refusal must be written verbatim, and no question may be counted
// as served: a refused paper is not a view.
func TestRefusedPaperIsNeitherServedNorCounted(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Refused paper")

	gate := &stubGate{allowPaper: false, refusal: insufficientCoins()}
	h := NewHandler(svc).WithPaperGate(gate)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, signedInAs(7, "student"), nil, h)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/mock-tests/"+itoa(dto.ID), nil))

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (body %s)", rec.Code, rec.Body.String())
	}
	for _, forbidden := range []string{"question_text", "option_text", "What is 2 + 2?"} {
		if strings.Contains(rec.Body.String(), forbidden) {
			t.Errorf("the refusal carried paper content (%s): %s", forbidden, rec.Body.String())
		}
	}
	if len(gate.paperCalls) != 1 || gate.paperCalls[0] != uint64(dto.ID) {
		t.Errorf("gate paper calls = %v, want exactly one for paper %d", gate.paperCalls, dto.ID)
	}

	// Views must not have moved.
	var reloaded MockTest
	if err := db.First(&reloaded, dto.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Views != 0 {
		t.Errorf("views = %d, want 0: a refused paper was not read", reloaded.Views)
	}
}

// The submit route must not grade, and must not leak a correctness bit, for a
// student the gate says does not hold the paper. This is the enumeration path:
// question and option ids are small sequential integers, so is_correct is one
// bit of the answer key per request.
func TestSubmitRefusesAStudentWhoDoesNotHoldThePaper(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Unheld paper")

	// First answer a question id and an option id that really belong to the
	// paper, so a leak would be visible rather than a validation error.
	var question MockQuestion
	if err := db.Where("mock_test_id = ?", dto.ID).Order("id").First(&question).Error; err != nil {
		t.Fatalf("load question: %v", err)
	}
	var correct MockOption
	if err := db.Where("mock_question_id = ? AND id = ?", question.ID, question.CorrectOptionID).First(&correct).Error; err != nil {
		t.Fatalf("load correct option: %v", err)
	}
	payload := `{"answers":[{"question_id":` + itoa(question.ID) + `,"option_id":` + itoa(correct.ID) + `}]}`

	gate := &stubGate{allowPaper: true, allowSubmission: false, refusal: insufficientCoins()}
	h := NewHandler(svc).WithPaperGate(gate)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, signedInAs(7, "student"), nil, h)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/mock-tests/"+itoa(dto.ID)+"/submit", strings.NewReader(payload)))

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"is_correct"`) {
		t.Errorf("the refusal leaked a correctness bit: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"score"`) {
		t.Errorf("the refusal leaked a score: %s", rec.Body.String())
	}
	// And nothing was persisted: a refusal must not create an attempt, because
	// an attempt is the durable record that the student holds the paper.
	var attempts int64
	db.Model(&MockAttempt{}).Where("user_id = ?", 7).Count(&attempts)
	if attempts != 0 {
		t.Errorf("attempts recorded = %d, want 0", attempts)
	}

	// The gate must have been asked about THIS paper and THIS user.
	if len(gate.submissionCalls) != 1 || gate.submissionCalls[0] != uint64(dto.ID) {
		t.Errorf("gate submission calls = %v, want one for paper %d", gate.submissionCalls, dto.ID)
	}
	if len(gate.submissionUsers) != 1 || gate.submissionUsers[0] != 7 {
		t.Errorf("gate submission users = %v, want [7]", gate.submissionUsers)
	}
}

// A student who DOES hold the paper gets graded, and only then.
func TestSubmitGradesAStudentWhoHoldsThePaper(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Held paper")

	var question MockQuestion
	if err := db.Where("mock_test_id = ?", dto.ID).Order("id").First(&question).Error; err != nil {
		t.Fatalf("load question: %v", err)
	}
	var correct MockOption
	if err := db.Where("mock_question_id = ? AND id = ?", question.ID, question.CorrectOptionID).First(&correct).Error; err != nil {
		t.Fatalf("load correct option: %v", err)
	}
	payload := `{"answers":[{"question_id":` + itoa(question.ID) + `,"option_id":` + itoa(correct.ID) + `}]}`

	gate := &stubGate{allowPaper: true, allowSubmission: true}
	h := NewHandler(svc).WithPaperGate(gate)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, signedInAs(7, "student"), nil, h)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/mock-tests/"+itoa(dto.ID)+"/submit", strings.NewReader(payload)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"is_correct"`) {
		t.Errorf("a held paper should be graded: %s", rec.Body.String())
	}
}

// An attempt result is owner-scoped. Combined with the submit check above this
// closes the second read path: there is no attempt to fetch for a paper you
// were never served, and no attempt of yours to read someone else's.
func TestAttemptResultsAreOwnerScoped(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Owner scoped")

	result, err := svc.SubmitTest(dto.ID, 7, []AnswerInput{
		{QuestionID: dto.Questions[0].ID, OptionID: dto.Questions[0].Options[1].ID},
	})
	if err != nil {
		t.Fatalf("seed attempt: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, signedInAs(8, "student"), nil, NewHandler(svc))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/v1/mock-tests/"+itoa(dto.ID)+"/attempts/"+itoa(result.AttemptID), nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("another user's attempt = %d, want 403 (body %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"is_correct"`) {
		t.Errorf("another user's attempt leaked correctness bits: %s", rec.Body.String())
	}
}

// The admin group returns the full answer key with no coin check. That is only
// acceptable because it is genuinely superadmin-guarded, so this uses the REAL
// middleware.RequireRole with the exact allow-list main.go passes — not a stub
// that always aborts.
func TestAdminAnswerKeyIsRefusedToANonSuperadminSession(t *testing.T) {
	previous := config.AppConfig
	t.Cleanup(func() { config.AppConfig = previous })
	config.AppConfig = &config.Config{JWTSecret: "test-secret", JWTExpiry: "24h"}

	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Admin only")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, middleware.Auth(), middleware.RequireRole("superadmin", "super_admin"), NewHandler(svc))

	for _, role := range []string{"student", "admin", "institution", "scholarship_provider", "scholarship_provider_subuser", ""} {
		t.Run("role "+role, func(t *testing.T) {
			session, err := utils.GenerateToken(11, "someone@example.com", role, 0)
			if err != nil {
				t.Fatalf("mint session: %v", err)
			}
			for _, path := range []string{
				"/api/v1/admin/mock-tests/" + itoa(dto.ID),
				"/api/v1/admin/mock-tests",
			} {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				req.Header.Set("Authorization", "Bearer "+session)
				r.ServeHTTP(rec, req)
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q on %s = %d, want 403 (body %s)", role, path, rec.Code, rec.Body.String())
				}
				if strings.Contains(rec.Body.String(), "correct_option_id") {
					t.Errorf("role %q read the answer key from %s: %s", role, path, rec.Body.String())
				}
			}
		})
	}

	// A real superadmin does get it, so the guard is a check rather than a wall.
	session, err := utils.GenerateToken(11, "boss@example.com", "superadmin", 0)
	if err != nil {
		t.Fatalf("mint session: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/mock-tests/"+itoa(dto.ID), nil)
	req.Header.Set("Authorization", "Bearer "+session)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("superadmin on the answer key = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "correct_option_id") {
		t.Errorf("the admin editor must still see the answer key: %s", rec.Body.String())
	}
}

// The public list is unauthenticated and entirely unaffected by the gate, so it
// has to stay summary-only: no question graph, no answer key, nothing a
// student could enumerate into.
func TestPublicListStaysSummaryOnly(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	createTest(t, svc, "Listed paper")

	// No gate at all: the list must still not carry a paper.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, nil, nil, NewHandler(svc))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/mock-tests", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("public list = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	for _, forbidden := range []string{"questions", "question_text", "option_text", "correct_option_id", "is_correct"} {
		if strings.Contains(rec.Body.String(), forbidden) {
			t.Errorf("the public list carries %q: %s", forbidden, rec.Body.String())
		}
	}
}

// A draft must be a 404 on the paper route and absent from the list, whatever
// the gate says. Gating before the publication check would answer 402 to a
// request for something nobody is allowed to know exists.
func TestDraftsAreUnreachableRegardlessOfTheGate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Draft paper")
	if _, err := svc.UpdateTest(dto.ID, UpdateMockTestRequest{IsPublished: boolPtr(false)}); err != nil {
		t.Fatalf("unpublish: %v", err)
	}

	// The gate ALLOWS everything, so a 402 here could only be a gate-before-
	// publication ordering bug.
	gate := &stubGate{allowPaper: false, refusal: insufficientCoins()}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, signedInAs(7, "student"), nil, NewHandler(svc).WithPaperGate(gate))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/mock-tests/"+itoa(dto.ID), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("draft paper detail = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/mock-tests", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("public list = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Draft paper") {
		t.Errorf("the public list disclosed a draft: %s", rec.Body.String())
	}
}
