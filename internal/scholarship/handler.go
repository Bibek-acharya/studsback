package scholarship

import (
	"encoding/json"
	"strconv"
	"strings"
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

func (h *Handler) GetScholarships(c *gin.Context) {
	search := c.Query("search")
	categoryFilter := c.Query("category")
	typeFilter := c.Query("type")
	locationFilter := c.Query("location")
	levelFilter := c.Query("level")
	statusFilter := strings.ToUpper(c.Query("status"))
	sortBy := strings.ToLower(c.DefaultQuery("sort", "deadline"))
	order := strings.ToUpper(c.DefaultQuery("order", "ASC"))
	if order != "ASC" && order != "DESC" {
		order = "ASC"
	}

	scholarships, categories, err := h.service.GetScholarships(search, categoryFilter, typeFilter, locationFilter, levelFilter, statusFilter, sortBy, order)
	if err != nil {
		response.Error(c, 500, "Failed to fetch scholarships")
		return
	}

	normalized := make([]ScholarshipResponse, 0, len(scholarships))
	for _, s := range scholarships {
		normalized = append(normalized, toScholarshipResponse(s))
	}

	response.Success(c, 200, "Scholarships retrieved successfully", ScholarshipListResponse{
		Scholarships: normalized,
		Categories:   categories,
	})
}

func (h *Handler) GetScholarshipByID(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid scholarship ID")
		return
	}

	scholarship, err := h.service.GetScholarshipByID(id)
	if err != nil {
		response.Error(c, 404, "Scholarship not found")
		return
	}

	resp := toScholarshipDetailResponse(*scholarship)
	response.Success(c, 200, "Scholarship details retrieved successfully", resp)
}

func (h *Handler) GetSimilarScholarships(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid scholarship ID")
		return
	}

	scholarships, err := h.service.GetSimilarScholarships(id)
	if err != nil {
		response.Error(c, 404, "Scholarship not found")
		return
	}

	items := make([]ScholarshipSummary, 0, len(scholarships))
	for _, s := range scholarships {
		items = append(items, toScholarshipSummary(s))
	}

	response.Success(c, 200, "Similar scholarships retrieved successfully", gin.H{
		"scholarships": items,
	})
}

func (h *Handler) ApplyScholarship(c *gin.Context) {
	scholarshipID, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid scholarship ID")
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var req ScholarshipApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	application, err := h.service.ApplyScholarship(scholarshipID, uid, req)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "already applied"):
			response.Error(c, 409, err.Error())
		case strings.Contains(err.Error(), "not found"):
			response.Error(c, 404, err.Error())
		default:
			response.Error(c, 500, err.Error())
		}
		return
	}

	response.Success(c, 201, "Application submitted successfully", application)
}

func (h *Handler) GetMyApplications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	applications, err := h.service.GetMyApplications(uid)
	if err != nil {
		response.Error(c, 500, "Failed to fetch applications")
		return
	}

	var resp []ScholarshipApplicationResponse
	for _, a := range applications {
		resp = append(resp, toApplicationResponse(a))
	}

	response.Success(c, 200, "Applications retrieved successfully", resp)
}

func (h *Handler) GetApplication(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid application ID")
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	application, err := h.service.GetApplication(id, uid)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.Error(c, 404, "Application not found")
		} else {
			response.Error(c, 403, err.Error())
		}
		return
	}

	response.Success(c, 200, "Application retrieved successfully", toApplicationResponse(*application))
}

func (h *Handler) UpdateApplication(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid application ID")
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var req UpdateScholarshipApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	application, err := h.service.UpdateApplication(id, uid, req)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Application updated successfully", toApplicationResponse(*application))
}

func (h *Handler) DeleteApplication(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid application ID")
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	if err := h.service.DeleteApplication(id, uid); err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Application deleted successfully", nil)
}

func (h *Handler) GetAllApplications(c *gin.Context) {
	status := c.Query("status")

	applications, err := h.service.GetAllApplications(status)
	if err != nil {
		response.Error(c, 500, "Failed to fetch applications")
		return
	}

	var resp []ScholarshipApplicationResponse
	for _, a := range applications {
		resp = append(resp, toApplicationResponse(a))
	}

	response.Success(c, 200, "Applications retrieved successfully", resp)
}

func (h *Handler) GetApplicationsByScholarship(c *gin.Context) {
	scholarshipID := c.Param("scholarshipId")
	status := c.Query("status")

	applications, err := h.service.GetApplicationsByScholarship(scholarshipID, status)
	if err != nil {
		response.Error(c, 500, "Failed to fetch applications")
		return
	}

	var resp []ScholarshipApplicationResponse
	for _, a := range applications {
		resp = append(resp, toApplicationResponse(a))
	}

	response.Success(c, 200, "Applications retrieved successfully", resp)
}

