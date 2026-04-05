package admission

import (
	"strconv"
	"time"

	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateAdmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	var userID *uint
	if id, exists := c.Get("user_id"); exists {
		if uid, ok := id.(uint); ok {
			userID = &uid
		}
	}

	admission, err := h.service.Create(req, userID)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	resp := toAdmissionResponse(admission)
	response.Success(c, 201, "Admission application created successfully", resp)
}

func (h *Handler) GetMyAdmissions(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	admissions, err := h.service.GetMyAdmissions(uid)
	if err != nil {
		response.Error(c, 500, "Failed to fetch admissions")
		return
	}

	var resp []AdmissionResponse
	for _, a := range admissions {
		resp = append(resp, toAdmissionResponse(&a))
	}

	response.Success(c, 200, "Admissions retrieved successfully", resp)
}

func (h *Handler) GetByID(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid admission ID")
		return
	}

	admission, err := h.service.GetByID(id)
	if err != nil {
		response.Error(c, 404, "Admission not found")
		return
	}

	if userID, exists := c.Get("user_id"); exists {
		if uid := userID.(uint); admission.UserID != nil && *admission.UserID != uid {
			role, _ := c.Get("user_role")
			roleStr := ""
			if r, ok := role.(string); ok {
				roleStr = r
			}
			if roleStr != "admin" && roleStr != "super_admin" {
				response.Error(c, 403, "You can only view your own admissions")
				return
			}
		}
	}

	resp := toAdmissionResponse(admission)
	response.Success(c, 200, "Admission retrieved successfully", resp)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid admission ID")
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)
	role, _ := c.Get("user_role")
	roleStr := ""
	if r, ok := role.(string); ok {
		roleStr = r
	}

	var req UpdateAdmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	admission, err := h.service.Update(id, uid, roleStr, req)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	resp := toAdmissionResponse(admission)
	response.Success(c, 200, "Admission application updated successfully", resp)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid admission ID")
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	if err := h.service.Delete(id, uid); err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Admission application deleted successfully", nil)
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid admission ID")
		return
	}

	var req UpdateAdmissionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	admission, err := h.service.UpdateStatus(id, req, uid)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	resp := toAdmissionResponse(admission)
	response.Success(c, 200, "Admission status updated successfully", resp)
}

func (h *Handler) GetByCollegeID(c *gin.Context) {
	collegeID := c.Param("collegeId")
	status := c.Query("status")

	admissions, err := h.service.GetByCollegeID(collegeID, status)
	if err != nil {
		response.Error(c, 500, "Failed to fetch college admissions")
		return
	}

	var resp []AdmissionResponse
	for _, a := range admissions {
		resp = append(resp, toAdmissionResponse(&a))
	}

	response.Success(c, 200, "College admissions retrieved successfully", resp)
}

func (h *Handler) GetAll(c *gin.Context) {
	status := c.Query("status")
	collegeID := c.Query("college_id")

	admissions, err := h.service.GetAll(status, collegeID)
	if err != nil {
		response.Error(c, 500, "Failed to fetch admissions")
		return
	}

	var resp []AdmissionResponse
	for _, a := range admissions {
		resp = append(resp, toAdmissionResponse(&a))
	}

	response.Success(c, 200, "Admissions retrieved successfully", resp)
}

func toAdmissionResponse(a *Admission) AdmissionResponse {
	resp := AdmissionResponse{
		ID:                a.ID,
		CreatedAt:         a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         a.UpdatedAt.Format(time.RFC3339),
		UserID:            a.UserID,
		CollegeID:         a.CollegeID,
		ProgramName:       a.ProgramName,
		ProgramLevel:      a.ProgramLevel,
		StudentName:       a.StudentName,
		StudentEmail:      a.StudentEmail,
		StudentPhone:      a.StudentPhone,
		Gender:            a.Gender,
		Address:           a.Address,
		City:              a.City,
		LastQualification: a.LastQualification,
		Institution:       a.Institution,
		GPA:               a.GPA,
		EntranceScore:     a.EntranceScore,
		Statement:         a.Statement,
		Status:            a.Status,
		Notes:             a.Notes,
		ReviewedBy:        a.ReviewedBy,
	}

	if a.DateOfBirth != nil {
		dob := a.DateOfBirth.Format("2006-01-02")
		resp.DateOfBirth = &dob
	}

	if a.ReviewedAt != nil {
		reviewedAt := a.ReviewedAt.Format(time.RFC3339)
		resp.ReviewedAt = &reviewedAt
	}

	if a.College.ID != 0 {
		resp.College = &CollegeDTO{
			ID:   a.College.ID,
			Name: a.College.Name,
		}
	}

	if a.User != nil {
		resp.User = &UserDTO{
			ID:    a.User.ID,
			Email: a.User.Email,
		}
	}

	return resp
}

func parseID(s string) (uint, error) {
	parsed, err := strconv.ParseUint(s, 10, 64)
	if err != nil || parsed == 0 {
		return 0, err
	}
	return uint(parsed), nil
}
