package scholarship

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/slug"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service        *Service
	paymentService *PaymentService
}

func NewHandler(service *Service, paymentService *PaymentService) *Handler {
	return &Handler{service: service, paymentService: paymentService}
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
	param := c.Param("id")

	if id, err := parseID(param); err == nil {
		scholarship, err := h.service.GetScholarshipByID(id)
		if err == nil {
			resp := toScholarshipDetailResponse(*scholarship)
			response.Success(c, 200, "Scholarship details retrieved successfully", resp)
			return
		}
	}

	scholarship, err := h.service.GetScholarshipBySlug(param)
	if err != nil {
		response.Error(c, 404, "Scholarship not found")
		return
	}

	resp := toScholarshipDetailResponse(*scholarship)
	response.Success(c, 200, "Scholarship details retrieved successfully", resp)
}

func (h *Handler) RecommendScholarships(c *gin.Context) {
	var req ScholarshipRecommendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	var userID *uint
	if uid, exists := c.Get("user_id"); exists && uid != nil {
		if id, ok := uid.(uint); ok && id > 0 {
			userID = &id
		}
	}

	results, err := h.service.RecommendScholarships(req, userID)
	if err != nil {
		response.Error(c, 500, "Failed to get recommendations")
		return
	}

	response.Success(c, 200, "Recommendations retrieved successfully", ScholarshipRecommendResponse{
		Scholarships: results,
	})
}