func (h *Handler) UpdateApplicationStatus(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid application ID")
		return
	}

	var req UpdateScholarshipApplicationStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	application, err := h.service.UpdateApplicationStatus(id, req.Status)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Application status updated successfully", toApplicationResponse(*application))
}

func (h *Handler) AdminCreateScholarship(c *gin.Context) {
	var req CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	scholarship, err := h.service.AdminCreateScholarship(req)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 201, "Scholarship created successfully", scholarship)
}

func (h *Handler) AdminUpdateScholarship(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid scholarship ID")
		return
	}

	var req CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	scholarship, err := h.service.AdminUpdateScholarship(id, req)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Scholarship updated successfully", scholarship)
}

func (h *Handler) AdminDeleteScholarship(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid scholarship ID")
		return
	}

	if err := h.service.AdminDeleteScholarship(id); err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Scholarship deleted successfully", nil)
}

func toScholarshipResponse(s Scholarship) ScholarshipResponse {
	return ScholarshipResponse{
		ID:              s.ID,
		Title:           s.Title,
		Provider:        s.Provider,
		Location:        s.Location,
		Type:            s.FundingType,
		Amount:          s.Value,
		Deadline:        formatDeadline(s.Deadline),
		Status:          deriveScholarshipStatus(s.Deadline),
		Category:        toScholarshipCategory(s),
		Description:     s.Description,
		Image:           s.ImageURL,
		Eligibility:     s.DegreeLevel,
		Tags:            []string{toScholarshipCategory(s), s.DegreeLevel},
		ScholarshipType: s.ScholarshipType,
		FundingType:     s.FundingType,
		DegreeLevel:     s.DegreeLevel,
	}
}

func toScholarshipDetailResponse(s Scholarship) gin.H {
	return gin.H{
		"id":                   s.ID,
		"title":                s.Title,
		"provider":             s.Provider,
		"location":             s.Location,
		"value":                s.Value,
		"deadline":             formatDeadline(s.Deadline),
		"degree_level":         s.DegreeLevel,
		"funding_type":         s.FundingType,
		"scholarship_type":     s.ScholarshipType,
		"description":          s.Description,
		"image_url":            s.ImageURL,
		"status":               deriveScholarshipStatus(s.Deadline),
		"field_of_study":       parseStringArray(s.FieldOfStudy),
		"selection_process":    parseDetailFieldArray(s.SelectionProcess),
		"eligibility_criteria": parseDetailFieldArray(s.EligibilityCriteria),
		"excluded_regions":     parseStringArray(s.ExcludedRegions),
		"required_documents":   parseDetailFieldArray(s.RequiredDocuments),
		"timeline":             parseDetailFieldArray(s.Timeline),
		"benefits":             parseDetailFieldArray(s.Benefits),
		"faqs":                 parseDetailFieldArray(s.FAQs),
	}
}

func toScholarshipSummary(s Scholarship) ScholarshipSummary {
	return ScholarshipSummary{
		ID:          s.ID,
		Title:       s.Title,
		Provider:    s.Provider,
		Deadline:    formatDeadline(s.Deadline),
		Status:      deriveScholarshipStatus(s.Deadline),
		Location:    s.Location,
		FundingType: s.FundingType,
		DegreeLevel: s.DegreeLevel,
		ImageURL:    s.ImageURL,
		Category:    toScholarshipCategory(s),
		Description: s.Description,
	}
}

