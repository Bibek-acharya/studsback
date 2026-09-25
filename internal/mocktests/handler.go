package mocktests

import (
	"errors"
	"net/http"
	"strconv"

	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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
func (h *Handler) GetTest(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	test, err := h.service.GetPublicTest(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, ErrTestNotFound.Error())
		return
	}
	response.Success(c, http.StatusOK, "Mock test fetched successfully", test)
}

// SubmitTest handles POST /api/v1/mock-tests/:id/submit (authentication
// required). It grades the submitted question_id/option_id pairs server-side.
func (h *Handler) SubmitTest(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
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
	userID, ok := currentUserID(c)
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
	createdBy, _ := currentUserID(c)

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

// currentUserID reads the authenticated user set by the auth middleware. The
// value is a uint, but int/int64 are tolerated for custom middleware.
func currentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	switch v := value.(type) {
	case uint:
		return v, true
	case uint64:
		return uint(v), true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	default:
		return 0, false
	}
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
