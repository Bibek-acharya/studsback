package mocktests

import (
	"errors"
	"net/http"
	"strconv"

	"studsphere/backend/internal/shared/httpx"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
	// gate is the coin entitlement check on papers. It is a field with a setter
	// rather than a constructor argument so that everything which knows nothing
	// about the coin economy — and the tests for it — keep constructing this
	// handler the way they always have. See paper_gate.go for the port and for why
	// it is declared on this side rather than in internal/coins.
	gate PaperGate
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// WithPaperGate wires the coin gate. Called once from main.go; a handler without
// it serves every paper ungated, which is the pre-coin behaviour and not a
// failure mode.
func (h *Handler) WithPaperGate(gate PaperGate) *Handler {
	h.gate = gate
	return h
}

// ListTests handles GET /api/v1/mock-tests (published tests only).
func (h *Handler) ListTests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := TestFilters{
		Search:        c.Query("q"),
		Course:        c.Query("course"),
		Year:          c.Query("year"),
		PublishedOnly: true,
	}

	tests, total, err := h.service.GetTests(filters, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch mock tests")
		return
	}

	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Mock tests fetched successfully", gin.H{
		"page":       page,
		"limit":      limit,
		"total":      total,
		"mock_tests": tests,
	})
}

// GetTest handles GET /api/v1/mock-tests/:id. The payload is the explicit
// public DTO, which cannot carry the answer key.
//
// ── this is where the coin gate is, and it is the only place ────────────────
//
// This route is where a paper is RELEASED. It is the only route in the module
// that returns a question or an option, so it is the only route through which the
// product a student is being asked to pay 60 coins for is actually delivered.
// SubmitTest is the obvious alternative and is explicitly not used; service.go's
// SubmitTest comment says why at length, and the short version is that by submit
// time every question and every option is already in the student's hands.
//
// The order of the three things below is the contract:
//
//   - AFTER the publication check, so a draft is a 404 whether or not anyone can
//     pay for it. Gating first would answer 402 to a request for a paper the
//     public is not allowed to know exists, which both leaks the draft and leaves
//     a student unable to tell "this does not exist" from "this costs coins".
//   - BEFORE the view count and the projection, so a refused paper is not a view.
//     Engagement reported for a paper nobody read is how the success metrics in
//     08-success-metrics-and-kill-criteria.md stop meaning what they say.
//
// A nil gate means no gate, exactly as on the document and video routes: the check
// is skipped rather than defaulting to "refuse", so a deployment that has not
// wired the economy serves papers byte-identically to how it always has.
func (h *Handler) GetTest(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	test, err := h.service.GetPublishedTest(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, ErrTestNotFound.Error())
		return
	}

	if h.gate != nil {
		// The route is behind OptionalAuth, not Auth, so that an ungated paper
		// stays browsable by a logged-out visitor — see routes.go. That means userID
		// is legitimately absent here, and the gate is the thing that decides what
		// that means: it reads the kill switch FIRST and returns "allowed" before it
		// ever looks at identity, so with the switch off this is a no-op, and with
		// the switch on an anonymous reader is refused with the 401 that asks them
		// to sign in. That ordering is the gate's, not this handler's, and the
		// handler deliberately does not second-guess it by refusing a missing id
		// itself.
		userID, _ := httpx.CurrentUserID(c)
		decision := h.gate.AuthorizePaper(c.Request.Context(), userID, uint64(test.ID), test.Title)
		if !decision.Allowed {
			writePaperRefusal(c, decision.Refusal)
			return
		}
	}

	response.Success(c, http.StatusOK, "Mock test fetched successfully", h.service.ServePublicTest(test))
}

// writePaperRefusal writes a refusal and stops the handler. See
// writeGateRefusal's counterpart in internal/studyresources, and the same reason
// for the two implementations existing separately: the two modules each own their
// own envelope, and neither imports the other.
func writePaperRefusal(c *gin.Context, refusal *GateRefusal) {
	if refusal == nil {
		// A gate that says no without saying why is a bug in the gate, and 500 is
		// the only honest response.
		refusal = NewGateErrorRefusal(ErrGateFailed)
	}
	c.JSON(refusal.Status, refusal.RefusalBody())
}