func toApplicationResponse(a ScholarshipApplication) ScholarshipApplicationResponse {
	resp := ScholarshipApplicationResponse{
		ID:                  a.ID,
		CreatedAt:           a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           a.UpdatedAt.Format(time.RFC3339),
		ScholarshipID:       a.ScholarshipID,
		UserID:              a.UserID,
		NationalID:          a.NationalID,
		FirstName:           a.FirstName,
		LastName:            a.LastName,
		DateOfBirth:         a.DateOfBirth.Format("2006-01-02"),
		Gender:              a.Gender,
		StreetAddress:       a.StreetAddress,
		City:                a.City,
		PostCode:            a.PostCode,
		Country:             a.Country,
		PhoneCode:           a.PhoneCode,
		PhoneNumber:         a.PhoneNumber,
		Email:               a.Email,
		LatestInstitution:   a.LatestInstitution,
		LevelCompleted:      a.LevelCompleted,
		GPAPercentage:       a.GPAPercentage,
		AnnualFamilyIncome:  a.AnnualFamilyIncome,
		PrimaryIncomeSource: a.PrimaryIncomeSource,
		PersonalStatement:   a.PersonalStatement,
		Status:              a.Status,
	}

	resp.SpecialCircumstances = parseStringArray(a.SpecialCircumstances)

	if a.Scholarship.ID != 0 {
		resp.Scholarship = &ScholarshipSummary{
			ID:          a.Scholarship.ID,
			Title:       a.Scholarship.Title,
			Provider:    a.Scholarship.Provider,
			Deadline:    formatDeadline(a.Scholarship.Deadline),
			Status:      deriveScholarshipStatus(a.Scholarship.Deadline),
			Location:    a.Scholarship.Location,
			FundingType: a.Scholarship.FundingType,
			DegreeLevel: a.Scholarship.DegreeLevel,
			ImageURL:    a.Scholarship.ImageURL,
			Category:    toScholarshipCategory(a.Scholarship),
			Description: a.Scholarship.Description,
		}
	}

	if a.User.ID != 0 {
		resp.User = &UserSummary{
			ID:        a.User.ID,
			Email:     a.User.Email,
			FirstName: a.User.FirstName,
			LastName:  a.User.LastName,
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

func formatDeadline(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("Jan 02, 2006")
}

func parseStringArray(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	out := []string{}
	if err := json.Unmarshal(data, &out); err != nil {
		return []string{}
	}
	return out
}

func parseDetailFieldArray(data []byte) []DetailField {
	if len(data) == 0 {
		return []DetailField{}
	}
	out := []DetailField{}
	if err := json.Unmarshal(data, &out); err != nil {
		return []DetailField{}
	}
	return out
}

func deriveScholarshipStatus(deadline time.Time) string {
	if deadline.IsZero() {
		return "OPEN"
	}

	now := time.Now()
	if deadline.Before(now) {
		return "CLOSED"
	}
	if deadline.Before(now.AddDate(0, 0, 21)) {
		return "CLOSING SOON"
	}
	return "OPEN"
}

func toScholarshipCategory(s Scholarship) string {
	if s.ScholarshipType != "" {
		return s.ScholarshipType
	}
	if s.FundingType != "" {
		return s.FundingType
	}
	return "General"
}

func normalizeCategoryID(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(value, "college"):
		return "college"
	case strings.Contains(value, "school"):
		return "school"
	case strings.Contains(value, "institutional need") || strings.Contains(value, "need"):
		return "need"
	case strings.Contains(value, "institution") || strings.Contains(value, "merit"):
		return "institutional"
	case strings.Contains(value, "entrance"):
		return "entrance"
	case strings.Contains(value, "ngo") || strings.Contains(value, "ingo"):
		return "ngo"
	case strings.Contains(value, "department"):
		return "departmental"
	case strings.Contains(value, "waiver") || strings.Contains(value, "tuition") || strings.Contains(value, "fee"):
		return "fee-waiver"
	case strings.Contains(value, "research"):
		return "research"
	default:
		return ""
	}
}

func mapScholarshipToCategoryID(s Scholarship) string {
	if id := normalizeCategoryID(s.ScholarshipType); id != "" {
		return id
	}
	if id := normalizeCategoryID(s.FundingType); id != "" {
		return id
	}
	return ""
}

func isOpenStatus(status string) bool {
	return status == "OPEN" || status == "CLOSING SOON"
}

func splitFilterValues(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func categoryDefinitions() []categoryMeta {
	return []categoryMeta{
		{ID: "college", Name: "College Based", Title: "College-Based", Desc: "Direct aid from universities for enrolled students.", Icon: "fa-building-columns", Color: "blue"},
		{ID: "school", Name: "School Based", Title: "School-Based", Desc: "For students excelling in secondary education.", Icon: "fa-graduation-cap", Color: "indigo"},
		{ID: "institutional", Name: "Institutional Merit", Title: "Institutional Merit", Desc: "Awarded to students with outstanding academic achievements.", Icon: "fa-medal", Color: "emerald"},
		{ID: "need", Name: "Institutional Need", Title: "Institutional Need", Desc: "Financial aid for students demonstrating significant financial need.", Icon: "fa-hand-holding-heart", Color: "amber"},
		{ID: "entrance", Name: "Entrance", Title: "Entrance", Desc: "Scholarships for top rankers in IOE, IOM, and exams.", Icon: "fa-pencil", Color: "purple"},
		{ID: "ngo", Name: "NGO / INGO", Title: "NGO / INGO", Desc: "Supported by international and national organizations.", Icon: "fa-globe", Color: "rose"},
		{ID: "departmental", Name: "Departmental", Title: "Departmental", Desc: "Specific to faculties like Engineering, Medicine, and IT.", Icon: "fa-laptop-code", Color: "cyan"},
		{ID: "fee-waiver", Name: "Fee Waiver", Title: "Fee Waiver", Desc: "Full or partial tuition fee waivers for deserving candidates.", Icon: "fa-file-invoice-dollar", Color: "teal"},
		{ID: "research", Name: "Research", Title: "Research", Desc: "Funding support for project and thesis based study.", Icon: "fa-flask", Color: "blue"},
	}
}

type categoryMeta struct {
	ID    string
	Name  string
	Title string
	Desc  string
	Icon  string
	Color string
}

func countString(count int) string {
	return strconv.Itoa(count) + " Scholarships Open"
}