func (h *Handler) GetSimilarScholarships(c *gin.Context) {
	param := c.Param("id")

	var s *Scholarship
	var err error

	if id, e := parseID(param); e == nil {
		s, err = h.service.GetScholarshipByID(id)
	} else {
		s, err = h.service.GetScholarshipBySlug(param)
	}
	if err != nil {
		response.Error(c, 404, "Scholarship not found")
		return
	}

	scholarships, err := h.service.GetSimilarScholarships(s.ID)
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

func (h *Handler) GetAvailableExamCenters(c *gin.Context) {
	param := c.Param("id")

	var scholarshipID uint
	var err error

	if id, e := parseID(param); e == nil {
		scholarshipID = id
	} else {
		s, e := h.service.GetScholarshipBySlug(param)
		if e != nil {
			response.Error(c, 404, "Scholarship not found")
			return
		}
		scholarshipID = s.ID
	}

	centers, err := h.service.GetAvailableExamCenters(scholarshipID)
	if err != nil {
		response.Error(c, 500, "Failed to fetch exam centers")
		return
	}

	if centers == nil {
		centers = []string{}
	}

	response.Success(c, 200, "Exam centers retrieved successfully", gin.H{
		"exam_centers": centers,
	})
}

func (h *Handler) ApplyScholarship(c *gin.Context) {
	param := c.Param("id")

	var scholarshipID uint
	var err error

	if id, e := parseID(param); e == nil {
		scholarshipID = id
	} else {
		s, e := h.service.GetScholarshipBySlug(param)
		if e != nil {
			response.Error(c, 404, "Scholarship not found")
			return
		}
		scholarshipID = s.ID
	}

	var req ScholarshipApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	var uid *uint
	if userID != nil {
		currentUserID := userID.(uint)
		uid = &currentUserID
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

func (h *Handler) AdminListScholarships(c *gin.Context) {
	scholarships, err := h.service.AdminListScholarships()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list scholarships")
		return
	}

	type listItem struct {
		ID    uint   `json:"id"`
		Slug  string `json:"slug"`
		Title string `json:"title"`
	}
	items := make([]listItem, 0, len(scholarships))
	for _, s := range scholarships {
		items = append(items, listItem{
			ID:    s.ID,
			Slug:  scholarshipSlug(s),
			Title: s.Title,
		})
	}

	response.Success(c, http.StatusOK, "Scholarships retrieved successfully", items)
}

func scholarshipSlug(s Scholarship) string {
	if s.Slug != "" {
		return s.Slug
	}
	return slug.Generate(s.Title)
}

func toScholarshipResponse(s Scholarship) ScholarshipResponse {
	return ScholarshipResponse{
		ID:                   s.ID,
		Slug:                 scholarshipSlug(s),
		Title:                s.Title,
		Provider:             s.Provider,
		Location:             s.Location,
		Type:                 s.FundingType,
		Amount:               s.Value,
		Deadline:             formatDeadline(s.Deadline),
		ApplicationStartDate: formatDeadline(s.ApplicationStartDate),
		ApplicationEndDate:   formatDeadline(s.Deadline),
		Status:               deriveScholarshipStatus(s.Deadline),
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
		"slug":                 scholarshipSlug(s),
		"title":                s.Title,
		"provider":             s.Provider,
		"provider_id":          s.ProviderID,
		"location":             s.Location,
		"value":                s.Value,
		"deadline":             formatDeadline(s.Deadline),
		"degree_level":         s.DegreeLevel,
		"funding_type":         s.FundingType,
		"scholarship_type":     s.ScholarshipType,
		"description":          s.Description,
		"image_url":            s.ImageURL,
		"application_start_date": formatDeadline(s.ApplicationStartDate),
		"application_end_date":   formatDeadline(s.Deadline),
		"status":                 deriveScholarshipStatus(s.Deadline),
		"field_of_study":       parseStringArray(s.FieldOfStudy),
		"selection_process":    parseDetailFieldArray(s.SelectionProcess),
		"eligibility_criteria": parseDetailFieldArray(s.EligibilityCriteria),
		"excluded_regions":     parseStringArray(s.ExcludedRegions),
		"required_documents":   parseStringArray(s.RequiredDocuments),
		"timeline":             parseDetailFieldArray(s.Timeline),
		"benefits":             parseDetailFieldArray(s.Benefits),
		"faqs":                 parseDetailFieldArray(s.FAQs),
		"provider_name":        s.ProviderName,
		"funding_type_other":   s.FundingTypeOther,
		"scholarship_type_other": s.ScholarshipTypeOther,
		"education_level":       s.EducationLevel,
		"education_level_other": s.EducationLevelOther,
		"apply_link":           s.ApplyLink,
		"coverage_area":        s.CoverageArea,
		"contact_email":        s.ContactEmail,
		"primary_phone":        s.PrimaryPhone,
		"secondary_phone":      s.SecondaryPhone,
		"website_url":          s.WebsiteUrl,
		"office_address":       s.OfficeAddress,
		"map_url":              s.MapUrl,
		"about_paragraph_1":    s.AboutParagraph1,
		"video_tutorials":      parseDetailFieldArray(s.VideoTutorials),
		"journey_timeline":     parseDetailFieldArray(s.JourneyTimeline),
		"scholarship_section_title": s.ScholarshipSectionTitle,
		"scholarship_subtitle":      s.ScholarshipSubtitle,
		"scholarship_description_1": s.ScholarshipDescription1,
		"scholarship_description_2": s.ScholarshipDescription2,
		"scholarship_types":         parseDetailFieldArray(s.ScholarshipTypes),
		"scholarship_types_new":     parseDetailFieldArray(s.ScholarshipTypesNew),
		"selection_rubric":          parseDetailFieldArray(s.SelectionRubric),
		"selection_rubric_new":       parseDetailFieldArray(s.SelectionRubricNew),
		"eligibility_section_title": s.EligibilitySectionTitle,
		"eligibility_subtitle":      s.EligibilitySubtitle,
		"basic_eligibility_criteria": parseStringArray(s.BasicEligibilityCriteria),
		"fully_funded_criteria":      parseStringArray(s.FullyFundedCriteria),
		"partially_funded_criteria":  parseStringArray(s.PartiallyFundedCriteria),
		"selection_process_steps":    parseDetailFieldArray(s.SelectionProcessSteps),
		"faqs_new":                  parseDetailFieldArray(s.FAQsNew),
		"gallery_images":            parseDetailFieldArray(s.GalleryImages),
		"gallery_images_new":         parseDetailFieldArray(s.GalleryImagesNew),
		"partner_groups":            parsePartnerGroups(s.PartnerGroups),
		"partner_messages":          parseDetailFieldArray(s.PartnerMessages),
		"exam_centers":              parseDetailFieldArray(s.ExamCenters),
		"exam_centers_new":           parseDetailFieldArray(s.ExamCentersNew),
		"downloads":                parseDetailFieldArray(s.Downloads),
		"payment_config":           parseJSON(s.PaymentConfig),
	}
}

func toScholarshipSummary(s Scholarship) ScholarshipSummary {
	return ScholarshipSummary{
		ID:                   s.ID,
		Slug:                 scholarshipSlug(s),
		Title:                s.Title,
		Provider:             s.Provider,
		Deadline:             formatDeadline(s.Deadline),
		Status:               deriveScholarshipStatus(s.Deadline),
		ApplicationStartDate: formatDeadline(s.ApplicationStartDate),
		ApplicationEndDate:   formatDeadline(s.Deadline),
		Location:    s.Location,
		FundingType: s.FundingType,
		DegreeLevel: s.DegreeLevel,
		ImageURL:    s.ImageURL,
		Category:    toScholarshipCategory(s),
		Description: s.Description,
	}
}

func toApplicationResponse(a ScholarshipApplication) ScholarshipApplicationResponse {
	dobAD := ""
	if !a.DateOfBirthAD.IsZero() {
		dobAD = a.DateOfBirthAD.Format("2006-01-02")
	}

	var docs []DetailField
	if len(a.Documents) > 0 {
		json.Unmarshal(a.Documents, &docs)
	}

	var paymentResp *PaymentResponse
	if a.Payment != nil {
		paymentResp = &PaymentResponse{
			ID:             a.Payment.ID,
			Method:         a.Payment.Method,
			Amount:         a.Payment.Amount,
			Status:         a.Payment.Status,
			ReceiptURL:     a.Payment.ReceiptURL,
			TransactionID: a.Payment.TransactionID,
		}
		if a.Payment.PaidAt != nil {
			paymentResp.PaidAt = a.Payment.PaidAt.Format(time.RFC3339)
		}
	}

	resp := ScholarshipApplicationResponse{
		ID:                    a.ID,
		CreatedAt:             a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             a.UpdatedAt.Format(time.RFC3339),
		ScholarshipID:         a.ScholarshipID,
		UserID:                uintValueOrZero(a.UserID),
		FullName:              a.FullName,
		Gender:                a.Gender,
		Ethnicity:             a.Ethnicity,
		EthnicityOther:        a.EthnicityOther,
		DateOfBirthBS:         a.DateOfBirthBS,
		DateOfBirthAD:         dobAD,
		Age:                   a.Age,
		PhoneNumber:           a.PhoneNumber,
		Email:                 a.Email,
		PhotoURL:              a.PhotoURL,
		SEEGPA:                a.SEEGPA,
		SchoolType:            a.SchoolType,
		SchoolName:            a.SchoolName,
		SchoolProvince:        a.SchoolProvince,
		SchoolDistrict:        a.SchoolDistrict,
		SchoolMunicipality:    a.SchoolMunicipality,
		SchoolTole:            a.SchoolTole,
		PermanentProvince:     a.PermanentProvince,
		PermanentDistrict:     a.PermanentDistrict,
		PermanentMunicipality: a.PermanentMunicipality,
		PermanentWard:         a.PermanentWard,
		PermanentTole:         a.PermanentTole,
		TemporaryProvince:     a.TemporaryProvince,
		TemporaryDistrict:     a.TemporaryDistrict,
		TemporaryMunicipality: a.TemporaryMunicipality,
		TemporaryWard:         a.TemporaryWard,
		TemporaryTole:         a.TemporaryTole,
		GuardianName:          a.GuardianName,
		GuardianPhone:         a.GuardianPhone,
		GuardianEmail:         a.GuardianEmail,
		FatherOccupation:      a.FatherOccupation,
		FatherOccupationOther: a.FatherOccupationOther,
		MotherOccupation:      a.MotherOccupation,
		MotherOccupationOther: a.MotherOccupationOther,
		FamilyMonthlyIncome:   a.FamilyMonthlyIncome,
		FamilyMembersCount:    a.FamilyMembersCount,
		Stream:                a.Stream,
		ExamCenter:            a.ExamCenter,
		Status:                a.Status,
		PersonalStatement:    a.PersonalStatement,
		Documents:            docs,
		Payment:              paymentResp,
	}

	if a.Scholarship.ID != 0 {
		resp.Scholarship = &ScholarshipSummary{
			ID:                   a.Scholarship.ID,
			Title:                a.Scholarship.Title,
			Provider:             a.Scholarship.Provider,
			Deadline:             formatDeadline(a.Scholarship.Deadline),
			Status:               deriveScholarshipStatus(a.Scholarship.Deadline),
			ApplicationStartDate: formatDeadline(a.Scholarship.ApplicationStartDate),
			ApplicationEndDate:   formatDeadline(a.Scholarship.Deadline),
			Location:             a.Scholarship.Location,
			FundingType:          a.Scholarship.FundingType,
			DegreeLevel:          a.Scholarship.DegreeLevel,
			ImageURL:             a.Scholarship.ImageURL,
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

func uintValueOrZero(v *uint) uint {
	if v == nil {
		return 0
	}
	return *v
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

func parsePartnerGroups(data []byte) []PartnerGroupResponse {
	if len(data) == 0 {
		return []PartnerGroupResponse{}
	}
	out := []PartnerGroupResponse{}
	if err := json.Unmarshal(data, &out); err != nil {
		return []PartnerGroupResponse{}
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

func mapFieldsToCategoryID(scholarshipType, fundingType string) string {
	if id := normalizeCategoryID(scholarshipType); id != "" {
		return id
	}
	if id := normalizeCategoryID(fundingType); id != "" {
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

func toPaymentResponse(p *Payment) PaymentResponse {
	paidAt := ""
	if p.PaidAt != nil {
		paidAt = p.PaidAt.Format(time.RFC3339)
	}
	return PaymentResponse{
		ID:            p.ID,
		ApplicationID: p.ApplicationID,
		ScholarshipID: p.ScholarshipID,
		Method:        p.Method,
		Amount:        p.Amount,
		Status:        p.Status,
		ReceiptURL:    p.ReceiptURL,
		TransactionID: p.TransactionID,
		PaidAt:        paidAt,
	}
}

func countString(count int) string {
	return strconv.Itoa(count) + " Scholarships Open"
}

func (h *Handler) InitiateEsewaPayment(c *gin.Context) {
	var req EsewaInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.paymentService.InitiateEsewaPayment(req.ApplicationID, req.Amount)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "eSewa payment initiated", result)
}

func (h *Handler) VerifyEsewaPayment(c *gin.Context) {
	var req EsewaVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	payment, err := h.paymentService.VerifyEsewaPayment(req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "eSewa payment verified successfully", toPaymentResponse(payment))
}

func (h *Handler) VerifyPendingEsewaPayments(c *gin.Context) {
	summary := h.paymentService.VerifyPendingEsewaPayments()
	if summary.Error != "" {
		response.Error(c, http.StatusInternalServerError, summary.Error)
		return
	}
	response.Success(c, http.StatusOK, "eSewa payment verification complete", summary)
}

func (h *Handler) SendAdmitCards(c *gin.Context) {
	summary := h.paymentService.SendAdmitCards()
	response.Success(c, http.StatusOK, "Admit cards sent", summary)
}

func (h *Handler) ProcessPayment(c *gin.Context) {
	param := c.Param("id")

	var scholarshipID uint
	var err error
	if id, e := parseID(param); e == nil {
		scholarshipID = id
	} else {
		s, e := h.service.GetScholarshipBySlug(param)
		if e != nil {
			response.Error(c, 404, "Scholarship not found")
			return
		}
		scholarshipID = s.ID
	}

	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	var uid uint
	if userID != nil {
		uid = userID.(uint)
	}

	var app *ScholarshipApplication
	if req.ApplicationID != 0 {
		app, _ = h.service.GetApplicationForPayment(req.ApplicationID, scholarshipID)
	} else if uid != 0 {
		app, _ = h.service.GetApplicationByUserAndScholarshipID(scholarshipID, uid)
	}

	if app == nil {
		app, err = h.service.CreateDraftApplication(scholarshipID, uid)
		if err != nil {
			response.Error(c, 500, "Failed to create application")
			return
		}
	}

	var uidPtr *uint
	if uid != 0 {
		uidPtr = &uid
	}

	payment, err := h.paymentService.CreatePayment(app.ID, scholarshipID, uidPtr, req)
	if err != nil {
		response.Error(c, 500, "Failed to process payment")
		return
	}

	response.Success(c, 201, "Payment initiated", payment)
}

func (h *Handler) ConfirmPayment(c *gin.Context) {
	paymentID, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid payment ID")
		return
	}

	var req struct {
		TransactionID string `json:"transaction_id"`
	}
	c.ShouldBindJSON(&req)

	if err := h.paymentService.ProcessSuccessfulPayment(paymentID, req.TransactionID); err != nil {
		response.Error(c, 500, "Failed to confirm payment")
		return
	}

	response.Success(c, 200, "Payment confirmed", nil)
}

func (h *Handler) uploadBankReceipt(c *gin.Context) {
	paymentID, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid payment ID")
		return
	}

	var req struct {
		ReceiptImage string `json:"receipt_image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	if err := h.paymentService.UploadBankReceipt(paymentID, req.ReceiptImage); err != nil {
		response.Error(c, 500, "Failed to upload receipt")
		return
	}

	response.Success(c, 200, "Receipt uploaded", nil)
}

func (h *Handler) ApprovePayment(c *gin.Context) {
	paymentID, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid payment ID")
		return
	}

	var req ApprovePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	providerID, _ := c.Get("provider_id")

	if err := h.paymentService.ApproveBankPayment(paymentID, providerID.(uint), req.Reason); err != nil {
		response.Error(c, 500, "Failed to process approval")
		return
	}

	response.Success(c, 200, "Payment processed", nil)
}
func (h *Handler) UploadFile(c *gin.Context) {
	folder := c.Query("folder")
	if folder == "" {
		folder = "applications"
	}
	uploadPath := "scholarship/" + folder

	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "No file provided")
		return
	}

	var url string
	// Check if it's an image or document based on extension
	ext := strings.ToLower(header.Filename[strings.LastIndex(header.Filename, ".")+1:])
	isImage := false
	for _, imgExt := range []string{"jpg", "jpeg", "png", "gif", "webp"} {
		if ext == imgExt {
			isImage = true
			break
		}
	}

	if isImage {
		url, err = utils.SaveUploadedImage(header, uploadPath)
	} else {
		url, err = utils.SaveUploadedDocument(header, uploadPath)
	}

	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "File uploaded successfully", gin.H{"url": url})
}
func parseJSON(data []byte) interface{} { if len(data) == 0 { return nil }; var out interface{}; if err := json.Unmarshal(data, &out); err != nil { return nil }; return out }
