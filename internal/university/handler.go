package university

import (
	"errors"
	"strconv"
	"strings"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/sanitize"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// forbidden reports a service-layer authorisation refusal as 403, and false for
// every other error so the caller falls through to its own handling.
//
// The handlers below already had a status chosen for the error they expected —
// 404 for a missing row, 400 for a rejected field — and a role refusal arriving
// at the same branch would have been reported as one of those. The middleware
// normally answers first; this is the service repeating the check so a widened
// gate cannot re-open the route, and when it is the thing refusing, the answer
// has to be 403 to match. See access.go.
func forbidden(c *gin.Context, err error) bool {
	if errors.Is(err, ErrForbidden) {
		response.Error(c, 403, ErrForbidden.Error())
		return true
	}
	return false
}

func (h *Handler) GetUniversityFilterCounts(c *gin.Context) {
	isNepali := ""
	if v := c.Query("isNepali"); v != "" {
		isNepali = v
	}
	result, err := h.service.GetUniversityFilterCounts(isNepali)
	if err != nil {
		response.Error(c, 500, "Failed to fetch university filter counts")
		return
	}
	response.Success(c, 200, "University filter counts retrieved successfully", result)
}

func (h *Handler) GetUniversities(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))
	uniType := strings.TrimSpace(c.Query("type"))
	status := strings.TrimSpace(c.Query("status"))
	popular := c.Query("popular") == "true"
	isNepali := ""
	if v := c.Query("isNepali"); v != "" {
		isNepali = v
	}
	academicRaw := strings.TrimSpace(c.Query("academic"))
	var academic []string
	if academicRaw != "" {
		academic = strings.Split(academicRaw, ",")
	}

	results, err := h.service.GetUniversities(search, uniType, status, popular, isNepali, academic)
	if err != nil {
		response.Error(c, 500, "Failed to fetch universities")
		return
	}

	response.Success(c, 200, "Universities retrieved successfully", gin.H{
		"universities": results,
	})
}

func (h *Handler) GetUniversityByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid university ID")
		return
	}

	uni, colleges, err := h.service.GetUniversityByID(uint(id))
	if err != nil {
		response.Error(c, 404, "University not found")
		return
	}

	response.Success(c, 200, "University retrieved successfully", gin.H{
		"university": uni,
		"colleges":   colleges,
	})
}

func (h *Handler) AdminGetUniversityByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid university ID")
		return
	}

	uni, colleges, err := h.service.AdminGetUniversityByID(ViewerFrom(c), uint(id))
	if err != nil {
		if forbidden(c, err) {
			return
		}
		response.Error(c, 404, "University not found")
		return
	}

	response.Success(c, 200, "University retrieved successfully", gin.H{
		"university": uni,
		"colleges":   colleges,
	})
}

func (h *Handler) GetUniversityCourses(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid university ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	level := c.Query("level")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	courses, total, err := h.service.GetUniversityCourses(uint(id), page, limit, level)
	if err != nil {
		response.Error(c, 500, "Failed to fetch courses")
		return
	}

	response.Success(c, 200, "Courses retrieved successfully", gin.H{
		"courses": courses,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

func (h *Handler) GetUniversityScholarships(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid university ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	level := c.Query("level")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	scholarships, total, err := h.service.GetUniversityScholarships(uint(id), page, limit, level)
	if err != nil {
		response.Error(c, 500, "Failed to fetch scholarships")
		return
	}

	response.Success(c, 200, "Scholarships retrieved successfully", gin.H{
		"scholarships": scholarships,
		"total":        total,
		"page":         page,
		"limit":        limit,
	})
}

func (h *Handler) GetUniversityTab(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid university ID")
		return
	}

	tab := c.Param("tab")
	allowedTabs := map[string]bool{
		"about":        true,
		"contact":      true,
		"quick":        true,
		"overview":     true,
		"leadership":   true,
		"courses":      true,
		"programs":     true,
		"scholarships": true,
		"events":       true,
		"news":         true,
		"downloads":    true,
		"gallery":      true,
		"faculties":    true,
		"admissions":   true,
		"reviews":      true,
	}

	if !allowedTabs[tab] {
		response.Error(c, 400, "Invalid tab name")
		return
	}

	data, err := h.service.GetUniversityTab(uint(id), tab)
	if err != nil {
		response.Error(c, 500, "Failed to fetch tab data")
		return
	}

	response.Success(c, 200, "Data retrieved successfully", UniversityTabResponse{
		Tab:  tab,
		Data: data,
	})
}

func (h *Handler) CreateUniversity(c *gin.Context) {
	var req CreateUniversityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	req.Description = sanitize.HTML(req.Description)

	uni, err := h.service.CreateUniversity(ViewerFrom(c), req)
	if err != nil {
		if err == ErrNameRequired {
			response.Error(c, 400, "name is required")
			return
		}
		response.Error(c, 400, "Failed to create university: "+err.Error())
		return
	}

	response.Success(c, 201, "University created successfully", gin.H{
		"university": toUniversityResponse(*uni, []College{}),
	})
}

func (h *Handler) UpdateUniversity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid university ID")
		return
	}

	var req UpdateUniversityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	if req.Description != nil && *req.Description != "" {
		sanitized := sanitize.HTML(*req.Description)
		req.Description = &sanitized
	}

	uni, err := h.service.UpdateUniversity(ViewerFrom(c), uint(id), req)
	if err != nil {
		if forbidden(c, err) {
			return
		}
		if err == ErrNameRequired {
			response.Error(c, 400, "name is required")
			return
		}
		response.Error(c, 404, "University not found")
		return
	}

	response.Success(c, 200, "University updated successfully", gin.H{
		"university": toUniversityResponse(*uni, []College{}),
	})
}

func (h *Handler) DeleteUniversity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid university ID")
		return
	}

	err = h.service.DeleteUniversity(ViewerFrom(c), uint(id))
	if err != nil {
		if forbidden(c, err) {
			return
		}
		response.Error(c, 404, "University not found")
		return
	}

	response.Success(c, 200, "University deleted successfully", nil)
}

func (h *Handler) GetAffiliatedColleges(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid university ID")
		return
	}

	result, err := h.service.GetAffiliatedColleges(uint(id))
	if err != nil {
		response.Error(c, 404, "University not found")
		return
	}

	response.Success(c, 200, "Affiliated colleges retrieved successfully", result)
}