// SubmitTest handles POST /api/v1/mock-tests/:id/submit (authentication
// required). It grades the submitted question_id/option_id pairs server-side.
//
// ── the entitlement check here, and why it never SPENDS ─────────────────────
//
// This is not the gate. The gate is on GetTest, where the paper is released, and
// service.go's SubmitTest comment sets out at length why a charge here would be
// worse than no gate: the whole paper is already in the student's hands by now.
//
// What IS required here is that the student was actually served that paper, and
// the reason is specific rather than general. A submission needs question ids and
// option ids, and those come from PublicMockTestDetailDTO — which is exactly what
// the gate now protects. But they are small sequential integers, so a student who
// was REFUSED the paper can still enumerate them against this route and read the
// answer key off is_correct, one bit per request. That is a bypass of the gate
// rather than a use of it: they would end up knowing the key to a paper they never
// paid for, which is strictly more than an ungated product gives them.
//
// So the check is for POSSESSION and only for possession:
//
//   - no spend, no allowance burn, no journal. The money moved at the serve, and a
//     second charge for the same paper would be a double sale of one entitlement.
//   - with the switch off it costs one cached config read and nothing else, so
//     submit behaves exactly as it does today.
//   - a student who legitimately took the paper holds the unlock, because the
//     serve wrote it, so this cannot refuse anyone who has done the thing it is
//     checking. The one population it can strand is a student who read a paper
//     while the switch was off and submits after an admin turns it on. That is
//     inherent to turning a class on and is the same event the document gate has:
//     a gate that is off is a promise that the content was free, and the escape is
//     to turn it back off. It is recorded here so the next reader knows it was a
//     choice rather than an oversight.
func (h *Handler) SubmitTest(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	if h.gate != nil {
		decision := h.gate.AuthorizeSubmission(c.Request.Context(), userID, uint64(id))
		if !decision.Allowed {
			writePaperRefusal(c, decision.Refusal)
			return
		}
	}

	var req SubmitMockTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.SubmitTest(id, userID, req.Answers)
	if err != nil {
		response.Error(c, statusForError(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Mock test submitted successfully", result)
}

// GetAttempt handles GET /api/v1/mock-tests/:id/attempts/:attempt_id. The
// attempt is scoped to its owner.
func (h *Handler) GetAttempt(c *gin.Context) {
	testID, ok := parseID(c)
	if !ok {
		return
	}
	attemptID, err := strconv.ParseUint(c.Param("attempt_id"), 10, 64)
	if err != nil || attemptID == 0 {
		response.Error(c, http.StatusBadRequest, "Invalid attempt ID")
		return
	}
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	result, err := h.service.GetAttempt(uint(attemptID), userID)
	if err != nil {
		if errors.Is(err, ErrAttemptForbidden) {
			response.Error(c, http.StatusForbidden, ErrAttemptForbidden.Error())
			return
		}
		response.Error(c, http.StatusNotFound, ErrAttemptNotFound.Error())
		return
	}
	if result.MockTestID != testID {
		response.Error(c, http.StatusNotFound, ErrAttemptNotFound.Error())
		return
	}
	response.Success(c, http.StatusOK, "Attempt result fetched successfully", result)
}

// AdminListTests handles GET /api/v1/admin/mock-tests (all tests, drafts
// included).
func (h *Handler) AdminListTests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	tests, total, err := h.service.GetAdminTests(TestFilters{
		Search: c.Query("q"),
		Course: c.Query("course"),
		Year:   c.Query("year"),
	}, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch mock tests")
		return
	}

	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Mock tests fetched successfully", gin.H{
		"page":       page,
		"limit":      limit,
		"total":      total,
		"mock_tests": tests,
	})
}

// AdminGetTest handles GET /api/v1/admin/mock-tests/:id including the answer
// key, which is what the admin editor needs.
func (h *Handler) AdminGetTest(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	test, err := h.service.GetAdminTest(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, ErrTestNotFound.Error())
		return
	}
	response.Success(c, http.StatusOK, "Mock test fetched successfully", test)
}

// CreateTest handles POST /api/v1/admin/mock-tests.
func (h *Handler) CreateTest(c *gin.Context) {
	var req CreateMockTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	createdBy, _ := httpx.CurrentUserID(c)

	test, err := h.service.CreateTest(req, createdBy)
	if err != nil {
		response.Error(c, statusForError(err), err.Error())
		return
	}
	response.Success(c, http.StatusCreated, "Mock test created", test)
}

// UpdateTest handles PUT /api/v1/admin/mock-tests/:id, replacing the question
// graph in a single transaction.
func (h *Handler) UpdateTest(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req UpdateMockTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	test, err := h.service.UpdateTest(id, req)
	if err != nil {
		response.Error(c, statusForError(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Mock test updated", test)
}

// DeleteTest handles DELETE /api/v1/admin/mock-tests/:id.
func (h *Handler) DeleteTest(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteTest(id); err != nil {
		response.Error(c, http.StatusNotFound, ErrTestNotFound.Error())
		return
	}
	response.Success(c, http.StatusOK, "Mock test deleted", nil)
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "Invalid mock test ID")
		return 0, false
	}
	return uint(id), true
}

// statusForError maps domain errors onto HTTP status codes. Validation and
// graph problems are 400; a missing test is 404.
func statusForError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrTestNotFound), errors.Is(err, ErrAttemptNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAttemptForbidden):
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}
